package agent

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"regexp"
	"time"

	"idata-client/internal/protocol"
)

var idataReadPath = regexp.MustCompile(`^/api/(devices|settings|model-config|test-cases|test-cases/update|test-runs|test-runs/[A-Za-z0-9._%-]+/reports/[A-Za-z0-9._%-]+/content)$`)
var idataWritePath = regexp.MustCompile(`^/api/(test-cases/update|test-runs(/[A-Za-z0-9._%-]+/(close|reports/[A-Za-z0-9._%-]+/open))?)$`)

// The target is fixed loopback: remote requests cannot select a host, port or arbitrary file.
func forwardIDATA(parent context.Context, message protocol.Message) protocol.Message {
	return forwardIDATAWithService(parent, message, nil)
}

func forwardIDATAWithService(parent context.Context, message protocol.Message, ensure func(context.Context) error) protocol.Message {
	result := protocol.Message{Type: protocol.TypeAPIResponse, ProtocolVersion: protocol.Version, RequestID: message.RequestID}
	valid := message.Method == "GET" && idataReadPath.MatchString(message.Path) || message.Method == "POST" && idataWritePath.MatchString(message.Path) || message.Method == "PUT" && (message.Path == "/api/settings" || message.Path == "/api/model-config")
	if !valid || len(message.Data) > 64<<10 || message.RequestID == "" {
		result.Status, result.Error = 400, "Invalid IDATA operation."
		return result
	}
	ctx, cancel := context.WithTimeout(parent, 30*time.Second)
	defer cancel()
	if ensure != nil {
		if err := ensure(ctx); err != nil {
			result.Status, result.Error = 503, "The local execution service could not start. Reopen IDATA Client to view the startup error and check its log."
			return result
		}
	}
	request, err := http.NewRequestWithContext(ctx, message.Method, "http://127.0.0.1:54321"+message.Path, bytes.NewReader(message.Data))
	if err != nil {
		result.Status, result.Error = 400, "Invalid IDATA request."
		return result
	}
	request.Header.Set("Content-Type", "application/json")
	client := &http.Client{Transport: &http.Transport{Proxy: nil, DisableKeepAlives: true}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	response, err := client.Do(request)
	if err != nil {
		result.Status, result.Error = 502, "The local execution service stopped or could not be reached. Check IDATA Client. The next request will attempt recovery; verify test run status before starting a test again."
		return result
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, (8<<20)+1))
	if err != nil || len(data) > 8<<20 {
		result.Status, result.Error = 502, "IDATA response exceeded the 8 MiB limit or could not be read."
		return result
	}
	result.Status, result.ContentType, result.Data = response.StatusCode, response.Header.Get("Content-Type"), data
	return result
}
