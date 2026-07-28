#!/usr/bin/env python3
"""Serve the local application on its only supported port."""

from datetime import datetime
from functools import partial
from http.server import SimpleHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from threading import Lock
from urllib.parse import unquote, urlparse
import json
import os
import socket
import subprocess
import sys
import tempfile
import time


HOST = "127.0.0.1"
PORT = 54321
APP_DIRECTORY = Path(__file__).resolve().parent
VENDOR_DIRECTORY = APP_DIRECTORY.parent / "vendor"
SETTINGS_PATH = APP_DIRECTORY / "config" / "settings.json"
PID_PATH = APP_DIRECTORY / ".ohwemby.pid"
VENDOR_PACKAGES = frozenset(("vue-3.5.24", "element-plus-2.11.8"))
HDC_TIMEOUT_SECONDS = 10
TEST_RUNS = {}
TEST_RUNS_LOCK = Lock()
DEFAULT_SETTINGS = {
    "projectName": "Oh Wemby",
    "releaseName": "Release 2.4",
    "defaultEnvironment": "QA staging",
    "defaultOwner": "kouyanan 30030842",
    "testCaseLibraryPath": "",
    "pythonExecutablePath": "",
    "autoLoadDevices": True,
    "deviceRefreshSeconds": 30,
    "tablePageSize": 20,
}
SETTING_FIELD_TYPES = {
    "projectName": str,
    "releaseName": str,
    "defaultEnvironment": str,
    "defaultOwner": str,
    "testCaseLibraryPath": str,
    "pythonExecutablePath": str,
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


def configured_path(raw_path: str) -> Path:
    return Path(raw_path).expanduser().resolve()


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
        if path.is_file() and path.suffix.lower() == ".py"
    )
    test_cases = []
    for case_id, test_case_path in enumerate(test_case_paths, start=1):
        relative_name = (
            test_case_path.relative_to(library_path).with_suffix("").as_posix()
        )
        modified_time = datetime.fromtimestamp(
            test_case_path.stat().st_mtime
        ).astimezone()
        test_cases.append(
            {
                "id": str(case_id),
                "title": test_case_path.stem,
                "path": relative_name,
                "category": "Standard",
                "status": "Not run",
                "owner": settings["defaultOwner"],
                "updated": modified_time.isoformat(timespec="seconds"),
            }
        )

    return {"testCases": test_cases, "error": None}


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
    if not raw_library_path:
        raise RuntimeError("Set the test case library path in Settings.")
    if not raw_python_path:
        raise RuntimeError("Set the Python executable path in Settings.")

    library_path = configured_path(raw_library_path)
    python_path = configured_path(raw_python_path)
    if not library_path.is_dir():
        raise RuntimeError(f"Test case library directory was not found: {library_path}")
    if not python_path.is_file():
        raise RuntimeError(f"Python executable was not found: {python_path}")
    if python_path.name.lower() != "python.exe":
        raise RuntimeError("Python executable path must end with python.exe.")

    discovered_cases = {
        test_case["id"]: test_case["title"]
        for test_case in discover_test_cases(settings)["testCases"]
    }
    case_ids = list(dict.fromkeys(raw_case_names))
    missing_names = [name for name in case_ids if name not in discovered_cases]
    if missing_names:
        names = ", ".join(missing_names)
        raise RuntimeError(f"Unknown test case selection: {names}")

    processes = []
    for case_id in case_ids:
        case_name = discovered_cases[case_id]
        command = [str(python_path), case_name, str(inspection_mode)]
        popen_options = {
            "cwd": library_path,
        }
        if os.name == "nt":
            popen_options["creationflags"] = subprocess.CREATE_NEW_CONSOLE
        try:
            process = subprocess.Popen(command, **popen_options)
        except OSError as error:
            raise RuntimeError(f"Unable to start {case_name}: {error}") from error
        processes.append(
            {
                "testCase": case_id,
                "testCaseName": case_name,
                "inspectionMode": inspection_mode,
                "processId": process.pid,
                "_process": process,
            }
        )

    started_at = datetime.now().astimezone()
    run_id = f"TR-{int(time.time() * 1000)}"
    run = {
        "id": run_id,
        "title": run_name.strip(),
        "device": device.strip(),
        "inspectionMode": inspection_mode,
        "startedAt": started_at.isoformat(timespec="seconds"),
        "processes": processes,
    }
    with TEST_RUNS_LOCK:
        TEST_RUNS[run_id] = run
    return serialize_test_run(run)


def serialize_test_run(run: dict) -> dict:
    processes = run["processes"]
    running_count = sum(
        1 for process_info in processes if process_info["_process"].poll() is None
    )
    return {
        "id": run["id"],
        "title": run["title"],
        "device": run["device"],
        "inspectionMode": run["inspectionMode"],
        "startedAt": run["startedAt"],
        "status": "Running" if running_count else "Completed",
        "runningProcesses": running_count,
        "totalProcesses": len(processes),
        "started": [
            {key: value for key, value in process_info.items() if key != "_process"}
            for process_info in processes
        ],
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


class AppRequestHandler(SimpleHTTPRequestHandler):
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
                result = {"settings": read_settings()}
                status = 200
            except RuntimeError as error:
                result = {"error": str(error)}
                status = 500
            send_json(self, status, result)
            return

        super().do_GET()

    def do_POST(self) -> None:
        if urlparse(self.path).path != "/api/test-runs":
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
        if urlparse(self.path).path != "/api/settings":
            self.send_error(404)
            return

        try:
            content_length = int(self.headers.get("Content-Length", "0"))
            raw_body = self.rfile.read(content_length)
            request_body = json.loads(raw_body.decode("utf-8") or "{}")
            if not isinstance(request_body, dict):
                raise RuntimeError("Settings request must contain a JSON object.")
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
            result = {"settings": settings}
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
