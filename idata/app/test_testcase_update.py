import importlib.util
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

spec = importlib.util.spec_from_file_location('worker', Path(__file__).with_name('start.py'))
worker = importlib.util.module_from_spec(spec)
spec.loader.exec_module(worker)


class RepositoryUpdateTests(unittest.TestCase):
    def test_idata_executable_defaults_to_client_directory(self):
        with tempfile.TemporaryDirectory() as temporary:
            client_directory = Path(temporary) / 'installed client'
            environment = {
                worker.IDATA_CLIENT_EXECUTABLE_DIRECTORY_ENVIRONMENT_VARIABLE:
                    str(client_directory),
            }
            with patch.dict(worker.os.environ, environment):
                expected = (client_directory / 'IDATA.exe').resolve()
                self.assertEqual(worker.configured_idata_path('IDATA.exe'), expected)
                self.assertEqual(worker.configured_idata_path('../IDATA.exe'), expected)

    def test_legacy_idata_default_is_migrated(self):
        settings = worker.normalize_settings({'idataExecutablePath': '../IDATA.exe'})
        self.assertEqual(settings['idataExecutablePath'], 'IDATA.exe')

    def test_update_clones_release_branch_and_preserves_library_on_failure(self):
        with tempfile.TemporaryDirectory() as temporary:
            home = Path(temporary)
            settings_path = home / 'settings.json'
            with patch.object(worker.Path, 'home', return_value=home), patch.object(worker, 'SETTINGS_PATH', settings_path):
                worker.write_settings(worker.DEFAULT_SETTINGS)
                commands = []

                def clone(command, **kwargs):
                    commands.append(command)
                    repository = Path(command[-1])
                    repository.mkdir()
                    (repository / '中英文映射.csv').write_text(
                        '模块_名称,模块_编号,应用_名称,应用_编号,用例_名称,用例_编号\n'
                        '模块,M,应用,A,示例,TC001\n',
                        encoding='utf-8',
                    )
                    (repository / 'TC001.py').write_text('print(1)', encoding='utf-8')
                    (repository / 'run_testcase.py').write_text('print(2)', encoding='utf-8')
                    return type('Result', (), {'returncode': 0, 'stdout': '', 'stderr': ''})()

                with patch.object(worker.subprocess, 'run', side_effect=clone):
                    worker.install_test_case_repository(worker.read_settings())
                self.assertEqual(worker.TEST_CASE_UPDATE['status'], 'complete', worker.TEST_CASE_UPDATE)
                self.assertEqual(
                    commands[0][0:7],
                    ['git', 'clone', '--branch', 'release_Idata', '--single-branch', '--depth', '1'],
                )
                library = home / '.idata/newest_testcases'
                self.assertTrue((library / 'TC001.py').is_file())
                self.assertEqual(len(worker.discover_test_cases()['testCases']), 1)
                self.assertEqual(worker.read_settings()['testCaseLibraryPath'], str(library))
                with patch.object(worker.subprocess, 'run', side_effect=OSError('Offline')):
                    worker.install_test_case_repository(worker.read_settings())
                self.assertEqual(worker.TEST_CASE_UPDATE['status'], 'failed')
                self.assertTrue((library / 'TC001.py').is_file())
                with patch.object(worker.subprocess, 'run', side_effect=clone):
                    worker.install_test_case_repository(worker.read_settings())
                self.assertEqual(worker.TEST_CASE_UPDATE['status'], 'complete')
                self.assertFalse(list((home / '.idata').glob('.testcases-*')))

    def test_manual_update_command_names_release_branch(self):
        command = worker.test_case_update_command(worker.DEFAULT_SETTINGS)
        self.assertTrue(command.endswith('pull --ff-only origin release_Idata'))

if __name__ == '__main__':
    unittest.main()
