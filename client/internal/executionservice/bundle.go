package executionservice

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// Release builds populate this with an offline worker and Python runtime.
var bundledRuntime []byte

func prepareBundle(data []byte, root string) (string, string, error) {
	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", "", fmt.Errorf("read bundled execution service: %w", err)
	}
	// Only immutable runtime files belong in the bundle. Settings and logs stay
	// under the stable worker directory and survive client upgrades.
	for _, entry := range archive.File {
		name := entry.Name
		if path.Clean(name) != name || strings.ContainsAny(name, "\\:") || strings.HasPrefix(name, "/") || strings.HasPrefix(name, "../") || entry.Mode()&os.ModeSymlink != 0 || entry.FileInfo().IsDir() {
			return "", "", fmt.Errorf("invalid bundled runtime path: %q", name)
		}
		if !(strings.HasPrefix(name, "python/") || strings.HasPrefix(name, "idata/vendor/") || name == "idata/app/start.py" || name == "idata/app/run_test_process.py") {
			return "", "", fmt.Errorf("unexpected bundled runtime file: %q", name)
		}
		if entry.UncompressedSize64 > 64<<20 {
			return "", "", fmt.Errorf("bundled runtime file too large: %q", name)
		}
	}
	for _, entry := range archive.File {
		reader, err := entry.Open()
		if err != nil {
			return "", "", err
		}
		contents, readErr := io.ReadAll(io.LimitReader(reader, (64<<20)+1))
		reader.Close()
		if readErr != nil {
			return "", "", readErr
		}
		if len(contents) > 64<<20 {
			return "", "", fmt.Errorf("bundled runtime file too large")
		}
		target := filepath.Join(root, filepath.FromSlash(entry.Name))
		existing, readErr := os.ReadFile(target)
		if readErr == nil && sha256.Sum256(existing) == sha256.Sum256(contents) {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
			return "", "", err
		}
		temporary, err := os.CreateTemp(filepath.Dir(target), ".runtime-*")
		if err != nil {
			return "", "", err
		}
		_, writeErr := temporary.Write(contents)
		closeErr := temporary.Close()
		if writeErr == nil {
			writeErr = closeErr
		}
		if writeErr == nil {
			writeErr = os.Rename(temporary.Name(), target)
		}
		if writeErr != nil {
			os.Remove(temporary.Name())
			return "", "", writeErr
		}
	}
	script := filepath.Join(root, "idata", "app", "start.py")
	python := filepath.Join(root, "python", "python.exe")
	for _, file := range []string{script, python} {
		if info, err := os.Stat(file); err != nil || info.IsDir() {
			return "", "", fmt.Errorf("bundled runtime is incomplete: %s", file)
		}
	}
	return script, python, nil
}

func resolveRuntime(config Config) (string, string, error) {
	if config.ScriptPath == "" && len(bundledRuntime) != 0 {
		directory, err := os.UserCacheDir()
		if err != nil {
			return "", "", err
		}
		script, python, err := prepareBundle(bundledRuntime, filepath.Join(directory, "IDATA", "execution-service"))
		if config.PythonExecutable != "" {
			python = config.PythonExecutable
		}
		return script, python, err
	}
	script, err := locateScript(config.ScriptPath)
	python := config.PythonExecutable
	if python == "" {
		python = "python"
	}
	return script, python, err
}
