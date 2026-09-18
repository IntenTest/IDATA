# Validation — 18 September 2026 (v0.2.47)

Passed:

- Server Go tests with the race detector and Go vet; Client Go tests and Go vet.
- Linux amd64 Server and Windows amd64 Client builds (Client remains 0.7.17).
- PowerShell 7 and Python worker regression tests: mixed Passed/Failed/Blocked
  cases leave the task Completed; cancellation remains Interrupted; live polling
  remains Running; case totals and results remain independent.
- Report regression tests select the final same-line 检测报告 marker, produce a
  local file URI, recover references from historical console output, open paths
  containing Chinese characters and spaces, and reject non-HTML targets.
- Python app and mirrored app tests, Python compilation, JavaScript syntax,
  browser launch/API routing tests, and deployment health-check test.
- Chrome at http://localhost:54321: Completed, Running and Interrupted task
  labels appear independently from failed/blocked case counts. The status filter
  contains only Running, Ready, Interrupted and Completed. Connection buttons
  and the download acknowledgement button both use rgb(92, 108, 255).

Limits:

- A real Windows desktop, Windows PowerShell 5.1, D: volume, physical device and
  private-network deployment are unavailable here. Local report launching is
  tested with a mocked OS opener; native Windows browser launch needs an intranet
  smoke test after deployment.
- Eight Ubuntu-only installer checks are skipped on macOS. Installer unchanged.
- Windows now defaults to D:\.idata. Existing C: data is preserved, but copying
  historical records/settings is an explicit deployment step described in
  LINUX_SERVER_DEPLOY.md. Historical absolute report paths retain their location.

No dependencies were added. The local server remains available at
http://localhost:54321 for manual inspection; temporary UI fixture disconnected.
