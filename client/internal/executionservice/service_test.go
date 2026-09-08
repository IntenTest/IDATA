package executionservice

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
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

func TestReadinessRejectsUnrelatedServices(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
		ready  bool
	}{
		{"worker", 200, `{"settings":{},"networkZone":"blue"}`, true},
		{"missing", 404, `{"settings":{}}`, false},
		{"error", 500, `{"error":"invalid settings"}`, false},
		{"html", 200, `<html>Other service</html>`, false},
		{"unrelated JSON", 200, `{"status":"ok"}`, false},
		{"redirect", 302, `{"settings":{}}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(tc.status); io.WriteString(w, tc.body) }))
			defer server.Close()
			if got := checkService(context.Background(), server.URL); got != tc.ready {
				t.Fatalf("ready=%v, want %v", got, tc.ready)
			}
		})
	}
}

func TestManagerStartsAndRecoversRealWorker(t *testing.T) {
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("Python 3 is needed for real worker integration")
	}
	listener, err := net.Listen("tcp", "127.0.0.1:54321")
	if err != nil {
		t.Skip("fixed worker port is occupied")
	}
	listener.Close()
	root := t.TempDir()
	app := filepath.Join(root, "idata", "app")
	if err := os.MkdirAll(app, 0700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"start.py", "run_test_process.py"} {
		data, err := os.ReadFile(filepath.Join("..", "..", "..", "idata", "app", name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(app, name), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"vue-3.5.24", "element-plus-2.11.8"} {
		if err := os.MkdirAll(filepath.Join(root, "idata", "vendor", name), 0700); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	logFile, err := os.Create(filepath.Join(root, "worker.log"))
	if err != nil {
		t.Fatal(err)
	}
	defer logFile.Close()
	manager := NewManager(ctx, Config{ScriptPath: filepath.Join(app, "start.py"), PythonExecutable: python, Output: logFile})
	defer manager.Close()
	var group sync.WaitGroup
	failures := make(chan error, 4)
	for i := 0; i < 4; i++ {
		group.Add(1)
		go func() { defer group.Done(); failures <- manager.Ensure(ctx) }()
	}
	group.Wait()
	close(failures)
	for err := range failures {
		if err != nil {
			data, _ := os.ReadFile(logFile.Name())
			t.Fatalf("startup: %v\n%s", err, data)
		}
	}
	oldPID, err := os.ReadFile(filepath.Join(app, ".idata.pid"))
	if err != nil {
		t.Fatal(err)
	}
	// Simulate a stopped worker, then prove the next operation recovers it.
	manager.stop()
	if serviceAvailable(ctx) {
		t.Fatal("worker did not stop")
	}
	if err := manager.Ensure(ctx); err != nil {
		t.Fatal(err)
	}
	newPID, err := os.ReadFile(filepath.Join(app, ".idata.pid"))
	if err != nil {
		t.Fatal(err)
	}
	if string(oldPID) == string(newPID) {
		t.Fatal("worker was not restarted")
	}
	requestCtx, requestCancel := context.WithCancel(ctx)
	if err := manager.Ensure(requestCtx); err != nil {
		t.Fatal(err)
	}
	requestCancel()
	if !serviceAvailable(ctx) {
		t.Fatal("request cancellation stopped the worker")
	}
	manager.Close()
	if serviceAvailable(ctx) {
		t.Fatal("owned worker survived manager shutdown")
	}
	if err := manager.Ensure(ctx); err == nil {
		t.Fatal("closed manager restarted a worker")
	}
}

func TestManagerDoesNotKillLiveUnhealthyWorker(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:54321")
	if err != nil {
		t.Skip("fixed worker port is occupied")
	}
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(500) })}
	go server.Serve(listener)
	defer server.Close()
	stopped := false
	manager := NewManager(context.Background(), Config{})
	manager.stop = func() { stopped = true }
	if err := manager.Ensure(context.Background()); err == nil {
		t.Fatal("unhealthy service was accepted")
	}
	if stopped {
		t.Fatal("readiness failure terminated a live worker")
	}
	manager.Close()
	if !stopped {
		t.Fatal("close did not release owned worker")
	}
}
