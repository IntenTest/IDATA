# Validation — 7 September 2026

Passed:

- Existing client tests, Go vet, and Windows amd64 cross-compilation.
- Existing server tests, Go race detector, Go vet, and Linux amd64 build.
- New forwarding tests: fixed local target, request size limits, authentication,
  request/body matching, and pending-request cleanup on disconnect.
- Python compilation and JavaScript syntax checks.
- Local subprocess smoke test using the original IDATA command builder.
- Live deployment: remote HTTP request → server → outbound client WebSocket →
  local Python worker → subprocess on this Mac.
- Live settings update/read/restore and test discovery through the remote server.
- Original IDATA result evaluation: smoke test completed with one passing case.
- Remote HTML report retrieval and sandboxed report rendering in Chrome.
- Chrome verification of the original interface at the deployed URL and at
  localhost:54321, and PC selection retained in navigation URLs.

The fixture uses a small generated test runner; it does not exercise physical
HarmonyOS devices, the user's actual test suite, or the external model service.
Those dependencies were not configured by the three repositories. The fixture
settings were restored after testing. The local combined launcher remains running
as `idata-remote-mac` for manual inspection.

No new third-party dependencies were added. Runtime credentials and logs are
excluded from the downloadable package. The original deployed server binary was
retained for rollback before replacement.
