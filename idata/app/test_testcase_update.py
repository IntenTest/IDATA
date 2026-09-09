import importlib.util
import io
import tarfile
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

spec = importlib.util.spec_from_file_location('worker', Path(__file__).with_name('start.py'))
worker = importlib.util.module_from_spec(spec)
spec.loader.exec_module(worker)


class ArchiveUpdateTests(unittest.TestCase):
    def test_install_and_failed_update_preserves_library(self):
        with tempfile.TemporaryDirectory() as temporary:
            home = Path(temporary)
            settings_path = home / 'settings.json'
            with patch.object(worker.Path, 'home', return_value=home), patch.object(worker, 'SETTINGS_PATH', settings_path):
                worker.write_settings(worker.DEFAULT_SETTINGS)
                def download(command, **kwargs):
                    destination = command[command.index('--output') + 1]
                    with tarfile.open(destination, 'w:gz') as archive:
                        entries = {'Testcases/中英文映射.csv': '模块_名称,模块_编号,应用_名称,应用_编号,用例_名称,用例_编号\n模块,M,应用,A,示例,TC001\n', 'Testcases/TC001.py': 'print(1)', 'Testcases/run_testcase.py': 'print(2)'}
                        for name, value in entries.items():
                            data = value.encode('utf-8')
                            info = tarfile.TarInfo(name)
                            info.size = len(data)
                            archive.addfile(info, io.BytesIO(data))
                    return type('Result', (), {'returncode': 0})()
                with patch.object(worker.subprocess, 'run', side_effect=download):
                    worker.install_test_case_archive(worker.read_settings())
                self.assertEqual(worker.TEST_CASE_UPDATE['status'], 'complete', worker.TEST_CASE_UPDATE)
                library = home / '.idata/newest_testcases'
                self.assertTrue((library / 'TC001.py').is_file())
                self.assertEqual(len(worker.discover_test_cases()['testCases']), 1)
                self.assertEqual(worker.read_settings()['testCaseLibraryPath'], str(library))
                with patch.object(worker.subprocess, 'run', side_effect=OSError('Offline')):
                    worker.install_test_case_archive(worker.read_settings())
                self.assertEqual(worker.TEST_CASE_UPDATE['status'], 'failed')
                self.assertTrue((library / 'TC001.py').is_file())
                def unsafe(command, **kwargs):
                    with tarfile.open(command[command.index('--output') + 1], 'w:gz') as archive:
                        info = tarfile.TarInfo('../escape')
                        archive.addfile(info, io.BytesIO())
                    return type('Result', (), {'returncode': 0})()
                with patch.object(worker.subprocess, 'run', side_effect=unsafe):
                    worker.install_test_case_archive(worker.read_settings())
                self.assertEqual(worker.TEST_CASE_UPDATE['status'], 'failed')
                self.assertFalse((home / 'escape').exists())
                self.assertTrue((library / 'TC001.py').is_file())
                with patch.object(worker.subprocess, 'run', side_effect=download):
                    worker.install_test_case_archive(worker.read_settings())
                self.assertEqual(worker.TEST_CASE_UPDATE['status'], 'complete')
                self.assertFalse(list((home / '.idata').glob('.testcases-*')))

if __name__ == '__main__':
    unittest.main()
