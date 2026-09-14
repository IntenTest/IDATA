package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPublicOriginValidationAndHeaders(t *testing.T) {
	for _, origin := range []string{"", "http://server.example:8080", "https://server.example"} {
		if err := validatePublicOrigin(origin); err != nil {
			t.Fatal(err)
		}
	}
	for _, origin := range []string{"ftp://server.example", "https://user:pass@server.example", "https://server.example/path", "https://server.example?", "https://server.example#x"} {
		if err := validatePublicOrigin(origin); err == nil {
			t.Fatalf("accepted %q", origin)
		}
	}
	request := httptest.NewRequest("GET", "http://server.example/", nil)
	request.Header.Set("X-Forwarded-Proto", "https")
	if requestIsSecure(request) {
		t.Fatal("trusted an unconfigured forwarding header")
	}
	handler := publicOriginHandler("https://server.example", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !requestIsSecure(r) {
			t.Error("external TLS not recognized")
		}
		if r.RemoteAddr != request.RemoteAddr {
			t.Error("changed source-IP authorization")
		}
		w.WriteHeader(204)
	}))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != 204 {
		t.Fatal(response.Code)
	}
	wrong := httptest.NewRequest("GET", "http://other.example/", nil)
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, wrong)
	if response.Code != 403 {
		t.Fatal("accepted incorrect public host")
	}
}
