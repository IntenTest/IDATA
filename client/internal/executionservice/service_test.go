package executionservice

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLocateConfiguredScript(t *testing.T) {
	directory := t.TempDir()
	script := filepath.Join(directory, "start.py")
	if err := os.WriteFile(script, []byte("# test\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := locateScript(script)
	if err != nil {
		t.Fatal(err)
	}
	if got != script {
		t.Fatalf("locateScript() = %q, want %q", got, script)
	}
}

func TestLocateMissingConfiguredScript(t *testing.T) {
	if _, err := locateScript(filepath.Join(t.TempDir(), "missing.py")); err == nil {
		t.Fatal("locateScript accepted a missing configured script")
	}
}
