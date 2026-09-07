# iData Server 离线部署（Ubuntu x86_64）

目标 Ubuntu 不需要安装 Go，也不会在部署期间下载 Go module。开始前运行 `uname -m`，
结果应为 `x86_64`。

## 下载部署包

下载以下两个文件：

```text
idata-server-linux-amd64.tar.gz
idata-server-linux-amd64.tar.gz.sha256
```

如果 Ubuntu 能访问 GitHub，可以直接下载：

```bash
wget https://github.com/IntenTest/idata-server/releases/download/v0.7.0/idata-server-linux-amd64.tar.gz
wget https://github.com/IntenTest/idata-server/releases/download/v0.7.0/idata-server-linux-amd64.tar.gz.sha256
```

如果内网 Ubuntu 不能访问 GitHub，请在能联网的电脑上下载这两个文件，再使用 U 盘、内网
文件服务或其他获准方式搬到 Ubuntu。不要只复制解压后的部分文件。

## 解压并校验

先在两个下载文件所在目录校验压缩包：

```bash
sha256sum -c idata-server-linux-amd64.tar.gz.sha256
```

看到 `idata-server-linux-amd64.tar.gz: OK` 后再解压：

```bash
tar -xzf idata-server-linux-amd64.tar.gz
cd idata-server-linux-amd64
sha256sum -c SHA256SUMS
```

最后两行应分别显示 `idata-server: OK` 和 `idatactl: OK`。如果任一校验失败，不要运行文件，
应重新传输或下载。

## 安装

```bash
sudo useradd --system --no-create-home --shell /usr/sbin/nologin idata || true
sudo install -d -m 0755 /opt/idata
sudo install -d -m 0750 -o root -g idata /etc/idata
sudo install -m 0755 ./idata-server /opt/idata/idata-server
sudo install -m 0755 ./idatactl /opt/idata/idatactl
sudo install -m 0644 ./idata-server.service /etc/systemd/system/idata-server.service
sudo install -m 0640 -o root -g idata ./idata-server.env.example /etc/idata/idata-server.env
```

生成管理员 Token，并保存输出：

```bash
openssl rand -hex 32
```

编辑 `/etc/idata/idata-server.env`，把 `IDATA_ADMIN_TOKEN` 替换为生成的 Token。新部署建议
保持 `IDATA_AGENT_TOKEN=` 为空，让 Client 取得设备专属凭据。专用部署默认设置
`IDATA_ENROLLMENT_AUTO_APPROVE=true`，所有有效的原生 Client 申请都会自动通过；必须使用
防火墙把 Server 限制在可信公司网络。设置为 `false` 可恢复管理员逐台审批。

```bash
sudo nano /etc/idata/idata-server.env
```

启动服务：

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now idata-server
sudo systemctl status idata-server
```

## 验证

```bash
curl --fail http://127.0.0.1/healthz
```

正常响应为 `{"status":"ok"}`。管理员页面为 `http://服务器内网IP/admin/`。

如果 Server 的本机网卡地址包含 `10.90.65.189`，默认监听端口是 `12345`，此时使用：

```bash
curl --fail http://127.0.0.1:12345/healthz
```

管理员页面为 `http://10.90.65.189:12345/admin/`。其他服务器仍默认使用端口 80。需要覆盖
自动选择时，在 `/etc/idata/idata-server.env` 中明确设置 `IDATA_LISTEN_ADDR`。

查看日志：

```bash
sudo journalctl -u idata-server -f
```

查看在线 Client：

```bash
export IDATA_SERVER_HTTP_URL='http://127.0.0.1'
export IDATA_ADMIN_TOKEN='管理员Token'
/opt/idata/idatactl clients
```

当前服务使用明文 HTTP/WS，只能部署在可信内网。请在防火墙中仅允许实际内网网段访问
TCP 80，不要直接开放到公网。
