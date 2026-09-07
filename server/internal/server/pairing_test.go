package server

import (
	"encoding/json"
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

func newPairingTestServer(t *testing.T) *Server {
	t.Helper()
	app, err := New(Config{
		AgentToken: "agent-test-token", AdminToken: "admin-test-token",
		DefaultTimeout: 2 * time.Second, MaxCommandTimeout: 10 * time.Second,
		BrowserPairingEnabled: true, DeviceSessionTTL: time.Hour, PairingRequestTTL: 30 * time.Second,
	}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	return app
}

func TestPairingCandidatesUseDirectSourceIPWithoutGrantingAccess(t *testing.T) {
	app := newPairingTestServer(t)
	app.hub.clients["eligible"] = &clientConn{info: protocol.ClientInfo{
		ID: "eligible", Hostname: "local", OS: "windows", RemoteAddress: "203.0.113.10:41000",
		Capabilities: []string{"browser_pairing_v1"},
	}, deviceTokenHash: hashDeviceToken(testDeviceToken)}
	app.hub.clients["wrong-ip"] = &clientConn{info: protocol.ClientInfo{
		ID: "wrong-ip", OS: "windows", RemoteAddress: "198.51.100.20:41000",
		Capabilities: []string{"browser_pairing_v1"},
	}}
	app.hub.clients["old-client"] = &clientConn{info: protocol.ClientInfo{
		ID: "old-client", OS: "windows", RemoteAddress: "203.0.113.10:41001",
	}}

	request := httptest.NewRequest(http.MethodGet, "/api/v1/pairing-candidates", nil)
	request.RemoteAddr = "203.0.113.10:52000"
	request.Header.Set("X-Forwarded-For", "198.51.100.20")
	request.Header.Set("X-Real-IP", "198.51.100.20")
	response := httptest.NewRecorder()
	app.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d; body = %s", response.Code, response.Body.String())
	}
	var body struct {
		Clients []protocol.ClientInfo `json:"clients"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Clients) != 1 || body.Clients[0].ID != "eligible" {
		t.Fatalf("candidates = %#v, want only eligible", body.Clients)
	}
	if strings.Contains(response.Body.String(), "remote_address") {
		t.Fatalf("candidate response leaked remote address: %s", response.Body.String())
	}

	selfRequest := httptest.NewRequest(http.MethodGet, "/api/v1/self", nil)
	selfRequest.RemoteAddr = request.RemoteAddr
	selfResponse := httptest.NewRecorder()
	app.Handler().ServeHTTP(selfResponse, selfRequest)
	if selfResponse.Code != http.StatusUnauthorized {
		t.Fatalf("candidate discovery granted access: status = %d", selfResponse.Code)
	}
}

func TestBrowserPairingCreatesScopedRevocableSession(t *testing.T) {
	app := newPairingTestServer(t)
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
		ClientID: "paired-pc", Hostname: "paired-host", OS: "windows", Arch: "amd64",
		DeviceTokenHash: hashDeviceToken(testDeviceToken),
		Capabilities:    []string{"terminal_v1", "browser_pairing_v1"},
	}); err != nil {
		t.Fatal(err)
	}
	waitForClient(t, app, "paired-pc")
	other := &clientConn{info: protocol.ClientInfo{ID: "other-pc", OS: "windows"}, deviceTokenHash: hashDeviceToken(strings.Repeat("a", 64))}
	app.hub.clients[other.info.ID] = other

	candidatesResponse, err := http.Get(httpServer.URL + "/api/v1/pairing-candidates")
	if err != nil {
		t.Fatal(err)
	}
	defer candidatesResponse.Body.Close()
	var candidates struct {
		Clients []protocol.ClientInfo `json:"clients"`
	}
	if err := json.NewDecoder(candidatesResponse.Body).Decode(&candidates); err != nil {
		t.Fatal(err)
	}
	if len(candidates.Clients) != 1 || candidates.Clients[0].ID != "paired-pc" {
		t.Fatalf("candidates = %#v", candidates.Clients)
	}

	startRequest, err := http.NewRequest(http.MethodPost, httpServer.URL+"/api/v1/pairings/paired-pc", nil)
	if err != nil {
		t.Fatal(err)
	}
	startRequest.Header.Set("Origin", httpServer.URL)
	startResponse, err := http.DefaultClient.Do(startRequest)
	if err != nil {
		t.Fatal(err)
	}
	defer startResponse.Body.Close()
	if startResponse.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(startResponse.Body)
		t.Fatalf("start status = %d; body = %s", startResponse.StatusCode, body)
	}
	var started pairingStart
	if err := json.NewDecoder(startResponse.Body).Decode(&started); err != nil {
		t.Fatal(err)
	}
	if !pairingIDPattern.MatchString(started.PairingID) || started.PollToken == "" || !strings.HasPrefix(started.Challenge, "PAIR IDATA ") {
		t.Fatalf("invalid pairing start: %#v", started)
	}

	_ = agent.SetReadDeadline(time.Now().Add(2 * time.Second))
	var pairingRequest protocol.Message
	if err := agent.ReadJSON(&pairingRequest); err != nil {
		t.Fatal(err)
	}
	if pairingRequest.Type != protocol.TypePairingRequest || pairingRequest.PairingID != started.PairingID || pairingRequest.Challenge != started.Challenge {
		t.Fatalf("unexpected Client pairing request: %#v", pairingRequest)
	}
	if pairingRequest.BrowserIP == "" || pairingRequest.ServerHost == "" || pairingRequest.SessionTTL != 3600 {
		t.Fatalf("pairing request is missing display context: %#v", pairingRequest)
	}

	secondRequest, _ := http.NewRequest(http.MethodPost, httpServer.URL+"/api/v1/pairings/paired-pc", nil)
	secondRequest.Header.Set("Origin", httpServer.URL)
	secondResponse, err := http.DefaultClient.Do(secondRequest)
	if err != nil {
		t.Fatal(err)
	}
	secondResponse.Body.Close()
	if secondResponse.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("second pairing status = %d, want 429", secondResponse.StatusCode)
	}

	if err := agent.WriteJSON(protocol.Message{
		Type: protocol.TypePairingResult, ProtocolVersion: protocol.Version,
		PairingID: started.PairingID, Approved: true,
	}); err != nil {
		t.Fatal(err)
	}

	var sessionCookie *http.Cookie
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		pollRequest, _ := http.NewRequest(http.MethodPost, httpServer.URL+"/api/v1/pairings/"+started.PairingID+"/status", nil)
		pollRequest.Header.Set("Origin", httpServer.URL)
		pollRequest.Header.Set("Authorization", "Pairing "+started.PollToken)
		pollResponse, err := http.DefaultClient.Do(pollRequest)
		if err != nil {
			t.Fatal(err)
		}
		if pollResponse.StatusCode == http.StatusOK {
			var result map[string]string
			if err := json.NewDecoder(pollResponse.Body).Decode(&result); err != nil {
				pollResponse.Body.Close()
				t.Fatal(err)
			}
			for _, cookie := range pollResponse.Cookies() {
				if cookie.Name == deviceSessionCookie {
					sessionCookie = cookie
				}
			}
			pollResponse.Body.Close()
			if result["status"] != "approved" {
				t.Fatalf("pairing result = %#v", result)
			}
			break
		}
		pollResponse.Body.Close()
		time.Sleep(10 * time.Millisecond)
	}
	if sessionCookie == nil || !sessionCookie.HttpOnly || sessionCookie.SameSite != http.SameSiteStrictMode || sessionCookie.MaxAge <= 0 {
		t.Fatalf("invalid session cookie: %#v", sessionCookie)
	}

	selfRequest, _ := http.NewRequest(http.MethodGet, httpServer.URL+"/api/v1/self", nil)
	selfRequest.AddCookie(sessionCookie)
	selfResponse, err := http.DefaultClient.Do(selfRequest)
	if err != nil {
		t.Fatal(err)
	}
	defer selfResponse.Body.Close()
	var self struct {
		SelfClientID string `json:"self_client_id"`
		AuthMode     string `json:"auth_mode"`
	}
	if err := json.NewDecoder(selfResponse.Body).Decode(&self); err != nil {
		t.Fatal(err)
	}
	if selfResponse.StatusCode != http.StatusOK || self.SelfClientID != "paired-pc" || self.AuthMode != "session" {
		t.Fatalf("self = %#v, status = %d", self, selfResponse.StatusCode)
	}
	terminalRequest := httptest.NewRequest(http.MethodGet, "/api/v1/clients/paired-pc/terminal", nil)
	terminalRequest.RemoteAddr = candidatesResponse.Request.RemoteAddr
	terminalRequest.AddCookie(sessionCookie)
	// The httptest client does not expose its local address. Use the address recorded in the session.
	app.pairings.mu.Lock()
	for _, session := range app.pairings.sessions {
		terminalRequest.RemoteAddr = session.browserIP.String() + ":55000"
	}
	app.pairings.mu.Unlock()
	client, authError := app.terminalClientForAuth("paired-pc", terminalAuth{Type: "auth", Mode: "session"}, terminalRequest)
	if authError != "" || client == nil || client.info.ID != "paired-pc" {
		t.Fatalf("session authorization failed: client=%#v error=%q", client, authError)
	}
	if _, authError := app.terminalClientForAuth("other-pc", terminalAuth{Type: "auth", Mode: "session"}, terminalRequest); authError != "device_scope_violation" {
		t.Fatalf("other client error = %q, want device_scope_violation", authError)
	}
	wrongIPRequest := terminalRequest.Clone(terminalRequest.Context())
	wrongIPRequest.RemoteAddr = "192.0.2.44:55000"
	if _, authError := app.terminalClientForAuth("paired-pc", terminalAuth{Type: "auth", Mode: "session"}, wrongIPRequest); authError != "unauthorized" {
		t.Fatalf("different IP error = %q, want unauthorized", authError)
	}

	logoutRequest, _ := http.NewRequest(http.MethodPost, httpServer.URL+"/api/v1/self/logout", nil)
	logoutRequest.Header.Set("Origin", httpServer.URL)
	logoutRequest.AddCookie(sessionCookie)
	logoutResponse, err := http.DefaultClient.Do(logoutRequest)
	if err != nil {
		t.Fatal(err)
	}
	logoutResponse.Body.Close()
	if logoutResponse.StatusCode != http.StatusNoContent {
		t.Fatalf("logout status = %d", logoutResponse.StatusCode)
	}
	selfAfterLogout, _ := http.NewRequest(http.MethodGet, httpServer.URL+"/api/v1/self", nil)
	selfAfterLogout.AddCookie(sessionCookie)
	afterResponse, err := http.DefaultClient.Do(selfAfterLogout)
	if err != nil {
		t.Fatal(err)
	}
	afterResponse.Body.Close()
	if afterResponse.StatusCode != http.StatusUnauthorized {
		t.Fatalf("session still valid after logout: status = %d", afterResponse.StatusCode)
	}
}

func TestPairingMutationRequiresSameOrigin(t *testing.T) {
	app := newPairingTestServer(t)
	for _, origin := range []string{"", "http://attacker.example"} {
		request := httptest.NewRequest(http.MethodPost, "/api/v1/pairings/pc", nil)
		request.RemoteAddr = "203.0.113.10:52000"
		request.Host = "idata.example"
		if origin != "" {
			request.Header.Set("Origin", origin)
		}
		response := httptest.NewRecorder()
		app.Handler().ServeHTTP(response, request)
		if response.Code != http.StatusForbidden {
			t.Fatalf("origin %q: status = %d, want 403", origin, response.Code)
		}
	}
}

func waitForClient(t *testing.T, app *Server, clientID string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for app.hub.get(clientID) == nil && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if app.hub.get(clientID) == nil {
		t.Fatalf("client %q was not registered", clientID)
	}
}
