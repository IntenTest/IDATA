"""Regression checks for client test command construction."""
import subprocess
from pathlib import Path
from types import SimpleNamespace
import unittest
from unittest.mock import patch

import test_commands as commands


class CommandTests(unittest.TestCase):
    def test_runner_arguments_preserve_spaces_and_unicode(self):
        self.assertEqual(commands.build_test_command(
            "C:/Program Files/IDATA/IDATA.exe", "C:/Test Library/run_testcase.py",
            "中文 case", 2, "FMR 设备"),
            ["C:/Program Files/IDATA/IDATA.exe", "cli", "bundle", "run",
             "--path", "C:/Test Library/run_testcase.py", "--", "中文 case", "2", "--sn", "FMR 设备"])

    def test_launch_on_both_platforms(self):
        test = ["C:/Program Files/IDATA/IDATA.exe", "cli", "bundle", "run",
                "--path", "runner.py", "--", "中文 case", "0"]
        worker = ["python.exe", "worker.py", "run.log", "status.json", "--", *test]
        for platform in ("nt", "posix"):
            with self.subTest(platform=platform), patch.object(
                commands, "os", SimpleNamespace(name=platform, environ={"SystemRoot": "D:/Windows"})
            ), patch.object(subprocess, "CREATE_NEW_CONSOLE", 16, create=True):
                command, options = commands.build_launch_command(
                    test, "python.exe", "worker.py", "run.log", "status.json",
                    Path("library"))
                if platform == "nt":
                    self.assertEqual(command, [str(Path("D:/Windows/System32/cmd.exe")),
                        "/d", "/k", 'chcp 65001 >nul & set "PYTHONUTF8=1" & '
                        + subprocess.list2cmdline(worker)])
                    self.assertEqual(options, {"cwd": Path("library"), "creationflags": 16})
                else:
                    self.assertEqual(command, worker)
                    self.assertEqual(options, {"cwd": Path("library"), "start_new_session": True})


if __name__ == "__main__":
    unittest.main()
