package server

import (
	"encoding/base64"
	"encoding/binary"
	"github.com/gorilla/websocket"
	"idata-server/internal/protocol"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestWindowsIDATACommandIsEntirelyServerGenerated(t *testing.T) {
	command := idataWorkerCommand("windows", "test-cases/update", http.MethodPost, []byte(`{"force":true}`))
	for _, expected := range []string{"curl.exe", "archive extraction/replacement", "IDATA.exe cli bundle run", "test-cases/update"} {
		if !strings.Contains(command, expected) {
			t.Fatalf("command does not describe server-owned %q step", expected)
		}
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
	for _, expected := range []string{"IDATA_CLIENT_EXECUTABLE_DIRECTORY", "IDATA.exe", "cli bundle run", "worker.py"} {
		if !strings.Contains(script, expected) {
			t.Fatalf("PowerShell payload does not contain %q", expected)
		}
	}
	if strings.Contains(script, "IDATA_COMMAND_PYTHON") {
		t.Fatal("Windows command still depends on a Client-provided Python runtime")
	}
	worker := string(idataWorkerSource)
	for _, expected := range []string{`"curl.exe"`, `"cli", "bundle", "run"`, "source.rename(LIBRARY)"} {
		if !strings.Contains(worker, expected) {
			t.Fatalf("Server-owned worker does not contain %q", expected)
		}
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
	_ = agent.WriteJSON(protocol.Message{Type: protocol.TypeHello, ProtocolVersion: protocol.Version, ClientID: "pc", OS: "darwin", Capabilities: []string{"server_commands_v1"}})
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
		if request.Type != protocol.TypeCommand || !strings.Contains(request.Command, "python3 -c") {
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
