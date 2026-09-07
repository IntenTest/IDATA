package main

import "testing"

func TestEnvBool(t *testing.T) {
	t.Setenv("IDATA_TEST_BOOL", "true")
	if !envBool("IDATA_TEST_BOOL", false) {
		t.Fatal("true environment value was not parsed")
	}

	t.Setenv("IDATA_TEST_BOOL", "false")
	if envBool("IDATA_TEST_BOOL", true) {
		t.Fatal("false environment value was not parsed")
	}

	t.Setenv("IDATA_TEST_BOOL", "invalid")
	if !envBool("IDATA_TEST_BOOL", true) {
		t.Fatal("invalid environment value did not use fallback")
	}
}

func TestListenAddrForInterfaceAddresses(t *testing.T) {
	tests := []struct {
		name      string
		addresses []string
		want      string
	}{
		{name: "special intranet address", addresses: []string{"127.0.0.1/8", "10.90.65.189/24"}, want: ":12345"},
		{name: "other address", addresses: []string{"127.0.0.1/8", "10.90.65.190/24"}, want: ":80"},
		{name: "invalid address ignored", addresses: []string{"not-an-address"}, want: ":80"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := listenAddrForInterfaceAddresses(test.addresses); got != test.want {
				t.Fatalf("listenAddrForInterfaceAddresses() = %q, want %q", got, test.want)
			}
		})
	}
}
