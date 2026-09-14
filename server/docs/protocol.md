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
Browser IP sessions are scoped to the unique Client at the same effective PC
IP. Explicitly configured proxies supply X-Real-IP; other peers use their socket
address. All HTTP/WebSocket handlers apply the same PC scope. HTML reports are
sandboxed and cannot run scripts against the control origin.

Test case repositories: POST /api/test-cases/update starts or rejoins a background update;
GET /api/test-cases/update returns idle/running/complete/failed and a message.
The worker shallow-clones the saved testCaseRepositoryUrl's `release_Idata` branch
on the execution PC, validates the mapping CSV, replaces ~/.idata/newest_testcases, and
saves the new library path. Request deadlines remain unchanged.

Browser launch accepts `idata://connect?server=HOST&port=PORT&secure=0|1`.
HOST may be an ASCII DNS name, IPv4, or IPv6; PORT is mandatory in 1..65535.
The optional `path` parameter supplies a deployment-prefixed agent path such as
`/team/idata/ws/agent`. It must end in `/ws/agent`; only ASCII unreserved path
segments are allowed, without empty, dot, parent, or escaped-separator segments.
Omitting it retains `/ws/agent` for older root deployments. Unknown or duplicate
parameters, credentials, queries within the destination, and fragments are
rejected. Launch endpoints are preserved through running-client handoff and
subsequent enrollment/polling. Page query strings and fragments are not included.
