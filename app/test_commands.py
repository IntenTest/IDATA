"""Build test-run commands executed on the client PC.

Edit this module to change runner arguments or Windows console startup.
Command construction only; scheduling and process management remain in start.py.
"""

import os
from pathlib import Path
import subprocess


def build_test_command(python_path, runner_path, case_name, inspection_mode, device):
    """Arguments passed to run_testcase.py, in runner-defined order."""
    return [str(python_path), str(runner_path), case_name,
            str(inspection_mode), device.strip()]


def build_launch_command(test_command, process_runner, log_path, status_path,
                         library_path):
    """Wrap a test with persistent logging and platform-specific startup."""
    worker_command = [test_command[0], str(process_runner), str(log_path),
                      str(status_path), "--", *test_command]
    options = {"cwd": library_path}
    if os.name == "nt":
        command = build_windows_command(worker_command)
        options["creationflags"] = subprocess.CREATE_NEW_CONSOLE
    else:
        command = worker_command
        options["start_new_session"] = True
    return command, options


def build_windows_command(worker_command):
    """Open a UTF-8 console and keep it visible after the test completes."""
    system_root = Path(os.environ.get("SystemRoot", r"C:\Windows"))
    cmd_path = system_root / "System32" / "cmd.exe"
    cmd_command = (
        "chcp 65001 >nul"
        ' & set "PYTHONUTF8=1"'
        f" & {subprocess.list2cmdline(worker_command)}"
    )
    return [str(cmd_path), "/d", "/k", cmd_command]
