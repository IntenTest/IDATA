package server

import (
	"idata-server/internal/protocol"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestProxySourceAddressNormalization(t *testing.T) {
	trust, err := newProxyTrust("127.0.0.1,2001:db8:1::/64")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		peer, real, want string
		status           int
	}{
		{"127.0.0.1:50000", "192.0.2.10", "192.0.2.10:50000", 204},
		{"[2001:db8:1::2]:50000", "2001:db8:2::3", "[2001:db8:2::3]:50000", 204},
		{"192.0.2.20:50000", "192.0.2.10", "192.0.2.20:50000", 204},
		{"127.0.0.1:50000", "", "", 400},
		{"127.0.0.1:50000", "192.0.2.10,192.0.2.20", "", 400},
	} {
		r := httptest.NewRequest("GET", "/api/v1/self", nil)
		r.RemoteAddr = tc.peer
		r.Header.Set("X-Real-IP", tc.real)
		r.Header.Set("X-Forwarded-For", "198.51.100.99")
		w := httptest.NewRecorder()
		trust.handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.RemoteAddr != tc.want {
				t.Errorf("source=%s want=%s", r.RemoteAddr, tc.want)
			}
			w.WriteHeader(204)
		})).ServeHTTP(w, r)
		if w.Code != tc.status {
			t.Errorf("%s: %d want %d", tc.peer, w.Code, tc.status)
		}
	}
	for _, bad := range []string{"*", "not-an-IP", "127.0.0.1,"} {
		if _, err := newProxyTrust(bad); err == nil {
			t.Fatalf("accepted proxy configuration %q", bad)
		}
	}
}

func TestDifferentPCAddressesMayUseTheSameClientID(t *testing.T) {
	hub := NewHub()
	own := &clientConn{info: protocol.ClientInfo{ID: "same-name", RemoteAddress: "192.0.2.10:40000"}, deviceTokenHash: "first-token"}
	other := &clientConn{info: protocol.ClientInfo{ID: "same-name", RemoteAddress: "192.0.2.20:40000"}, deviceTokenHash: "second-token"}
	if err := hub.register(own); err != nil {
		t.Fatal(err)
	}
	if err := hub.register(other); err != nil {
		t.Fatal(err)
	}
	if hub.get("same-name") != nil {
		t.Fatal("an ambiguous global Client ID must not select either PC")
	}
	if client, err := hub.clientForIP("same-name", "192.0.2.10:50000"); err != nil || client != own {
		t.Fatal("first PC was not addressable in its own IP scope")
	}
	if client, err := hub.clientForIP("same-name", "192.0.2.20:50000"); err != nil || client != other {
		t.Fatal("second PC was not addressable in its own IP scope")
	}
	if hub.clientForSession("same-name", "first-token") != own || hub.clientForSession("same-name", "second-token") != other {
		t.Fatal("device sessions were not isolated when Client IDs matched")
	}
}
