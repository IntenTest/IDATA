package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"idata-server/internal/protocol"
)

func TestIPLoginAuthorizesOnlyTheUniqueSameIPClient(t *testing.T) {
	app := newPairingTestServer(t)
	first := &clientConn{info: protocol.ClientInfo{ID: "first-pc", OS: "windows", RemoteAddress: "203.0.113.10:41000"}}
	second := &clientConn{info: protocol.ClientInfo{ID: "second-pc", OS: "darwin", RemoteAddress: "203.0.113.11:41001"}}
	other := &clientConn{info: protocol.ClientInfo{ID: "other-ip-pc", OS: "windows", RemoteAddress: "198.51.100.20:41000"}}
	app.hub.clients[first.info.ID] = first
	app.hub.clients[second.info.ID] = second
	app.hub.clients[other.info.ID] = other

	loginRequest := httptest.NewRequest(http.MethodPost, "http://idata.example/api/v1/ip-login", nil)
	loginRequest.RemoteAddr = "203.0.113.10:52000"
	loginRequest.Header.Set("Origin", "http://idata.example")
	loginRequest.Header.Set("X-Forwarded-For", "198.51.100.20")
	loginResponse := httptest.NewRecorder()
	app.Handler().ServeHTTP(loginResponse, loginRequest)
	if loginResponse.Code != http.StatusOK {
		t.Fatalf("login status = %d; body = %s", loginResponse.Code, loginResponse.Body.String())
	}
	var loginBody struct {
		Status      string `json:"status"`
		ClientCount int    `json:"client_count"`
		BrowserIP   string `json:"browser_ip"`
		ClientIP    string `json:"client_ip"`
	}
	if err := json.Unmarshal(loginResponse.Body.Bytes(), &loginBody); err != nil {
		t.Fatal(err)
	}
	if loginBody.Status != "approved" || loginBody.ClientCount != 1 || loginBody.BrowserIP != "203.0.113.10" || loginBody.ClientIP != loginBody.BrowserIP {
		t.Fatalf("login response = %#v", loginBody)
	}
	result := loginResponse.Result()
	defer result.Body.Close()
	var sessionCookie *http.Cookie
	for _, cookie := range result.Cookies() {
		if cookie.Name == deviceSessionCookie {
			sessionCookie = cookie
		}
	}
	if sessionCookie == nil || !sessionCookie.HttpOnly || sessionCookie.SameSite != http.SameSiteStrictMode {
		t.Fatalf("invalid session cookie: %#v", sessionCookie)
	}

	selfRequest := httptest.NewRequest(http.MethodGet, "http://idata.example/api/v1/self", nil)
	selfRequest.RemoteAddr = "203.0.113.10:53000"
	selfRequest.AddCookie(sessionCookie)
	selfResponse := httptest.NewRecorder()
	app.Handler().ServeHTTP(selfResponse, selfRequest)
	if selfResponse.Code != http.StatusOK {
		t.Fatalf("self status = %d; body = %s", selfResponse.Code, selfResponse.Body.String())
	}
	var selfBody struct {
		AuthMode  string                `json:"auth_mode"`
		Clients   []protocol.ClientInfo `json:"clients"`
		BrowserIP string                `json:"browser_ip"`
		ClientIP  string                `json:"client_ip"`
	}
	if err := json.Unmarshal(selfResponse.Body.Bytes(), &selfBody); err != nil {
		t.Fatal(err)
	}
	if selfBody.AuthMode != "ip_session" || len(selfBody.Clients) != 1 || selfBody.Clients[0].ID != "first-pc" || selfBody.BrowserIP != "203.0.113.10" || selfBody.ClientIP != selfBody.BrowserIP {
		t.Fatalf("self response = %#v", selfBody)
	}
	if selfBody.Clients[0].RemoteAddress != "" {
		t.Fatal("self response leaked a Client remote address")
	}

	terminalRequest := httptest.NewRequest(http.MethodGet, "http://idata.example/api/v1/clients/first-pc/terminal", nil)
	terminalRequest.RemoteAddr = "203.0.113.10:54000"
	terminalRequest.AddCookie(sessionCookie)
	for _, clientID := range []string{"first-pc"} {
		client, authError := app.terminalClientForAuth(clientID, terminalAuth{Type: "auth", Mode: "ip_session"}, terminalRequest)
		if authError != "" || client == nil || client.info.ID != clientID {
			t.Fatalf("%s authorization failed: client=%#v error=%q", clientID, client, authError)
		}
	}
	if _, authError := app.terminalClientForAuth("other-ip-pc", terminalAuth{Type: "auth", Mode: "ip_session"}, terminalRequest); authError != "device_scope_violation" {
		t.Fatalf("different-IP client error = %q, want device_scope_violation", authError)
	}

	wrongBrowserIP := terminalRequest.Clone(terminalRequest.Context())
	wrongBrowserIP.RemoteAddr = "192.0.2.30:54000"
	if _, authError := app.terminalClientForAuth("first-pc", terminalAuth{Type: "auth", Mode: "ip_session"}, wrongBrowserIP); authError != "device_scope_violation" {
		t.Fatalf("moved browser error = %q, want device_scope_violation", authError)
	}
}

func TestIPLoginWaitsForAClientAndRequiresSameOrigin(t *testing.T) {
	app := newPairingTestServer(t)

	missingOrigin := httptest.NewRequest(http.MethodPost, "http://idata.example/api/v1/ip-login", nil)
	missingOrigin.RemoteAddr = "203.0.113.10:52000"
	missingOriginResponse := httptest.NewRecorder()
	app.Handler().ServeHTTP(missingOriginResponse, missingOrigin)
	if missingOriginResponse.Code != http.StatusForbidden {
		t.Fatalf("missing-origin status = %d, want 403", missingOriginResponse.Code)
	}

	waiting := httptest.NewRequest(http.MethodPost, "http://idata.example/api/v1/ip-login", nil)
	waiting.RemoteAddr = "203.0.113.10:52000"
	waiting.Header.Set("Origin", "http://idata.example")
	waitingResponse := httptest.NewRecorder()
	app.Handler().ServeHTTP(waitingResponse, waiting)
	if waitingResponse.Code != http.StatusAccepted || len(waitingResponse.Result().Cookies()) != 0 {
		t.Fatalf("waiting response status = %d cookies = %#v", waitingResponse.Code, waitingResponse.Result().Cookies())
	}
	var waitingBody struct {
		Status    string `json:"status"`
		BrowserIP string `json:"browser_ip"`
		ClientIP  string `json:"client_ip"`
	}
	if json.Unmarshal(waitingResponse.Body.Bytes(), &waitingBody) != nil || waitingBody.Status != "waiting" || waitingBody.BrowserIP != "203.0.113.10" || waitingBody.ClientIP != "" {
		t.Fatalf("waiting response body = %s", waitingResponse.Body.String())
	}
}

func TestDuplicatePCAddressDoesNotChooseAClient(t *testing.T) {
	app := newTestServer(t)
	for _, id := range []string{"one", "two"} {
		app.hub.clients[id] = &clientConn{info: protocol.ClientInfo{ID: id, RemoteAddress: "192.0.2.10:40000"}}
	}
	r := httptest.NewRequest("POST", "http://server.example/api/v1/ip-login", nil)
	r.RemoteAddr = "192.0.2.10:50000"
	r.Header.Set("Origin", "http://server.example")
	w := httptest.NewRecorder()
	app.Handler().ServeHTTP(w, r)
	if w.Code != 409 || len(w.Result().Cookies()) != 0 {
		t.Fatal("ambiguous PC address was automatically selected")
	}
}
