#!/usr/bin/env python3
"""Build the offline Windows client with its private execution runtime."""
import argparse
import hashlib
import io
import os
from pathlib import Path
import subprocess
import tempfile
import urllib.request
import zipfile

ROOT = Path(__file__).resolve().parents[2]
PYTHON_URL = "https://www.python.org/ftp/python/3.12.10/python-3.12.10-embed-amd64.zip"
PYTHON_SHA256 = "4acbed6dd1c744b0376e3b1cf57ce906f9dc9e95e68824584c8099a63025a3c3"


def prepare(archive_path):
    archive = archive_path.read_bytes()
    if hashlib.sha256(archive).hexdigest() != PYTHON_SHA256:
        raise SystemExit("Python runtime SHA-256 mismatch.")
    files = {}
    with zipfile.ZipFile(io.BytesIO(archive)) as python:
        for entry in python.infolist():
            if not entry.is_dir():
                if "/" in entry.filename or "\\" in entry.filename or ":" in entry.filename:
                    raise SystemExit("Unexpected Python runtime path.")
                files["python/" + entry.filename] = python.read(entry)
    for name in ("start.py", "run_test_process.py", "test_commands.py"):
        files["idata/app/" + name] = (ROOT / "idata/app" / name).read_bytes()
    for package in ("vue-3.5.24", "element-plus-2.11.8"):
        for source in sorted((ROOT / "idata/vendor" / package).rglob("*")):
            if source.is_file():
                files[source.relative_to(ROOT).as_posix()] = source.read_bytes()
    # Do not distribute settings, credentials, logs, or test data.
    output = ROOT / "client/internal/executionservice/runtime.zip"
    with zipfile.ZipFile(output, "w", compression=zipfile.ZIP_DEFLATED) as bundle:
        for name, contents in sorted(files.items()):
            info = zipfile.ZipInfo(name, date_time=(2025, 4, 8, 0, 0, 0))
            info.compress_type = zipfile.ZIP_DEFLATED
            info.external_attr = 0o100600 << 16
            bundle.writestr(info, contents)
    print(f"Prepared offline runtime: {len(files)} files.")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--python-archive", type=Path, help="Previously downloaded, checksum-verified Python archive")
    parser.add_argument("--output", type=Path, default=ROOT / "bin/idata-client-windows-amd64.exe")
    parser.add_argument("--prepare-only", action="store_true")
    args = parser.parse_args()
    if args.python_archive:
        prepare(args.python_archive)
    else:
        with tempfile.TemporaryDirectory() as directory:
            archive = Path(directory) / "python.zip"
            with urllib.request.urlopen(PYTHON_URL, timeout=60) as response:
                archive.write_bytes(response.read())
            prepare(archive)
    if not args.prepare_only:
        args.output = args.output.resolve()
        args.output.parent.mkdir(parents=True, exist_ok=True)
        environment = dict(os.environ, GOOS="windows", GOARCH="amd64", CGO_ENABLED="0")
        subprocess.run([
            "go", "build", "-trimpath", "-tags=idata_bundle", "-ldflags=-H=windowsgui",
            "-o", str(args.output), "./cmd/idata-client",
        ], cwd=ROOT / "client", env=environment, check=True)
        print(f"Built {args.output}")


if __name__ == "__main__":
    main()
