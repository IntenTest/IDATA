package server

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestWindowsCaseMarkersAndLocalReport(t *testing.T) {
	shell, err := exec.LookPath("pwsh")
	if err != nil {
		shell, err = exec.LookPath("powershell.exe")
	}
	if err != nil {
		t.Skip("PowerShell is required")
	}
	directory := t.TempDir()
	source := strings.Split(string(idataWindowsWorkerSource), "if ($BackgroundUpdate) { Install-TestCases;")[0]
	script := source + `
$name = '登录[1].*'
$checks = @(
    @("用例登录[1].*执行成功", 'Passed'),
    @("用例登录[1].*执行失败", 'Failed'),
    @("用例登录[1].*执行成功` + "`n" + `用例登录[1].*执行失败", 'Failed'),
    @("用例登录[1].*执行失败` + "`n" + `用例登录[1].*执行成功", 'Passed'),
    @('用例其他执行成功', 'Blocked'),
    @('exit code: 0', 'Blocked'),
    @('用例登录[1].*扩展执行成功', 'Blocked')
)
foreach ($check in $checks) {
    if ((Get-CaseResult $check[0] $name) -ne $check[1]) { throw ('Wrong result: ' + $check[0]) }
}
$run = [pscustomobject]@{id='TR-1'; libraryPath=$State; startedAt='2026-09-18T12:00:00+08:00'; started=@(
    [pscustomobject]@{testCase='1'; result='Passed'; reportLocation='报告 空格.html'},
    [pscustomobject]@{testCase='2'; result='Failed'},
    [pscustomobject]@{testCase='3'; result='Blocked'}
)}
$historical = [pscustomobject]@{started=@([pscustomobject]@{result='Passed'; testCaseName='登录'; consoleOutput='process exited with code 0'})}
if ((Serialize-Run $historical).blockedProcesses -ne 1) { throw 'Historical exit-code result was not reclassified' }
$historical.started[0].result = 'Interrupted'
if ((Serialize-Run $historical).status -ne 'Interrupted') { throw 'Explicit cancellation was overwritten' }
$summary = Serialize-Run $run
if ($summary.passedProcesses -ne 1 -or $summary.failedProcesses -ne 1 -or $summary.blockedProcesses -ne 1 -or $summary.progress -ne 100) { throw 'Wrong summary' }
$run.started = @($run.started[2])
if ((Serialize-Run $run).status -ne 'Blocked') { throw 'Blocked run reported completed' }
$run.started = @([pscustomobject]@{testCase='1'; result='Passed'; reportLocation='报告 空格.html'})
Write-JsonFile (Get-RunPath 'TR-1') $run
$report = Join-Path $State '报告 空格.html'
[IO.File]::WriteAllText($report, '<img src="assets/image.png">', $Utf8)
function Start-Process { param($FilePath, $ErrorAction) $script:opened = $FilePath }
$response = Handle-Request 'test-runs/TR-1/reports/1/open' 'POST' ([pscustomobject]@{})
if ($script:opened -ne $report -or $response.reportUrl -notlike 'file:*') { throw ("Report location mismatch: opened=" + $script:opened + "; expected=" + $report + "; uri=" + $response.reportUrl) }
$run.started[0].reportLocation = 'unsafe.exe'
[IO.File]::WriteAllText((Join-Path $State 'unsafe.exe'), 'not executable', $Utf8)
Write-JsonFile (Get-RunPath 'TR-1') $run
$rejected = $false
try { Handle-Request 'test-runs/TR-1/reports/1/open' 'POST' ([pscustomobject]@{}) } catch { $rejected = $true }
if (-not $rejected) { throw 'Non-HTML file was opened' }
`
	path := filepath.Join(directory, "results.ps1")
	if err := os.WriteFile(path, append([]byte{0xef, 0xbb, 0xbf}, []byte(script)...), 0600); err != nil {
		t.Fatal(err)
	}
	command := exec.Command(shell, "-NoLogo", "-NoProfile", "-NonInteractive", "-File", path)
	command.Env = append(os.Environ(), "USERPROFILE="+directory)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("worker results: %v\n%s", err, output)
	}
}

func TestReportOpenOperationAllowlist(t *testing.T) {
	for _, operation := range []string{"test-runs/TR-1/reports/case-1/open", "test-runs/TR-123/reports/1/open"} {
		if !idataWriteOperation.MatchString(operation) {
			t.Fatalf("report open rejected: %s", operation)
		}
		if idataReadOperation.MatchString(operation) {
			t.Fatalf("report open allowed as GET: %s", operation)
		}
	}
	for _, operation := range []string{"test-runs/TR-1/reports/../../open", "test-runs/TR-1/reports/file.exe/execute", "test-runs/TR-1/reports/a/b/open"} {
		if idataWriteOperation.MatchString(operation) {
			t.Fatalf("invalid operation accepted: %s", operation)
		}
	}
}

func TestPythonCaseMarkersAndLocalReport(t *testing.T) {
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("Python is required")
	}
	source := strings.TrimSuffix(strings.TrimSpace(string(idataWorkerSource)), "main()")
	script := source + `
import tempfile
from unittest.mock import patch
for output, expected in [
    ("用例登录[1].*执行成功", "Passed"),
    ("用例登录[1].*执行失败", "Failed"),
    ("用例登录[1].*执行成功\n用例登录[1].*执行失败", "Failed"),
    ("用例登录[1].*执行失败\n用例登录[1].*执行成功", "Passed"),
    ("用例其他执行成功", "Blocked"),
    ("exit code: 0", "Blocked"),
    ("用例登录[1].*扩展执行成功", "Blocked"),
]:
    assert case_result(output, "登录[1].*") == expected, output
historical = {"started": [{"result":"Passed", "testCaseName":"登录", "consoleOutput":"process exited with code 0"}]}
assert serialize_run(historical)["blockedProcesses"] == 1
assert historical["started"][0]["result"] == "Passed", "Polling mutated saved state"
historical["started"][0]["result"] = "Interrupted"
assert serialize_run(historical)["status"] == "Interrupted"
with tempfile.TemporaryDirectory() as directory:
    RUNS = Path(directory)
    report = (RUNS / "报告 空格.html").resolve()
    report.write_text('<img src="assets/image.png">')
    run = {"id": "TR-1", "libraryPath": directory, "started": [
        {"testCase":"1", "result":"Passed", "reportLocation":report.name},
        {"testCase":"2", "result":"Failed"},
        {"testCase":"3", "result":"Blocked"},
    ]}
    summary = serialize_run(run)
    assert (summary["passedProcesses"], summary["failedProcesses"], summary["blockedProcesses"], summary["progress"]) == (1, 1, 1, 100)
    atomic_json(run_path("TR-1"), run)
    with patch.object(subprocess, "run") as opener:
        result = handle("test-runs/TR-1/reports/1/open", "POST", {})
        assert result["reportUrl"] == report.as_uri()
        assert opener.call_args.args[0][-1] == str(report)
    report.rename(report.with_suffix(".exe"))
    run["started"][0]["reportLocation"] = report.with_suffix(".exe").name
    atomic_json(run_path("TR-1"), run)
    with patch.object(subprocess, "run") as opener:
        try:
            handle("test-runs/TR-1/reports/1/open", "POST", {})
        except RuntimeError:
            pass
        else:
            raise AssertionError("Non-HTML file was opened")
        opener.assert_not_called()
`
	command := exec.Command(python, "-c", script)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("Python results: %v\n%s", err, output)
	}
}
