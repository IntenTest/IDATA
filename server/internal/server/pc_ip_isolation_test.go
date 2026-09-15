package server

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"idata-server/internal/protocol"
)

// Simulate distinct LAN PC addresses behind one proxy TCP peer. The test-only
// source selector is consumed by the proxy; production uses Nginx $remote_addr.
func TestTwoPCsBehindNginxRouteOnlyToTheirOwnClient(t *testing.T) {
	app := newTestServer(t)
	app.proxyTrust, _ = newProxyTrust("127.0.0.1")
	backend := httptest.NewServer(app.Handler())
	defer backend.Close()
	target, _ := url.Parse(backend.URL)
	proxy := httputil.NewSingleHostReverseProxy(target)
	original := proxy.Director
	proxy.Director = func(r *http.Request) {
		original(r)
		r.Header.Set("X-Real-IP", r.Header.Get("X-Test-Source-IP"))
		r.Header.Del("X-Test-Source-IP")
	}
	gateway := httptest.NewServer(proxy)
	defer gateway.Close()
	type pc struct {
		id, ip string
		socket *websocket.Conn
		cookie *http.Cookie
		calls  atomic.Int32
		done   chan struct{}
	}
	computers := []*pc{{id: "a-pc", ip: "192.0.2.10"}, {id: "b-pc", ip: "192.0.2.20"}}
	request := func(computer *pc, method, path string) (*http.Response, []byte) {
		t.Helper()
		r, _ := http.NewRequest(method, gateway.URL+path, strings.NewReader(`{}`))
		r.Header.Set("Origin", gateway.URL)
		r.Header.Set("X-Test-Source-IP", computer.ip)
		if computer.cookie != nil {
			r.AddCookie(computer.cookie)
		}
		resp, err := (&http.Client{Timeout: 3 * time.Second}).Do(r)
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			t.Fatal(err)
		}
		return resp, data
	}
	for _, computer := range computers {
		socket, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(gateway.URL, "http")+"/ws/agent", http.Header{"Authorization": {"Bearer agent-test-token"}, "X-Test-Source-IP": {computer.ip}})
		if err != nil {
			t.Fatal(err)
		}
		computer.socket = socket
		computer.done = make(chan struct{})
		if err := socket.WriteJSON(protocol.Message{Type: protocol.TypeHello, ProtocolVersion: protocol.Version, ClientID: computer.id, OS: "windows", Capabilities: []string{"server_commands_v1", "command_stdin_v1", "terminal_v1"}}); err != nil {
			t.Fatal(err)
		}
		go func(computer *pc) {
			defer close(computer.done)
			for {
				var message protocol.Message
				if err := computer.socket.ReadJSON(&message); err != nil {
					return
				}
				if message.Type != protocol.TypeCommand {
					return
				}
				call := computer.calls.Add(1)
				result := fmt.Sprintf(`{"ok":true,"data":{"pc":%q}}`, computer.id)
				if call%3 == 0 {
					result = fmt.Sprintf(`{"ok":true,"data":{"contentBase64":%q}}`, base64.StdEncoding.EncodeToString([]byte(computer.id)))
				}
				if err := computer.socket.WriteJSON(protocol.Message{Type: protocol.TypeResult, ProtocolVersion: protocol.Version, RequestID: message.RequestID, ExitCode: 0, Stdout: result}); err != nil {
					return
				}
			}
		}(computer)
		defer func(computer *pc) { computer.socket.Close(); <-computer.done }(computer)
	}
	deadline := time.Now().Add(time.Second)
	for len(app.hub.List()) != 2 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if len(app.hub.List()) != 2 {
		t.Fatal("PCs did not register")
	}
	for index, computer := range computers {
		other := computers[1-index]
		resp, data := request(computer, "POST", "/api/v1/ip-login")
		if resp.StatusCode != 200 || len(resp.Cookies()) != 1 {
			t.Fatalf("login %s: %d %s", computer.id, resp.StatusCode, data)
		}
		computer.cookie = resp.Cookies()[0]
		resp, data = request(computer, "GET", "/api/v1/self?client="+other.id)
		var self struct {
			Clients []protocol.ClientInfo `json:"clients"`
		}
		if err := json.Unmarshal(data, &self); err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != 200 || len(self.Clients) != 1 || self.Clients[0].ID != computer.id || self.Clients[0].RemoteAddress != "" {
			t.Fatalf("%s saw incorrect devices: %s", computer.id, data)
		}
		for _, operation := range []struct{ method, path string }{{"GET", "devices"}, {"POST", "test-runs"}, {"GET", "test-runs/TR-1/reports/1/content"}} {
			resp, data = request(computer, operation.method, "/api/v1/clients/"+computer.id+"/idata/"+operation.path)
			if resp.StatusCode != 200 || !strings.Contains(string(data), computer.id) {
				t.Fatalf("%s operation %s routed incorrectly: status=%d body=%s", computer.id, operation.path, resp.StatusCode, data)
			}
			resp, _ = request(computer, operation.method, "/api/v1/clients/"+other.id+"/idata/"+operation.path)
			if resp.StatusCode != 403 {
				t.Fatalf("%s operation reached %s: %d", computer.id, other.id, resp.StatusCode)
			}
		}
		header := http.Header{"Origin": {gateway.URL}, "Cookie": {computer.cookie.String()}, "X-Test-Source-IP": {computer.ip}}
		terminal, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(gateway.URL, "http")+"/api/v1/clients/"+other.id+"/terminal", header)
		if err != nil {
			t.Fatal(err)
		}
		_ = terminal.SetReadDeadline(time.Now().Add(time.Second))
		err = terminal.WriteJSON(map[string]string{"type": "auth", "mode": "ip_session"})
		if err != nil {
			t.Fatal(err)
		}
		_, result, err := terminal.ReadMessage()
		terminal.Close()
		if err != nil || !strings.Contains(string(result), "device_scope_violation") {
			t.Fatalf("cross-PC terminal was not refused: %s %v", result, err)
		}
	}
	for _, computer := range computers {
		if computer.calls.Load() != 3 {
			t.Fatalf("%s handled unexpected operations: %d", computer.id, computer.calls.Load())
		}
	}
	// Losing A must never make A's browser fall back to the still-online B.
	computers[0].socket.Close()
	<-computers[0].done
	deadline = time.Now().Add(time.Second)
	for app.hub.get("a-pc") != nil && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	resp, data := request(computers[0], "GET", "/api/v1/self")
	if resp.StatusCode != 200 || strings.Contains(string(data), "b-pc") {
		t.Fatalf("offline PC fell back to B: %s", data)
	}
}
