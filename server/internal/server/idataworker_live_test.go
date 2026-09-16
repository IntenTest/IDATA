package server

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestWindowsRunPollReadsLiveLog(t *testing.T) {
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
[IO.File]::WriteAllText($log, "first line` + "`n" + `", $Utf8)
$run = [pscustomobject]@{id='TR-1'; started=@([pscustomobject]@{result='Running'; logPath=$log; consoleOutput=''})}
$path = Get-RunPath 'TR-1'
Write-JsonFile $path $run
$before = [IO.File]::ReadAllText($path)
$first = Serialize-Run (Read-JsonFile $path $null)
if ($first.status -ne 'Running' -or $first.consoleOutput -notmatch 'first line') { throw 'Running output is missing' }
[IO.File]::AppendAllText($log, 'second line', $Utf8)
$second = Serialize-Run (Read-JsonFile $path $null)
if ($second.consoleOutput -notmatch 'second line' -or $second.started[0].consoleOutput -notmatch 'second line') { throw 'Next poll did not refresh output' }
if ([IO.File]::ReadAllText($path) -ne $before) { throw 'Polling changed persistent run state' }
[IO.File]::WriteAllText($log, (('x' * 2100000) + 'tail marker'), $Utf8)
$bounded = Serialize-Run (Read-JsonFile $path $null)
if ($bounded.consoleOutput.Length -gt 2000200 -or $bounded.consoleOutput -notmatch 'tail marker' -or $bounded.consoleOutput -notmatch 'Earlier output omitted') { throw 'Log tail is not bounded' }
Remove-Item -LiteralPath $log
$run.started[0].consoleOutput = 'saved output'
if ((Serialize-Run $run).consoleOutput -ne 'saved output') { throw 'Missing log erased saved output' }
$run.started[0].logPath = ([string][char]0)
if ((Serialize-Run $run).status -ne 'Running') { throw 'Unreadable log broke polling' }
$run.started[0].result = 'Passed'
if ((Serialize-Run $run).consoleOutput -ne 'saved output') { throw 'Completed output changed' }
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
