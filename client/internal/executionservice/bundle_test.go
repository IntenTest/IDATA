package executionservice

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func testBundle(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	archive := zip.NewWriter(&buffer)
	for name, data := range files {
		writer, err := archive.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := writer.Write([]byte(data)); err != nil {
			t.Fatal(err)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func TestBundleUpgradePreservesSettingsAndRepairsRuntime(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{"python/python.exe": "runtime", "idata/app/start.py": "worker v1", "idata/app/run_test_process.py": "runner"}
	script, python, err := prepareBundle(testBundle(t, files), root)
	if err != nil {
		t.Fatal(err)
	}
	if script != filepath.Join(root, "idata", "app", "start.py") || python != filepath.Join(root, "python", "python.exe") {
		t.Fatal("incorrect bundle paths")
	}
	settings := filepath.Join(root, "idata", "app", "config", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settings), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(settings, []byte(`{"settings":"preserve"}`), 0600); err != nil {
		t.Fatal(err)
	}
	files["idata/app/start.py"] = "worker v2"
	if err := os.Remove(python); err != nil {
		t.Fatal(err)
	}
	if _, _, err := prepareBundle(testBundle(t, files), root); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(script); string(data) != "worker v2" {
		t.Fatal("runtime was not upgraded")
	}
	if data, _ := os.ReadFile(settings); string(data) != `{"settings":"preserve"}` {
		t.Fatal("settings were overwritten")
	}
	if _, err := os.Stat(python); err != nil {
		t.Fatal("runtime was not repaired")
	}
}

func TestBundleRejectsUnexpectedPaths(t *testing.T) {
	for _, name := range []string{"../escape", "/absolute", "python/../../escape", `python\escape`, "python/C:escape", "idata/app/config/settings.json"} {
		t.Run(name, func(t *testing.T) {
			if _, _, err := prepareBundle(testBundle(t, map[string]string{name: "bad"}), t.TempDir()); err == nil {
				t.Fatal("unsafe bundle accepted")
			}
		})
	}
}
