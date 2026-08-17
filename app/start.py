#!/usr/bin/env python3
"""Serve the local application on its only supported port."""

from datetime import datetime
from functools import partial
from http.server import SimpleHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from threading import Lock, Thread
from urllib.parse import unquote, urlparse
import ast
import csv
import json
import os
import re
import socket
import subprocess
import sys
import tempfile
import time
import webbrowser


HOST = "127.0.0.1"
PORT = 54321
APP_DIRECTORY = Path(__file__).resolve().parent
PROJECT_DIRECTORY = APP_DIRECTORY.parent
VENDOR_DIRECTORY = APP_DIRECTORY.parent / "vendor"
SETTINGS_PATH = APP_DIRECTORY / "config" / "settings.json"
MODEL_CONFIG_PATH = (
    APP_DIRECTORY / ".." / ".." / "Phoebe-main" / "Phoebe" / "tools" / "llm_analyzer.py"
).resolve()
PID_PATH = APP_DIRECTORY / ".ohwemby.pid"
VENDOR_PACKAGES = frozenset(("vue-3.5.24", "element-plus-2.11.8"))
HDC_TIMEOUT_SECONDS = 10
GIT_SYNC_TIMEOUT_SECONDS = 120
NETWORK_ZONE_PROBE_HOST = "10.90.65.189"
NETWORK_ZONE_PROBE_TIMEOUT_SECONDS = 3
DEFAULT_TEST_CASE_REPOSITORY_URL = (
    "https://codehub-dg-y.huawei.com/k30030842/Testcases.git"
)
FALLBACK_TEST_CASE_REPOSITORY_URL = "https://github.com/IntenTest/Testcases.git"
TEST_RUNS = {}
TEST_RUNS_LOCK = Lock()
TEST_RUN_LOG_DIRECTORY = APP_DIRECTORY / "logs" / "test-runs"
TEST_PROCESS_RUNNER = APP_DIRECTORY / "run_test_process.py"
DEFAULT_SETTINGS = {
    "projectName": "UI自动化测试平台",
    "releaseName": "FangTian 1.10-1.12",
    "defaultEnvironment": "HarmonyOS",
    "defaultOwner": "kouyanan 30030842",
    "testCaseRepositoryUrl": DEFAULT_TEST_CASE_REPOSITORY_URL,
    "testCaseLibraryPath": "../Phoebe-main",
    "pythonExecutablePath": "../python310/python.exe",
    "runTestCasesPath": "../Phoebe-main/run_testcase.py",
    "autoLoadDevices": True,
    "deviceRefreshSeconds": 30,
    "tablePageSize": 20,
}
SETTING_FIELD_TYPES = {
    "projectName": str,
    "releaseName": str,
    "defaultEnvironment": str,
    "defaultOwner": str,
    "testCaseRepositoryUrl": str,
    "testCaseLibraryPath": str,
    "pythonExecutablePath": str,
    "runTestCasesPath": str,
    "autoLoadDevices": bool,
    "deviceRefreshSeconds": int,
    "tablePageSize": int,
}
DEVICE_PARAMETER_KEYS = (
    ("model", "const.product.model"),
    ("name", "const.product.name"),
    ("osVersion", "const.product.os.dist.version"),
    ("deviceType", "const.product.devicetype"),
)


def discover_hdc_devices() -> dict:
    """Return connected HDC targets with useful system information."""
    try:
        result = subprocess.run(
            ["hdc", "list", "targets", "-v"],
            capture_output=True,
            check=False,
            text=True,
            timeout=HDC_TIMEOUT_SECONDS,
        )
    except FileNotFoundError:
        return {
            "devices": [],
            "error": "HDC was not found. Install HDC and ensure it is available on PATH.",
        }
    except subprocess.TimeoutExpired:
        return {
            "devices": [],
            "error": "HDC did not respond within 10 seconds.",
        }
    except OSError as error:
        return {
            "devices": [],
            "error": f"Unable to start HDC: {error}.",
        }

    output = result.stdout.strip()
    error_output = result.stderr.strip()
    if result.returncode != 0:
        detail = error_output or output or f"exit code {result.returncode}"
        return {"devices": [], "error": f"HDC device search failed: {detail}"}

    devices = []
    for line in output.splitlines():
        columns = line.split()
        if not columns or columns[0].lower() in {"[empty]", "empty"}:
            continue

        target_id = columns[0]
        status = columns[2] if len(columns) > 2 else "Connected"
        if status.lower() != "connected":
            continue

        command = "; ".join(
            f"param get {parameter}" for _, parameter in DEVICE_PARAMETER_KEYS
        )
        try:
            details = subprocess.run(
                ["hdc", "-t", target_id, "shell", command],
                capture_output=True,
                check=False,
                text=True,
                timeout=HDC_TIMEOUT_SECONDS,
            )
            values = [value.strip() for value in details.stdout.splitlines()]
        except (OSError, subprocess.TimeoutExpired):
            values = []
        device = {
            "id": target_id,
            "status": status,
        }
        for index, (field, _) in enumerate(DEVICE_PARAMETER_KEYS):
            device[field] = values[index] if index < len(values) else ""
        devices.append(device)

    return {"devices": devices, "error": None}


def read_settings() -> dict:
    if not SETTINGS_PATH.exists():
        write_settings(DEFAULT_SETTINGS)
        return dict(DEFAULT_SETTINGS)

    try:
        with SETTINGS_PATH.open(encoding="utf-8") as config_file:
            loaded = json.load(config_file)
    except (OSError, json.JSONDecodeError) as error:
        raise RuntimeError(f"Unable to read settings config: {error}") from error

    if not isinstance(loaded, dict):
        raise RuntimeError("Settings config must contain a JSON object.")

    return normalize_settings(loaded)


def normalize_settings(raw_settings: dict) -> dict:
    settings = dict(DEFAULT_SETTINGS)
    for key, expected_type in SETTING_FIELD_TYPES.items():
        if key not in raw_settings:
            continue
        value = raw_settings[key]
        if expected_type is bool:
            if not isinstance(value, bool):
                raise RuntimeError(f"{key} must be true or false.")
        elif expected_type is int:
            if not isinstance(value, int) or isinstance(value, bool):
                raise RuntimeError(f"{key} must be an integer.")
            if key in {"deviceRefreshSeconds", "tablePageSize"} and value < 1:
                raise RuntimeError(f"{key} must be greater than zero.")
        elif not isinstance(value, expected_type):
            raise RuntimeError(f"{key} must be a string.")
        settings[key] = value
    return settings


def write_settings(settings: dict) -> None:
    SETTINGS_PATH.parent.mkdir(parents=True, exist_ok=True)
    payload = json.dumps(settings, indent=2, ensure_ascii=False).encode("utf-8")
    payload += b"\n"

    with tempfile.NamedTemporaryFile(
        "wb",
        delete=False,
        dir=str(SETTINGS_PATH.parent),
        prefix=".settings.",
    ) as temp_file:
        temp_file.write(payload)
        temp_name = temp_file.name

    Path(temp_name).replace(SETTINGS_PATH)


def model_config_assignment(source: str) -> tuple[ast.Assign, dict]:
    """Return the MODEL_CONFIG assignment and its literal dictionary value."""
    try:
        module = ast.parse(source, filename=str(MODEL_CONFIG_PATH))
    except SyntaxError as error:
        raise RuntimeError(f"Unable to parse the model configuration file: {error}") from error

    for node in module.body:
        if not isinstance(node, ast.Assign):
            continue
        if not any(
            isinstance(target, ast.Name) and target.id == "MODEL_CONFIG"
            for target in node.targets
        ):
            continue
        try:
            value = ast.literal_eval(node.value)
        except (ValueError, TypeError, SyntaxError) as error:
            raise RuntimeError("MODEL_CONFIG must be a Python dictionary literal.") from error
        if not isinstance(value, dict):
            raise RuntimeError("MODEL_CONFIG must be a Python dictionary literal.")
        return node, value

    raise RuntimeError("MODEL_CONFIG was not found in the model configuration file.")


def read_model_config() -> dict:
    if not MODEL_CONFIG_PATH.is_file():
        raise RuntimeError(f"Model configuration file was not found: {MODEL_CONFIG_PATH}")
    try:
        source = MODEL_CONFIG_PATH.read_text(encoding="utf-8")
    except OSError as error:
        raise RuntimeError(f"Unable to read the model configuration file: {error}") from error

    _, config = model_config_assignment(source)
    return {
        "api_base": str(config.get("api_base", "")),
        "api_key": str(config.get("api_key", "")),
        "model_name": str(config.get("model_name", "")),
    }


def write_model_config(incoming_config: dict) -> dict:
    required_keys = ("api_base", "api_key", "model_name")
    for key in required_keys:
        if not isinstance(incoming_config.get(key), str):
            raise RuntimeError(f"{key} must be a string.")

    try:
        source = MODEL_CONFIG_PATH.read_text(encoding="utf-8")
    except OSError as error:
        raise RuntimeError(f"Unable to read the model configuration file: {error}") from error

    assignment, current_config = model_config_assignment(source)
    current_config.update({key: incoming_config[key] for key in required_keys})
    replacement = "MODEL_CONFIG = " + repr(current_config)
    source_lines = source.splitlines(keepends=True)
    start_offset = sum(len(line) for line in source_lines[: assignment.lineno - 1]) + assignment.col_offset
    end_offset = sum(len(line) for line in source_lines[: assignment.end_lineno - 1]) + assignment.end_col_offset
    updated_source = source[:start_offset] + replacement + source[end_offset:]

    try:
        with tempfile.NamedTemporaryFile(
            "w",
            delete=False,
            dir=str(MODEL_CONFIG_PATH.parent),
            prefix=".llm_analyzer.",
            encoding="utf-8",
            newline="",
        ) as temp_file:
            temp_file.write(updated_source)
            temp_name = temp_file.name
        Path(temp_name).replace(MODEL_CONFIG_PATH)
    except OSError as error:
        raise RuntimeError(f"Unable to save the model configuration file: {error}") from error

    return {key: incoming_config[key] for key in required_keys}


def configured_path(raw_path: str) -> Path:
    path = Path(raw_path).expanduser()
    if not path.is_absolute():
        path = PROJECT_DIRECTORY / path
    return path.resolve()


def detect_network_zone() -> str:
    command = (
        ["ping", "-n", "1", "-w", "3000", NETWORK_ZONE_PROBE_HOST]
        if os.name == "nt"
        else ["ping", "-c", "1", NETWORK_ZONE_PROBE_HOST]
    )
    try:
        result = subprocess.run(
            command,
            capture_output=True,
            check=False,
            timeout=NETWORK_ZONE_PROBE_TIMEOUT_SECONDS,
        )
    except (FileNotFoundError, OSError, subprocess.TimeoutExpired):
        return "blue"
    return "yellow" if result.returncode == 0 else "blue"


def apply_network_zone_repository() -> tuple[dict, str]:
    zone = detect_network_zone()
    repository_url = (
        DEFAULT_TEST_CASE_REPOSITORY_URL
        if zone == "yellow"
        else FALLBACK_TEST_CASE_REPOSITORY_URL
    )
    settings = read_settings()
    if settings["testCaseRepositoryUrl"] != repository_url:
        settings["testCaseRepositoryUrl"] = repository_url
        write_settings(settings)
    return settings, zone


def run_git(command: list[str], *, cwd: Path | None = None) -> str:
    try:
        result = subprocess.run(
            ["git", *command],
            cwd=cwd,
            capture_output=True,
            check=False,
            text=True,
            timeout=GIT_SYNC_TIMEOUT_SECONDS,
        )
    except FileNotFoundError as error:
        raise RuntimeError("Git was not found. Install Git and ensure it is available on PATH.") from error
    except subprocess.TimeoutExpired as error:
        raise RuntimeError("The test case repository update timed out after 120 seconds.") from error
    except OSError as error:
        raise RuntimeError(f"Unable to run Git: {error}") from error

    if result.returncode != 0:
        detail = result.stderr.strip() or result.stdout.strip() or f"exit code {result.returncode}"
        raise RuntimeError(f"Unable to update the test case repository: {detail}")
    return result.stdout.strip()


def sync_test_case_repository(settings: dict | None = None) -> dict:
    settings = settings or read_settings()
    repository_url = settings["testCaseRepositoryUrl"].strip()
    raw_library_path = settings["testCaseLibraryPath"].strip()
    if not repository_url:
        raise RuntimeError("Set the test case repository URL in Settings.")
    if not raw_library_path:
        raise RuntimeError("Set the test case library path in Settings.")

    library_path = configured_path(raw_library_path)
    git_directory = library_path / ".git"
    if library_path.exists() and not git_directory.is_dir():
        if any(library_path.iterdir()):
            raise RuntimeError(
                f"The test case library path exists but is not a Git repository: {library_path}"
            )
    repository_urls = [repository_url]
    errors = []
    output = ""
    selected_url = ""

    if git_directory.is_dir():
        original_url = run_git(["remote", "get-url", "origin"], cwd=library_path)
        for candidate_url in repository_urls:
            try:
                run_git(["remote", "set-url", "origin", candidate_url], cwd=library_path)
                output = run_git(["pull", "--ff-only"], cwd=library_path)
                selected_url = candidate_url
                break
            except RuntimeError as error:
                errors.append(str(error))
        if not selected_url:
            run_git(["remote", "set-url", "origin", original_url], cwd=library_path)
        action = "updated"
    else:
        library_path.parent.mkdir(parents=True, exist_ok=True)
        for candidate_url in repository_urls:
            try:
                with tempfile.TemporaryDirectory(
                    dir=library_path.parent,
                    prefix=".testcases-clone-",
                ) as temp_directory:
                    clone_path = Path(temp_directory) / "repository"
                    run_git(["clone", "--", candidate_url, str(clone_path)])
                    if library_path.is_dir():
                        library_path.rmdir()
                    clone_path.replace(library_path)
                output = "Repository cloned."
                selected_url = candidate_url
                break
            except (RuntimeError, OSError) as error:
                errors.append(str(error))
        action = "cloned"

    if not selected_url:
        raise RuntimeError(errors[-1] if errors else "Unable to update the test case repository.")

    return {
        "action": action,
        "message": output or "The test case repository is up to date.",
        "path": str(library_path),
        "repositoryUrl": selected_url,
        "usedFallback": False,
    }


def read_test_case_mapping(library_path: Path) -> tuple[dict[str, dict], Path]:
    required_columns = {
        "模块_名称",
        "模块_编号",
        "应用_名称",
        "应用_编号",
        "用例_名称",
        "用例_编号",
    }
    mapping_path = library_path / "中英文映射.csv"
    if not mapping_path.is_file():
        raise RuntimeError(f"The test case mapping CSV was not found: {mapping_path}")
    try:
        with mapping_path.open(encoding="utf-8-sig", newline="") as mapping_file:
            reader = csv.DictReader(mapping_file)
            if not required_columns.issubset(reader.fieldnames or []):
                raise RuntimeError(
                    f"The test case mapping CSV does not contain the required columns: {mapping_path}"
                )
            mapping = {}
            for row in reader:
                case_number = (row.get("用例_编号") or "").strip()
                if not case_number or case_number in mapping:
                    continue
                mapping[case_number] = {
                    "moduleName": (row.get("模块_名称") or "").strip(),
                    "moduleCode": (row.get("模块_编号") or "").strip(),
                    "applicationName": (row.get("应用_名称") or "").strip(),
                    "applicationCode": (row.get("应用_编号") or "").strip(),
                    "mappedCaseName": (row.get("用例_名称") or "").strip(),
                }
            return mapping, mapping_path
    except (OSError, UnicodeError, csv.Error) as error:
        raise RuntimeError(f"Unable to read the test case mapping CSV: {error}") from error


def discover_test_cases(settings: dict | None = None) -> dict:
    settings = settings or read_settings()
    raw_library_path = settings["testCaseLibraryPath"].strip()
    if not raw_library_path:
        return {
            "testCases": [],
            "error": "Set the test case library path in Settings.",
        }

    library_path = configured_path(raw_library_path)
    if not library_path.is_dir():
        return {
            "testCases": [],
            "error": f"Test case library directory was not found: {library_path}",
        }

    test_case_paths = sorted(
        path
        for path in library_path.rglob("*")
        if path.is_file()
        and path.suffix.lower() == ".py"
        and path.name.lower() != "__init__.py"
    )
    mapping, mapping_path = read_test_case_mapping(library_path)
    paths_by_case_number = {
        test_case_path.stem: test_case_path for test_case_path in test_case_paths
    }
    test_cases = []
    for case_number, mapping_entry in mapping.items():
        test_case_path = paths_by_case_number.get(case_number)
        if test_case_path is None:
            continue
        relative_name = (
            test_case_path.relative_to(library_path).with_suffix("").as_posix()
        )
        modified_time = datetime.fromtimestamp(
            test_case_path.stat().st_mtime
        ).astimezone()
        test_cases.append(
            {
                "id": str(len(test_cases) + 1),
                "title": mapping_entry["mappedCaseName"],
                "executionName": case_number,
                "path": relative_name,
                "moduleName": mapping_entry["moduleName"],
                "moduleCode": mapping_entry["moduleCode"],
                "applicationName": mapping_entry["applicationName"],
                "applicationCode": mapping_entry["applicationCode"],
                "mappedCaseName": mapping_entry["mappedCaseName"],
                "category": "Standard",
                "status": "Not run",
                "owner": settings["defaultOwner"],
                "updated": modified_time.isoformat(timespec="seconds"),
            }
        )

    discovered_names = {test_case_path.stem for test_case_path in test_case_paths}
    discrepancies = [
        {
            "caseName": case_name,
            "inMapping": case_name in mapping,
            "hasFile": case_name in discovered_names,
        }
        for case_name in sorted(discovered_names.symmetric_difference(mapping))
    ]
    return {
        "testCases": test_cases,
        "mappingValidation": {
            "mappingFile": mapping_path.relative_to(library_path).as_posix(),
            "discrepancies": discrepancies,
        },
        "error": None,
    }


def start_test_cases(request_body: dict) -> dict:
    raw_case_names = request_body.get("testCases")
    inspection_mode = request_body.get("inspectionMode")
    run_name = request_body.get("name")
    device = request_body.get("device")
    if (
        not isinstance(raw_case_names, list)
        or not raw_case_names
        or not all(isinstance(name, str) and name for name in raw_case_names)
    ):
        raise RuntimeError("Select at least one test case.")
    if inspection_mode not in (0, 1, 2):
        raise RuntimeError("Inspection mode must be 0, 1, or 2.")
    if not isinstance(run_name, str) or not run_name.strip():
        raise RuntimeError("Enter a test run name.")
    if not isinstance(device, str) or not device.strip():
        raise RuntimeError("Select a device.")

    settings = read_settings()
    raw_library_path = settings["testCaseLibraryPath"].strip()
    raw_python_path = settings["pythonExecutablePath"].strip()
    raw_runner_path = settings["runTestCasesPath"].strip()
    if not raw_library_path:
        raise RuntimeError("Set the test case library path in Settings.")
    if not raw_python_path:
        raise RuntimeError("Set the Python executable path in Settings.")
    if not raw_runner_path:
        raise RuntimeError("Set the run_testcases path in Settings.")

    library_path = configured_path(raw_library_path)
    python_path = configured_path(raw_python_path)
    runner_path = configured_path(raw_runner_path)
    if not library_path.is_dir():
        raise RuntimeError(f"Test case library directory was not found: {library_path}")
    if not python_path.is_file():
        raise RuntimeError(f"Python executable was not found: {python_path}")
    if python_path.name.lower() != "python.exe":
        raise RuntimeError("Python executable path must end with python.exe.")
    if not runner_path.is_file():
        raise RuntimeError(f"run_testcases file was not found: {runner_path}")

    discovered_cases = {
        test_case["id"]: test_case
        for test_case in discover_test_cases(settings)["testCases"]
    }
    case_ids = list(dict.fromkeys(raw_case_names))
    missing_names = [name for name in case_ids if name not in discovered_cases]
    if missing_names:
        names = ", ".join(missing_names)
        raise RuntimeError(f"Unknown test case selection: {names}")

    run_id = f"TR-{int(time.time() * 1000)}"
    processes = []
    for case_id in case_ids:
        test_case = discovered_cases[case_id]
        case_name = test_case.get("executionName", test_case["title"])
        test_command = [
            str(python_path),
            str(runner_path),
            case_name,
            str(inspection_mode),
        ]
        log_path = TEST_RUN_LOG_DIRECTORY / f"{run_id}-{case_id}.log"
        status_path = TEST_RUN_LOG_DIRECTORY / f"{run_id}-{case_id}.status.json"
        worker_command = [
            str(python_path),
            str(TEST_PROCESS_RUNNER),
            str(log_path),
            str(status_path),
            "--",
            *test_command,
        ]
        popen_options = {
            "cwd": library_path,
        }
        if os.name == "nt":
            system_root = Path(os.environ.get("SystemRoot", r"C:\Windows"))
            cmd_path = system_root / "System32" / "cmd.exe"
            cmd_command = (
                "chcp 65001 >nul"
                ' & set "PYTHONUTF8=1"'
                f" & {subprocess.list2cmdline(worker_command)}"
            )
            command = [
                str(cmd_path),
                "/d",
                "/k",
                cmd_command,
            ]
            popen_options["creationflags"] = subprocess.CREATE_NEW_CONSOLE
        else:
            command = worker_command
        display_command = subprocess.list2cmdline(test_command)
        processes.append(
            {
                "testCase": case_id,
                "testCaseName": case_name,
                "inspectionMode": inspection_mode,
                "processId": None,
                "command": display_command,
                "_logPath": log_path,
                "_statusPath": status_path,
                "_command": command,
                "_popenOptions": popen_options,
                "_state": "Pending",
            }
        )

    started_at = datetime.now().astimezone()
    run = {
        "id": run_id,
        "title": run_name.strip(),
        "device": device.strip(),
        "inspectionMode": inspection_mode,
        "startedAt": started_at.isoformat(timespec="seconds"),
        "libraryPath": library_path,
        "processes": processes,
    }
    with TEST_RUNS_LOCK:
        TEST_RUNS[run_id] = run
    Thread(
        target=execute_test_run_sequentially,
        args=(run,),
        daemon=True,
        name=f"test-run-{run_id}",
    ).start()
    return serialize_test_run(run)


def execute_test_run_sequentially(run: dict) -> None:
    """Execute every selected case in order, with at most one active process."""
    for process_info in run["processes"]:
        process_info["_state"] = "Running"
        print(
            f"[test run: {run['title']}] cwd: {run['libraryPath']}\n"
            f"[test case: {process_info['testCaseName']}] command: "
            f"{process_info['command']}",
            flush=True,
        )
        try:
            process = subprocess.Popen(
                process_info["_command"],
                **process_info["_popenOptions"],
            )
            process_info["_process"] = process
            process_info["processId"] = process.pid
            while not process_info["_statusPath"].exists():
                time.sleep(0.2)
        except OSError as error:
            message = (
                f"Unable to start {process_info['testCaseName']} with command "
                f"{process_info['command']}: {error}\n"
            )
            process_info["_logPath"].write_text(message, encoding="utf-8")
            process_info["_statusPath"].write_text(
                json.dumps({"exitCode": 1}),
                encoding="utf-8",
            )
        finally:
            process_info["_state"] = "Finished"


def console_marker(console_output: str, label: str, expected: str) -> bool:
    pattern = rf"{re.escape(label)}\s*[：:]?\s*{re.escape(expected)}"
    return re.search(pattern, console_output, re.IGNORECASE) is not None


def report_reference(console_output: str) -> tuple[str, str] | None:
    lines = console_output.splitlines()

    # Screenshot inspection prints the complete report path on this line.
    # Prefer it over the older recording output, whose report location is
    # inferred from the lines following "日志存储路径".
    for line in lines:
        match = re.search(r"汇总\s*HTML\s*[：:]\s*(.+?)\s*$", line, re.IGNORECASE)
        if match:
            report_value = match.group(1).strip().strip("\"'")
            if report_value:
                return "", report_value

    for index, line in enumerate(lines):
        match = re.search(r"日志存储路径\s*[：:]\s*(.*)", line)
        if not match:
            continue
        storage_path = match.group(1).strip().strip("\"'")
        for report_line in lines[index + 1 :]:
            report_line = report_line.strip()
            if not report_line:
                continue
            url_match = re.search(r"https?://\S+", report_line)
            if url_match:
                return storage_path, url_match.group(0).rstrip(".,;")
            if re.match(r"^[A-Za-z]:[\\/]", report_line) or report_line.startswith(
                "\\\\"
            ):
                path_value = report_line
            else:
                path_value = re.split(r"[：:]", report_line, maxsplit=1)[-1]
            return storage_path, path_value.strip().strip("\"'")
    return None


def resolve_report(
    run: dict,
    process_info: dict,
    console_output: str,
) -> tuple[str, str] | None:
    process_info.pop("_reportPath", None)
    reference = report_reference(console_output)
    if not reference:
        return None
    storage_path, report_value = reference
    if report_value.startswith(("http://", "https://")):
        return report_value, report_value

    report_path = Path(report_value).expanduser()
    candidates = [report_path]
    if not report_path.is_absolute():
        library_path = run["libraryPath"]
        candidates = [
            library_path / report_path,
            library_path / storage_path / report_path,
        ]
    for candidate in candidates:
        candidate = candidate.resolve()
        if candidate.is_dir():
            candidate = candidate / "index.html"
        if candidate.is_file():
            process_info["_reportPath"] = candidate
            return candidate.as_uri(), str(candidate)
    return None


def evaluate_test_case(
    run: dict,
    process_info: dict,
    console_output: str,
    exit_code: int | None,
) -> dict:
    inspection_mode = process_info["inspectionMode"]
    checks = [
        {
            "label": "Automation result",
            "passed": console_marker(console_output, "【最终结果】", "pass"),
        },
        {
            "label": "Log inspection",
            "passed": console_marker(console_output, "日志检查", "True"),
        },
    ]
    if inspection_mode == 1:
        checks.append(
            {
                "label": "Screenshot inspection",
                "passed": console_marker(console_output, "用例截图检测", "True"),
            }
        )
    elif inspection_mode == 2:
        checks.append(
            {
                "label": "Recording inspection",
                "passed": console_marker(console_output, "用例录屏检测", "True"),
            }
        )

    finished = exit_code is not None
    result = "Running" if not finished else "Passed" if all(
        check["passed"] for check in checks
    ) else "Failed"
    report = resolve_report(run, process_info, console_output)
    return {
        "result": result,
        "checks": checks,
        "reportUrl": report[0] if report else None,
        "reportLocation": report[1] if report else None,
    }


def serialize_test_run(run: dict) -> dict:
    processes = run["processes"]
    unfinished_count = sum(
        1 for process_info in processes
        if process_info.get("_state") in {"Pending", "Running"}
    )
    serialized_processes = []
    combined_output = []
    for process_info in processes:
        process_state = process_info.get("_state", "Finished")
        try:
            console_output = process_info["_logPath"].read_text(
                encoding="utf-8",
                errors="replace",
            )
        except OSError:
            console_output = ""
        exit_code = None
        try:
            exit_code = json.loads(
                process_info["_statusPath"].read_text(encoding="utf-8")
            )["exitCode"]
        except (OSError, json.JSONDecodeError, KeyError, TypeError):
            pass
        evaluation = (
            {
                "result": "Pending",
                "checks": [],
                "reportUrl": None,
                "reportLocation": None,
            }
            if process_state == "Pending"
            else evaluate_test_case(run, process_info, console_output, exit_code)
        )
        combined_output.append(
            f"===== {process_info['testCaseName']} =====\n{console_output}"
        )
        serialized_processes.append(
            {
                key: value
                for key, value in process_info.items()
                if not key.startswith("_")
            }
            | {
                "consoleOutput": console_output,
                "exitCode": exit_code,
                **evaluation,
            }
        )
    finished_processes = [
        process_info
        for process_info in serialized_processes
        if process_info["result"] != "Running"
    ]
    passed_count = sum(
        1 for process_info in finished_processes if process_info["result"] == "Passed"
    )
    failed_count = sum(
        1 for process_info in finished_processes if process_info["result"] == "Failed"
    )
    total_processes = len(processes)
    return {
        "id": run["id"],
        "title": run["title"],
        "device": run["device"],
        "inspectionMode": run["inspectionMode"],
        "startedAt": run["startedAt"],
        "status": (
            "Running"
            if unfinished_count
            else "Failed"
            if failed_count
            else "Completed"
        ),
        "runningProcesses": unfinished_count,
        "totalProcesses": total_processes,
        "executedProcesses": len(finished_processes),
        "passedProcesses": passed_count,
        "failedProcesses": failed_count,
        "progress": (
            round(len(finished_processes) / total_processes * 100)
            if total_processes
            else 0
        ),
        "consoleOutput": "\n\n".join(combined_output),
        "started": serialized_processes,
    }


def list_test_runs() -> dict:
    with TEST_RUNS_LOCK:
        runs = [serialize_test_run(run) for run in TEST_RUNS.values()]
    runs.sort(key=lambda run: run["startedAt"], reverse=True)
    return {"testRuns": runs}


def send_json(handler: SimpleHTTPRequestHandler, status: int, value: dict) -> None:
    payload = json.dumps(value, ensure_ascii=False).encode("utf-8")
    handler.send_response(status)
    handler.send_header("Content-Type", "application/json; charset=utf-8")
    handler.send_header("Content-Length", str(len(payload)))
    handler.send_header("Cache-Control", "no-store")
    handler.end_headers()
    handler.wfile.write(payload)


def open_test_report(run_id: str, case_id: str) -> str:
    with TEST_RUNS_LOCK:
        run = TEST_RUNS.get(run_id)
        if run is None:
            raise RuntimeError("Test run was not found.")
        serialize_test_run(run)
        process_info = next(
            (
                item
                for item in run["processes"]
                if item["testCase"] == case_id
            ),
            None,
        )
        report_path = (
            process_info.get("_reportPath") if process_info is not None else None
        )
    if report_path is None or not report_path.is_file():
        raise RuntimeError("Test report was not found.")
    report_uri = report_path.as_uri()
    if os.name == "nt":
        os.startfile(str(report_path))
    elif not webbrowser.open(report_uri):
        raise RuntimeError("Unable to open the test report in a browser.")
    return report_uri


class AppRequestHandler(SimpleHTTPRequestHandler):
    network_zone = "blue"

    def do_GET(self) -> None:
        request_path = urlparse(self.path).path
        if request_path == "/api/devices":
            send_json(self, 200, discover_hdc_devices())
            return

        if request_path == "/api/test-cases":
            try:
                result = discover_test_cases()
                status = 200
            except RuntimeError as error:
                result = {"testCases": [], "error": str(error)}
                status = 500
            send_json(self, status, result)
            return

        if request_path == "/api/test-runs":
            send_json(self, 200, list_test_runs())
            return

        if request_path == "/api/settings":
            try:
                result = {
                    "settings": read_settings(),
                    "networkZone": self.network_zone,
                }
                status = 200
            except RuntimeError as error:
                result = {"error": str(error)}
                status = 500
            send_json(self, status, result)
            return

        if request_path == "/api/model-config":
            try:
                result = {"modelConfig": read_model_config()}
                status = 200
            except RuntimeError as error:
                result = {"error": str(error)}
                status = 500
            send_json(self, status, result)
            return

        super().do_GET()

    def do_POST(self) -> None:
        request_path = urlparse(self.path).path
        if request_path == "/api/test-cases/sync":
            try:
                sync_result = sync_test_case_repository()
                result = {**discover_test_cases(), "sync": sync_result}
                status = 200
            except (RuntimeError, OSError) as error:
                result = {"testCases": [], "error": str(error)}
                status = 500
            send_json(self, status, result)
            return

        report_match = re.fullmatch(
            r"/api/test-runs/([^/]+)/reports/([^/]+)/open",
            request_path,
        )
        if report_match:
            try:
                report_uri = open_test_report(*report_match.groups())
                result = {"reportUrl": report_uri}
                status = 200
            except (RuntimeError, OSError) as error:
                result = {"error": str(error)}
                status = 404
            send_json(self, status, result)
            return

        if request_path != "/api/test-runs":
            self.send_error(404)
            return

        try:
            content_length = int(self.headers.get("Content-Length", "0"))
            raw_body = self.rfile.read(content_length)
            request_body = json.loads(raw_body.decode("utf-8") or "{}")
            if not isinstance(request_body, dict):
                raise RuntimeError("Test run request must contain a JSON object.")
            result = start_test_cases(request_body)
            status = 202
        except (json.JSONDecodeError, RuntimeError, OSError) as error:
            result = {"error": str(error)}
            status = 400
        send_json(self, status, result)

    def do_PUT(self) -> None:
        request_path = urlparse(self.path).path
        if request_path not in {"/api/settings", "/api/model-config"}:
            self.send_error(404)
            return

        try:
            content_length = int(self.headers.get("Content-Length", "0"))
            raw_body = self.rfile.read(content_length)
            request_body = json.loads(raw_body.decode("utf-8") or "{}")
            if not isinstance(request_body, dict):
                raise RuntimeError("Settings request must contain a JSON object.")
            if request_path == "/api/model-config":
                incoming_config = request_body.get("modelConfig", request_body)
                if not isinstance(incoming_config, dict):
                    raise RuntimeError("modelConfig must contain a JSON object.")
                model_config = write_model_config(incoming_config)
            else:
                incoming_settings = request_body.get("settings", request_body)
                if not isinstance(incoming_settings, dict):
                    raise RuntimeError("settings must contain a JSON object.")
                settings = normalize_settings(incoming_settings)
                write_settings(settings)
            status = 200
        except (json.JSONDecodeError, RuntimeError, OSError) as error:
            result = {"error": str(error)}
            status = 400
        else:
            result = (
                {"modelConfig": model_config}
                if request_path == "/api/model-config"
                else {"settings": settings}
            )
        send_json(self, status, result)

    def translate_path(self, path: str) -> str:
        request_path = unquote(urlparse(path).path)
        parts = Path(request_path.lstrip("/")).parts

        if len(parts) >= 3 and parts[0] == "vendor" and parts[1] in VENDOR_PACKAGES:
            vendor_path = VENDOR_DIRECTORY.joinpath(*parts[1:]).resolve()
            package_root = (VENDOR_DIRECTORY / parts[1]).resolve()

            try:
                vendor_path.relative_to(package_root)
            except ValueError:
                return str(APP_DIRECTORY / "__not_found__")

            return str(vendor_path)

        return super().translate_path(path)


class AppServer(ThreadingHTTPServer):
    allow_reuse_address = True


def port_is_available() -> bool:
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as probe:
        return probe.connect_ex((HOST, PORT)) != 0


def ensure_required_port_available() -> None:
    if not port_is_available():
        raise RuntimeError(f"port {PORT} is already in use")


def create_server(handler) -> AppServer:
    return AppServer((HOST, PORT), handler)


def write_pid_file() -> None:
    PID_PATH.write_text(f"{os.getpid()}\n", encoding="utf-8")


def remove_pid_file() -> None:
    try:
        if PID_PATH.read_text(encoding="utf-8").strip() == str(os.getpid()):
            PID_PATH.unlink()
    except FileNotFoundError:
        return
    except OSError:
        return


def main() -> None:
    missing_packages = [
        package for package in VENDOR_PACKAGES if not (VENDOR_DIRECTORY / package).is_dir()
    ]
    if missing_packages:
        packages = ", ".join(sorted(missing_packages))
        print(
            f"Unable to start the app: missing local vendor packages: {packages}.",
            file=sys.stderr,
        )
        raise SystemExit(1)

    handler = partial(AppRequestHandler, directory=str(APP_DIRECTORY))

    try:
        ensure_required_port_available()
    except (OSError, RuntimeError) as error:
        print(
            f"Unable to start the app: {error}. Stop the existing process and try again.",
            file=sys.stderr,
        )
        raise SystemExit(1)

    try:
        settings, network_zone = apply_network_zone_repository()
        AppRequestHandler.network_zone = network_zone
        print(
            f"Network zone: {network_zone}; test case repository: "
            f"{settings['testCaseRepositoryUrl']}"
        )
        sync_result = sync_test_case_repository(settings)
        print(f"Test case repository {sync_result['action']}: {sync_result['path']}")
    except (RuntimeError, OSError) as error:
        print(f"Warning: unable to update the test case repository: {error}", file=sys.stderr)

    try:
        server = create_server(handler)
    except OSError as error:
        print(
            f"Unable to start the app: port {PORT} is unavailable ({error}).",
            file=sys.stderr,
        )
        raise SystemExit(1)

    print(f"Serving the app at http://localhost:{PORT}")
    print("Press Ctrl+C to stop.")
    write_pid_file()

    try:
        server.serve_forever()
    except KeyboardInterrupt:
        print("\nStopping the app.")
    finally:
        server.server_close()
        remove_pid_file()


if __name__ == "__main__":
    main()
