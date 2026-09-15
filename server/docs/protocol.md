# IDATA connection protocol

Protocol version 1 retains the upstream hello, command/result, terminal, enrollment,
and browser-session behavior. Agent and administrator credentials remain separate.

The Client advertises `server_commands_v1` in hello. It has no built-in IDATA API,
update workflow, test command builder, worker service, or Python runtime. The Server
turns each web operation into a normal `command` request. The Client executes that
command with the generic shell executor and returns the matching `result`.

Allowed operations:

- GET /api/devices, /api/settings, /api/model-config, /api/test-cases, /api/test-runs
- POST /api/test-runs
- POST /api/test-runs/{run}/close
- POST /api/test-runs/{run}/reports/{case}/open (legacy local report opening)
- GET /api/test-runs/{run}/reports/{case}/content (remote report viewing)
- PUT /api/settings, /api/model-config

Arbitrary URLs, query parameters, static-file paths, and other methods are rejected
by the Server before command generation. The Client allows four concurrent commands,
commands up to 128 KiB and generic stdin payloads up to 1 MiB, and enforces the
supplied timeout and output limit. Server-owned workers travel through stdin rather
than the Windows command line. On Windows the worker writes its JSON envelope to a
unique temporary result file; the Server-generated wrapper returns it as a marked
ASCII Base64 value, independent of IDATA.exe console output behavior.
Long-running tests return after launch and are polled separately. Disconnecting the
browser does not cancel a test; the explicit close operation does. If launch status
is uncertain after a connection failure, inspect test runs before retrying.

The Client returns `result` with the matching request ID, exit code, stdout, stderr,
duration, truncation flags, timeout state, or executor error. Writers use the existing WebSocket lock.
Disconnected server connections release pending requests. Older clients keep their
terminal/command support but return a capability error for the new workspace.

The server exposes these operations at
`/api/v1/clients/{client_id}/idata/{operation}` and authenticates every request using
an administrator bearer token or a device/IP browser session authorized for that PC.
Browser IP sessions are scoped to the unique Client at the same effective PC
IP. Explicitly configured proxies supply X-Real-IP; other peers use their socket
address. All HTTP/WebSocket handlers apply the same PC scope. HTML reports are
sandboxed and cannot run scripts against the control origin.

Test case archives: POST /api/test-cases/update starts or rejoins a background update;
GET /api/test-cases/update returns idle/running/complete/failed and a message.
The Server-owned worker downloads the saved testCaseArchiveUrl on the execution PC with `curl.exe`, validates
UTF-8 archive paths and the mapping CSV, replaces ~/.idata/newest_testcases, and
saves the new library path. The Server also constructs every
`IDATA.exe cli bundle run --path ...` test command and sends it through the same
generic command channel.

Browser launch accepts `idata://connect?server=HOST&port=PORT&secure=0|1`.
HOST may be an ASCII DNS name, IPv4, or IPv6; PORT is mandatory in 1..65535.
The optional `path` parameter supplies a deployment-prefixed agent path such as
`/team/idata/ws/agent`. It must end in `/ws/agent`; only ASCII unreserved path
segments are allowed, without empty, dot, parent, or escaped-separator segments.
Omitting it retains `/ws/agent` for older root deployments. Unknown or duplicate
parameters, credentials, queries within the destination, and fragments are
rejected. Launch endpoints are preserved through running-client handoff and
subsequent enrollment/polling. Page query strings and fragments are not included.
