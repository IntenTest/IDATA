package server

import (
	"github.com/gorilla/websocket"
	"idata-server/internal/protocol"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestIDATAAuthorizationAndAssets(t *testing.T) {
	app := newTestServer(t)
	for _, tc := range []struct {
		path, origin string
		status       int
	}{
		{"/idata/", "", 200}, {"/idata/vendor/vue-3.5.24/vue.global.prod.js", "", 200},
		{"/connect/", "", 200}, {"/console/app.js", "", 200},
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
	_ = agent.WriteJSON(protocol.Message{Type: protocol.TypeHello, ProtocolVersion: protocol.Version, ClientID: "pc", OS: "darwin", Capabilities: []string{"idata_api_v1"}})
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
		if request.Type != protocol.TypeAPIRequest || request.Path != "/api/settings" || request.Method != "PUT" || string(request.Data) != `{"settings":{}}` {
			t.Fatalf("unexpected request %+v", request)
		}
		want := 200
		if disconnect {
			agent.Close()
			want = 502
		} else {
			_ = agent.WriteJSON(protocol.Message{Type: protocol.TypeAPIResponse, ProtocolVersion: protocol.Version, RequestID: request.RequestID, Status: 200, Data: []byte(`{"settings":{}}`)})
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
