package agent

import (
	"context"
	"idata-client/internal/protocol"
	"net"
	"net/http"
	"strings"
	"testing"
)

func TestIDATARejectsArbitraryTargets(t *testing.T) {
	for _, path := range []string{"http://example.com", "/api/../config/settings.json", "/api/test-runs?x=1", "/api/test-runs/../../secret", "/api/test-runs/%2f/close", "/api/unknown"} {
		result := forwardIDATA(context.Background(), protocol.Message{RequestID: "test", Method: "GET", Path: path})
		if result.Status != 400 {
			t.Fatalf("%s: %d", path, result.Status)
		}
	}
	result := forwardIDATA(context.Background(), protocol.Message{RequestID: "test", Method: "PUT", Path: "/api/settings", Data: []byte(strings.Repeat("a", 65537))})
	if result.Status != 400 {
		t.Fatal("oversized request accepted")
	}
}

func TestIDATALoopbackRoundTrip(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:54321")
	if err != nil {
		t.Skip("IDATA local service already occupies its fixed port")
	}
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/api/test-runs" {
			t.Error("operation changed")
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(202)
		_, _ = w.Write([]byte(`{"run":{"id":"local-run"}}`))
	})}
	go server.Serve(listener)
	defer server.Close()
	response := forwardIDATA(context.Background(), protocol.Message{RequestID: "roundtrip", Method: "POST", Path: "/api/test-runs", Data: []byte(`{}`)})
	if response.Status != 202 || !strings.Contains(string(response.Data), "local-run") || response.RequestID != "roundtrip" {
		t.Fatalf("unexpected result: %+v", response)
	}
}
