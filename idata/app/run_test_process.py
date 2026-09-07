#!/usr/bin/env python3
"""Run a test command while mirroring all output to a persistent log."""

from pathlib import Path
import ctypes
import json
import locale
import os
import re
import subprocess
import sys


ANSI_ESCAPE = re.compile(
    r"""
    \x1B
    (?:
        \][^\x07]*(?:\x07|\x1B\\)
        |
        \[[0-?]*[ -/]*[@-~]
    )
    """,
    re.VERBOSE,
)


def enable_windows_console_output() -> None:
    if os.name != "nt":
        return

    kernel32 = ctypes.windll.kernel32
    stdout_handle = kernel32.GetStdHandle(-11)
    mode = ctypes.c_uint()
    if kernel32.GetConsoleMode(stdout_handle, ctypes.byref(mode)):
        kernel32.SetConsoleMode(stdout_handle, mode.value | 0x0004)
    kernel32.SetConsoleOutputCP(65001)
    kernel32.SetConsoleCP(65001)


def decode_console_line(raw_line: bytes) -> str:
    encodings = (
        "utf-8",
        locale.getpreferredencoding(False),
        "gb18030",
        "cp936",
    )
    for encoding in dict.fromkeys(encodings):
        try:
            return raw_line.decode(encoding)
        except (LookupError, UnicodeDecodeError):
            continue
    return raw_line.decode("utf-8", errors="replace")


def main() -> int:
    if len(sys.argv) < 5 or sys.argv[3] != "--":
        print(
            "Usage: run_test_process.py <log-path> <status-path> -- <command...>",
            file=sys.stderr,
        )
        return 2

    log_path = Path(sys.argv[1])
    status_path = Path(sys.argv[2])
    command = sys.argv[4:]
    log_path.parent.mkdir(parents=True, exist_ok=True)
    enable_windows_console_output()

    with log_path.open("w", encoding="utf-8", errors="replace") as log_file:
        heading = f"> {subprocess.list2cmdline(command)}\n\n"
        print(heading, end="", flush=True)
        log_file.write(heading)
        log_file.flush()

        try:
            child_environment = os.environ.copy()
            child_environment.pop("NO_COLOR", None)
            child_environment.update(
                {
                    "CLICOLOR": "1",
                    "CLICOLOR_FORCE": "1",
                    "FORCE_COLOR": "1",
                    "PY_COLORS": "1",
                    "TERM": "xterm-256color",
                }
            )
            process = subprocess.Popen(
                command,
                stdout=subprocess.PIPE,
                stderr=subprocess.STDOUT,
                env=child_environment,
                bufsize=0,
            )
            assert process.stdout is not None
            for raw_line in iter(process.stdout.readline, b""):
                line = decode_console_line(raw_line)
                print(line, end="", flush=True)
                log_file.write(ANSI_ESCAPE.sub("", line))
                log_file.flush()
            exit_code = process.wait()
        except OSError as error:
            message = f"Unable to execute test command: {error}\n"
            print(message, end="", file=sys.stderr, flush=True)
            log_file.write(message)
            log_file.flush()
            exit_code = 1

        completion = f"\nCommand finished with exit code {exit_code}.\n"
        print(completion, end="", flush=True)
        log_file.write(completion)
        log_file.flush()

    status_path.write_text(
        json.dumps({"exitCode": exit_code}),
        encoding="utf-8",
    )
    return exit_code


if __name__ == "__main__":
    raise SystemExit(main())
