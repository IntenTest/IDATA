# Ubuntu Server 一键离线部署 / 升级

将脚本和同一个 Release 的服务端文件拷到 Ubuntu，执行一次命令，即可首次安装或重新部署。
固定监听 **TCP 12345**，由 systemd 自动启动。Ubuntu 不需要联网、Go、Python 或源码编译。

## 1. 准备三个文件

在能访问 GitHub 的电脑上打开 [最新正式 Release](https://github.com/IntenTest/IDATA/releases/latest)，
下载同一个版本的两个附件：

- `idata-server-linux-amd64`
- `SHA256SUMS`

另存仓库 main 中的 [deploy-ubuntu.sh](https://raw.githubusercontent.com/IntenTest/IDATA/main/server/deploy/deploy-ubuntu.sh)。
三个文件保持原名，放在同一目录，拷贝到 Ubuntu，例如：

```text
/home/ubuntu/idata-install/
├── deploy-ubuntu.sh
├── idata-server-linux-amd64
└── SHA256SUMS
```

截至本次文档更新，最新正式版为 `v0.2.10`。之后请从最新 Release 获取二进制和对应的
校验文件，不要混用不同版本。脚本部署的是你拷入的版本，不会联网查询或下载版本。
不用下载 Windows Client、源码压缩包或单独的 `idata-server.service`；服务定义已内置在脚本中。
从浏览器保存脚本时应保存原始文件，不能保存 GitHub HTML 页面。

## 2. 执行部署

支持运行 systemd 的 **Ubuntu Server x86_64 / amd64**，使用有 sudo 权限的账号：

```bash
sudo bash /home/ubuntu/idata-install/deploy-ubuntu.sh
```

也可以让脚本和 Release 文件分开存放：

```bash
sudo bash /path/to/deploy-ubuntu.sh /path/to/release-files
```

不用事先创建账号、配置 Token、停止旧服务或删除旧版本。脚本使用 Ubuntu 自带的 Bash、
coreutils、util-linux、iproute2 和账号管理工具；若使用裁剪镜像缺少这些工具，会在停止旧服务前报错，
不会自动访问软件源安装依赖。

脚本会：

1. 将二进制暂存，校验 `SHA256SUMS` 中唯一的服务端条目及 ELF/x86_64 标识。校验失败不触碰现有安装。
2. 创建 `idata` 服务账号；首次安装生成随机管理员 Token，保存到 `/etc/idata/idata-server.env`。
3. 升级时保留现有配置和 Token，只将 `IDATA_LISTEN_ADDR` 改为 `:12345`。
4. 在 `/var/backups/idata-deploy.*` 保存原二进制、配置和 service 文件。备份仅 root 可读。
5. 停止旧服务，确认 12345 未被其他进程占用，替换程序并启动。
6. 检查服务运行状态与 `http://127.0.0.1:12345/healthz`，成功后开启开机自启。
7. 安装或健康检查失败时恢复原二进制、配置及 service，并恢复原来的启动/启用状态。

设备凭据 `/var/lib/idata` 不删除、不覆盖，已注册设备可继续连接。
升级会短暂中断连接，正在执行任务时应先等待任务结束。
现有 systemd drop-in、自定义 ExecStart 或安装路径符号链接会被拒绝，避免误覆盖特殊部署；
该脚本支持本仓库标准安装路径。现有配置如果无效，启动检查会失败并回滚，不会擅自重置 Token。

## 3. 访问与查看状态

部署成功后访问：

```text
http://服务器IP:12345/
http://服务器IP:12345/admin/
```

查看管理员 Token（脚本不会把密钥写到部署输出中）：

```bash
sudo cat /etc/idata/idata-server.env
```

使用其中的 `IDATA_ADMIN_TOKEN` 登录管理页面。查看运行状态与日志：

```bash
sudo systemctl status idata-server --no-pager
sudo journalctl -u idata-server -n 100 --no-pager
```

如已安装 curl，可手动检查：

```bash
curl --fail http://127.0.0.1:12345/healthz
```

正常响应为 `{"status":"ok"}`。脚本自己的检查不依赖 curl。

## 4. 网络与 Windows Client

脚本不修改防火墙或云安全组。如果启用了访问限制，请允许可信内网访问 TCP **12345**。
例如已启用 UFW 时，按实际内网网段配置：

```bash
sudo ufw allow from 192.168.1.0/24 to any port 12345 proto tcp
```

默认采用 HTTP/WS，首次安装自动批准原生 Client 注册，仅适用于可信内网；不要直接暴露到公网。
需要逐台审批时，将配置中的 `IDATA_ENROLLMENT_AUTO_APPROVE` 设为 `false`，然后重启服务。

Windows 上建议使用同一 Release 的 `idata-client-windows-amd64.exe`。退出旧 Client，
替换 EXE，保留旁边的 `idata-client.json`，运行一次以刷新 URL 协议注册。
打开 `http://服务器IP:12345/`，点击 **Open IDATA Client**，网页会将服务端地址和端口传给 Client。
Client 本机使用的 54321、17891 与服务端 12345 是不同用途的端口。

Client 内含执行 worker 和私有 Python 运行时；测试执行所需的 HDC、测试依赖及 Python 配置仍需保留。
测试用例库仍从网页 Settings 中的 **Test case archive URL** 下载到 Windows 执行机，
默认地址为 `http://10.90.65.189:54322/Testcases.tar.gz`。
在 Test Cases 点击 **Update test case library**，验证后安装到 `%USERPROFILE%\.idata\newest_testcases`。
无需将测试用例压缩包上传到 Ubuntu。

## 5. 后续升级与恢复

每次更新只需用同一个新 Release 的二进制和 `SHA256SUMS` 替换安装目录里的文件，再运行同一命令。
不要删除 `/etc/idata` 或 `/var/lib/idata`。旧设备凭据不会因重装而丢失。

启动失败会自动回滚。首次安装失败会移除本次安装的程序、配置及 service，留下账号和目录以便重试。
回滚失败会报错，应通过上面的 systemd 日志排查。健康检查仅验证服务可运行，不代表所有业务功能已验证。

成功升级后若发现业务问题，可以将保存的旧 Release 二进制和对应 `SHA256SUMS` 放回安装目录，
再次运行脚本。这样仍保持 12345 和当前 Token。
备份目录保存部署前的程序、配置和 service，可用于手动恢复原配置；脚本输出具体备份路径。
备份含密钥，应妥善保管，并在确认新版本稳定后按需要清理。
备份不包含设备凭据数据库；脚本不会对数据库做降级回写。重大版本升级前应另外备份
`/var/lib/idata`（以及自定义凭据路径），并确认版本的数据兼容性。

## 验证脚本的维护说明

在 Linux 环境运行隔离测试（临时目录与模拟 systemd，不操作当前服务）：

```bash
python3 -m unittest discover -s server/deploy/tests -v
bash -n server/deploy/deploy-ubuntu.sh
```
