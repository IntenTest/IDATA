package agent

import (
	"context"
	"errors"
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

func TestIDATAReadinessFailurePreventsForwarding(t *testing.T) {
	calls := 0
	ensure := func(context.Context) error { calls++; return errors.New("worker failed") }
	invalid := forwardIDATAWithService(context.Background(), protocol.Message{RequestID: "invalid", Method: "GET", Path: "/api/unknown"}, ensure)
	if invalid.Status != 400 || calls != 0 {
		t.Fatal("invalid operation triggered worker startup")
	}
	result := forwardIDATAWithService(context.Background(), protocol.Message{RequestID: "blocked", Method: "POST", Path: "/api/test-runs", Data: []byte(`{}`)}, ensure)
	if result.Status != 503 || calls != 1 || result.RequestID != "blocked" {
		t.Fatalf("unexpected result: %+v, calls=%d", result, calls)
	}
}

func TestIDATAArchiveUpdateRoutes(t *testing.T) {
	if !idataReadPath.MatchString("/api/test-cases/update") || !idataWritePath.MatchString("/api/test-cases/update") {
		t.Fatal("archive update start and status routes must be forwarded")
	}
	for _, path := range []string{"/api/test-cases/update/extra", "/api/test-cases/update?url=http://example.com", "/api/test-cases/delete"} {
		if idataReadPath.MatchString(path) || idataWritePath.MatchString(path) {
			t.Fatalf("unexpected route allowed: %s", path)
		}
	}
}
