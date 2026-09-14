package server

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"idata-server/internal/protocol"
)

// Exercise the public HTTP origin through a real reverse-proxy connection,
// including both WebSocket hops and cookie-based workspace authorization.
func TestDomainHTTPReverseProxy(t *testing.T) {
	for _, tc := range []struct{ origin, prefix string }{{"http://idata.test.huawei.com", ""}, {"http://arbitrary.example:18080", "/team/tools"}, {"https://secure.example:8443", "/workspace"}} {
		t.Run(tc.origin+tc.prefix, func(t *testing.T) { testDomainProxy(t, tc.origin, tc.prefix) })
	}
}

func testDomainProxy(t *testing.T, origin, prefix string) {
	app := newTestServer(t)
	app.proxyTrust, _ = newProxyTrust("127.0.0.1,::1")
	if strings.HasPrefix(origin, "https:") {
		app.config.PublicOrigin = origin
	}
	backend := httptest.NewServer(app.Handler())
	defer backend.Close()
	target, _ := url.Parse(backend.URL)
	proxy := httputil.NewSingleHostReverseProxy(target)
	director := proxy.Director
	proxy.Director = func(r *http.Request) {
		director(r)
		r.URL.Path = strings.TrimPrefix(r.URL.Path, prefix)
		r.Header.Set("X-Real-IP", "192.0.2.10")
		r.Header.Set("X-Forwarded-Proto", "http")
	}
	gateway := httptest.NewUnstartedServer(proxy)
	if strings.HasPrefix(origin, "https:") {
		gateway.StartTLS()
	} else {
		gateway.Start()
	}
	defer gateway.Close()
	gatewayURL, _ := url.Parse(gateway.URL)
	dial := func(ctx context.Context, network, address string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, gatewayURL.Host)
	}
	transport := &http.Transport{DialContext: dial}
	if gateway.TLS != nil {
		transport.TLSClientConfig = gateway.Client().Transport.(*http.Transport).TLSClientConfig.Clone()
		transport.TLSClientConfig.ServerName = "example.com"
	}
	defer transport.CloseIdleConnections()
	browserHTTP := &http.Client{Transport: transport, Timeout: 3 * time.Second}
	wsDialer := websocket.Dialer{NetDialContext: dial, HandshakeTimeout: 3 * time.Second, TLSClientConfig: transport.TLSClientConfig}
	wsOrigin := "ws" + strings.TrimPrefix(origin, "http") + prefix
	publicURL := origin + prefix
	page, err := browserHTTP.Get(publicURL + "/")
	if err != nil {
		t.Fatal(err)
	}
	html, _ := io.ReadAll(page.Body)
	page.Body.Close()
	if page.StatusCode != 200 || !strings.Contains(string(html), `href="./vendor/`) || !strings.Contains(string(html), `name="idata-api-base" content="./"`) {
		t.Fatal("deployment assets or API base are incorrect")
	}
	agent, _, err := wsDialer.Dial(wsOrigin+"/ws/agent", http.Header{"Authorization": {"Bearer agent-test-token"}})
	if err != nil {
		t.Fatal(err)
	}
	defer agent.Close()
	if err := agent.WriteJSON(protocol.Message{Type: protocol.TypeHello, ProtocolVersion: protocol.Version, ClientID: "domain-pc", OS: "windows", DeviceTokenHash: hashDeviceToken(testDeviceToken), Capabilities: []string{"idata_api_v1", "terminal_v1"}}); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for app.hub.get("domain-pc") == nil && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if app.hub.get("domain-pc") == nil {
		t.Fatal("proxied agent did not register")
	}
	request, _ := http.NewRequest("POST", publicURL+"/api/v1/ip-login", nil)
	request.Header.Set("Origin", origin)
	response, err := browserHTTP.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 200 || len(response.Cookies()) == 0 {
		t.Fatalf("proxied login: %d", response.StatusCode)
	}
	cookie := response.Cookies()[0]
	if cookie.Secure != strings.HasPrefix(origin, "https:") {
		t.Fatal("incorrect public cookie security")
	}
	request, _ = http.NewRequest("GET", publicURL+"/api/v1/self", nil)
	request.AddCookie(cookie)
	response, err = browserHTTP.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	var self struct {
		Clients []protocol.ClientInfo `json:"clients"`
	}
	err = json.NewDecoder(response.Body).Decode(&self)
	response.Body.Close()
	if err != nil || response.StatusCode != 200 || len(self.Clients) != 1 || self.Clients[0].ID != "domain-pc" {
		t.Fatalf("proxied device discovery: %+v, %v", self, err)
	}
	done := make(chan error, 1)
	go func() {
		_ = agent.SetReadDeadline(time.Now().Add(3 * time.Second))
		var message protocol.Message
		if err := agent.ReadJSON(&message); err != nil {
			done <- err
			return
		}
		if message.Type != protocol.TypeAPIRequest || message.Path != "/api/settings" {
			done <- io.ErrUnexpectedEOF
			return
		}
		done <- agent.WriteJSON(protocol.Message{Type: protocol.TypeAPIResponse, ProtocolVersion: protocol.Version, RequestID: message.RequestID, Status: 200, Data: []byte(`{"settings":{}}`)})
	}()
	request, _ = http.NewRequest("GET", publicURL+"/api/v1/clients/domain-pc/idata/settings", nil)
	request.AddCookie(cookie)
	response, err = browserHTTP.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(response.Body)
	response.Body.Close()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != 200 || !strings.Contains(string(body), "settings") {
		t.Fatalf("proxied workspace: %d %s", response.StatusCode, body)
	}
	header := http.Header{"Origin": {origin}, "Cookie": {cookie.String()}}
	terminal, _, err := wsDialer.Dial(wsOrigin+"/api/v1/clients/domain-pc/terminal", header)
	if err != nil {
		t.Fatal(err)
	}
	defer terminal.Close()
	if err := terminal.WriteJSON(map[string]string{"type": "auth", "mode": "ip_session"}); err != nil {
		t.Fatal(err)
	}
	_ = agent.SetReadDeadline(time.Now().Add(3 * time.Second))
	var open protocol.Message
	if err := agent.ReadJSON(&open); err != nil {
		t.Fatal(err)
	}
	if open.Type != protocol.TypeTerminalOpen {
		t.Fatalf("unexpected terminal message: %s", open.Type)
	}
	if err := agent.WriteJSON(protocol.Message{Type: protocol.TypeTerminalOpened, ProtocolVersion: protocol.Version, SessionID: open.SessionID}); err != nil {
		t.Fatal(err)
	}
	_ = terminal.SetReadDeadline(time.Now().Add(3 * time.Second))
	var opened protocol.Message
	if err := terminal.ReadJSON(&opened); err != nil {
		t.Fatal(err)
	}
	if opened.Type != protocol.TypeTerminalOpened {
		t.Fatalf("terminal not opened: %s", opened.Type)
	}
	header.Set("Origin", "http://other.example")
	rejected, denial, err := wsDialer.Dial(wsOrigin+"/api/v1/clients/domain-pc/terminal", header)
	if rejected != nil {
		rejected.Close()
	}
	if denial != nil {
		denial.Body.Close()
	}
	if err == nil || denial == nil || denial.StatusCode != http.StatusForbidden {
		t.Fatal("cross-origin terminal was not rejected")
	}
}
