package server

import (
	"github.com/gorilla/websocket"
	"idata-server/internal/protocol"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDeleteRunOperation(t *testing.T) {
	for _, op := range []string{"test-runs/TR-1", "test-runs/TR-123456"} {
		if !idataDeleteOperation.MatchString(op) {
			t.Fatal(op)
		}
	}
	for _, op := range []string{"settings", "test-runs", "test-runs/TR-1/close", "test-runs/../TR-1", "test-runs/TR-a"} {
		if idataDeleteOperation.MatchString(op) {
			t.Fatal(op)
		}
	}
}

func TestPythonDeleteRun(t *testing.T) {
	source := strings.TrimSuffix(strings.TrimSpace(string(idataWorkerSource)), "main()")
	script := source + `
import tempfile
with tempfile.TemporaryDirectory() as directory:
    STATE = Path(directory)
    RUNS = STATE / 'runs'
    RUNS.mkdir()
    report = STATE / 'report.html'
    report.write_text('keep')
    for index, result in enumerate(['Pending', 'Running', 'Passed', 'Failed', 'Blocked', 'Interrupted']):
        run_id = f'TR-{index}'
        run = {'id': run_id, 'startedAt': '2026-09-18', 'started': [{'testCase': '1', 'result': result}]}
        atomic_json(run_path(run_id), run)
        if result in {'Pending', 'Running'}:
            try:
                handle(f'test-runs/{run_id}', 'DELETE', {})
            except RuntimeError as error:
                assert 'Close' in str(error)
            else:
                raise AssertionError('Active run deleted')
            handle(f'test-runs/{run_id}/close', 'POST', {})
        assert handle(f'test-runs/{run_id}', 'DELETE', {})['deleted']
        assert handle(f'test-runs/{run_id}', 'DELETE', {})['deleted']
        atomic_json(run_path(run_id), run)  # Late background write must not resurrect it.
        assert not handle('test-runs', 'GET', {})['testRuns']
    assert report.read_text() == 'keep'
`
	out, err := exec.Command("python3", "-c", script).CombinedOutput()
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
}

func TestWindowsDeleteRun(t *testing.T) {
	shell, err := exec.LookPath("pwsh")
	if err != nil {
		t.Skip("PowerShell is required")
	}
	directory := t.TempDir()
	source := strings.Split(string(idataWindowsWorkerSource), "if ($BackgroundUpdate) { Install-TestCases;")[0]
	script := source + `
$report = Join-Path $State 'report.html'
[IO.File]::WriteAllText($report, 'keep')
$i = 0
foreach ($result in @('Pending','Running','Passed','Failed','Blocked','Interrupted')) {
    $id = 'TR-' + $i; $i++
    $run = [pscustomobject]@{id=$id; startedAt='2026-09-18'; started=@([pscustomobject]@{testCase='1';result=$result})}
    Write-JsonFile (Get-RunPath $id) $run
    if ($result -in @('Pending','Running')) {
        $rejected = $false
        try { Handle-Request "test-runs/$id" 'DELETE' @{} } catch { $rejected = $_.Exception.Message -match 'Close' }
        if (-not $rejected) { throw 'Active run deleted' }
        Set-ObjectValue $run 'stopRequested' $true
        Write-JsonFile (Get-RunPath $id) $run
    }
    if (-not (Handle-Request "test-runs/$id" 'DELETE' @{}).deleted) { throw 'Delete failed' }
    if (-not (Handle-Request "test-runs/$id" 'DELETE' @{}).deleted) { throw 'Retry failed' }
    Write-JsonFile (Get-RunPath $id) $run
    if (@((Handle-Request 'test-runs' 'GET' @{}).testRuns).Count -ne 0) { throw 'Deleted run resurrected' }
}
if ([IO.File]::ReadAllText($report) -ne 'keep') { throw 'Report removed' }
`
	path := filepath.Join(directory, "delete.ps1")
	if err := os.WriteFile(path, append([]byte{0xef, 0xbb, 0xbf}, []byte(script)...), 0600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(shell, "-NoLogo", "-NoProfile", "-NonInteractive", "-File", path)
	cmd.Env = append(os.Environ(), "USERPROFILE="+directory)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
}

func TestDeleteRunProxyAuthorization(t *testing.T) {
	app := newTestServer(t)
	srv := httptest.NewServer(app.Handler())
	defer srv.Close()
	agent, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http")+"/ws/agent", http.Header{"Authorization": []string{"Bearer agent-test-token"}})
	if err != nil {
		t.Fatal(err)
	}
	defer agent.Close()
	_ = agent.WriteJSON(protocol.Message{Type: protocol.TypeHello, ProtocolVersion: protocol.Version, ClientID: "pc", OS: "darwin", Capabilities: []string{"server_commands_v1", "command_stdin_v1"}})
	deadline := time.Now().Add(time.Second)
	for app.hub.get("pc") == nil && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	endpoint := srv.URL + "/api/v1/clients/pc/idata/test-runs/TR-1"
	for _, crossOrigin := range []bool{false, true} {
		req, _ := http.NewRequest("DELETE", endpoint, nil)
		if crossOrigin {
			req.Header.Set("Authorization", "Bearer admin-test-token")
			req.Header.Set("Origin", "https://evil.example")
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != 403 {
			t.Fatalf("unauthorized delete: %d", res.StatusCode)
		}
	}
	done := make(chan int, 1)
	go func() {
		req, _ := http.NewRequest("DELETE", endpoint, nil)
		req.Header.Set("Authorization", "Bearer admin-test-token")
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			done <- 0
			return
		}
		res.Body.Close()
		done <- res.StatusCode
	}()
	_ = agent.SetReadDeadline(time.Now().Add(2 * time.Second))
	var command protocol.Message
	if err := agent.ReadJSON(&command); err != nil {
		t.Fatal(err)
	}
	if command.Command != idataWorkerCommand("darwin", "test-runs/TR-1", "DELETE") {
		t.Fatal("wrong delete command")
	}
	_ = agent.WriteJSON(protocol.Message{Type: protocol.TypeResult, ProtocolVersion: protocol.Version, RequestID: command.RequestID, Stdout: `{"ok":true,"data":{"deleted":true,"id":"TR-1"}}`})
	select {
	case status := <-done:
		if status != 200 {
			t.Fatalf("delete status %d", status)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("delete timed out")
	}
}
