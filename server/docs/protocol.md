# IDATA connection protocol

Protocol version 1 retains the upstream hello, command/result, terminal, enrollment,
and browser-session behavior. Agent and administrator credentials remain separate.

The combined client advertises `idata_api_v1` in hello. An authenticated server can
send `idata_api_request` with a unique `request_id`, `method`, `path`, and base64
`data`. The client invokes the local IDATA execution service at the fixed numeric
loopback address `127.0.0.1:54321`. That service constructs and executes the original
IDATA commands on the PC. The remote server never runs test subprocesses.

Allowed operations:

- GET /api/devices, /api/settings, /api/model-config, /api/test-cases, /api/test-runs
- POST /api/test-runs
- POST /api/test-runs/{run}/close
- POST /api/test-runs/{run}/reports/{case}/open (legacy local report opening)
- GET /api/test-runs/{run}/reports/{case}/content (remote report viewing)
- PUT /api/settings, /api/model-config

Arbitrary URLs, query parameters, static-file paths, and other methods are rejected.
The client allows four concurrent operations, requests up to 64 KiB, responses up to
8 MiB, and a 30-second request deadline. Redirects and HTTP proxies are disabled.
Long-running tests return after launch and are polled separately. Disconnecting the
browser does not cancel a test; the explicit close operation does. If launch status
is uncertain after a connection failure, inspect test runs before retrying.

The client returns `idata_api_response` with the matching request ID, HTTP `status`,
`content_type`, base64 `data`, or `error`. Writers use the existing WebSocket lock.
Disconnected server connections release pending requests. Older clients keep their
terminal/command support but return a capability error for the new workspace.

The server exposes these operations at
`/api/v1/clients/{client_id}/idata/{operation}` and authenticates every request using
an administrator bearer token or a device/IP browser session authorized for that PC.
The established shared-NAT/IP access model remains unchanged. HTML reports are
sandboxed and cannot run scripts against the control origin.
