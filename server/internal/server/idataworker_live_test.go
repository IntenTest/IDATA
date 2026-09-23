package server

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestWindowsRunListUsesSavedSnapshot(t *testing.T) {
	shell, err := exec.LookPath("pwsh")
	if err != nil {
		shell, err = exec.LookPath("powershell.exe")
	}
	if err != nil {
		t.Skip("PowerShell is required for the Windows worker integration test")
	}
	directory := t.TempDir()
	source := strings.Split(string(idataWindowsWorkerSource), "if ($BackgroundUpdate) { Install-TestCases;")[0]
	script := source + `
$log = Join-Path $State 'live.log'
[IO.File]::WriteAllText($log, 'new output that must not be read', $Utf8)
$run = [pscustomobject]@{id='TR-1'; startedAt='2026-09-23'; started=@([pscustomobject]@{result='Running'; logPath=$log; consoleOutput='saved output'; reportLocation='saved.html'; reportUrl='file:///old.html'})}
$path = Get-RunPath 'TR-1'
Write-JsonFile $path $run
$before = [IO.File]::ReadAllText($path)
function Read-LogText { throw 'Listing must not read logs' }
$first = (Handle-Request 'test-runs' 'GET' @{}).testRuns[0]
if ($first.status -ne 'Running' -or $first.Contains('consoleOutput') -or $first.started[0].PSObject.Properties.Name -contains 'consoleOutput') { throw 'Unexpected API log output' }
if ($first.started[0].reportLocation -ne 'saved.html' -or $first.started[0].PSObject.Properties.Name -contains 'reportUrl') { throw 'Wrong report fields' }
if ([IO.File]::ReadAllText($path) -ne $before) { throw 'Listing changed persistent run state' }
Remove-Item -LiteralPath $log
if ($run.started[0].consoleOutput -ne 'saved output') { throw 'Saved output changed' }
$run.started[0].consoleOutput = ('日志' * 100000)
$preview = Serialize-Run $run
if ($preview.Contains('consoleOutput') -or $preview.started[0].PSObject.Properties.Name -contains 'consoleOutput') { throw 'API includes logs' }
if ($run.started[0].consoleOutput.Length -ne 200000) { throw 'Preview changed original log' }
$run.started[0].result = 'Passed'
if ((Serialize-Run $run).passedProcesses -ne 1) { throw 'Saved result changed' }

`
	path := filepath.Join(directory, "live.ps1")
	if err := os.WriteFile(path, append([]byte{0xef, 0xbb, 0xbf}, []byte(script)...), 0600); err != nil {
		t.Fatal(err)
	}
	command := exec.Command(shell, "-NoLogo", "-NoProfile", "-NonInteractive", "-File", path)
	command.Env = append(os.Environ(), "USERPROFILE="+directory)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("live log polling: %v\n%s", err, output)
	}
}

func TestPythonRunResponseOmitsSavedLogs(t *testing.T) {
	source := strings.Split(string(idataWorkerSource), "# IDATA's bundle runner")[0]
	script := source + `
import copy
run = {"id": "TR-1", "consoleOutput": "old aggregate", "started": [{"result": "Blocked", "consoleOutput": "日志" * 100000, "reportLocation": "report.html"} for _ in range(100)]}
before = copy.deepcopy(run)
preview = serialize_run(run)
assert run == before
assert "consoleOutput" not in preview
assert preview["blockedProcesses"] == 100
assert all(item["reportLocation"] == "report.html" for item in preview["started"])
assert all("consoleOutput" not in item for item in preview["started"])
assert len(json.dumps(preview, ensure_ascii=False).encode()) < 200000
`
	command := exec.Command("python3", "-c", script)
	command.Env = append(os.Environ(), "HOME="+t.TempDir())
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("run preview: %v\n%s", err, output)
	}
}
