package executionservice

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

const healthURL = "http://127.0.0.1:54321/api/settings"

type Config struct {
	ScriptPath       string
	PythonExecutable string
	Output           io.Writer
	Logger           *slog.Logger
}

func Ensure(ctx context.Context, config Config) (func(), error) {
	return ensure(ctx, ctx, config)
}

func ensure(lifetime, startup context.Context, config Config) (func(), error) {
	if config.Logger == nil {
		config.Logger = slog.Default()
	}
	if err := startup.Err(); err != nil {
		return func() {}, err
	}
	if serviceAvailable(startup) {
		return func() {}, nil
	}
	probe, err := net.DialTimeout("tcp", "127.0.0.1:54321", 300*time.Millisecond)
	if err == nil {
		probe.Close()
		return func() {}, errors.New("local port 54321 is occupied by an unrecognized or unhealthy service; close that service and retry")
	}
	script, pythonName, err := resolveRuntime(config)
	if err != nil {
		return func() {}, err
	}
	python, err := exec.LookPath(pythonName)
	if err != nil {
		return func() {}, fmt.Errorf("locate Python executable %q: %w", pythonName, err)
	}
	command := exec.Command(python, "-u", script)
	command.Dir = filepath.Dir(script)
	command.Stdout, command.Stderr = config.Output, config.Output
	configureHiddenProcess(command)
	if err := command.Start(); err != nil {
		return func() {}, fmt.Errorf("start IDATA execution service: %w", err)
	}
	exited := make(chan struct{})
	var processErr error
	go func() { processErr = command.Wait(); close(exited) }()
	var once sync.Once
	stop := func() {
		once.Do(func() {
			select {
			case <-exited:
				return
			default:
			}
			_ = terminateProcess(command.Process)
			select {
			case <-exited:
			case <-time.After(3 * time.Second):
			}
		})
	}
	config.Logger.Info("local IDATA execution service starting", "script", script, "pid", command.Process.Pid)
	readyCtx, cancel := context.WithTimeout(startup, 15*time.Second)
	defer cancel()
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		if serviceAvailable(readyCtx) {
			config.Logger.Info("local IDATA execution service ready", "pid", command.Process.Pid)
			go func() {
				select {
				case <-lifetime.Done():
					stop()
				case <-exited:
				}
			}()
			return stop, nil
		}
		select {
		case <-readyCtx.Done():
			stop()
			return func() {}, fmt.Errorf("IDATA execution service was not ready on 127.0.0.1:54321: %w", readyCtx.Err())
		case <-lifetime.Done():
			stop()
			return func() {}, lifetime.Err()
		case <-exited:
			return func() {}, fmt.Errorf("IDATA execution service exited before becoming ready (%v); see the client log for details", processErr)
		case <-ticker.C:
		}
	}
}

func serviceAvailable(ctx context.Context) bool {
	return checkService(ctx, healthURL)
}

func checkService(ctx context.Context, endpoint string) bool {
	client := &http.Client{Timeout: time.Second, Transport: &http.Transport{Proxy: nil, DisableKeepAlives: true},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return false
	}
	response, err := client.Do(request)
	if err != nil {
		return false
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return false
	}
	var payload struct {
		Settings map[string]json.RawMessage `json:"settings"`
	}
	return json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&payload) == nil && payload.Settings != nil
}

func locateScript(configured string) (string, error) {
	if configured != "" {
		path, err := filepath.Abs(configured)
		if err == nil {
			if info, statErr := os.Stat(path); statErr == nil && !info.IsDir() {
				return path, nil
			}
		}
		return "", fmt.Errorf("configured IDATA execution script was not found: %s", configured)
	}
	executable, _ := os.Executable()
	workingDirectory, _ := os.Getwd()
	candidates := []string{
		filepath.Join(filepath.Dir(executable), "idata", "app", "start.py"),
		filepath.Join(filepath.Dir(executable), "..", "idata", "app", "start.py"),
		filepath.Join(workingDirectory, "idata", "app", "start.py"),
		filepath.Join(workingDirectory, "app", "start.py"),
	}
	for _, candidate := range candidates {
		path, err := filepath.Abs(candidate)
		if err != nil {
			continue
		}
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path, nil
		}
	}
	return "", errors.New("IDATA execution script was not found; set execution_script in idata-client.json")
}
