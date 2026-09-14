package server

import (
	"bytes"
	"context"
	"embed"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"regexp"
	"strings"
	"time"
)

//go:embed idataweb
var idataFiles embed.FS

//go:embed idataworker/worker.py
var idataWorkerSource []byte

type idataWorkerResponse struct {
	OK    bool            `json:"ok"`
	Data  json.RawMessage `json:"data"`
	Error string          `json:"error"`
}

var (
	idataReadOperation  = regexp.MustCompile(`^(devices|settings|model-config|test-cases|test-cases/update|test-runs|test-runs/TR-[0-9]+/reports/[A-Za-z0-9._%-]+/content)$`)
	idataWriteOperation = regexp.MustCompile(`^(test-cases/update|test-runs|test-runs/TR-[0-9]+/close)$`)
)

func (s *Server) registerIDATA(mux *http.ServeMux) {
	assets, err := fs.Sub(idataFiles, "idataweb")
	if err != nil {
		panic(err)
	}
	index, err := fs.ReadFile(assets, "index.html")
	if err != nil {
		panic(err)
	}
	serveIndex := func(w http.ResponseWriter, r *http.Request, legacy bool) {
		content := index
		if legacy {
			content = bytes.Replace(index, []byte(`name="idata-api-base" content="./"`), []byte(`name="idata-api-base" content="../"`), 1)
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		http.ServeContent(w, r, "index.html", time.Time{}, bytes.NewReader(content))
	}
	rootFiles := http.FileServer(http.FS(assets))
	files := http.StripPrefix("/idata/", rootFiles)
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		// The existing Vue browser build compiles its templates at runtime.
		w.Header().Set("Content-Security-Policy", "default-src 'self'; connect-src 'self'; frame-src blob:; img-src 'self' data: blob:; style-src 'self' 'unsafe-inline'; script-src 'self' 'unsafe-eval'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'")
		if r.URL.Path == "/" {
			serveIndex(w, r, false)
			return
		}
		rootFiles.ServeHTTP(w, r)
	})
	mux.HandleFunc("GET /idata/", func(w http.ResponseWriter, r *http.Request) {
		// The existing Vue browser build compiles its templates at runtime.
		w.Header().Set("Content-Security-Policy", "default-src 'self'; connect-src 'self'; frame-src blob:; img-src 'self' data: blob:; style-src 'self' 'unsafe-inline'; script-src 'self' 'unsafe-eval'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'")
		if r.URL.Path == "/idata/" {
			serveIndex(w, r, true)
			return
		}
		files.ServeHTTP(w, r)
	})
	for _, method := range []string{"GET", "POST", "PUT"} {
		mux.HandleFunc(method+" /api/v1/clients/{client_id}/idata/{operation...}", s.handleIDATA)
	}
}

func (s *Server) handleIDATA(w http.ResponseWriter, r *http.Request) {
	if !sameOriginOrNative(r) || r.Header.Get("Sec-Fetch-Site") == "cross-site" {
		writeError(w, 403, "Cross-origin request rejected.")
		return
	}
	id := r.PathValue("client_id")
	if !clientIDPattern.MatchString(id) {
		writeError(w, 400, "Invalid client ID.")
		return
	}
	var client *clientConn
	if token, ok := bearerTokenFromRequest(r); ok && secureEqual(token, s.config.AdminToken) {
		client = s.hub.get(id)
	} else {
		var err error
		client, err = s.pairings.clientForIPSession(r, s.hub, id)
		if err != nil {
			client, _, err = s.pairings.clientForRequest(r, s.hub)
			if err != nil || client == nil || client.info.ID != id {
				writeError(w, 403, "A session authorized for this PC is required.")
				return
			}
		}
	}
	if client == nil {
		writeError(w, 503, "Selected PC is offline.")
		return
	}
	if !hasCapability(client.info.Capabilities, "server_commands_v1") {
		writeError(w, 409, "Update the selected PC to a server-command IDATA client.")
		return
	}
	if r.URL.RawQuery != "" {
		writeError(w, 400, "Query parameters are not supported.")
		return
	}
	operation := r.PathValue("operation")
	validOperation := r.Method == http.MethodGet && idataReadOperation.MatchString(operation) ||
		r.Method == http.MethodPost && idataWriteOperation.MatchString(operation) ||
		r.Method == http.MethodPut && (operation == "settings" || operation == "model-config")
	if !validOperation {
		writeError(w, 400, "Invalid IDATA operation.")
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxRequestBody))
	if err != nil {
		writeError(w, 413, "Request is too large.")
		return
	}
	if len(bytes.TrimSpace(body)) > 0 && !json.Valid(body) {
		writeError(w, 400, "Invalid JSON request.")
		return
	}
	timeout := 40 * time.Second
	command := idataWorkerCommand(client.info.OS, operation, r.Method, body)
	ctx, cancel := context.WithTimeout(r.Context(), timeout+15*time.Second)
	defer cancel()
	result, err := client.sendCommand(ctx, command, timeout)
	if err != nil {
		writeError(w, 502, "PC disconnected or the server command timed out. Check operation status before retrying.")
		return
	}
	if result.ExitCode != 0 || result.Error != "" {
		detail := strings.TrimSpace(result.Stderr)
		if detail == "" {
			detail = result.Error
		}
		writeError(w, 502, "The server command failed: "+detail)
		return
	}
	var response idataWorkerResponse
	if err := json.Unmarshal([]byte(strings.TrimSpace(result.Stdout)), &response); err != nil {
		writeError(w, 502, "The server command returned an invalid response.")
		return
	}
	if !response.OK {
		writeError(w, 400, response.Error)
		return
	}
	if strings.HasSuffix(r.PathValue("operation"), "/content") {
		var report struct {
			ContentBase64 string `json:"contentBase64"`
		}
		if json.Unmarshal(response.Data, &report) != nil {
			writeError(w, 502, "The report response was invalid.")
			return
		}
		content, err := base64.StdEncoding.DecodeString(report.ContentBase64)
		if err != nil || len(content) > 8<<20 {
			writeError(w, 502, "The report response exceeded the viewing limit.")
			return
		}
		w.Header().Set("Content-Security-Policy", "sandbox; default-src 'none'; img-src data:; style-src 'unsafe-inline'")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(content)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(response.Data)
}

func idataWorkerCommand(goos, operation, method string, body []byte) string {
	source := base64.StdEncoding.EncodeToString(idataWorkerSource)
	encodedOperation := base64.StdEncoding.EncodeToString([]byte(operation))
	encodedMethod := base64.StdEncoding.EncodeToString([]byte(method))
	payload := base64.StdEncoding.EncodeToString(body)
	loader := "import base64,sys;source=base64.b64decode(sys.argv.pop(1));sys.argv[1:3]=[base64.b64decode(v).decode() for v in sys.argv[1:3]];globals()['SERVER_WORKER_SOURCE']=source;exec(compile(source,'<idata-server-worker>','exec'))"
	if goos == "windows" {
		// The Server materializes its own worker and tells IDATA.exe to run it. The
		// Client neither stores business logic nor decides which executable/args to use.
		script := fmt.Sprintf(`$ErrorActionPreference='Stop'; $root=Join-Path $env:LOCALAPPDATA 'IDATA\server-command-runtime'; [IO.Directory]::CreateDirectory($root) | Out-Null; $worker=Join-Path $root 'worker.py'; [IO.File]::WriteAllBytes($worker,[Convert]::FromBase64String('%s')); $operation=[Text.Encoding]::UTF8.GetString([Convert]::FromBase64String('%s')); $method=[Text.Encoding]::UTF8.GetString([Convert]::FromBase64String('%s')); $idata=Join-Path $env:IDATA_CLIENT_EXECUTABLE_DIRECTORY 'IDATA.exe'; & $idata cli bundle run --path $worker -- $operation $method '%s'; exit $LASTEXITCODE`, source, encodedOperation, encodedMethod, payload)
		label := strings.Map(func(value rune) rune {
			if value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z' || value >= '0' && value <= '9' || strings.ContainsRune("/_-", value) {
				return value
			}
			return '-'
		}, operation)
		return "rem Server operation " + label + ": curl.exe download, archive extraction/replacement, IDATA.exe cli bundle run & powershell.exe -NoLogo -NoProfile -NonInteractive -ExecutionPolicy Bypass -EncodedCommand " + powershellEncodedCommand(script)
	}
	return fmt.Sprintf(`python3 -c '%s' '%s' '%s' '%s' '%s'`, loader, source, encodedOperation, encodedMethod, payload)
}

func powershellEncodedCommand(script string) string {
	utf16 := make([]byte, len([]rune(script))*2)
	for index, value := range []rune(script) {
		binary.LittleEndian.PutUint16(utf16[index*2:], uint16(value))
	}
	return base64.StdEncoding.EncodeToString(utf16)
}
