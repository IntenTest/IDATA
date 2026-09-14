# idata-server 开发指南

先阅读仓库根目录 `AGENTS.md` 和 `docs/protocol.md`。

## 职责

- `cmd/idata-server`：Ubuntu 上运行的长驻 HTTP/WebSocket 服务。
- `cmd/idatactl`：面向管理员的 API 命令行客户端。
- `internal/protocol`：线上 JSON 类型；任何改动都要同步协议文档和 client。
- `internal/server`：认证、连接注册、命令调度、结果匹配和 HTTP handler。
- `internal/server/idataweb`：嵌入服务端二进制的 IDATA Vue 页面和 Client 连接窗口。

## 不变量

- agent token 与 admin token 用途严格分离。
- 一个 WebSocket 只能有一个 reader；所有 writer 必须经连接对象的写锁。
- 断线时要释放该连接的 pending command，不能泄漏 goroutine。
- API 输入必须限制 body 大小，命令和 client ID 必须验证。
- `/healthz` 可匿名访问；普通 `/` 只提供 IDATA 页面。连接窗口通过 `idata://` 唤起 Client
  后创建短期来源 IP 会话，只允许操作有效来源 IP 相同的在线 Client。仅允许从显式配置的可信 Nginx 地址读取单值 X-Real-IP；其他转发 header、
  client ID 或页面状态不得授予设备访问。所有 HTTP 和 WebSocket 必须统一使用有效来源 IP。
- 不提供独立 Server 或管理员网页；管理 API 必须使用管理员 token。
- 不记录命令输出和认证密钥；命令文本只在明确开启审计策略后才可持久化。
- IP 会话 Cookie 必须 HttpOnly、SameSite=Strict、限时且可退出；每台用户 PC 使用不同 IP 且最多运行一个 Client；同一有效 IP 出现多个 Client 时
  必须提示冲突，不能自动选择其他设备。

## 验证

在本目录运行：

```bash
go test ./...
go vet ./...
```
