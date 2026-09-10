#!/usr/bin/env bash
# Offline Ubuntu install/upgrade. Run with sudo bash deploy-ubuntu.sh [release-directory].
set -Eeuo pipefail
umask 077
export PATH=/usr/sbin:/usr/bin:/sbin:/bin
fail() { echo "ERROR: $*" >&2; exit 1; }
[[ $EUID == 0 ]] || fail 'Run with sudo bash deploy-ubuntu.sh'
[[ $(uname -s) == Linux && $(uname -m) == x86_64 ]] || fail 'Ubuntu x86_64 is required.'
[[ -f /etc/os-release ]] || fail 'Missing /etc/os-release'
. /etc/os-release
[[ $ID == ubuntu ]] || fail 'This installer supports Ubuntu Server.'
[[ -d /run/systemd/system ]] || fail 'systemd must be running.'
for command in systemctl sha256sum awk od timeout ss flock install cp mv getent useradd groupadd; do
    command -v "$command" >/dev/null || fail "Required Ubuntu utility missing: $command"
done
exec 9>/run/lock/idata-deploy.lock
flock -n 9 || fail 'Another deployment is running.'
source_dir=$(cd -- "${1:-$(dirname -- "${BASH_SOURCE[0]}")}" && pwd)
binary=idata-server-linux-amd64
[[ -f $source_dir/$binary && -f $source_dir/SHA256SUMS ]] || fail "Place $binary and SHA256SUMS beside the script (or pass their directory)."
work=$(mktemp -d /opt/idata-deploy.XXXXXXXX)
backup=
changed=false
was_active=false
was_enabled=false
cleanup() { rm -rf -- "$work"; }
trap cleanup EXIT
# Stage first so the exact bytes verified are the bytes installed.
cp -- "$source_dir/$binary" "$work/$binary"
awk '$2 == "idata-server-linux-amd64" || $2 == "*idata-server-linux-amd64" {print}' "$source_dir/SHA256SUMS" > "$work/checksum"
[[ $(wc -l < "$work/checksum") == 1 ]] || fail 'SHA256SUMS must contain exactly one server binary entry.'
(cd "$work" && sha256sum --check checksum) || fail 'Release checksum mismatch; installation unchanged.'
[[ $(od -An -tx1 -N4 "$work/$binary" | tr -d ' \n') == 7f454c46 ]] || fail 'Release binary is not ELF.'
[[ $(od -An -tx1 -j18 -N2 "$work/$binary" | tr -d ' \n') == 3e00 ]] || fail 'Release binary is not x86_64.'
unit=/etc/systemd/system/idata-server.service
env_file=/etc/idata/idata-server.env
# Do not silently replace a custom layout or ignore systemd overrides.
[[ -z $(systemctl show idata-server.service -p DropInPaths --value) ]] || fail 'Custom systemd drop-ins found; migrate them before using this installer.'
exec_start=$(systemctl show idata-server.service -p ExecStart --value)
[[ -z $exec_start || $exec_start == *'path=/opt/idata/idata-server ;'* ]] || fail 'Existing service uses a custom ExecStart; migrate it to /opt/idata/idata-server first.'
for path in /opt/idata /opt/idata/idata-server /etc/idata "$env_file" "$unit" /var/lib/idata; do
    [[ ! -L $path ]] || fail "Unsupported symlink: $path"
done
if [[ -f $env_file ]]; then
    awk '!/^[[:space:]]*IDATA_LISTEN_ADDR[[:space:]]*=/' "$env_file" > "$work/env"
else
    token=$(od -An -N32 -tx1 /dev/urandom | tr -d ' \n')
    cat > "$work/env" <<CONFIG
IDATA_AGENT_TOKEN=
IDATA_ADMIN_TOKEN=$token
IDATA_DEVICE_CREDENTIALS_FILE=/var/lib/idata/device-credentials.json
IDATA_ENROLLMENT_AUTO_APPROVE=true
IDATA_BROWSER_PAIRING=false
IDATA_PAIRING_REQUEST_TTL=2m
IDATA_DEVICE_SESSION_TTL=8h
CONFIG
    unset token
fi
printf '\nIDATA_LISTEN_ADDR=:12345\n' >> "$work/env"
cat > "$work/unit" <<'UNIT'
[Unit]
Description=iData remote command server
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=idata
Group=idata
StateDirectory=idata
StateDirectoryMode=0700
EnvironmentFile=/etc/idata/idata-server.env
ExecStart=/opt/idata/idata-server
Restart=on-failure
RestartSec=3
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true

[Install]
WantedBy=multi-user.target
UNIT
systemctl is-active --quiet idata-server && was_active=true
systemctl is-enabled --quiet idata-server 2>/dev/null && was_enabled=true
backup=$(mktemp -d /var/backups/idata-deploy.XXXXXXXX)
for path in /opt/idata/idata-server "$env_file" "$unit"; do
    if [[ -e $path ]]; then cp -a --parents -- "$path" "$backup"; fi
done
rollback() {
    rc=$?
    trap - ERR INT TERM
    set +e
    if $changed; then
        echo "Deployment failed. Restoring previous executable, configuration and service from $backup" >&2
        systemctl stop idata-server
        for path in /opt/idata/idata-server "$env_file" "$unit"; do
            rm -f -- "$path"
            if [[ -e $backup$path ]]; then cp -a -- "$backup$path" "$path"; fi
        done
        systemctl daemon-reload
        if $was_enabled; then systemctl enable idata-server; else systemctl disable idata-server; fi
        if $was_active; then
            systemctl start idata-server || echo 'ERROR: Previous service could not restart; inspect journalctl.' >&2
        fi
    fi
    exit "${rc:-1}"
}
trap rollback ERR
trap 'false' INT TERM
getent group idata >/dev/null || groupadd --system idata
id -u idata >/dev/null 2>&1 || useradd --system --gid idata --no-create-home --shell /usr/sbin/nologin idata
changed=true
systemctl stop idata-server 2>/dev/null || { [[ -z $exec_start ]]; }
# A different process must not be mistaken for the newly installed service.
if [[ -n $(ss -H -ltn 'sport = :12345') ]]; then
    echo 'ERROR: TCP 12345 is occupied by another process.' >&2
    false
fi
install -d -m 0755 /opt/idata
install -d -m 0750 -o root -g idata /etc/idata
install -m 0755 "$work/$binary" /opt/idata/idata-server.new
mv -f /opt/idata/idata-server.new /opt/idata/idata-server
install -m 0640 -o root -g idata "$work/env" "$env_file"
install -m 0644 "$work/unit" "$unit"
systemctl daemon-reload
systemctl restart idata-server
healthy=false
for ((attempt=0; attempt<30; attempt++)); do
    if systemctl is-active --quiet idata-server && timeout 2 bash -c '
        exec 3<>/dev/tcp/127.0.0.1/12345
        printf "GET /healthz HTTP/1.0\r\nHost: localhost\r\n\r\n" >&3
        response=$(cat <&3)
        [[ $response == *"200 OK"* && $response == *"\"status\":\"ok\""* ]]
    ' 2>/dev/null; then healthy=true; break; fi
    sleep 1
done
if ! $healthy; then
    echo 'ERROR: Health check failed. Inspect journalctl -u idata-server.' >&2
    false
fi
systemctl enable idata-server
trap - ERR INT TERM
echo 'IDATA deployed successfully: http://SERVER_IP:12345/ (admin: /admin/)'
echo "Previous executable/configuration/service backup: $backup"
echo 'Admin token remains in /etc/idata/idata-server.env (sudo cat to view).'
echo 'Device credentials under /var/lib/idata were preserved. Allow TCP 12345 from your trusted network if a firewall is enabled.'
