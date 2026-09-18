"""Server-owned IDATA operation executed by the generic remote command client."""

import base64
import csv
import json
import os
import re
import shutil
import subprocess
import sys
import tarfile
import tempfile
import time
from datetime import datetime
from pathlib import Path, PureWindowsPath
from urllib.parse import urlparse

_embedded_header = globals().get("SERVER_EMBEDDED_HEADER")
if _embedded_header:
    _request_path_value = base64.b64decode(_embedded_header[0]).decode("utf-8")
    _worker_path_value = base64.b64decode(_embedded_header[1]).decode("utf-8")
else:
    _request_path_value = ""
    _worker_path_value = os.environ.get("IDATA_SERVER_WORKER_PATH") or globals().get("__file__", "")
WORKER_PATH = Path(_worker_path_value).resolve() if _worker_path_value else None
SERVER_WORKER_SOURCE = globals().get("SERVER_WORKER_SOURCE")
DATA_ROOT = Path("D:/.idata") if os.name == "nt" else Path.home() / ".idata"
STATE = DATA_ROOT / "server-command-runtime"
SETTINGS = STATE / "settings.json"
RUNS = STATE / "runs"
UPDATE_STATUS = STATE / "test-case-update.json"
LIBRARY = DATA_ROOT / "newest_testcases"
DEFAULTS = {
    "projectName": "IDATA", "releaseName": "FangTian 1.10-1.12",
    "defaultEnvironment": "HarmonyOS", "defaultOwner": "kouyanan 30030842",
    "testCaseArchiveUrl": "http://10.90.65.189:54322/Testcases.tar.gz",
    "testCaseLibraryPath": str(LIBRARY), "idataExecutablePath": "IDATA.exe",
    "autoLoadDevices": True, "deviceRefreshSeconds": 30, "tablePageSize": 20,
}


def server_worker_source():
    global SERVER_WORKER_SOURCE
    if SERVER_WORKER_SOURCE is None:
        if WORKER_PATH is None or not WORKER_PATH.is_file():
            raise RuntimeError("The Server worker path was not provided to IDATA.")
        source = WORKER_PATH.read_bytes()
        first_line, separator, remainder = source.partition(b"\n")
        if separator and first_line.startswith(b"SERVER_EMBEDDED_HEADER = "):
            source = remainder
        SERVER_WORKER_SOURCE = source
    return SERVER_WORKER_SOURCE


def write_background_worker(name, operation, argument=""):
    worker = STATE / name
    worker.parent.mkdir(parents=True, exist_ok=True)
    header = json.dumps((operation, argument), ensure_ascii=True)
    worker.write_bytes(f"SERVER_BACKGROUND_REQUEST = {header}\n".encode("ascii") + server_worker_source())
    return worker


def atomic_json(path, value):
    path.parent.mkdir(parents=True, exist_ok=True)
    with tempfile.NamedTemporaryFile("w", encoding="utf-8", delete=False, dir=path.parent) as output:
        json.dump(value, output, ensure_ascii=False, indent=2)
        output.write("\n")
        temporary = output.name
    Path(temporary).replace(path)


def settings():
    value = dict(DEFAULTS)
    if SETTINGS.is_file():
        loaded = json.loads(SETTINGS.read_text(encoding="utf-8"))
        if isinstance(loaded, dict):
            value.update({key: loaded[key] for key in DEFAULTS if key in loaded})
    return value


def update_status(status=None, message=None):
    if status is not None:
        atomic_json(UPDATE_STATUS, {"status": status, "message": message or ""})
    if UPDATE_STATUS.is_file():
        return json.loads(UPDATE_STATUS.read_text(encoding="utf-8"))
    return {"status": "idle", "message": "Ready to update the test case library."}


def discover(current=None):
    current = current or settings()
    root = Path(current["testCaseLibraryPath"]).expanduser()
    mapping_path = root / "mapping.csv"
    if not mapping_path.is_file():
        return {"testCases": [], "error": f"Test case mapping was not found: {mapping_path}"}
    files = {path.stem: path for path in root.rglob("*.py") if path.name != "__init__.py"}
    cases = []
    with mapping_path.open(encoding="utf-8-sig", newline="") as source:
        for row in csv.DictReader(source):
            number = (row.get("用例_编号") or "").strip()
            path = files.get(number)
            if not number or path is None:
                continue
            cases.append({
                "id": str(len(cases) + 1), "title": (row.get("用例_名称") or number).strip(),
                "executionName": number, "path": path.relative_to(root).with_suffix("").as_posix(),
                "moduleName": (row.get("模块_名称") or "").strip(),
                "moduleCode": (row.get("模块_编号") or "").strip(),
                "applicationName": (row.get("应用_名称") or "").strip(),
                "applicationCode": (row.get("应用_编号") or "").strip(),
                "updated": datetime.fromtimestamp(path.stat().st_mtime).astimezone().isoformat(timespec="seconds"),
                "category": "Standard",
            })
    return {"testCases": cases, "mappingPath": str(mapping_path)}


def install_archive(current):
    url = current["testCaseArchiveUrl"].strip()
    parsed = urlparse(url)
    if parsed.scheme not in {"http", "https"} or not parsed.hostname or parsed.username or parsed.password:
        raise RuntimeError("Set a valid HTTP or HTTPS test case archive URL in Settings.")
    parent = LIBRARY.parent
    parent.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(prefix=".testcases-", dir=parent) as temporary:
        stage = Path(temporary)
        archive_path = stage / "Testcases.tar.gz"
        update_status("running", "Downloading test case archive on the execution PC…")
        command = ["curl.exe" if os.name == "nt" else "curl", "--fail", "--location", "--silent", "--show-error", "--connect-timeout", "30", "--max-time", "600", "--max-filesize", "2147483648", "--proto", "=http,https", "--proto-redir", "=http,https", "--output", str(archive_path), url]
        result = subprocess.run(command, capture_output=True, timeout=620)
        if result.returncode:
            raise RuntimeError("Archive download failed. Check the URL and network connection.")
        extracted = stage / "extracted"
        update_status("running", "Extracting and validating test cases…")
        extracted.mkdir()
        with tarfile.open(archive_path, "r:gz", encoding="utf-8") as package:
            total = 0
            for count, member in enumerate(package, 1):
                parts = member.name.replace("\\", "/").split("/")
                if member.name.startswith(("/", "\\")) or ".." in parts or any(":" in part for part in parts) or not (member.isdir() or member.isfile()):
                    raise RuntimeError("The archive contains an unsafe path or unsupported file type.")
                total += member.size
                if count > 100000 or total > 4 * 1024**3:
                    raise RuntimeError("The extracted archive exceeds the supported size limit.")
                destination = extracted.joinpath(*parts)
                if member.isdir():
                    destination.mkdir(parents=True, exist_ok=True)
                else:
                    destination.parent.mkdir(parents=True, exist_ok=True)
                    with package.extractfile(member) as source, destination.open("wb") as output:
                        shutil.copyfileobj(source, output)
        mappings = list(extracted.rglob("mapping.csv"))
        if len(mappings) != 1:
            raise RuntimeError("The archive must contain exactly one test case mapping CSV.")
        source = mappings[0].parent
        check = discover({**current, "testCaseLibraryPath": str(source)})
        if not check["testCases"]:
            raise RuntimeError("The archive contains no mapped test cases.")
        backup = stage / "previous"
        if LIBRARY.exists():
            LIBRARY.rename(backup)
        try:
            source.rename(LIBRARY)
        except Exception:
            if LIBRARY.exists():
                shutil.rmtree(LIBRARY)
            if backup.exists():
                backup.rename(LIBRARY)
            raise
    current["testCaseLibraryPath"] = str(LIBRARY)
    atomic_json(SETTINGS, current)
    return {"status": "complete", "message": "Test case library updated successfully."}


def execute_update():
    try:
        result = install_archive(settings())
        update_status(result["status"], result["message"])
    except Exception as error:
        update_status("failed", str(error))


def idata_path(current):
    raw = current["idataExecutablePath"].strip()
    path = Path(raw).expanduser()
    if path.is_absolute():
        return path
    directory = Path(os.environ.get("IDATA_CLIENT_EXECUTABLE_DIRECTORY", "."))
    return (directory / ("IDATA.exe" if raw.replace("\\", "/").lower() == "../idata.exe" else raw)).resolve()


def case_result(output, case_name):
    if not case_name or not case_name.strip():
        return "Blocked"
    markers = re.findall(r"用例\s*" + re.escape(case_name.strip()) + r"\s*执行(成功|失败)", output)
    return "Blocked" if not markers else "Passed" if markers[-1] == "成功" else "Failed"


def set_report_reference(item, library_path=None):
    markers = re.findall(r"检测报告[ \t]*[:：][ \t]*([^\r\n]+)", item.get("consoleOutput", ""))
    if not markers:
        return
    path = markers[-1].strip().strip("\"'")
    item["reportLocation"] = path
    item["reportUrl"] = None
    if re.match(r"(?i)^(?:https?://|file:|\\\\)", path) or Path(path).suffix.lower() not in {".html", ".htm"}:
        return
    if re.match(r"^[A-Za-z]:[\\/]", path):
        item["reportUrl"] = PureWindowsPath(path).as_uri()
    else:
        report = Path(path).expanduser()
        if not report.is_absolute():
            if not library_path:
                return
            report = Path(library_path) / report
        item["reportUrl"] = report.resolve().as_uri()


def serialize_run(run):
    processes = [dict(item) for item in run["started"]]
    for item in processes:
        set_report_reference(item, run.get("libraryPath"))
        if item["result"] in {"Passed", "Failed", "Blocked"} and item.get("testCaseName"):
            item["result"] = case_result(item.get("consoleOutput", ""), item["testCaseName"])
    finished = [item for item in processes if item["result"] not in {"Pending", "Running"}]
    failed = sum(item["result"] == "Failed" for item in finished)
    blocked = sum(item["result"] == "Blocked" for item in finished)
    interrupted = sum(item["result"] == "Interrupted" for item in finished)
    return {**run, "started": processes, "status": "Interrupted" if run.get("stopRequested") else "Running" if len(finished) < len(processes) else "Interrupted" if interrupted else "Completed", "runningProcesses": len(processes) - len(finished), "totalProcesses": len(processes), "executedProcesses": len(finished), "passedProcesses": sum(item["result"] == "Passed" for item in finished), "failedProcesses": failed, "blockedProcesses": blocked, "interruptedProcesses": interrupted, "progress": round(len(finished) / len(processes) * 100) if processes else 0, "consoleOutput": "\n\n".join(item.get("consoleOutput", "") for item in processes)}


def run_path(run_id):
    if not re.fullmatch(r"TR-[0-9]+", run_id):
        raise RuntimeError("Invalid test run ID.")
    return RUNS / f"{run_id}.json"


def execute_run(run_id):
    path = run_path(run_id)
    run = json.loads(path.read_text(encoding="utf-8"))
    for item in run["started"]:
        latest = json.loads(path.read_text(encoding="utf-8"))
        if latest.get("stopRequested"):
            run["stopRequested"] = True
            item.update(result="Interrupted", exitCode=None, interruptionMessage="The test run was closed manually.")
            atomic_json(path, run)
            continue
        item["result"] = "Running"
        atomic_json(path, run)
        command = item.pop("executionCommand")
        try:
            result = subprocess.run(command, cwd=run["libraryPath"], stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True, errors="replace")
            output, exit_code = result.stdout or "", result.returncode
        except OSError as error:
            output, exit_code = str(error), -1

        latest = json.loads(path.read_text(encoding="utf-8"))
        if latest.get("stopRequested"):
            run["stopRequested"] = True
            item.update(result="Interrupted", exitCode=None, interruptionMessage="The test run was closed manually.", consoleOutput=output)
        else:
            item.update(result=case_result(output, item["testCaseName"]), exitCode=exit_code, consoleOutput=output)
        set_report_reference(item, run.get("libraryPath"))
        atomic_json(path, run)


def handle(operation, method, body):
    current = settings()
    if operation == "settings":
        if method == "PUT":
            incoming = body.get("settings", body)
            current.update({key: incoming[key] for key in DEFAULTS if key in incoming})
            atomic_json(SETTINGS, current)
        return {"settings": current, "networkZone": "blue", "testCaseUpdateCommand": ""}
    if operation == "devices":
        result = subprocess.run(["hdc", "list", "targets", "-v"], capture_output=True, text=True, timeout=10)
        devices = [{"id": columns[0], "status": columns[2] if len(columns) > 2 else "Connected"} for line in result.stdout.splitlines() if (columns := line.split()) and columns[0].lower() not in {"empty", "[empty]"}]
        return {"devices": devices, "error": None if result.returncode == 0 else (result.stderr.strip() or "HDC device search failed.")}
    if operation == "test-cases":
        return discover(current)
    if operation == "test-cases/update":
        status = update_status()
        if method != "POST" or status.get("status") == "running":
            return status
        executable = idata_path(current)
        if not executable.is_file():
            raise RuntimeError("IDATA.exe was not found.")
        update_status("running", "Preparing test case update…")
        worker = write_background_worker("update-worker.py", "update")
        subprocess.Popen([str(executable), "cli", "bundle", "run", "--path", str(worker)], stdin=subprocess.DEVNULL, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, close_fds=True)
        return update_status()
    if operation == "test-runs" and method == "POST":
        selected = body.get("testCases")
        mode = body.get("inspectionMode")
        device = str(body.get("device", "")).strip()
        available = {item["id"]: item for item in discover(current)["testCases"]}
        if not isinstance(selected, list) or not selected or mode not in (0, 1, 2) or not device:
            raise RuntimeError("Select test cases, a device, and a valid inspection mode.")
        root = Path(current["testCaseLibraryPath"])
        executable = idata_path(current)
        runner = root / "run_testcase.py"
        if not executable.is_file() or not runner.is_file():
            raise RuntimeError("IDATA.exe or run_testcase.py was not found.")
        run_id = f"TR-{int(time.time() * 1000)}"
        started = []
        for case_id in dict.fromkeys(selected):
            case = available.get(case_id)
            if case is None:
                raise RuntimeError(f"Unknown test case selection: {case_id}")
            command = [str(executable), "cli", "bundle", "run", "--path", str(runner), "--", case["executionName"], str(mode), device]
            started.append({"testCase": case_id, "testCaseName": case["executionName"], "inspectionMode": mode, "processId": None, "command": subprocess.list2cmdline(command), "executionCommand": command, "result": "Pending", "consoleOutput": "", "exitCode": None, "reportUrl": None, "reportLocation": None, "checks": []})
        run = {"id": run_id, "title": str(body.get("name", "")).strip(), "device": device, "inspectionMode": mode, "startedAt": datetime.now().astimezone().isoformat(timespec="milliseconds"), "libraryPath": str(root), "started": started}
        RUNS.mkdir(parents=True, exist_ok=True)
        atomic_json(run_path(run_id), run)
        worker = write_background_worker("run-worker.py", "run", run_id)
        subprocess.Popen([str(executable), "cli", "bundle", "run", "--path", str(worker)], stdin=subprocess.DEVNULL, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, close_fds=True)
        return serialize_run(run)
    if operation == "test-runs" and method == "GET":
        runs = [serialize_run(json.loads(path.read_text(encoding="utf-8"))) for path in RUNS.glob("TR-*.json") if not path.with_suffix(".deleted").exists()] if RUNS.is_dir() else []
        return {"testRuns": sorted(runs, key=lambda item: item["startedAt"], reverse=True)}
    match = re.fullmatch(r"test-runs/(TR-[0-9]+)", operation)
    if match and method == "DELETE":
        path = run_path(match.group(1))
        marker = path.with_suffix(".deleted")
        if not marker.exists():
            run = json.loads(path.read_text(encoding="utf-8"))
            if serialize_run(run)["status"] == "Running":
                raise RuntimeError("Close the running test run before deleting it.")
            # Separate tombstone survives any late write from a closing runner.
            atomic_json(marker, {"id": match.group(1)})
        return {"deleted": True, "id": match.group(1)}
    match = re.fullmatch(r"test-runs/(TR-[0-9]+)/close", operation)
    if match:
        path = run_path(match.group(1)); run = json.loads(path.read_text(encoding="utf-8")); run["stopRequested"] = True
        for item in run["started"]:
            if item["result"] in {"Pending", "Running"}:
                item.update(result="Interrupted", exitCode=None, error="The test run was closed manually.", interruptionMessage="The test run was closed manually.")
        atomic_json(path, run); return serialize_run(run)
    match = re.fullmatch(r"test-runs/(TR-[0-9]+)/reports/([^/]+)/open", operation)
    if match and method == "POST":
        run = json.loads(run_path(match.group(1)).read_text(encoding="utf-8"))
        item = next((value for value in run["started"] if value["testCase"] == match.group(2)), None)
        if item:
            set_report_reference(item, run.get("libraryPath"))
        if not item or not item.get("reportLocation"):
            raise RuntimeError("Test report was not found.")
        report = Path(item["reportLocation"]).expanduser()
        if not report.is_absolute():
            report = Path(run["libraryPath"]) / report
        report = report.resolve()
        if not report.is_file() or report.suffix.lower() not in {".html", ".htm"}:
            raise RuntimeError("Only local HTML reports can be opened.")
        if sys.platform == "win32":
            os.startfile(str(report))
        else:
            subprocess.run(["open" if sys.platform == "darwin" else "xdg-open", str(report)], check=True)
        return {"reportUrl": report.as_uri()}
    match = re.fullmatch(r"test-runs/(TR-[0-9]+)/reports/([^/]+)/content", operation)
    if match:
        run = json.loads(run_path(match.group(1)).read_text(encoding="utf-8")); item = next((value for value in run["started"] if value["testCase"] == match.group(2)), None)
        report = Path(item.get("reportLocation", "")) if item else Path()
        if not item or not report.is_file() or report.stat().st_size > 8 * 1024 * 1024:
            raise RuntimeError("Test report was not found or exceeds the viewing limit.")
        return {"contentBase64": base64.b64encode(report.read_bytes()).decode("ascii")}
    match = re.fullmatch(r"test-runs/(TR-[0-9]+)/logs/([^/]+)/content", operation)
    if match:
        run = json.loads(run_path(match.group(1)).read_text(encoding="utf-8")); item = next((value for value in run["started"] if value["testCase"] == match.group(2)), None)
        if not item:
            raise RuntimeError("Test execution log was not found.")
        content = item.get("consoleOutput", "").encode("utf-8")
        if len(content) > 8 * 1024 * 1024:
            raise RuntimeError("Test execution log exceeds the download limit.")
        return {"contentBase64": base64.b64encode(content).decode("ascii")}
    raise RuntimeError("Unsupported IDATA operation.")


def main():
    background_request = globals().get("SERVER_BACKGROUND_REQUEST")
    if background_request:
        if background_request[0] == "update":
            execute_update()
        elif background_request[0] == "run":
            execute_run(background_request[1])
        return
    if _request_path_value:
        request_path = Path(_request_path_value)
        fields = request_path.read_text(encoding="utf-8").splitlines()
        request_path.unlink(missing_ok=True)
        if len(fields) == 3:
            fields.append("")
        if len(fields) != 4:
            raise RuntimeError("The Server request file was invalid.")
        result_path = Path(base64.b64decode(fields[0]).decode("utf-8"))
        operation = base64.b64decode(fields[1]).decode("utf-8")
        method = base64.b64decode(fields[2]).decode("utf-8")
        encoded_payload = fields[3]
        write_response(result_path, operation, method, encoded_payload)
        return
    if len(sys.argv) >= 2 and sys.argv[1] == "__execute_update":
        execute_update()
        return
    if len(sys.argv) >= 2 and sys.argv[1] == "__execute_run":
        execute_run(sys.argv[3])
        return
    result_path = None
    offset = 1
    if len(sys.argv) >= 2 and sys.argv[1] == "__request":
        result_path = Path(sys.argv[2])
        offset = 4
    operation, method = sys.argv[offset], sys.argv[offset + 1]
    encoded_payload = sys.argv[offset + 2] if len(sys.argv) > offset + 2 else ""
    write_response(result_path, operation, method, encoded_payload)


def write_response(result_path, operation, method, encoded_payload):
    payload = json.loads(base64.b64decode(encoded_payload)) if encoded_payload else {}
    try:
        response = {"ok": True, "data": handle(operation, method, payload)}
    except Exception as error:
        response = {"ok": False, "error": str(error)}
    encoded_response = json.dumps(response, ensure_ascii=False)
    if result_path is not None:
        result_path.write_text(encoded_response, encoding="utf-8")
    else:
        print(encoded_response)


# IDATA's bundle runner may execute scripts under a non-standard module name.
# This file is an executable Server command payload, not an importable library.
main()
