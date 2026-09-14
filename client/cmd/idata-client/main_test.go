package main

import (
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateServerURL(t *testing.T) {
	tests := []struct {
		name          string
		url           string
		allowInsecure bool
		wantError     bool
	}{
		{name: "secure", url: "wss://idata.example.com/ws/agent"},
		{name: "localhost", url: "ws://127.0.0.1/ws/agent"},
		{name: "ipv6 localhost", url: "ws://[::1]/ws/agent"},
		{name: "remote plaintext rejected", url: "ws://10.0.0.2/ws/agent", wantError: true},
		{name: "remote plaintext explicitly allowed", url: "ws://10.0.0.2/ws/agent", allowInsecure: true},
		{name: "http rejected", url: "https://idata.example.com/ws/agent", wantError: true},
		{name: "empty rejected", url: "", wantError: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateServerURL(test.url, test.allowInsecure)
			if (err != nil) != test.wantError {
				t.Fatalf("validateServerURL() error = %v, wantError = %v", err, test.wantError)
			}
		})
	}
}

func TestLoadFileConfig(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(filepath.Dir(executable), "idata-client.json")
	contents := []byte(`{"server_url":"ws://127.0.0.1/ws/agent","agent_token":"test-token","client_id":"windows-test","device_token":"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef","output_limit":2048,"allow_insecure":false,"browser_bridge_address":"127.0.0.1:19000","confirm_browser_pairing":false,"register_url_protocol":false}`)
	if err := os.WriteFile(configPath, contents, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(configPath) })

	config, err := loadFileConfig()
	if err != nil {
		t.Fatal(err)
	}
	if config.ServerURL != "ws://127.0.0.1/ws/agent" || config.AgentToken != "test-token" || config.ClientID != "windows-test" {
		t.Fatalf("unexpected config: %#v", config)
	}
	if config.OutputLimit != 2048 || config.AllowInsecure == nil || *config.AllowInsecure {
		t.Fatalf("unexpected optional config: %#v", config)
	}
	if config.DeviceToken == "" || config.BrowserBridgeAddress != "127.0.0.1:19000" {
		t.Fatalf("unexpected device config: %#v", config)
	}
	if config.ConfirmBrowserPairing == nil || *config.ConfirmBrowserPairing {
		t.Fatalf("unexpected browser pairing confirmation config: %#v", config)
	}
	if config.RegisterURLProtocol == nil || *config.RegisterURLProtocol {
		t.Fatalf("unexpected URL protocol config: %#v", config)
	}
}

func TestLocalClientRunning(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	if !localClientRunning(listener.Addr().String()) {
		t.Fatal("listening local Client was not detected")
	}
	address := listener.Addr().String()
	listener.Close()
	if localClientRunning(address) {
		t.Fatal("closed local Client address was reported as running")
	}
}

func TestNewDeviceToken(t *testing.T) {
	first, err := newDeviceToken()
	if err != nil {
		t.Fatal(err)
	}
	second, err := newDeviceToken()
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 64 || len(second) != 64 || first == second {
		t.Fatalf("generated tokens are not unique 256-bit hex values: %q %q", first, second)
	}
}

func TestValidDeviceToken(t *testing.T) {
	if !validDeviceToken(strings.Repeat("a", 32)) {
		t.Fatal("32-character device token was rejected")
	}
	if !validDeviceToken("  " + strings.Repeat("b", 64) + "  ") {
		t.Fatal("valid device token with surrounding whitespace was rejected")
	}
	if validDeviceToken(strings.Repeat("c", 31)) || validDeviceToken(strings.Repeat("d", 257)) {
		t.Fatal("device token outside the allowed length was accepted")
	}
}

func TestDefaultAgentToken(t *testing.T) {
	if defaultAgentToken != "" {
		t.Fatalf("default agent token = %q", defaultAgentToken)
	}
}

func TestDefaultServerURL(t *testing.T) {
	if defaultServerURL != "ws://idata.test.huawei.com:80/ws/agent" {
		t.Fatalf("default server URL = %q", defaultServerURL)
	}
	host, port := serverEndpoint(defaultServerURL)
	if host != "idata.test.huawei.com" || port != "80" {
		t.Fatalf("default server endpoint = %s:%s", host, port)
	}
}

func TestServerURLFromLaunchLink(t *testing.T) {
	tests := []struct {
		name      string
		link      string
		want      string
		wantError bool
	}{
		{name: "legacy login", link: "idata://login"},
		{name: "browser-normalized legacy login", link: "idata://login/"},
		{name: "IPv4 connect", link: "idata://connect?server=192.168.8.87&port=12345", want: "ws://192.168.8.87:12345/ws/agent"},
		{name: "browser-normalized connect", link: "idata://connect/?server=192.168.8.87&port=12345&secure=0", want: "ws://192.168.8.87:12345/ws/agent"},
		{name: "secure IPv6 connect", link: "idata://connect?server=%3A%3A1&port=443&secure=1", want: "wss://[::1]:443/ws/agent"},
		{name: "unexpected path rejected", link: "idata://connect/anything?server=192.168.8.87&port=12345", wantError: true},
		{name: "HTTP domain through Nginx", link: "idata://connect?server=idata.test.huawei.com&port=80&secure=0", want: "ws://idata.test.huawei.com:80/ws/agent"},
		{name: "secure domain", link: "idata://connect?server=idata.test.huawei.com&port=443&secure=1", want: "wss://idata.test.huawei.com:443/ws/agent"},
		{name: "missing port rejected", link: "idata://connect?server=192.168.8.87", wantError: true},
		{name: "token parameter rejected", link: "idata://connect?server=192.168.8.87&port=12345&token=secret", wantError: true},
		{name: "arbitrary action rejected", link: "idata://run?server=192.168.8.87&port=12345", wantError: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := serverURLFromLaunchLink(test.link)
			if got != test.want || (err != nil) != test.wantError {
				t.Fatalf("serverURLFromLaunchLink() = %q, %v; want %q, error=%v", got, err, test.want, test.wantError)
			}
		})
	}
}

func TestValidServerHost(t *testing.T) {
	for _, value := range []string{"10.0.0.8", "127.0.0.1", "::1", "[2001:db8::1]", "idata.test.huawei.com", "LOCALHOST", "example.com."} {
		if !validServerHost(value) {
			t.Fatalf("valid server IP %q was rejected", value)
		}
	}
	for _, value := range []string{"", "-example.com", "example..com", "example.com/evil", "example.com:80", "a@b.com", "a_b.com", "10.0.0.999", "http://10.0.0.8"} {
		if validServerHost(value) {
			t.Fatalf("invalid server IP %q was accepted", value)
		}
	}
}

func TestValidateLaunchServerURL(t *testing.T) {
	for _, value := range []string{
		"ws://192.168.8.87:12345/ws/agent",
		"wss://[2001:db8::1]:443/ws/agent",
		"ws://idata.test.huawei.com:12345/ws/agent",
		"wss://idata.test.huawei.com:443/ws/agent",
	} {
		if err := validateLaunchServerURL(value); err != nil {
			t.Fatalf("valid launch URL %q was rejected: %v", value, err)
		}
	}
	for _, value := range []string{
		"ws://-example.com:12345/ws/agent",
		"ws://192.168.8.87/ws/agent",
		"ws://192.168.8.87:12345/other",
		"ws://192.168.8.87:12345/ws/agent?token=secret",
		"http://192.168.8.87:12345/ws/agent",
	} {
		if err := validateLaunchServerURL(value); err == nil {
			t.Fatalf("invalid launch URL %q was accepted", value)
		}
	}
}

func TestWebOriginFromServerURL(t *testing.T) {
	tests := map[string]string{
		"ws://10.0.0.2/ws/agent":         "http://10.0.0.2",
		"wss://idata.example:8443/agent": "https://idata.example:8443",
	}
	for input, want := range tests {
		got, err := webOriginFromServerURL(input)
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Fatalf("webOriginFromServerURL(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestServerEndpoint(t *testing.T) {
	tests := map[string]struct {
		input string
		host  string
		port  string
	}{
		"empty uses UI default": {input: "", port: "80"},
		"plain websocket":       {input: "ws://10.0.0.8/ws/agent", host: "10.0.0.8", port: "80"},
		"secure websocket":      {input: "wss://idata.example/ws/agent", host: "idata.example", port: "443"},
		"explicit port":         {input: "ws://10.0.0.8:8080/ws/agent", host: "10.0.0.8", port: "8080"},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			host, port := serverEndpoint(test.input)
			if host != test.host || port != test.port {
				t.Fatalf("serverEndpoint(%q) = %q, %q; want %q, %q", test.input, host, port, test.host, test.port)
			}
		})
	}
}

func TestServerURLFromEndpoint(t *testing.T) {
	tests := []struct {
		name     string
		host     string
		port     string
		previous string
		want     string
		wantErr  bool
	}{
		{name: "IPv4", host: "10.0.0.8", port: "80", want: "ws://10.0.0.8:80/ws/agent"},
		{name: "IPv6", host: "[::1]", port: "8080", want: "ws://[::1]:8080/ws/agent"},
		{name: "preserves secure scheme", host: "idata.example", port: "443", previous: "wss://idata.example/ws/agent", want: "wss://idata.example:443/ws/agent"},
		{name: "rejects URL in IP field", host: "http://10.0.0.8", port: "80", wantErr: true},
		{name: "rejects invalid port", host: "10.0.0.8", port: "70000", wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := serverURLFromEndpoint(test.host, test.port, test.previous)
			if (err != nil) != test.wantErr || got != test.want {
				t.Fatalf("serverURLFromEndpoint() = %q, %v; want %q, error=%v", got, err, test.want, test.wantErr)
			}
		})
	}
}

func TestServerPortForHost(t *testing.T) {
	tests := []struct {
		name, host, previous, want string
	}{
		{name: "IP has no implicit deployment override", host: "10.90.65.189", want: "80"},
		{name: "public server", host: "43.156.108.175", want: "80"},
		{name: "public server preserves supplied port", host: "43.156.108.175", previous: "ws://43.156.108.175:12345/ws/agent", want: "12345"},
		{name: "other server", host: "10.90.65.190", want: "80"},
		{name: "special server preserves supplied port", host: "10.90.65.189", previous: "ws://10.90.65.189:80/ws/agent", want: "80"},
		{name: "old special port does not follow another host", host: "10.90.65.190", previous: "ws://10.90.65.189:12345/ws/agent", want: "80"},
		{name: "secure same host defaults to TLS port", host: "idata.example", previous: "wss://idata.example/ws/agent", want: "443"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := serverPortForHost(test.host, test.previous); got != test.want {
				t.Fatalf("serverPortForHost(%q, %q) = %q, want %q", test.host, test.previous, got, test.want)
			}
		})
	}
}

// Both startup and running-client handoff use this endpoint resolution chain.
func TestLaunchEndpointSurvivesConnectionSetup(t *testing.T) {
	for _, host := range []string{"10.90.65.189", "43.156.108.175", "192.168.8.87", "::1", "idata.test.huawei.com"} {
		for _, endpoint := range []struct{ port, secure, scheme string }{
			{"54321", "0", "ws"}, {"18080", "0", "ws"},
			{"80", "0", "ws"}, {"443", "1", "wss"}, {"54443", "1", "wss"},
		} {
			t.Run(host+"/"+endpoint.port, func(t *testing.T) {
				link := "idata://connect/?server=" + url.QueryEscape(host) + "&port=" + endpoint.port + "&secure=" + endpoint.secure
				launchURL, err := serverURLFromLaunchLink(link)
				if err != nil {
					t.Fatal(err)
				}
				launchIP, _ := serverEndpoint(launchURL)
				candidate, err := serverURLFromEndpoint(launchIP, serverPortForHost(launchIP, launchURL), launchURL)
				if err != nil {
					t.Fatal(err)
				}
				if candidate != launchURL {
					t.Fatalf("launch endpoint %q changed to %q", launchURL, candidate)
				}
				parsed, err := url.Parse(candidate)
				if err != nil || parsed.Scheme != endpoint.scheme || parsed.Port() != endpoint.port || parsed.Hostname() != host {
					t.Fatalf("incorrect connection endpoint %q", candidate)
				}
			})
		}
	}
}

func TestConfigurableServerInput(t *testing.T) {
	for _, tc := range []struct{ input, previous, want string }{
		{"http://other.example:18080/tools/idata/", "wss://old.example:443/ws/agent", "ws://other.example:18080/tools/idata/ws/agent"},
		{"https://another.example/", "", "wss://another.example:443/ws/agent"},
		{"http://[2001:db8::1]:8888/", "", "ws://[2001:db8::1]:8888/ws/agent"},
		{"another.example:9090", "wss://old.example:443/ws/agent", "ws://another.example:9090/ws/agent"},
		{"other.example", "wss://other.example:8443/tools/ws/agent", "wss://other.example:8443/tools/ws/agent"},
		{"other.example", "wss://old.example:8443/tools/ws/agent", "ws://other.example:80/ws/agent"},
	} {
		got, err := serverURLFromInput(tc.input, tc.previous)
		if err != nil || got != tc.want {
			t.Fatalf("input %q: %q, %v; want %q", tc.input, got, err, tc.want)
		}
	}
	for _, input := range []string{"https://user:pass@example.com/", "https://example.com/?token=x", "http://example.com/a/../b", "http://example.com/a%2fb/", "ftp://example.com/"} {
		if _, err := serverURLFromInput(input, ""); err == nil {
			t.Fatalf("accepted %q", input)
		}
	}
}

func TestPrefixedLaunchAndHandoff(t *testing.T) {
	link := "idata://connect?server=other.example&port=8443&secure=1&path=%2Fteam%2Fidata%2Fws%2Fagent"
	got, err := serverURLFromLaunchLink(link)
	if err != nil {
		t.Fatal(err)
	}
	const want = "wss://other.example:8443/team/idata/ws/agent"
	if got != want {
		t.Fatal(got)
	}
	if err := validateLaunchServerURL(got); err != nil {
		t.Fatal(err)
	}
	rebuilt, err := serverURLFromInput(got, "ws://old.example:12345/ws/agent")
	if err != nil || rebuilt != want {
		t.Fatalf("handoff changed endpoint: %s %v", rebuilt, err)
	}
	for _, path := range []string{"/../ws/agent", "//ws/agent", "/a%2fb/ws/agent", "/other", "/a/./ws/agent"} {
		if _, err := serverURLFromLaunchLink("idata://connect?server=other.example&port=80&path=" + url.QueryEscape(path)); err == nil {
			t.Fatalf("accepted %q", path)
		}
	}
}
