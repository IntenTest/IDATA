"""Linux integration tests with sandboxed paths and simulated systemd.
Run: python3 -m unittest discover -s server/deploy/tests -v
No root permissions or production service changes are needed.
"""
import hashlib
import http.server
import threading
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

SCRIPT = Path(__file__).resolve().parents[1] / 'deploy-ubuntu.sh'


@unittest.skipUnless(os.uname().sysname == 'Linux', 'requires GNU Ubuntu utilities')
class DeployTest(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        for name in ('opt', 'etc/systemd/system', 'var/backups', 'run/lock', 'run/systemd/system', 'bin', 'release'):
            (self.root / name).mkdir(parents=True)
        (self.root / 'etc/os-release').write_text('ID=ubuntu\n')
        # Redirect only in the test copy, never add a root override to the installer.
        script = SCRIPT.read_text().replace('[[ $EUID == 0 ]]', '[[ 0 == 0 ]]')
        script = script.replace('export PATH=/usr/sbin:/usr/bin:/sbin:/bin',
                                f'export PATH={self.root}/bin:/usr/sbin:/usr/bin:/sbin:/bin')
        for prefix in ('/opt/', '/etc/', '/var/', '/run/'):
            script = script.replace(prefix, str(self.root) + prefix)
        script = script.replace('sleep 1', 'sleep 0').replace('attempt<30', 'attempt<2')
        self.script = self.root / 'deploy.sh'
        self.script.write_text(script)
        self.mock('systemctl', '''
case "$1" in
show) exit 0;;
is-active) test -f "$TEST_ROOT/active";;
is-enabled) test -f "$TEST_ROOT/enabled";;
stop) rm -f "$TEST_ROOT/active";;
start|restart) touch "$TEST_ROOT/active";;
enable) touch "$TEST_ROOT/enabled";;
disable) rm -f "$TEST_ROOT/enabled";;
*) exit 0;;
esac
''')
        self.mock('ss', 'test ! -f "$TEST_ROOT/occupied" || echo LISTEN\nexit 0')
        self.mock('timeout', 'test ! -f "$TEST_ROOT/unhealthy"')
        # Keep the real GNU install but use the test user's ownership.
        self.mock('install', '''
args=()
while (($#)); do
  case "$1" in -o|-g) shift 2;; *) args+=("$1"); shift;; esac
done
exec /usr/bin/install "${args[@]}"
''')
        self.mock('getent', 'exit 0')
        self.mock('id', 'echo 1000')
        self.env = dict(os.environ, TEST_ROOT=str(self.root))
        self.release(b'first')

    def mock(self, name, body):
        path = self.root / 'bin' / name
        path.write_text('#!/bin/bash\n' + body + '\n')
        path.chmod(0o755)

    def release(self, suffix):
        data = b'\x7fELF' + bytes(14) + b'\x3e\x00' + suffix
        (self.root / 'release/idata-server-linux-amd64').write_bytes(data)
        self.manifest = self.root / 'release/SHA256SUMS'
        self.manifest.write_text(hashlib.sha256(data).hexdigest() + '  idata-server-linux-amd64\n')
        return data

    def run_deploy(self, success=True):
        result = subprocess.run(['bash', str(self.script), str(self.root / 'release')],
                                env=self.env, text=True, capture_output=True)
        self.assertEqual(result.returncode == 0, success, result.stdout + result.stderr)
        return result

    def test_fresh_and_repeat_preserve_credentials(self):
        self.run_deploy()
        config = self.root / 'etc/idata/idata-server.env'
        original = config.read_text()
        self.assertIn('IDATA_LISTEN_ADDR=:12345', original)
        self.assertEqual(config.stat().st_mode & 0o777, 0o640)
        state = self.root / 'var/lib/idata'
        state.mkdir(parents=True)
        credentials = state / 'device-credentials.json'
        credentials.write_text('existing device credentials')
        config.write_text(original.replace(':12345', ':80') + 'IDATA_COMMAND_TIMEOUT=50s\n')
        new = self.release(b'second')
        self.run_deploy()
        self.assertEqual((self.root / 'opt/idata/idata-server').read_bytes(), new)
        self.assertIn(original.split('IDATA_ADMIN_TOKEN=')[1].splitlines()[0], config.read_text())
        self.assertIn('IDATA_COMMAND_TIMEOUT=50s', config.read_text())
        self.assertEqual(config.read_text().count('IDATA_LISTEN_ADDR='), 1)
        self.assertEqual(credentials.read_text(), 'existing device credentials')
        self.assertTrue((self.root / 'enabled').exists())

    def test_bad_checksum_does_not_stop_service(self):
        self.run_deploy()
        old = (self.root / 'opt/idata/idata-server').read_bytes()
        (self.root / 'release/idata-server-linux-amd64').write_bytes(b'corrupt')
        self.run_deploy(False)
        self.assertEqual((self.root / 'opt/idata/idata-server').read_bytes(), old)
        self.assertTrue((self.root / 'active').exists())

    def test_missing_or_duplicate_manifest_entry(self):
        for content in ('a' * 64 + '  another-file\n', self.manifest.read_text() * 2):
            self.manifest.write_text(content)
            self.run_deploy(False)
            self.assertFalse((self.root / 'opt/idata').exists())

    def test_failed_upgrade_restores_previous_install(self):
        self.run_deploy()
        paths = ['opt/idata/idata-server', 'etc/idata/idata-server.env',
                 'etc/systemd/system/idata-server.service']
        previous = {name: (self.root / name).read_bytes() for name in paths}
        self.release(b'broken-new')
        (self.root / 'unhealthy').touch()
        self.run_deploy(False)
        for name, data in previous.items():
            self.assertEqual((self.root / name).read_bytes(), data)
        self.assertTrue((self.root / 'active').exists())
        self.assertTrue((self.root / 'enabled').exists())

    def test_failed_fresh_install_removes_new_service(self):
        (self.root / 'unhealthy').touch()
        self.run_deploy(False)
        self.assertFalse((self.root / 'etc/systemd/system/idata-server.service').exists())
        self.assertFalse((self.root / 'active').exists())
        self.assertFalse((self.root / 'enabled').exists())

    def test_custom_systemd_override_is_rejected(self):
        self.mock('systemctl', 'echo /etc/systemd/system/idata-server.service.d/custom.conf')
        self.run_deploy(False)
        self.assertFalse((self.root / 'opt/idata').exists())

    def test_wrong_architecture_is_rejected(self):
        data = b'\x7fELF' + bytes(14) + b'\xb7\x00'
        (self.root / 'release/idata-server-linux-amd64').write_bytes(data)
        self.manifest.write_text(hashlib.sha256(data).hexdigest() + '  idata-server-linux-amd64\n')
        self.run_deploy(False)
        self.assertFalse((self.root / 'opt/idata').exists())

    def test_occupied_port_restores_service(self):
        self.run_deploy()
        (self.root / 'occupied').touch()
        self.run_deploy(False)
        self.assertTrue((self.root / 'active').exists())


class HealthCheckTest(unittest.TestCase):
    def test_real_http_response(self):
        class Handler(http.server.BaseHTTPRequestHandler):
            status = 200
            body = b'{"status":"ok"}'

            def do_GET(self):
                self.send_response(self.status)
                self.end_headers()
                self.wfile.write(self.body)

            def log_message(self, *args):
                pass

        server = http.server.HTTPServer(('127.0.0.1', 0), Handler)
        thread = threading.Thread(target=server.serve_forever, daemon=True)
        thread.start()
        try:
            code = SCRIPT.read_text().split("timeout 2 bash -c '", 1)[1].split("' 2>/dev/null", 1)[0]
            code = code.replace('/12345', '/' + str(server.server_port))
            for status, body, expected in ((200, b'{"status":"ok"}', 0),
                                           (500, b'{"status":"ok"}', 1),
                                           (200, b'wrong service', 1)):
                Handler.status, Handler.body = status, body
                result = subprocess.run(['bash', '-c', code], timeout=3)
                self.assertEqual(result.returncode, expected)
        finally:
            server.shutdown()
            server.server_close()
            thread.join()


if __name__ == '__main__':
    unittest.main()
