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
if ($first.status -ne 'Running' -or $first.consoleOutput -ne 'saved output') { throw 'Saved output was replaced' }
if ($first.started[0].reportLocation -ne 'saved.html' -or $first.started[0].PSObject.Properties.Name -contains 'reportUrl') { throw 'Wrong report fields' }
if ([IO.File]::ReadAllText($path) -ne $before) { throw 'Listing changed persistent run state' }
Remove-Item -LiteralPath $log
if ((Serialize-Run $run).consoleOutput -ne 'saved output') { throw 'Missing log erased saved output' }
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
