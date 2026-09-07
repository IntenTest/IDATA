#!/usr/bin/env python3
"""Stop the local application started by start.py."""

from __future__ import annotations

from pathlib import Path
import argparse
import os
import signal
import socket
import subprocess
import sys
import time


HOST = "127.0.0.1"
PORT = 54321
APP_DIRECTORY = Path(__file__).resolve().parent
START_SCRIPT = APP_DIRECTORY / "start.py"
PID_PATH = APP_DIRECTORY / ".idata.pid"


def port_is_available() -> bool:
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as probe:
        return probe.connect_ex((HOST, PORT)) != 0


def listener_pids() -> set[int]:
    if os.name == "nt":
        result = subprocess.run(
            ["netstat", "-ano", "-p", "tcp"],
            capture_output=True,
            check=False,
            text=True,
        )
        pids: set[int] = set()
        for line in result.stdout.splitlines():
            columns = line.split()
            if (
                len(columns) >= 5
                and columns[1].endswith(f":{PORT}")
                and columns[3] == "LISTENING"
                and columns[4].isdigit()
            ):
                pids.add(int(columns[4]))
        return pids

    result = subprocess.run(
        ["lsof", "-nP", f"-iTCP:{PORT}", "-sTCP:LISTEN", "-t"],
        capture_output=True,
        check=False,
        text=True,
    )
    return {int(line) for line in result.stdout.splitlines() if line.strip().isdigit()}


def pid_file_pid() -> int | None:
    try:
        raw_pid = PID_PATH.read_text(encoding="utf-8").strip()
    except FileNotFoundError:
        return None
    except OSError:
        return None

    if not raw_pid.isdigit():
        return None
    return int(raw_pid)


def command_line(pid: int) -> str:
    command = [
        "powershell",
        "-NoProfile",
        "-Command",
        (
            "Get-CimInstance Win32_Process -Filter "
            f"'ProcessId = {pid}' | Select-Object -ExpandProperty CommandLine"
        ),
    ]
    result = subprocess.run(command, capture_output=True, check=False, text=True)
    return result.stdout.strip()


def working_directory(pid: int) -> Path | None:
    result = subprocess.run(
        ["lsof", "-a", "-p", str(pid), "-d", "cwd", "-Fn"],
        capture_output=True,
        check=False,
        text=True,
    )
    for line in result.stdout.splitlines():
        if line.startswith("n"):
            return Path(line[1:]).resolve()
    return None


def is_start_process(pid: int) -> bool:
    if os.name != "nt":
        return working_directory(pid) == APP_DIRECTORY

    command = command_line(pid)
    if not command:
        return False

    normalized_command = command.replace("\\", "/")
    normalized_start_script = str(START_SCRIPT).replace("\\", "/")

    if normalized_start_script in normalized_command:
        return True

    return "start.py" in normalized_command and "python" in normalized_command.lower()


def stop_process(pid: int) -> None:
    if os.name == "nt":
        result = subprocess.run(
            ["taskkill", "/PID", str(pid), "/F"],
            capture_output=True,
            check=False,
            text=True,
        )
        if result.returncode != 0:
            detail = result.stderr.strip() or result.stdout.strip()
            raise RuntimeError(detail or f"taskkill exited with {result.returncode}")
        return

    os.kill(pid, signal.SIGTERM)
    for _ in range(20):
        if port_is_available():
            return
        time.sleep(0.1)

    os.kill(pid, signal.SIGKILL)


def stop_start_processes(dry_run: bool) -> int:
    listening_pids = listener_pids()
    pid_from_file = pid_file_pid()
    pids = {
        pid for pid in listening_pids if is_start_process(pid)
    }
    if pid_from_file in listening_pids:
        pids.add(pid_from_file)
    pids = sorted(pids)
    if not pids:
        print(f"No start.py process is listening on port {PORT}.")
        return 0

    for pid in pids:
        if dry_run:
            print(f"Would stop start.py process {pid} on port {PORT}.")
        else:
            print(f"Stopping start.py process {pid} on port {PORT}.")
            stop_process(pid)

    if dry_run:
        return 0

    for _ in range(30):
        if port_is_available():
            print(f"Port {PORT} is clear.")
            return 0
        time.sleep(0.1)

    print(f"Unable to clear port {PORT}.", file=sys.stderr)
    return 1


def main() -> None:
    parser = argparse.ArgumentParser(
        description="Stop the local application process started by start.py."
    )
    parser.add_argument(
        "--dry-run",
        action="store_true",
        help="Show matching processes without stopping them.",
    )
    args = parser.parse_args()

    raise SystemExit(stop_start_processes(args.dry_run))


if __name__ == "__main__":
    main()
