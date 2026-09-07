# idata-server 开发指南

先阅读仓库根目录 `AGENTS.md` 和 `docs/protocol.md`。

## 职责

- `cmd/idata-server`：Ubuntu 上运行的长驻 HTTP/WebSocket 服务。
- `cmd/idatactl`：面向管理员的 API 命令行客户端。
- `internal/protocol`：线上 JSON 类型；任何改动都要同步协议文档和 client。
- `internal/server`：认证、连接注册、命令调度、结果匹配和 HTTP handler。
- `internal/server/web`：嵌入服务端二进制的无框架 Web 控制台。

## 不变量

- agent token 与 admin token 用途严格分离。
- 一个 WebSocket 只能有一个 reader；所有 writer 必须经连接对象的写锁。
- 断线时要释放该连接的 pending command，不能泄漏 goroutine。
- API 输入必须限制 body 大小，命令和 client ID 必须验证。
- `/healthz` 可匿名访问；普通 `/` 通过 `idata://` 唤起 Client 后创建短期来源 IP 会话，
  只列出并允许操作直接来源 IP 相同的在线 Client。不得信任转发 header、client ID 或页面
  状态，终端 WebSocket 必须重新验证来源 IP 范围。
- `/admin/` 和其他管理 API 必须使用管理员 token，token 只存于 sessionStorage。
- 不记录命令输出和认证密钥；命令文本只在明确开启审计策略后才可持久化。
- IP 会话 Cookie 必须 HttpOnly、SameSite=Strict、限时且可退出；共享代理/NAT 会共享设备
  列表，这一设计边界必须在 README 中明确说明。

## 验证

在本目录运行：

```bash
go test ./...
go vet ./...
```
