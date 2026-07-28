#!/usr/bin/env python3
"""Run a test command while mirroring all output to a persistent log."""

from pathlib import Path
import json
import locale
import subprocess
import sys


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

    with log_path.open("w", encoding="utf-8", errors="replace") as log_file:
        heading = f"> {subprocess.list2cmdline(command)}\n\n"
        print(heading, end="", flush=True)
        log_file.write(heading)
        log_file.flush()

        try:
            process = subprocess.Popen(
                command,
                stdout=subprocess.PIPE,
                stderr=subprocess.STDOUT,
                text=True,
                encoding=locale.getpreferredencoding(False),
                errors="replace",
                bufsize=1,
            )
            assert process.stdout is not None
            for line in process.stdout:
                print(line, end="", flush=True)
                log_file.write(line)
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
