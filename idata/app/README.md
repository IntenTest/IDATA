# IDATA

A local Vue and Element Plus demo application.

## Run

Python 3 is the only local runtime requirement:

```shell
python start.py
```

Open `http://localhost:54321` in Google Chrome. If another process is listening
on port `54321`, the launcher exits with a clear error. Stop the existing
process and run the launcher again.

The inspection model defaults to `Qwen3.8-27B-Q4`. Supply its API key through
the `IDATA_MODEL_API_KEY` environment variable so credentials never enter
Git history.

To stop the local application from another terminal:

```shell
python stop.py
```

The page loads the approved, pinned Vue `3.5.24` and Element Plus `2.11.8`
browser builds from the sibling `vendor` directory. No internet connection is
required at runtime.


## Test execution commands

`test_commands.py` centralizes commands used when creating a test run:

- `build_test_command`: Python executable, runner, test case, inspection mode, and device arguments.
- `build_launch_command`: persistent log/status wrapper and working directory.
- `build_windows_command`: Windows `cmd.exe`, UTF-8 setup, and console lifetime (`/k`).

Edit this file on the execution PC and restart the local execution service to
apply changes when using an external worker (`execution_script` in the client
configuration). No Go client or server rebuild is required for that setup.
The bundled Windows EXE restores its packaged runtime on startup; to distribute
changes with the bundled client, rebuild it with `client/deploy/build_windows.py`. `start.py` retains
validation, scheduling, status tracking, and cancellation. Keep `app/test_commands.py`
and the packaged copy at `idata/app/test_commands.py` synchronized when packaging.
