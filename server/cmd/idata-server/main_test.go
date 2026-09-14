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

func TestListenAddressConfiguration(t *testing.T) {
	t.Setenv("IDATA_LISTEN_ADDR", "127.0.0.1:18080")
	if got := envOr("IDATA_LISTEN_ADDR", defaultListenAddress); got != "127.0.0.1:18080" {
		t.Fatal(got)
	}
	t.Setenv("IDATA_LISTEN_ADDR", "")
	if got := envOr("IDATA_LISTEN_ADDR", defaultListenAddress); got != ":12345" {
		t.Fatal(got)
	}
}
