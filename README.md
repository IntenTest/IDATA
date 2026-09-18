# IDATA Remote

The IDATA Vue/Element Plus interface is hosted by the Go `idata-server`. The Go
`idata-client` maintains the outbound WebSocket connection from each execution PC.
The Server generates the management worker and all concrete update/test commands;
the Client only executes generic commands and returns their results.

Browser → idata-server → outbound client WebSocket → generic command executor → local commands / HDC devices.

The supported corporate Windows baseline is Simplified Chinese Windows 11 x64
with the inbox Windows PowerShell 5.1. PowerShell 7 and a system Python installation
are not required for Server management operations. Server workers are written as
UTF-8 with BOM and run without a nested PowerShell process, so Chinese source text
does not depend on the machine's active code page and errors are not wrapped as
PowerShell CLIXML. The Client runs as the current user without elevation.

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

The official Windows Client EXE contains no IDATA business worker or private Python
runtime. Start the EXE once, then use the website launch link. Server-supplied
management workers keep persistent settings under
`%USERPROFILE%\.idata\server-command-runtime`. HDC, IDATA.exe, device drivers, and
the actual test interpreter, dependencies, and scripts remain part of the PC's
test environment. Use absolute paths in the web Settings page.

## Preserved functions and integration changes

- Original overview, devices, test cases, suites, test creation/details, settings,
  language selection, and styling are reused.
- Device discovery, settings, case discovery, test start/status,
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
python3 deploy/build_windows.py --output ../bin/IDATA-Client.exe
```

No new third-party dependencies were added. Both repositories contain the protocol
documentation in `docs/protocol.md`. The new tests cover authorization, request
routing, disconnect cleanup, request limits, and local worker forwarding.

## Download and update test cases

In Settings, set **Test case archive URL** (saved automatically). The default is
`http://10.90.65.189:54322/Testcases.tar.gz`. Select **Update test case library**
on the Test Cases page. The selected execution PC downloads the archive using
the inbox `curl.exe` and extracts it with the inbox `tar.exe` on Windows. No shell
command is constructed from the URL.

The worker stages and validates the archive, then replaces
`%USERPROFILE%\.idata\newest_testcases` on Windows (`~/.idata/newest_testcases` on
macOS). Missing directories are created. The previous managed directory is removed
only after successful installation; failed downloads or validation retain it.
Custom library directories are not deleted. The archive must contain one
`mapping.csv` with the existing required columns and matching Python test files.
Both a top-level Testcases directory and a flat archive are supported. Links and
unsafe archive paths are rejected. Downloads time out after ten minutes, with a
2 GiB download limit and 4 GiB extracted-file limit.

The library path is saved automatically. Test runs always use `run_testcase.py`
from the root of that library. The page displays progress and reloads cases through the client.
If the browser closes, the update continues on the PC; click Update again while
it is running to resume watching. Client or worker shutdown interrupts the update.
The Windows management worker is embedded in the Server release and requires no
separate deployment.

Windows test cases open in a visible PowerShell console. Each case records its
command, paths, Windows and PowerShell versions, combined stdout/stderr, exception,
and exit code in a UTF-8 log under
`%USERPROFILE%\.idata\server-command-runtime\logs\<run-id>`. The run details page
shows the collected output and provides a download button for the complete log.

Regression check: `python3 -m unittest discover -s idata/app -p 'test_*.py'`.

## Ubuntu Server 一键部署

按 [部署指导](LINUX_SERVER_DEPLOY.md) 将 `deploy-ubuntu.sh`、Release 服务端二进制和 `SHA256SUMS` 拷到 Ubuntu 后执行脚本。支持首次安装和重复升级，默认端口 `12345`（升级保留已有监听配置），保留已有 Token 与设备凭据。

### Task results and local reports

The task list shows the recorded start time and sorts newest first. A completed
case passes only when its last explicit `用例<case name>执行成功` marker is successful;
`用例<case name>执行失败` means failure. Without either marker, the case is Blocked
(yellow), even if its process exits with code zero. Historical completed cases
are reclassified from their saved console output on read. Explicit cancellation
remains Interrupted.

The inspection report action asks the connected execution PC to open the saved
local HTML file in its default browser, preserving relative image paths. It does
not copy report HTML into an empty browser page.

On first use, download every executable in the latest internal release to the
same folder and run IDATA-Client.exe once before launching it from the website.
Direct Client launch displays a read-only server address; browser launch continues
to supply the website's endpoint.
