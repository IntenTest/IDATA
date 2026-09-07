#!/usr/bin/env python3
"""Start the visible IDATA execution service and outbound client together."""
import argparse
import platform
import subprocess
import sys
import time
import urllib.request
from pathlib import Path

ROOT = Path(__file__).resolve().parent

def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--server', required=True, help='Server WebSocket URL, ending in /ws/agent')
    parser.add_argument('--id', help='Stable name for this execution PC')
    args = parser.parse_args()
    system = platform.system().lower()
    arch = 'arm64' if platform.machine().lower() in ('arm64', 'aarch64') else 'amd64'
    executable = ROOT / 'bin' / f'idata-client-{system}-{arch}{".exe" if system == "windows" else ""}'
    if not executable.is_file():
        raise SystemExit(f'Client executable is missing: {executable.name}')
    children = []
    try:
        worker = subprocess.Popen([sys.executable, str(ROOT / 'idata/app/start.py')])
        children.append(worker)
        for _ in range(100):
            if worker.poll() is not None:
                raise RuntimeError('The execution service could not start. Resolve the port 54321 conflict first.')
            try:
                with urllib.request.urlopen('http://127.0.0.1:54321/api/settings', timeout=1) as response:
                    if response.status == 200:
                        break
            except OSError:
                time.sleep(0.1)
        else:
            raise RuntimeError('The execution service did not become ready.')
        command = [str(executable), '--server', args.server, '--browser-bridge', 'off', '--register-url-protocol=false']
        if args.id:
            command += ['--id', args.id]
        client = subprocess.Popen(command)
        children.append(client)
        print('IDATA PC is running. Keep this window open. Press Ctrl+C to disconnect.', flush=True)
        while all(child.poll() is None for child in children):
            time.sleep(0.5)
    except KeyboardInterrupt:
        pass
    finally:
        for child in reversed(children):
            if child.poll() is None:
                child.terminate()
        for child in children:
            try:
                child.wait(timeout=10)
            except subprocess.TimeoutExpired:
                child.kill()
                child.wait()

if __name__ == '__main__':
    main()
