# Fast Linux Server Deployment

The fastest installation method is to download the prebuilt server binary from the latest IDATA release. This procedure does not require the Go compiler or a source build.

Current release: [IDATA Remote v0.2.5](https://github.com/IntenTest/IDATA/releases/tag/v0.2.5)

These instructions are for Ubuntu or Debian on an `x86_64` system.

## 1. Download and verify

```bash
cd /tmp

wget https://github.com/IntenTest/IDATA/releases/download/v0.2.5/idata-server-linux-amd64

echo "d03b860fa8cfd54d950a771ef18dacd0da4190b352ea22b68dc0878b6f603a17  idata-server-linux-amd64" \
  | sha256sum -c -
```

The verification should report:

```text
idata-server-linux-amd64: OK
```

Do not install or execute the file if its checksum does not match.

## 2. Install the binary and service account

```bash
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
sudo wget -qO /etc/systemd/system/idata-server.service \
  https://raw.githubusercontent.com/IntenTest/IDATA/v0.2.5/server/deploy/idata-server.service

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

## 6. Restrict network access

If UFW is enabled, allow access only from the trusted internal network. For example:

```bash
sudo ufw allow from 192.168.1.0/24 to any port 80 proto tcp
sudo ufw status
```

Replace `192.168.1.0/24` with the actual trusted subnet.

> [!WARNING]
> IDATA v0.2.5 uses unencrypted HTTP and WebSocket connections. Do not expose TCP port 80 directly to the public internet. For public access, use an HTTPS reverse proxy and appropriate access restrictions.

