#!/usr/bin/env python3
"""Build the generic Windows command client."""
import argparse
import os
from pathlib import Path
import subprocess

ROOT = Path(__file__).resolve().parents[2]


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", type=Path, default=ROOT / "bin/idata-client-windows-amd64.exe")
    args = parser.parse_args()
    args.output = args.output.resolve()
    args.output.parent.mkdir(parents=True, exist_ok=True)
    environment = dict(os.environ, GOOS="windows", GOARCH="amd64", CGO_ENABLED="0")
    subprocess.run([
        "go", "build", "-trimpath", "-ldflags=-H=windowsgui",
        "-o", str(args.output), "./cmd/idata-client",
    ], cwd=ROOT / "client", env=environment, check=True)
    print(f"Built {args.output}")


if __name__ == "__main__":
    main()
