package server

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"idata-server/internal/protocol"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestServerWorkerRunsFromGenericStdin(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix shell integration")
	}
	command := exec.Command("/bin/sh", "-c", idataWorkerCommand(runtime.GOOS, "settings", http.MethodGet))
	command.Stdin = bytes.NewReader(idataWorkerInput(runtime.GOOS, nil))
	command.Env = append(os.Environ(), "HOME="+t.TempDir())
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("worker failed: %v: %s", err, output)
	}
	response, err := decodeIDATAWorkerResponse(string(output))
	if err != nil || !response.OK || !strings.Contains(string(response.Data), "settings") {
		t.Fatalf("response = %+v, error = %v, output = %s", response, err, output)
	}
}

func TestWindowsIDATACommandIsEntirelyServerGenerated(t *testing.T) {
	command := idataWorkerCommand("windows", "test-cases/update", http.MethodPost)
	if len(command) >= 8191 {
		t.Fatalf("Windows command is too long: %d bytes", len(command))
	}
	if strings.HasPrefix(strings.ToLower(command), "rem ") {
		t.Fatal("Windows command is hidden behind a cmd.exe REM prefix")
	}
	if !strings.HasPrefix(strings.ToLower(command), "powershell.exe ") {
		t.Fatalf("Windows command does not directly invoke PowerShell: %s", command)
	}
	marker := "-EncodedCommand "
	encoded := command[strings.LastIndex(command, marker)+len(marker):]
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || len(raw)%2 != 0 {
		t.Fatalf("invalid PowerShell payload: %v", err)
	}
	runes := make([]rune, len(raw)/2)
	for index := range runes {
		runes[index] = rune(binary.LittleEndian.Uint16(raw[index*2:]))
	}
	script := string(runes)
	for _, expected := range []string{"powershell.exe", ".ps1", "ReadToEnd", "UTF8Encoding($true)", "__IDATA_SERVER_RESPONSE__", "OperationB64"} {
		if !strings.Contains(script, expected) {
			t.Fatalf("PowerShell payload does not contain %q", expected)
		}
	}
	if strings.Contains(script, "IDATA.exe") {
		t.Fatal("management command still routes its response through IDATA.exe")
	}
	if strings.Contains(script, ".result") || strings.Contains(script, "ResultPath") {
		t.Fatal("management response still relies on a result file")
	}
	if strings.Contains(script, "IDATA_COMMAND_PYTHON") {
		t.Fatal("Windows command still depends on a Client-provided Python runtime")
	}
	worker := string(idataWindowsWorkerSource)
	for _, expected := range []string{"curl.exe", "tar.exe", "Start-BackgroundUpdate", "IDATA.exe", "cli bundle run", "Handle-Request"} {
		if !strings.Contains(worker, expected) {
			t.Fatalf("Server-owned worker does not contain %q", expected)
		}
	}
	windowsInput := string(idataWorkerInput("windows", []byte(`{"settings":{}}`)))
	if !strings.Contains(windowsInput, "param(") || strings.Contains(windowsInput, "import base64") {
		t.Fatal("Windows request did not receive the Server-owned PowerShell worker")
	}
}

func TestDecodeIDATAWorkerResponseIgnoresLauncherOutput(t *testing.T) {
	payload := base64.StdEncoding.EncodeToString([]byte(`{"ok":true,"data":{"settings":{}}}`))
	response, err := decodeIDATAWorkerResponse("IDATA launcher message\r\n__IDATA_SERVER_RESPONSE__" + payload + "\r\n")
	if err != nil || !response.OK || !strings.Contains(string(response.Data), "settings") {
		t.Fatalf("response = %+v, error = %v", response, err)
	}
}

func TestWorkerWritesResultWhenBundleUsesCustomModuleName(t *testing.T) {
	directory := t.TempDir()
	workerPath := directory + "/worker.py"
	requestPath := directory + "/request"
	resultPath := directory + "/response.json"
	requestFields := []string{
		base64.StdEncoding.EncodeToString([]byte(resultPath)),
		base64.StdEncoding.EncodeToString([]byte("settings")),
		base64.StdEncoding.EncodeToString([]byte("GET")),
		"",
	}
	if err := os.WriteFile(requestPath, []byte(strings.Join(requestFields, "\n")), 0600); err != nil {
		t.Fatal(err)
	}
	header := fmt.Sprintf("SERVER_EMBEDDED_HEADER = (%q, %q)\n",
		base64.StdEncoding.EncodeToString([]byte(requestPath)),
		base64.StdEncoding.EncodeToString([]byte(workerPath)))
	source := append([]byte(header), idataWorkerSource...)
	if err := os.WriteFile(workerPath, source, 0600); err != nil {
		t.Fatal(err)
	}
	code := `import sys; source=sys.stdin.buffer.read(); exec(compile(source,"worker.py","exec"),{"__name__":"idata_bundle"})`
	command := exec.Command("python3", "-c", code)
	command.Stdin = bytes.NewReader(source)
	command.Env = append(os.Environ(), "HOME="+t.TempDir())
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("custom-name worker failed: %v: %s", err, output)
	}
	data, err := os.ReadFile(resultPath)
	if err != nil {
		t.Fatal(err)
	}
	var response idataWorkerResponse
	if json.Unmarshal(data, &response) != nil || !response.OK || !strings.Contains(string(response.Data), "settings") {
		t.Fatalf("invalid result file: %s", data)
	}
}

func TestIDATAAuthorizationAndAssets(t *testing.T) {
	app := newTestServer(t)
	for _, tc := range []struct {
		path, origin string
		status       int
	}{
		{"/", "", 200}, {"/app.js", "", 200}, {"/remote.js", "", 200},
		{"/idata/", "", 200}, {"/idata/vendor/vue-3.5.24/vue.global.prod.js", "", 200},
		{"/connect/", "", 404}, {"/admin/", "", 404}, {"/console/app.js", "", 404},
		{"/api/v1/clients/pc/idata/settings", "", 403},
		{"/api/v1/clients/pc/idata/settings", "https://evil.example", 403},
	} {
		request := httptest.NewRequest("GET", tc.path, nil)
		request.Header.Set("Origin", tc.origin)
		response := httptest.NewRecorder()
		app.Handler().ServeHTTP(response, request)
		if response.Code != tc.status {
			t.Fatalf("%s: %d", tc.path, response.Code)
		}
	}
}

func TestIDATARoundTripAndDisconnect(t *testing.T) {
	app := newTestServer(t)
	server := httptest.NewServer(app.Handler())
	defer server.Close()
	header := http.Header{"Authorization": []string{"Bearer agent-test-token"}}
	agent, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+"/ws/agent", header)
	if err != nil {
		t.Fatal(err)
	}
	defer agent.Close()
	_ = agent.WriteJSON(protocol.Message{Type: protocol.TypeHello, ProtocolVersion: protocol.Version, ClientID: "pc", OS: "darwin", Capabilities: []string{"server_commands_v1", "command_stdin_v1"}})
	deadline := time.Now().Add(time.Second)
	for app.hub.get("pc") == nil && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	for _, disconnect := range []bool{false, true} {
		done := make(chan int, 1)
		go func() {
			request, _ := http.NewRequest("PUT", server.URL+"/api/v1/clients/pc/idata/settings", strings.NewReader(`{"settings":{}}`))
			request.Header.Set("Authorization", "Bearer admin-test-token")
			response, err := http.DefaultClient.Do(request)
			if err != nil {
				done <- 0
				return
			}
			defer response.Body.Close()
			_, _ = io.Copy(io.Discard, response.Body)
			done <- response.StatusCode
		}()
		_ = agent.SetReadDeadline(time.Now().Add(time.Second))
		var request protocol.Message
		if err := agent.ReadJSON(&request); err != nil {
			t.Fatal(err)
		}
		stdin := idataWorkerInput("darwin", []byte(`{"settings":{}}`))
		if request.Type != protocol.TypeCommand || !strings.Contains(request.Command, "python3 -c") || string(request.Stdin) != string(stdin) {
			t.Fatalf("unexpected request %+v", request)
		}
		want := 200
		if disconnect {
			agent.Close()
			want = 502
		} else {
			_ = agent.WriteJSON(protocol.Message{Type: protocol.TypeResult, ProtocolVersion: protocol.Version, RequestID: request.RequestID, ExitCode: 0, Stdout: `{"ok":true,"data":{"settings":{}}}`})
		}
		select {
		case status := <-done:
			if status != want {
				t.Fatalf("status %d, want %d", status, want)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("pending operation leaked")
		}
	}
}

func TestIDATARejectsClientWithoutCommandStdin(t *testing.T) {
	app := newTestServer(t)
	server := httptest.NewServer(app.Handler())
	defer server.Close()
	agent, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+"/ws/agent", http.Header{"Authorization": []string{"Bearer agent-test-token"}})
	if err != nil {
		t.Fatal(err)
	}
	defer agent.Close()
	_ = agent.WriteJSON(protocol.Message{Type: protocol.TypeHello, ProtocolVersion: protocol.Version, ClientID: "old-pc", OS: "windows", ClientVersion: "0.7.12", Capabilities: []string{"server_commands_v1"}})
	deadline := time.Now().Add(time.Second)
	for app.hub.get("old-pc") == nil && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	request, _ := http.NewRequest("GET", server.URL+"/api/v1/clients/old-pc/idata/settings", nil)
	request.Header.Set("Authorization", "Bearer admin-test-token")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(response.Body)
	if response.StatusCode != http.StatusConflict || !strings.Contains(string(body), "0.7.14") {
		t.Fatalf("status=%d body=%s", response.StatusCode, body)
	}
}
