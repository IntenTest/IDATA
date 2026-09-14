# IDATA Remote

The IDATA Vue/Element Plus interface is hosted by the Go `idata-server`. The Go
`idata-client` maintains the outbound WebSocket connection from each execution PC.
IDATA's Python worker on that PC builds and runs the original test commands.

Browser → idata-server → outbound client WebSocket → local IDATA worker → local commands / HDC devices.

## Current deployment

- Connection console: http://43.156.108.175/
- IDATA workspace: select a PC, then choose **Open IDATA test workspace**.
- Administrator console: http://43.156.108.175/admin/
- Remote release: `/home/ubuntu/idata-test/releases/idata-combined-20260907`.
- Existing systemd service: `idata-server`. Its credentials and listen address were retained.
- Rollback binary: `idata-server.previous` inside the release directory.

The inherited deployment uses HTTP/WS and the upstream same-source-IP browser
session model. Users behind a shared NAT can operate PCs behind that NAT. This is
not individual-user isolation. HTTPS/WSS and stronger user/device authorization
remain necessary for a production public deployment. No firewall rules were changed.

## Start an execution PC

Use Python 3.12.10 as specified by the original project. The worker uses only the
standard library. The included launcher and worker support macOS and Windows.

From this directory:

```sh
python3 start_pc.py --server ws://43.156.108.175/ws/agent --id my-execution-pc
```

On Windows, use `python` instead of `python3`. Keep the visible terminal open.
The launcher starts both the local worker and client. Port 54321 must be free;
it does not choose another port. The worker is available at http://localhost:54321.

Open the connection console and click **Connect to running IDATA PC** to establish
the browser session. Select the PC, then open IDATA. The legacy platform launch
buttons remain available for existing terminal-only installations.

Configure **Settings** using paths on the execution PC:

- Test case library containing the original mapping CSV and test files.
- `IDATA.exe` used to run CLI bundles. It defaults to the copy beside the client
  executable.
- Test case library containing `run_testcase.py` at its root.

Install the original test runtime, HDC, device drivers, and model dependencies on
that PC. They are not included in the three source repositories. Model settings
retain IDATA's existing Phoebe file location relative to the worker installation.
No device hardware is emulated or installed by this integration.

Client credentials are saved beside its executable in `bin/idata-client.json`.
Keep this file private; do not distribute an installation directory after pairing
without excluding credentials and local logs/settings.

The official Windows Client EXE includes the execution worker and its private
Python runtime. Start the EXE once, then use the website launch link. It prepares
the local service automatically before connecting; a separate project checkout
or system Python is not required for this service. Worker files and persistent
settings live under `%LOCALAPPDATA%\IDATA\execution-service`. HDC, device drivers,
and the actual test interpreter, dependencies, and scripts remain part of the PC's
test environment. Use absolute paths in the web Settings page.

## Preserved functions and integration changes

- Original overview, devices, test cases, suites, test creation/details, settings,
  model configuration, language selection, and styling are reused.
- Device discovery, settings, model settings, case discovery, test start/status,
  and cancellation are routed to the selected PC through the existing WebSocket.
- Original sequential command construction, console windows, logs, and process
  cancellation remain in the PC worker. The server never runs tests.
- Reports can be read in the remote browser in a sandbox. Remote viewing supports
  self-contained HTML up to 8 MiB; report scripts and relative external assets are
  intentionally not enabled. The original local report endpoint is also retained.
- Existing interactive terminal, one-shot command API, enrollment and admin tools
  remain available. Existing clients need updating to use the new workspace.
- PC selection survives page navigation and refresh. Remote views start without
  inherited demo test runs or suites.
- macOS execution accepts the selected Python interpreter rather than requiring
  the Windows filename `python.exe`.

The original project has in-memory test runs and browser-only suite/custom-case
editing. This integration preserves those behaviors; it does not introduce a
shared persistent database. Restarting the worker loses its in-memory run index.
Tests continue if only the browser disconnects. If a launch response is lost,
inspect existing runs before retrying to avoid duplicate execution.

## Build and check

```sh
cd server
go test -race ./...
go vet ./...
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o ../bin/idata-server-linux-amd64 ./cmd/idata-server
cd ../client
go test ./...
go vet ./...
go build -o ../bin/idata-client-darwin-arm64 ./cmd/idata-client
GOOS=darwin GOARCH=amd64 go build -o ../bin/idata-client-darwin-amd64 ./cmd/idata-client
python3 deploy/build_windows.py --output ../bin/idata-client-windows-amd64.exe
```

No new third-party dependencies were added. Both repositories contain the protocol
documentation in `docs/protocol.md`. The new tests cover authorization, request
routing, disconnect cleanup, request limits, and local worker forwarding.

## Download and update test cases

In Settings, set **Test case repository URL** (saved automatically). Select
**Update test case library** on the Test Cases page. The selected execution PC
uses Git to make a shallow, single-branch clone of `release_Idata` into a staging
directory. The branch name is fixed so the repository's default branch is never
used accidentally.

The worker validates the staged repository, then replaces
`%USERPROFILE%\.idata\newest_testcases` on Windows (`~/.idata/newest_testcases` on
macOS). Missing directories are created. The previous managed directory is removed
only after successful installation; failed clones or validation retain it.
Custom library directories are not deleted. The repository root must contain
`中英文映射.csv` with the existing required columns and matching Python test files.
Git must be installed on the execution PC and available on `PATH`. Cloning times
out after ten minutes.

The library path is saved automatically. Test runs always use `run_testcase.py`
from the root of that library. The page displays progress and reloads cases through the client.
If the browser closes, the update continues on the PC; click Update again while
it is running to resume watching. Client or worker shutdown interrupts the update.
Deploy the updated server, client executable, and Python worker together.

Regression check: `python3 -m unittest discover -s idata/app -p 'test_*.py'`.

## Ubuntu Server 一键部署

按 [部署指导](LINUX_SERVER_DEPLOY.md) 将 `deploy-ubuntu.sh`、Release 服务端二进制和 `SHA256SUMS` 拷到 Ubuntu 后执行脚本。支持首次安装和重复升级，默认端口 `12345`（升级保留已有监听配置），保留已有 Token 与设备凭据。
