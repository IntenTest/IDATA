# Validation — 20 September 2026 (v0.2.51)

Passed:

- Full Server Go tests with the race detector, Client Go tests, and both Go vet checks.
- PowerShell 7 and Python deletion regressions: Pending/Running rejection, deletion
  after interruption, Completed/Failed/Blocked records, idempotent retry, preserved
  reports, and exclusion after a late background write. Strict DELETE path allowlist, authenticated proxy forwarding, and rejection of
  unauthenticated/cross-origin deletion.
- Existing connection/auth/IP isolation, settings, test execution, cancellation,
  report, log and case-result regressions included in the full Server/Client suites.
- Report actions use one button-only opening path and suppress concurrent duplicate
  requests for the same run and test case in both local and Server web interfaces.
- The test-run table action uses the concise Delete label while the confirmation
  dialog retains the explicit test-run wording.
- The Settings page reads the compiled Server version from `/healthz`, displays it
  as the corresponding `v`-prefixed GitHub Release tag, and keeps it read-only.
- Chrome UI with fixture API: confirmation, cancel without mutation, successful
  deletion, running-task warning, refresh persistence, and no browser page errors.
- Linux amd64 Server and Windows amd64 Client cross-builds.
- Browser launch/API routing in five deployment layouts; JavaScript syntax.
- Both Python app suites (7 tests); deployment suite health check (8 Ubuntu-only
  installer tests skipped on macOS).

Deployment: update Ubuntu Server only; web assets and management workers are
embedded. Client 0.7.17 and companion executables are unchanged. No dependency
or database migration. D: data migration is only needed when upgrading from
versions older than v0.2.47.

Limits: Chrome uses a fixture API, not physical devices. A real Windows desktop,
PowerShell 5.1 and private-network deployment were unavailable; validate one
real task and report locally after deployment. Deletion hides history using a
durable marker; it deliberately retains local reports, logs and internal records.
