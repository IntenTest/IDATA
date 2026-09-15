package executor

import (
	"context"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestRunCapturesOutputAndExitCode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix shell assertion")
	}
	result := Run(context.Background(), "printf out; printf err >&2; exit 7", nil, 2*time.Second, 1024)
	if result.Stdout != "out" || result.Stderr != "err" {
		t.Fatalf("unexpected output: stdout=%q stderr=%q", result.Stdout, result.Stderr)
	}
	if result.ExitCode != 7 {
		t.Fatalf("exit code = %d, want 7", result.ExitCode)
	}
}

func TestRunTruncatesEachOutputStream(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix shell assertion")
	}
	result := Run(context.Background(), "printf 123456789; printf abcdefghi >&2", nil, 2*time.Second, 4)
	if result.Stdout != "1234" || result.Stderr != "abcd" {
		t.Fatalf("unexpected truncated output: stdout=%q stderr=%q", result.Stdout, result.Stderr)
	}
	if !result.StdoutTruncated || !result.StderrTruncated {
		t.Fatalf("truncation flags not set: %+v", result)
	}
}

func TestRunTimesOut(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix shell assertion")
	}
	started := time.Now()
	result := Run(context.Background(), "sleep 2", nil, 25*time.Millisecond, 1024)
	if !result.TimedOut || !strings.Contains(result.Error, "timed out") {
		t.Fatalf("expected timeout, got %+v", result)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("timeout took too long: %v", elapsed)
	}
}

func TestRunSuppliesStandardInput(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix shell assertion")
	}
	result := Run(context.Background(), "read value; printf '%s' \"$value\"", []byte("server-owned-worker\n"), 2*time.Second, 1024)
	if result.ExitCode != 0 || result.Stdout != "server-owned-worker" {
		t.Fatalf("stdin was not supplied: %+v", result)
	}
}
