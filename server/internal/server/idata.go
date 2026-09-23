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

//go:embed idataworker/worker.ps1
var idataWindowsWorkerSource []byte

type idataWorkerResponse struct {
	OK    bool            `json:"ok"`
	Data  json.RawMessage `json:"data"`
	Error string          `json:"error"`
}

var (
	idataReadOperation   = regexp.MustCompile(`^(devices|settings|test-cases|test-cases/update|test-runs|test-runs/TR-[0-9]+/(reports|logs)/[A-Za-z0-9._%-]+/content)$`)
	idataDeleteOperation = regexp.MustCompile(`^test-runs/TR-[0-9]+$`)
	idataWriteOperation  = regexp.MustCompile(`^(test-cases/update|test-runs|test-runs/TR-[0-9]+/close|test-runs/TR-[0-9]+/(reports|logs)/[A-Za-z0-9._%-]+/open)$`)
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
	for _, method := range []string{"GET", "POST", "PUT", "DELETE"} {
		mux.HandleFunc(method+" /api/v1/clients/{client_id}/idata/{operation...}", s.handleIDATA)
	}
}

func (s *Server) handleIDATA(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-IDATA-Version", ReleaseVersion)
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
	if !hasCapability(client.info.Capabilities, "command_stdin_v1") {
		writeError(w, 409, "Update the selected PC to IDATA Client 0.7.14 or newer; this Client cannot receive Server worker input.")
		return
	}
	if r.URL.RawQuery != "" {
		writeError(w, 400, "Query parameters are not supported.")
		return
	}
	operation := r.PathValue("operation")
	validOperation := r.Method == http.MethodGet && idataReadOperation.MatchString(operation) ||
		r.Method == http.MethodPost && idataWriteOperation.MatchString(operation) ||
		r.Method == http.MethodPut && operation == "settings" ||
		r.Method == http.MethodDelete && idataDeleteOperation.MatchString(operation)
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
	command := idataWorkerCommand(client.info.OS, operation, r.Method)
	ctx, cancel := context.WithTimeout(r.Context(), timeout+15*time.Second)
	defer cancel()
	result, err := client.sendCommandWithInput(ctx, command, idataWorkerInput(client.info.OS, body), timeout)
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
	if result.StdoutTruncated {
		writeError(w, 502, "The Server worker response exceeded the Client output limit and was truncated. Full execution logs remain on the PC; reduce task history or increase IDATA_OUTPUT_LIMIT on the Client.")
		return
	}
	response, err := decodeIDATAWorkerResponse(result.Stdout)
	if err != nil {
		detail := strings.TrimSpace(result.Stderr)
		if detail == "" {
			detail = strings.TrimSpace(result.Stdout)
		}
		if len(detail) > 2000 {
			detail = detail[len(detail)-2000:]
		}
		if detail == "" {
			detail = "The Server worker returned no response."
		}
		writeError(w, 502, "The Server v"+ReleaseVersion+" command returned an invalid response: "+detail)
		return
	}
	if !response.OK {
		writeError(w, 400, response.Error)
		return
	}
	if strings.HasSuffix(r.PathValue("operation"), "/content") {
		var contentResponse struct {
			ContentBase64 string `json:"contentBase64"`
		}
		if json.Unmarshal(response.Data, &contentResponse) != nil {
			writeError(w, 502, "The file response was invalid.")
			return
		}
		content, err := base64.StdEncoding.DecodeString(contentResponse.ContentBase64)
		if err != nil || len(content) > 8<<20 {
			writeError(w, 502, "The file response exceeded the download limit.")
			return
		}
		if strings.Contains(operation, "/logs/") {
			parts := strings.Split(operation, "/")
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s-%s.log"`, parts[1], parts[3]))
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(content)
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

func idataWorkerCommand(goos, operation, method string) string {
	encodedOperation := base64.StdEncoding.EncodeToString([]byte(operation))
	encodedMethod := base64.StdEncoding.EncodeToString([]byte(method))
	loader := "import base64,sys;payload,_,source=sys.stdin.buffer.read().partition(b\"\\n\");sys.argv[1:3]=[base64.b64decode(v).decode() for v in sys.argv[1:3]];sys.argv.append(payload.decode());globals()[\"SERVER_WORKER_SOURCE\"]=source;exec(compile(source,\"<idata-server-worker>\",\"exec\"))"
	if goos == "windows" {
		// The Server materializes and invokes its PowerShell worker. IDATA.exe is
		// reserved for actual test-case bundle execution inside that worker.
		script := fmt.Sprintf(`$ErrorActionPreference='Stop'; $ProgressPreference='SilentlyContinue'; $root=Join-Path $env:LOCALAPPDATA 'IDATA\server-command-runtime'; [IO.Directory]::CreateDirectory($root) | Out-Null; $token=[guid]::NewGuid().ToString('N'); $worker=Join-Path $root ($token+'.ps1'); $request=Join-Path $root ($token+'.json'); $utf8=New-Object System.Text.UTF8Encoding($false); $scriptUtf8=New-Object System.Text.UTF8Encoding($true); [Console]::InputEncoding=$utf8; [Console]::OutputEncoding=$utf8; try{$wire=[Console]::In.ReadToEnd(); $split=$wire.IndexOf([char]10); if($split -lt 0){throw 'Invalid Server input'}; $payload=$wire.Substring(0,$split).TrimEnd([char]13); [IO.File]::WriteAllText($worker,$wire.Substring($split+1),$scriptUtf8); if($payload){[IO.File]::WriteAllBytes($request,[Convert]::FromBase64String($payload))}else{[IO.File]::WriteAllText($request,'',$utf8)}; & $worker -RequestPath $request -OperationB64 '%s' -MethodB64 '%s'; exit 0}catch{[Console]::Error.WriteLine(('Server PowerShell worker failed. '+$_.Exception.Message)); exit 1}finally{Remove-Item -LiteralPath $request,$worker -Force -ErrorAction SilentlyContinue}`, encodedOperation, encodedMethod)
		return "powershell.exe -NoLogo -NoProfile -NonInteractive -ExecutionPolicy Bypass -EncodedCommand " + powershellEncodedCommand(script)
	}
	return fmt.Sprintf(`python3 -c '%s' '%s' '%s'`, loader, encodedOperation, encodedMethod)
}

func idataWorkerInput(goos string, body []byte) []byte {
	worker := idataWorkerSource
	if goos == "windows" {
		worker = idataWindowsWorkerSource
	}
	payload := base64.StdEncoding.EncodeToString(body)
	input := make([]byte, 0, len(payload)+1+len(worker))
	input = append(input, payload...)
	input = append(input, '\n')
	return append(input, worker...)
}

func decodeIDATAWorkerResponse(stdout string) (idataWorkerResponse, error) {
	const marker = "__IDATA_SERVER_RESPONSE__"
	if offset := strings.LastIndex(stdout, marker); offset >= 0 {
		encoded := strings.TrimSpace(stdout[offset+len(marker):])
		if fields := strings.Fields(encoded); len(fields) > 0 {
			data, decodeErr := base64.StdEncoding.DecodeString(fields[0])
			if decodeErr == nil {
				var response idataWorkerResponse
				if json.Unmarshal(data, &response) == nil {
					return response, nil
				}
			}
		}
	}
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	for index := len(lines) - 1; index >= 0; index-- {
		var response idataWorkerResponse
		if json.Unmarshal([]byte(strings.TrimSpace(lines[index])), &response) == nil {
			return response, nil
		}
	}
	return idataWorkerResponse{}, fmt.Errorf("no worker response found")
}

func powershellEncodedCommand(script string) string {
	utf16 := make([]byte, len([]rune(script))*2)
	for index, value := range []rune(script) {
		binary.LittleEndian.PutUint16(utf16[index*2:], uint16(value))
	}
	return base64.StdEncoding.EncodeToString(utf16)
}
