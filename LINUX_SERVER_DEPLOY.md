# Fast Linux Server Deployment

The fastest installation method is to download the required files on a Windows computer, transfer them to the intranet Linux server, and install them locally. The Linux server does not need internet access, the Go compiler, or a source build.

Current production release: [IDATA Remote v0.2.6](https://github.com/IntenTest/IDATA/releases/tag/v0.2.6). The commands and checksum below are pinned to this release.

These instructions are for Ubuntu or Debian on an `x86_64` system.

## 1. Download on Windows and transfer to Linux

On a Windows computer with internet access, download these two files:

1. [idata-server-linux-amd64](https://github.com/IntenTest/IDATA/releases/download/v0.2.6/idata-server-linux-amd64)
2. [idata-server.service](https://raw.githubusercontent.com/IntenTest/IDATA/v0.2.6/server/deploy/idata-server.service)

Keep the filenames exactly as shown. Copy both files to the same folder on the Linux server using an approved method such as a USB drive, an internal file share, WinSCP, or `scp`. The following commands assume the files were copied to `/tmp/idata-install`:

```text
/tmp/idata-install/idata-server-linux-amd64
/tmp/idata-install/idata-server.service
```

On Windows, you can verify the server binary in PowerShell before transferring it:

```powershell
Get-FileHash .\idata-server-linux-amd64 -Algorithm SHA256
```

The expected SHA-256 value is:

```text
02bdf8d762487e4c639d6d8cc9077230951d1e6d66bb8d89e5a7b8141868e4bc
```

After transferring the files, verify the binary again on Linux:

```bash
cd /tmp/idata-install

echo "02bdf8d762487e4c639d6d8cc9077230951d1e6d66bb8d89e5a7b8141868e4bc  idata-server-linux-amd64" \
  | sha256sum -c -
```

The verification should report:

```text
idata-server-linux-amd64: OK
```

Do not install or execute the file if its checksum does not match.

## 2. Install the binary and service account

```bash
cd /tmp/idata-install

sudo useradd --system --no-create-home --shell /usr/sbin/nologin idata 2>/dev/null || true

sudo install -d -m 0755 /opt/idata
sudo install -d -m 0750 -o root -g idata /etc/idata
sudo install -m 0755 idata-server-linux-amd64 /opt/idata/idata-server
```

## 3. Create the configuration

Generate and display a secure administrator token:

```bash
ADMIN_TOKEN="$(openssl rand -hex 32)"
echo "Save this admin token: $ADMIN_TOKEN"
```

Save this token somewhere secure, and then create the server configuration:

```bash
sudo tee /etc/idata/idata-server.env >/dev/null <<EOF
IDATA_AGENT_TOKEN=
IDATA_ADMIN_TOKEN=$ADMIN_TOKEN
IDATA_LISTEN_ADDR=:80
IDATA_DEVICE_CREDENTIALS_FILE=/var/lib/idata/device-credentials.json
IDATA_ENROLLMENT_AUTO_APPROVE=true
IDATA_BROWSER_PAIRING=false
IDATA_PAIRING_REQUEST_TTL=2m
IDATA_DEVICE_SESSION_TTL=8h
EOF

sudo chown root:idata /etc/idata/idata-server.env
sudo chmod 0640 /etc/idata/idata-server.env
unset ADMIN_TOKEN
```

Automatic enrollment is convenient for a trusted internal network. Set `IDATA_ENROLLMENT_AUTO_APPROVE=false` if every new client must be approved manually.

## 4. Install and start the systemd service

```bash
cd /tmp/idata-install
sudo install -m 0644 idata-server.service /etc/systemd/system/idata-server.service

sudo systemctl daemon-reload
sudo systemctl enable --now idata-server
```

## 5. Verify the deployment

```bash
curl --fail http://127.0.0.1/healthz
sudo systemctl --no-pager status idata-server
```

The health endpoint should return:

```json
{"status":"ok"}
```

Open the IDATA interface in a browser:

```text
http://SERVER_IP/
```

To follow the server logs:

```bash
sudo journalctl -u idata-server -f
```

## 6. Connect the production Windows Client

Download `idata-client-windows-amd64.exe` from the same [v0.2.6 release](https://github.com/IntenTest/IDATA/releases/tag/v0.2.6) on the Windows PC.

The production Client does not need a Server port configured in advance:

1. Start `idata-client-windows-amd64.exe`.
2. Open `http://SERVER_IP/` in the Windows browser.
3. Click **Open IDATA Client**.
4. The website launches `idata://connect` with the Server IP, port, and security mode.
5. The running Client connects and displays the exact Server address and port it received.

With the configuration in this guide, the website is on TCP port `80`. Ports `54321` and `17891` are Client-side loopback ports and must not be opened on the Linux Server.

## 7. Upgrade an existing Linux installation

Download the new `idata-server-linux-amd64` on Windows, verify its SHA-256 value, and transfer it to `/tmp/idata-install` as described above. Then run:

```bash
cd /tmp/idata-install

echo "02bdf8d762487e4c639d6d8cc9077230951d1e6d66bb8d89e5a7b8141868e4bc  idata-server-linux-amd64" \
  | sha256sum -c -

sudo systemctl stop idata-server
sudo install -m 0755 idata-server-linux-amd64 /opt/idata/idata-server
sudo systemctl start idata-server

curl --fail http://127.0.0.1/healthz
sudo systemctl --no-pager status idata-server
```

The existing `/etc/idata/idata-server.env` configuration and `/var/lib/idata` device credentials are preserved.

## 8. Restrict network access

If UFW is enabled, allow access only from the trusted internal network. For example:

```bash
sudo ufw allow from 192.168.1.0/24 to any port 80 proto tcp
sudo ufw status
```

Replace `192.168.1.0/24` with the actual trusted subnet.

> [!WARNING]
> IDATA v0.2.6 uses unencrypted HTTP and WebSocket connections. Do not expose TCP port 80 directly to the public internet. For public access, use an HTTPS reverse proxy and appropriate access restrictions.
