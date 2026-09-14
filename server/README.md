# idata-server

Ubuntu 控制端，包含服务进程 `idata-server` 和管理员命令行 `idatactl`。

## 构建

```bash
mkdir -p bin
go build -o bin/idata-server ./cmd/idata-server
go build -o bin/idatactl ./cmd/idatactl
```

## 配置

服务端优先读取命令行参数，默认值可由环境变量提供：

| 环境变量 | 必填 | 默认值 | 说明 |
|---|---:|---|---|
| `IDATA_AGENT_TOKEN` | 否 | 无 | 旧版 PC 客户端共享凭据；v0.6 新客户端不需要 |
| `IDATA_ADMIN_TOKEN` | 是 | 无 | 管理 API / idatactl 凭据 |
| `IDATA_DEVICE_CREDENTIALS_FILE` | 否 | `/var/lib/idata/device-credentials.json` | 已批准设备凭据哈希存储 |
| `IDATA_ENROLLMENT_AUTO_APPROVE` | 否 | `true` | 自动批准所有有效的原生 Client 首次申请；设为 `false` 恢复人工审批 |
| `IDATA_LISTEN_ADDR` | 否 | `:12345` | HTTP 监听地址 |
| `IDATA_BROWSER_PAIRING` | 否 | `false` | 启用 v0.4 Windows 确认兼容 API；v0.5 普通页面不使用 |
| `IDATA_PAIRING_REQUEST_TTL` | 否 | `2m` | v0.4 兼容确认请求有效时间（`30s`–`10m`） |
| `IDATA_DEVICE_SESSION_TTL` | 否 | `8h` | 浏览器免密会话时长（`1m`–`24h`） |
| `IDATA_COMMAND_TIMEOUT` | 否 | `30s` | 默认命令超时 |
| `IDATA_MAX_COMMAND_TIMEOUT` | 否 | `5m` | 管理员可请求的最大超时 |

The default listener is `:12345` on every machine. Set `--listen` or
`IDATA_LISTEN_ADDR` to use another bind address or port. Public domain names,
ports, and reverse-proxy paths do not depend on network interface addresses.
See [URL deployment](deploy/DOMAIN_DEPLOY.md) for prefix routing and HTTPS
termination configuration.

如保留旧版 Agent Token，它必须与 Admin Token 不同。两者均通过 systemd 的
`EnvironmentFile` 注入，不要写入仓库。

## 启动

```bash
IDATA_ADMIN_TOKEN='...' ./bin/idata-server
```

The browser launch uses the current public URL to open the local Client.
Browser sessions are matched to the unique Client at the same effective PC IP.
Nginx's own address is configured through `IDATA_TRUSTED_PROXIES`; requests from
that proxy use its single `X-Real-IP` value. Direct connections use their socket
peer address. All HTTP and WebSocket operations use the same normalization.

Users only see their own PC's Client. Device queries, tests, reports, and terminal
operations are limited to that Client. Conflicting Clients at one IP produce an
explicit conflict; an offline PC never falls back to another PC. Sessions use
HttpOnly, SameSite=Strict cookies with a default eight-hour lifetime.

Administrator APIs and `idatactl` remain separate from ordinary browser sessions
and require the administrator token. They can manage enrollment and credentials.
The ordinary website does not expose the administrator device list.

新版 Client 会在选择设备时创建持续交互 Shell。Web 控制台通过同源 WebSocket 实时传输
输入与 stdout/stderr，因此 `cd`、环境变量等状态在当前页面会话中持续有效。关闭页面或
切换设备会关闭远程 Shell。普通 `POST /commands` 和 `idatactl exec` 仍是一次性命令，
不会共享工作目录或环境变量。

远程 Shell 自行退出、Client 短暂断线或终端 WebSocket 中断时，Web 控制台会禁用输入并按
有上限的指数退避自动重建终端；恢复后输入框会自动重新启用，无需刷新页面或重新选择设备。

Server 只审计终端会话的 client ID、session ID、来源地址、开始和结束时间，不记录终端
输入或输出内容。设备申请、批准、拒绝和撤销也会记录不含凭据的审计事件。专属凭据原文只在
批准后返回给发起申请的 Client；Server 持久化文件只保存 SHA-256 哈希并由 systemd
`StateDirectory` 限制访问。

The deployment assumes distinct user PC IPs and one Client per PC. See
[proxy configuration and verification](deploy/DOMAIN_DEPLOY.md) for the required
Nginx header and backend proxy-address setting.

健康检查：`GET /healthz`。客户端入口：`GET /ws/agent`。自助会话：`GET /api/v1/self`。

## idatactl

```bash
export IDATA_SERVER_HTTP_URL='http://服务器地址'
export IDATA_ADMIN_TOKEN='...'
./bin/idatactl clients
./bin/idatactl exec --client office-mac 'uname -a'
./bin/idatactl exec --client windows-01 --timeout 60s 'whoami'
```

`idatactl exec` 把远端 stdout 写到本地 stdout、stderr 写到本地 stderr，并以远端退出码
退出（远端退出码不在 0–125 时本地使用 1）。

## Ubuntu 内网部署

以下方案使用 systemd 在 Ubuntu 上长期运行 Server。服务使用独立的低权限 `idata` 账户，
并通过 `CAP_NET_BIND_SERVICE` 监听 TCP 80，不需要长期以 root 身份运行。Web 页面、管理 API
和 Client WebSocket 都复用该端口。

当前方案使用明文 HTTP/WS，必须只部署在受信任内网，并通过主机防火墙或上层安全组限制
来源网段。不要把 TCP 80 直接开放到公网；跨公网部署前必须增加 HTTPS/WSS。

### 1. 构建二进制

Go 版本要求为 1.23 或更新版本。在 Ubuntu 服务器或同架构的 Linux 构建机上执行：

```bash
cd idata-server
mkdir -p bin
go test ./...
go vet ./...
go build -o bin/idata-server ./cmd/idata-server
go build -o bin/idatactl ./cmd/idatactl
```

也可以从 macOS 或其他开发机交叉编译。先在目标 Ubuntu 上运行 `uname -m` 确认架构；
`x86_64` 对应 `amd64`，`aarch64` 对应 `arm64`。例如为 x86_64 Ubuntu 构建：

```bash
mkdir -p bin
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/idata-server ./cmd/idata-server
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/idatactl ./cmd/idatactl
```

将 `bin/idata-server`、`bin/idatactl`、`deploy/idata-server.service` 和
`deploy/idata-server.env.example` 上传到 Ubuntu 服务器。以下步骤假设这些文件位于服务器
上的当前项目目录。

### 2. 安装 systemd 服务

```bash
sudo useradd --system --no-create-home --shell /usr/sbin/nologin idata || true
sudo install -d -m 0755 /opt/idata
sudo install -d -m 0750 -o root -g idata /etc/idata
sudo install -m 0755 ./bin/idata-server /opt/idata/idata-server
sudo install -m 0755 ./bin/idatactl /opt/idata/idatactl
sudo install -m 0644 ./deploy/idata-server.service /etc/systemd/system/idata-server.service
sudo install -m 0640 -o root -g idata ./deploy/idata-server.env.example /etc/idata/idata-server.env
```

systemd unit 会创建权限为 `0700` 的 `/var/lib/idata`。管理员批准设备后，Server 默认把
设备专属凭据的 SHA-256 哈希保存在 `/var/lib/idata/device-credentials.json`；不会把凭据
原文写入该文件。

### 3. 设置认证配置

先生成一个高熵管理员 Token：

```bash
openssl rand -hex 32
```

然后编辑配置：

```bash
sudo nano /etc/idata/idata-server.env
```

推荐的新部署配置如下：

```dotenv
IDATA_AGENT_TOKEN=
IDATA_ADMIN_TOKEN=替换为刚生成的随机Token
IDATA_LISTEN_ADDR=
IDATA_DEVICE_CREDENTIALS_FILE=/var/lib/idata/device-credentials.json
IDATA_ENROLLMENT_AUTO_APPROVE=true
IDATA_BROWSER_PAIRING=false
IDATA_PAIRING_REQUEST_TTL=2m
IDATA_DEVICE_SESSION_TTL=8h
```

默认会为每个有效的原生 Client 申请自动签发设备专属凭据，无需打开管理页面批准。该模式只
适用于已通过防火墙限制访问的公司内网；如需恢复逐台人工批准，请设置
`IDATA_ENROLLMENT_AUTO_APPROVE=false`。若为兼容旧 Client 而设置共享 Agent Token，它必须与
Admin Token 不同。不要把任何
Token 写入仓库、命令行参数或聊天记录。

### 4. 启动并检查

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now idata-server
sudo systemctl status idata-server
```

查看实时日志：

```bash
sudo journalctl -u idata-server -f
```

在 Server 本机验证健康检查：

```bash
curl --fail http://127.0.0.1/healthz
```

正常响应为：

```json
{"status":"ok"}
```

Use the configured listener for health checks and firewall rules. With the
default listener, the local health URL is `http://127.0.0.1:12345/healthz`.

再从同一内网的另一台机器访问 `http://服务器IP/healthz`，确认路由和防火墙均已放行。

### 5. 限制内网访问

只允许实际使用的内网网段访问 TCP 80。例如内网是 `192.168.1.0/24`，使用 UFW 时执行：

```bash
sudo ufw allow from 192.168.1.0/24 to any port 80 proto tcp
sudo ufw status
```

请按实际网段替换示例地址。如果服务器还受云安全组、硬件防火墙或虚拟化平台防火墙保护，
也要在对应位置设置同样的来源限制。

### 6. 接入和批准 Client

Windows Client 的 Server 地址填写 Ubuntu 的内网 IP，例如 `192.168.1.10`。Client 会连接：

```text
ws://192.168.1.10/ws/agent
```

首次连接会提交设备申请。`IDATA_ENROLLMENT_AUTO_APPROVE=true` 时，Server 自动签发设备
专属凭据，Client 随即连接，不需要管理员操作。关闭自动批准后，管理员才需要打开
`http://服务器IP/admin/`，输入 `IDATA_ADMIN_TOKEN`，核对信息后批准。

批准后，可以在 Server 上检查在线状态并执行一条测试命令：

```bash
export IDATA_SERVER_HTTP_URL='http://127.0.0.1'
export IDATA_ADMIN_TOKEN='管理员Token'
/opt/idata/idatactl clients
/opt/idata/idatactl exec --client 客户端ID 'whoami'
```

普通用户入口是 `http://服务器IP/`，管理员入口是 `http://服务器IP/admin/`。

### 7. 更新或卸载

更新二进制：

```bash
sudo systemctl stop idata-server
sudo install -m 0755 ./bin/idata-server /opt/idata/idata-server
sudo systemctl start idata-server
sudo systemctl status idata-server
```

卸载服务但保留已批准设备数据：

```bash
sudo systemctl disable --now idata-server
sudo rm /etc/systemd/system/idata-server.service
sudo systemctl daemon-reload
```

设备凭据仍保存在 `/var/lib/idata`。只有确认不再需要恢复已批准设备时，才应另行删除该目录。

## Ubuntu Server 一键部署

按 [部署指导](../LINUX_SERVER_DEPLOY.md) 将 `deploy-ubuntu.sh`、Release 服务端二进制和 `SHA256SUMS` 拷到 Ubuntu 后执行脚本。支持首次安装和重复升级，默认端口 `12345`（升级保留配置，可覆盖），保留已有 Token 与设备凭据。
