# IDATA Linux 服务器重新部署（v0.2.17）

本文用于在 Ubuntu x86-64 服务器上首次安装或升级 IDATA Server。
重新部署会保留现有的监听地址、Token 和已批准的设备凭据。

## 1. 需要下载的文件

推荐只下载以下两个文件：

1. [`IDATA-ubuntu-v0.2.17.tar.gz`](https://github.com/IntenTest/IDATA/releases/download/v0.2.17/IDATA-ubuntu-v0.2.17.tar.gz)
   —— Ubuntu 完整部署包，内含 Linux Server、部署脚本、本文档和包内校验文件。
2. [`SHA256SUMS`](https://github.com/IntenTest/IDATA/releases/download/v0.2.16/SHA256SUMS)
   ——用于校验下载的 `.tar.gz` 是否完整。

Linux 服务器不需要下载 `idata-client-windows-amd64.exe`；该文件只用于
Windows 执行电脑。

如果 Ubuntu 服务器可以访问 GitHub，直接执行：

```bash
mkdir -p "$HOME/idata-release-v0.2.17"
cd "$HOME/idata-release-v0.2.17"
curl --fail --location --remote-name \
  https://github.com/IntenTest/IDATA/releases/download/v0.2.17/IDATA-ubuntu-v0.2.17.tar.gz
curl --fail --location --remote-name \
  https://github.com/IntenTest/IDATA/releases/download/v0.2.17/SHA256SUMS
```

如果服务器不能访问 GitHub，先在可联网电脑上下载上述两个文件，再通过
SCP、SFTP 或内网文件传输工具将它们放到 Ubuntu 服务器的同一目录。

## 2. 校验并解压部署包

进入两个下载文件所在的目录，执行：

```bash
grep ' IDATA-ubuntu-v0.2.17.tar.gz$' SHA256SUMS | sha256sum --check -
tar -xzf IDATA-ubuntu-v0.2.17.tar.gz
cd IDATA-ubuntu-v0.2.17
sha256sum --check SHA256SUMS
```

上述校验应全部显示 `OK`。如果出现 `FAILED` 或找不到文件，请重新下载，
不要继续部署。

## 3. 执行安装或升级

使用默认配置部署：

```bash
sudo bash deploy-ubuntu.sh
```

脚本会自动完成备份、替换程序、重启 systemd 服务和健康检查。升级时会保留
`IDATA_LISTEN_ADDR`、管理 Token 和 `/var/lib/idata` 下的设备凭据。首次安装
默认监听 `:12345`。

如果 Nginx 通过回环地址访问 IDATA Server，并需要使用 `X-Real-IP` 区分执行
电脑，首次配置时执行：

```bash
sudo env IDATA_DEPLOY_TRUSTED_PROXIES=127.0.0.1,::1 bash deploy-ubuntu.sh
```

如果 Nginx 使用服务器内网 IP 连接后端，还需将该 IP 加入列表，例如：

```bash
sudo env IDATA_DEPLOY_TRUSTED_PROXIES=127.0.0.1,::1,10.90.65.189 \
  bash deploy-ubuntu.sh
```

## 4. 验证部署结果

```bash
sudo systemctl status idata-server --no-pager
curl --fail --silent --show-error http://127.0.0.1:12345/healthz
```

默认端口的健康检查应返回：

```json
{"status":"ok"}
```

如果原服务使用的不是 `12345` 端口，请将命令中的端口替换为
`/etc/idata/idata-server.env` 里 `IDATA_LISTEN_ADDR` 的实际端口。最后再访问对外的
IDATA 网址，确认页面、Windows Client 连接和设备列表都正常。

## 5. 失败排查和恢复

查看最近日志：

```bash
sudo journalctl -u idata-server -n 100 --no-pager
```

如果新服务无法启动或健康检查失败，部署脚本会自动恢复上一版程序、
配置和 systemd 服务。脚本结束时会输出备份目录，请保留该目录直到确认
新版运行稳定。

## 6. 不使用整包时需要的文件

仅在无法使用 `.tar.gz` 整包时，才分别下载以下三个文件，并放在同一
目录：

- `idata-server-linux-amd64`
- `deploy-ubuntu.sh`
- `SHA256SUMS`

然后在该目录执行：

```bash
chmod 0755 deploy-ubuntu.sh
sudo bash deploy-ubuntu.sh
```

## 高级网络配置

The browser launch link uses the page's current scheme, hostname, effective port,
and deployment prefix. It never uses a baked-in public server address. Page query
strings and fragments are excluded. A link from https://server.example:8443/team/
connects to wss://server.example:8443/team/ws/agent. Root HTTP URLs use port 80;
root HTTPS URLs use 443 unless a different port is explicit.

A browser launch overrides Client defaults and previously saved destinations,
including when forwarded to a running Client. Without a launch link, configure a
complete HTTP(S) or WS(S) URL in the Windows connection window, server_url in the
JSON configuration, IDATA_SERVER_URL, or --server. CLI options override environment
values, which override JSON values, which override the fallback. The fallback is
http://idata.test.huawei.com/; it is not an allowlist or a routing rule. There is
no IP-specific port detection. Bare hosts use HTTP port 80 unless they match a
previously configured endpoint. Use a full URL to choose TLS, a port, or a prefix.
The launcher loopback port is unrelated to the public server URL. IDATA business
operations no longer use a Client-owned local execution service.

## Nginx root deployment

nginx-domain.conf.example is a template, not an application requirement. Replace
server.example and the upstream with your actual deployment values. The map goes
in the http context. Replace an existing server block rather than adding a
conflicting one. Keep the browser Host including its external port using $http_host.

The user's existing http://idata.test.huawei.com/ -> 10.90.65.189:12345 deployment
continues to work. Both the browser and Client must use the same proxy entry point.
No port should be added to the browser URL merely because the upstream uses one.

## Path-prefix deployment

For a public URL such as http://server.example/tools/idata/, use a directory URL
ending in /. Within the same Nginx server block, replace the root location with:

    location = /tools/idata {
        return 308 /tools/idata/;
    }
    location /tools/idata/ {
        proxy_pass http://127.0.0.1:12345/;
        proxy_http_version 1.1;
        proxy_set_header Host $http_host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection $idata_connection_upgrade;
        proxy_read_timeout 3600s;
        proxy_send_timeout 3600s;
        proxy_buffering off;
    }

The trailing slash in proxy_pass strips the public prefix before forwarding to
backend routes. The updated page preserves the external prefix for assets, API,
launch, enrollment, and polling. Prefix segments support ASCII letters, numbers,
and -._~; empty, dot, parent, or encoded separator segments are rejected by the
Client. Older root launch links remain accepted. Do not append arbitrary routes
or filenames to the deployment URL.

## HTTPS termination

When Nginx terminates HTTPS and forwards HTTP, set this in the server environment
file and restart the IDATA service (substitute the actual origin):

    IDATA_PUBLIC_ORIGIN=https://server.example:8443

Use the browser origin only, without a deployment path; omit default :443. Configure
Nginx TLS with your organization certificate and preserve Host using $http_host.
The server uses this explicit setting for origin checks and Secure cookies and
rejects other Host values. Client certificate verification remains enabled; the
execution PC must trust the certificate chain. This setting does not change any
launch link. Real PC IP handling is configured separately below. Leave it empty for
normal HTTP deployment. TLS certificates and Nginx installation are managed by
the existing deployment, not installed by the IDATA upgrade script.

## Verify and recover

Back up Nginx configuration before edits. Run sudo nginx -t, then reload Nginx only
if validation succeeds. To revert, restore the backup, validate, and reload again.
Check the public /healthz (under any prefix), launch the updated Client, confirm
an expected device, and exercise a harmless terminal command. /ws/agent must
upgrade with HTTP 101. HTTP and WebSocket requests share the same public origin.

Automated verification covers Client parsing/handoff, enrollment prefixes,
browser URL generation and API routing for DNS/IPv4/IPv6/custom ports/prefixes,
and proxy integration for HTTP, prefixed HTTP, and TLS-terminated HTTPS. The proxy
tests cover native WebSocket registration, cookie login, device discovery, workspace
API exchange, terminal opening, and cross-origin rejection. No live private-server
or Windows desktop test has been performed. Nginx itself is not installed locally;
the integration tests use Go's HTTP reverse proxy.

## PC isolation behind Nginx

Each user PC must have a distinct source IP as seen by Nginx, with one Client
per PC. The browser and Client use the same public gateway. Nginx supplies:

    proxy_set_header X-Real-IP $remote_addr;

Configure the backend with Nginx's own peer address (not the user PC addresses):

    IDATA_TRUSTED_PROXIES=127.0.0.1,::1

The example above applies when Nginx connects over loopback. If Nginx runs on the
same host but proxy_pass uses 10.90.65.189:12345, include 10.90.65.189. For that
layout the offline upgrade command is:

    sudo env IDATA_DEPLOY_TRUSTED_PROXIES=127.0.0.1,::1,10.90.65.189 bash deploy-ubuntu.sh

If Nginx is on another host, substitute the Nginx source address seen by the
backend. The installer preserves this setting on later upgrades unless the
IDATA_DEPLOY_TRUSTED_PROXIES override is supplied. Direct access without a reverse
proxy needs no setting. Address lists and CIDRs are supported; no proxy IP is
hard-coded into the application.

One source-IP adapter applies before all HTTP and WebSocket handlers. Device
lists, test operations, report reads, and terminal authorization use the same
effective PC IP. A browser can only reach the Client whose effective IP matches
its own. It cannot select another PC by changing a URL parameter. An offline
Client does not cause fallback to another PC. Multiple Clients on one effective
IP produce a conflict instead of an arbitrary selection. A duplicate Client ID
from another IP cannot replace an existing active Client; configure distinct
Client IDs if two PCs have identical hostnames.

Verification includes two simulated PCs sharing a reverse proxy: each sees only
its own Client and sends device queries, test requests, and report reads only to
that Client. Requests and terminal access to the other PC are refused. The test
also checks that disconnecting A does not expose B to A's browser. No separate
browser-to-Client binding or confirmation flow is required.
