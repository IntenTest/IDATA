# iData Server 离线部署（Ubuntu x86_64）

请使用仓库根目录的 [Ubuntu Server 一键部署指导](../LINUX_SERVER_DEPLOY.md)。
首次安装和后续升级使用同一脚本，固定端口 **12345**。

从 [最新 IDATA Release](https://github.com/IntenTest/IDATA/releases/latest) 下载同版本的
`idata-server-linux-amd64` 和 `SHA256SUMS`，与
[deploy-ubuntu.sh](deploy/deploy-ubuntu.sh) 放在同一目录后执行：

```bash
sudo bash /path/to/deploy-ubuntu.sh
```

无需联网或安装 Go。脚本校验 Release、保留已有 Token 和设备凭据、备份旧程序并进行启动检查，
失败时恢复旧部署。访问 `http://服务器IP:12345/`；管理员页面为 `/admin/`。
详细的网络配置、升级与恢复方法见上面的完整指导。
