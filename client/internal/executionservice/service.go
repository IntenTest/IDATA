package executionservice

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
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
	if serviceAvailable() {
		config.Logger.Info("local IDATA execution service already running")
		return func() {}, nil
	}
	script, err := locateScript(config.ScriptPath)
	if err != nil {
		return func() {}, err
	}
	python, err := exec.LookPath(config.PythonExecutable)
	if err != nil {
		return func() {}, fmt.Errorf("locate Python executable %q: %w", config.PythonExecutable, err)
	}
	command := exec.Command(python, script)
	command.Dir = filepath.Dir(script)
	command.Stdout = config.Output
	command.Stderr = config.Output
	configureHiddenProcess(command)
	if err := command.Start(); err != nil {
		return func() {}, fmt.Errorf("start IDATA execution service: %w", err)
	}
	exited := make(chan error, 1)
	go func() { exited <- command.Wait() }()
	config.Logger.Info("local IDATA execution service starting", "script", script, "pid", command.Process.Pid)
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if serviceAvailable() {
			config.Logger.Info("local IDATA execution service ready", "pid", command.Process.Pid)
			go func() {
				<-ctx.Done()
				_ = terminateProcess(command.Process)
			}()
			return func() { _ = terminateProcess(command.Process) }, nil
		}
		select {
		case processErr := <-exited:
			return func() {}, fmt.Errorf("IDATA execution service exited before becoming ready: %w", processErr)
		default:
		}
		time.Sleep(250 * time.Millisecond)
	}
	_ = terminateProcess(command.Process)
	return func() {}, errors.New("IDATA execution service did not become available on 127.0.0.1:54321")
}

func serviceAvailable() bool {
	client := &http.Client{Timeout: time.Second, Transport: &http.Transport{Proxy: nil}}
	response, err := client.Get(healthURL)
	if err != nil {
		return false
	}
	_ = response.Body.Close()
	return response.StatusCode >= 200 && response.StatusCode < 500
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
