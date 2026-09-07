package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"idata-server/internal/protocol"
)

type apiClient struct {
	baseURL string
	token   string
	http    *http.Client
}

func main() {
	code, err := run(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, "idatactl:", err)
	}
	os.Exit(code)
}

func run(args []string) (int, error) {
	if len(args) == 0 {
		usage()
		return 2, errors.New("a command is required")
	}
	baseURL := strings.TrimRight(envOr("IDATA_SERVER_HTTP_URL", "http://127.0.0.1"), "/")
	token := os.Getenv("IDATA_ADMIN_TOKEN")
	if token == "" {
		return 2, errors.New("IDATA_ADMIN_TOKEN is required")
	}
	client := &apiClient{baseURL: baseURL, token: token, http: &http.Client{Timeout: 6 * time.Minute}}

	switch args[0] {
	case "clients":
		return client.listClients()
	case "exec":
		return client.exec(args[1:])
	case "help", "-h", "--help":
		usage()
		return 0, nil
	default:
		usage()
		return 2, fmt.Errorf("unknown command %q", args[0])
	}
}

func (c *apiClient) listClients() (int, error) {
	var response struct {
		Clients []protocol.ClientInfo `json:"clients"`
	}
	if err := c.request(http.MethodGet, "/api/v1/clients", nil, &response); err != nil {
		return 1, err
	}
	if len(response.Clients) == 0 {
		fmt.Println("No clients online.")
		return 0, nil
	}
	fmt.Printf("%-24s %-10s %-10s %-24s %s\n", "CLIENT ID", "OS", "ARCH", "HOSTNAME", "CONNECTED")
	for _, item := range response.Clients {
		fmt.Printf("%-24s %-10s %-10s %-24s %s\n", item.ID, item.OS, item.Arch, item.Hostname, item.ConnectedAt)
	}
	return 0, nil
}

func (c *apiClient) exec(args []string) (int, error) {
	flags := flag.NewFlagSet("exec", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	clientID := flags.String("client", "", "target client ID")
	timeout := flags.Duration("timeout", 30*time.Second, "remote command timeout")
	if err := flags.Parse(args); err != nil {
		return 2, err
	}
	if *clientID == "" {
		return 2, errors.New("--client is required")
	}
	command := strings.Join(flags.Args(), " ")
	if strings.TrimSpace(command) == "" {
		return 2, errors.New("command is required")
	}
	if *timeout < time.Second {
		return 2, errors.New("--timeout must be at least 1s")
	}

	body := map[string]any{"command": command, "timeout_seconds": int(timeout.Seconds())}
	var result protocol.Message
	path := "/api/v1/clients/" + url.PathEscape(*clientID) + "/commands"
	if err := c.request(http.MethodPost, path, body, &result); err != nil {
		return 1, err
	}
	_, _ = io.WriteString(os.Stdout, result.Stdout)
	_, _ = io.WriteString(os.Stderr, result.Stderr)
	if result.StdoutTruncated || result.StderrTruncated {
		fmt.Fprintln(os.Stderr, "idatactl: warning: remote output was truncated")
	}
	if result.Error != "" {
		fmt.Fprintln(os.Stderr, "idatactl: remote error:", result.Error)
	}
	if result.ExitCode >= 0 && result.ExitCode <= 125 {
		return result.ExitCode, nil
	}
	if result.ExitCode != 0 {
		return 1, nil
	}
	return 0, nil
}

func (c *apiClient) request(method, path string, body any, output any) error {
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(encoded)
	}
	request, err := http.NewRequest(method, c.baseURL+path, reader)
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+c.token)
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := c.http.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		var apiError struct {
			Error string `json:"error"`
		}
		_ = json.NewDecoder(io.LimitReader(response.Body, 64<<10)).Decode(&apiError)
		if apiError.Error == "" {
			apiError.Error = response.Status
		}
		return errors.New(apiError.Error)
	}
	return json.NewDecoder(io.LimitReader(response.Body, 4<<20)).Decode(output)
}

func usage() {
	fmt.Fprintln(os.Stderr, "Usage:")
	fmt.Fprintln(os.Stderr, "  idatactl clients")
	fmt.Fprintln(os.Stderr, "  idatactl exec --client <id> [--timeout 30s] <command>")
}

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
