package server

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"idata-server/internal/protocol"
)

const testDeviceToken = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func hashDeviceToken(token string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(token)))
}

func newTestServer(t *testing.T) *Server {
	t.Helper()
	app, err := New(Config{
		AgentToken:        "agent-test-token",
		AdminToken:        "admin-test-token",
		DefaultTimeout:    2 * time.Second,
		MaxCommandTimeout: 10 * time.Second,
	}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	return app
}

func TestAdminAuthentication(t *testing.T) {
	app := newTestServer(t)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/clients", nil)
	response := httptest.NewRecorder()
	app.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}

	request = httptest.NewRequest(http.MethodGet, "/api/v1/clients", nil)
	request.Header.Set("Authorization", "Bearer admin-test-token")
	response = httptest.NewRecorder()
	app.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
}

func TestDeviceClientIdentificationDoesNotDependOnSourceIP(t *testing.T) {
	tests := []struct {
		name         string
		clients      []*clientConn
		deviceToken  string
		wantStatus   int
		wantClientID string
	}{
		{
			name: "selects token owner when every device shares one source IP",
			clients: []*clientConn{
				{info: protocol.ClientInfo{ID: "own-pc", OS: "windows", RemoteAddress: "203.0.113.10:41000"}, deviceTokenHash: hashDeviceToken(testDeviceToken)},
				{info: protocol.ClientInfo{ID: "other-pc", OS: "windows", RemoteAddress: "203.0.113.10:41001"}, deviceTokenHash: hashDeviceToken("abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789")},
			},
			deviceToken:  testDeviceToken,
			wantStatus:   http.StatusOK,
			wantClientID: "own-pc",
		},
		{
			name: "rejects unknown token",
			clients: []*clientConn{
				{info: protocol.ClientInfo{ID: "own-pc", OS: "windows"}, deviceTokenHash: hashDeviceToken(testDeviceToken)},
			},
			deviceToken: "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
			wantStatus:  http.StatusUnauthorized,
		},
		{
			name: "does not grant device console to non-Windows client",
			clients: []*clientConn{
				{info: protocol.ClientInfo{ID: "linux-pc", OS: "linux"}, deviceTokenHash: hashDeviceToken(testDeviceToken)},
			},
			deviceToken: testDeviceToken,
			wantStatus:  http.StatusUnauthorized,
		},
		{
			name: "rejects a token reused by multiple clients",
			clients: []*clientConn{
				{info: protocol.ClientInfo{ID: "own-pc-a", OS: "windows"}, deviceTokenHash: hashDeviceToken(testDeviceToken)},
				{info: protocol.ClientInfo{ID: "own-pc-b", OS: "windows"}, deviceTokenHash: hashDeviceToken(testDeviceToken)},
			},
			deviceToken: testDeviceToken,
			wantStatus:  http.StatusConflict,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			app := newTestServer(t)
			for _, client := range test.clients {
				app.hub.clients[client.info.ID] = client
			}
			request := httptest.NewRequest(http.MethodGet, "/api/v1/self", nil)
			request.RemoteAddr = "203.0.113.10:52000"
			request.Header.Set("Authorization", "Device "+test.deviceToken)
			response := httptest.NewRecorder()
			app.Handler().ServeHTTP(response, request)
			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d; body = %s", response.Code, test.wantStatus, response.Body.String())
			}
			if test.wantClientID != "" {
				var body struct {
					Clients      []protocol.ClientInfo `json:"clients"`
					SelfClientID string                `json:"self_client_id"`
				}
				if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
					t.Fatal(err)
				}
				if body.SelfClientID != test.wantClientID || len(body.Clients) != len(test.clients) {
					t.Fatalf("unexpected device list response: %#v", body)
				}
				if strings.Contains(response.Body.String(), "remote_address") {
					t.Fatalf("response leaked remote address: %s", response.Body.String())
				}
			}
		})
	}
}

func TestTerminalAuthorizationScopesDeviceTokenToItsClient(t *testing.T) {
	app := newTestServer(t)
	own := &clientConn{info: protocol.ClientInfo{ID: "own-pc", OS: "windows"}, deviceTokenHash: hashDeviceToken(testDeviceToken)}
	other := &clientConn{info: protocol.ClientInfo{ID: "other-pc", OS: "windows"}, deviceTokenHash: hashDeviceToken("abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789")}
	app.hub.clients[own.info.ID] = own
	app.hub.clients[other.info.ID] = other

	request := httptest.NewRequest(http.MethodGet, "/api/v1/clients/own-pc/terminal", nil)
	request.RemoteAddr = "198.51.100.10:50000"
	client, authError := app.terminalClientForAuth("own-pc", terminalAuth{Type: "auth", Mode: "device", Token: testDeviceToken}, request)
	if authError != "" || client != own {
		t.Fatalf("device authorization failed: %q", authError)
	}
	if _, authError := app.terminalClientForAuth("other-pc", terminalAuth{Type: "auth", Mode: "device", Token: testDeviceToken}, request); authError != "device_scope_violation" {
		t.Fatalf("other client error = %q, want device_scope_violation", authError)
	}
	client, authError = app.terminalClientForAuth("other-pc", terminalAuth{Type: "auth", Token: "admin-test-token"}, request)
	if authError != "" || client != other {
		t.Fatal("admin authorization did not retain access to the requested client")
	}
}

func TestWebConsoleIsEmbedded(t *testing.T) {
	app := newTestServer(t)
	request := httptest.NewRequest(http.MethodGet, "/connect/", nil)
	response := httptest.NewRecorder()
	app.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if !strings.Contains(response.Body.String(), "iData Console") {
		t.Fatal("web console content was not served")
	}
	for _, label := range []string{"Windows：启动 Client", "macOS：复制启动命令"} {
		if !strings.Contains(response.Body.String(), label) {
			t.Fatalf("web login action %q was not served", label)
		}
	}
	if strings.Contains(response.Body.String(), "本机设备访问令牌") {
		t.Fatal("ordinary web login still asks for a device token")
	}
	if response.Header().Get("Content-Security-Policy") == "" {
		t.Fatal("content security policy is missing")
	}
}

func TestCommandRoundTrip(t *testing.T) {
	app := newTestServer(t)
	httpServer := httptest.NewServer(app.Handler())
	defer httpServer.Close()

	header := http.Header{}
	header.Set("Authorization", "Bearer agent-test-token")
	wsURL := "ws" + strings.TrimPrefix(httpServer.URL, "http") + "/ws/agent"
	conn, response, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		if response != nil {
			t.Fatalf("dial: %v (HTTP %d)", err, response.StatusCode)
		}
		t.Fatal(err)
	}
	defer conn.Close()
	if err := conn.WriteJSON(protocol.Message{
		Type: protocol.TypeHello, ProtocolVersion: protocol.Version,
		ClientID: "test-pc", Hostname: "test-host", OS: "darwin", Arch: "arm64",
	}); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(time.Second)
	for len(app.hub.List()) != 1 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if len(app.hub.List()) != 1 {
		t.Fatal("client was not registered")
	}

	type apiResponse struct {
		status int
		body   protocol.Message
		err    error
	}
	resultChannel := make(chan apiResponse, 1)
	go func() {
		body := bytes.NewBufferString(`{"command":"printf hello","timeout_seconds":2}`)
		request, _ := http.NewRequest(http.MethodPost, httpServer.URL+"/api/v1/clients/test-pc/commands", body)
		request.Header.Set("Authorization", "Bearer admin-test-token")
		request.Header.Set("Content-Type", "application/json")
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			resultChannel <- apiResponse{err: err}
			return
		}
		defer response.Body.Close()
		var decoded protocol.Message
		err = json.NewDecoder(response.Body).Decode(&decoded)
		resultChannel <- apiResponse{status: response.StatusCode, body: decoded, err: err}
	}()

	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	var command protocol.Message
	if err := conn.ReadJSON(&command); err != nil {
		t.Fatal(err)
	}
	if command.Type != protocol.TypeCommand || command.Command != "printf hello" || command.RequestID == "" {
		t.Fatalf("unexpected command: %+v", command)
	}
	if err := conn.WriteJSON(protocol.Message{
		Type: protocol.TypeResult, ProtocolVersion: protocol.Version,
		RequestID: command.RequestID, ExitCode: 0, Stdout: "hello",
	}); err != nil {
		t.Fatal(err)
	}

	apiResult := <-resultChannel
	if apiResult.err != nil {
		t.Fatal(apiResult.err)
	}
	if apiResult.status != http.StatusOK || apiResult.body.Stdout != "hello" {
		t.Fatalf("unexpected API result: %+v", apiResult)
	}
}

func TestInteractiveTerminalRoundTrip(t *testing.T) {
	app := newTestServer(t)
	httpServer := httptest.NewServer(app.Handler())
	defer httpServer.Close()

	agentHeader := http.Header{}
	agentHeader.Set("Authorization", "Bearer agent-test-token")
	agentURL := "ws" + strings.TrimPrefix(httpServer.URL, "http") + "/ws/agent"
	agent, _, err := websocket.DefaultDialer.Dial(agentURL, agentHeader)
	if err != nil {
		t.Fatal(err)
	}
	defer agent.Close()
	if err := agent.WriteJSON(protocol.Message{
		Type: protocol.TypeHello, ProtocolVersion: protocol.Version,
		ClientID: "terminal-pc", Hostname: "test-host", OS: "windows", Arch: "amd64",
		DeviceTokenHash: hashDeviceToken(testDeviceToken), Capabilities: []string{"terminal_v1"},
	}); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for len(app.hub.List()) != 1 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}

	browserHeader := http.Header{}
	browserHeader.Set("Origin", httpServer.URL)
	browserURL := "ws" + strings.TrimPrefix(httpServer.URL, "http") + "/api/v1/clients/terminal-pc/terminal"
	browser, _, err := websocket.DefaultDialer.Dial(browserURL, browserHeader)
	if err != nil {
		t.Fatal(err)
	}
	defer browser.Close()
	if err := browser.WriteJSON(map[string]string{"type": "auth", "mode": "device", "token": testDeviceToken}); err != nil {
		t.Fatal(err)
	}

	_ = agent.SetReadDeadline(time.Now().Add(2 * time.Second))
	var open protocol.Message
	if err := agent.ReadJSON(&open); err != nil {
		t.Fatal(err)
	}
	if open.Type != protocol.TypeTerminalOpen || open.SessionID == "" {
		t.Fatalf("unexpected terminal open: %+v", open)
	}
	if err := agent.WriteJSON(protocol.Message{
		Type: protocol.TypeTerminalOpened, ProtocolVersion: protocol.Version, SessionID: open.SessionID,
	}); err != nil {
		t.Fatal(err)
	}

	_ = browser.SetReadDeadline(time.Now().Add(2 * time.Second))
	var opened protocol.Message
	if err := browser.ReadJSON(&opened); err != nil {
		t.Fatal(err)
	}
	if opened.Type != protocol.TypeTerminalOpened {
		t.Fatalf("unexpected browser message: %+v", opened)
	}
	input := []byte("cd C:\\Temp\r\n")
	if err := browser.WriteJSON(protocol.Message{Type: protocol.TypeTerminalInput, Data: input}); err != nil {
		t.Fatal(err)
	}
	var forwardedInput protocol.Message
	if err := agent.ReadJSON(&forwardedInput); err != nil {
		t.Fatal(err)
	}
	if forwardedInput.Type != protocol.TypeTerminalInput || forwardedInput.SessionID != open.SessionID || !bytes.Equal(forwardedInput.Data, input) {
		t.Fatalf("unexpected terminal input: %+v", forwardedInput)
	}

	output := []byte("C:\\Temp>")
	if err := agent.WriteJSON(protocol.Message{
		Type: protocol.TypeTerminalOutput, ProtocolVersion: protocol.Version,
		SessionID: open.SessionID, Stream: "stdout", Data: output,
	}); err != nil {
		t.Fatal(err)
	}
	var forwardedOutput protocol.Message
	if err := browser.ReadJSON(&forwardedOutput); err != nil {
		t.Fatal(err)
	}
	if forwardedOutput.Type != protocol.TypeTerminalOutput || !bytes.Equal(forwardedOutput.Data, output) {
		t.Fatalf("unexpected terminal output: %+v", forwardedOutput)
	}
}

func TestValidateHello(t *testing.T) {
	valid := protocol.Message{
		Type: protocol.TypeHello, ProtocolVersion: protocol.Version, ClientID: "pc-01",
		DeviceTokenHash: hashDeviceToken(testDeviceToken),
	}
	if err := validateHello(valid); err != nil {
		t.Fatalf("valid hello rejected: %v", err)
	}
	invalid := valid
	invalid.ClientID = "not/valid"
	if err := validateHello(invalid); err == nil {
		t.Fatal("invalid client ID accepted")
	}
	invalid = valid
	invalid.DeviceTokenHash = "not-a-valid-hash"
	if err := validateHello(invalid); err == nil {
		t.Fatal("invalid device token hash accepted")
	}
}
