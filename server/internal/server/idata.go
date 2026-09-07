package server

import (
	"context"
	"embed"
	"io"
	"io/fs"
	"net/http"
	"strings"
	"time"

	"idata-server/internal/protocol"
)

//go:embed idataweb
var idataFiles embed.FS

func (s *Server) registerIDATA(mux *http.ServeMux) {
	assets, err := fs.Sub(idataFiles, "idataweb")
	if err != nil {
		panic(err)
	}
	rootFiles := http.FileServer(http.FS(assets))
	files := http.StripPrefix("/idata/", rootFiles)
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		// The existing Vue browser build compiles its templates at runtime.
		w.Header().Set("Content-Security-Policy", "default-src 'self'; connect-src 'self'; frame-src blob:; img-src 'self' data: blob:; style-src 'self' 'unsafe-inline'; script-src 'self' 'unsafe-eval'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'")
		rootFiles.ServeHTTP(w, r)
	})
	mux.HandleFunc("GET /idata/", func(w http.ResponseWriter, r *http.Request) {
		// The existing Vue browser build compiles its templates at runtime.
		w.Header().Set("Content-Security-Policy", "default-src 'self'; connect-src 'self'; frame-src blob:; img-src 'self' data: blob:; style-src 'self' 'unsafe-inline'; script-src 'self' 'unsafe-eval'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'")
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
	if !hasCapability(client.info.Capabilities, "idata_api_v1") {
		writeError(w, 409, "Update the selected PC to the combined IDATA client.")
		return
	}
	if r.URL.RawQuery != "" {
		writeError(w, 400, "Query parameters are not supported.")
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxRequestBody))
	if err != nil {
		writeError(w, 413, "Request is too large.")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 35*time.Second)
	defer cancel()
	result, err := client.sendRequest(ctx, protocol.Message{Type: protocol.TypeAPIRequest, Method: r.Method, Path: "/api/" + r.PathValue("operation"), Data: body})
	if err != nil {
		writeError(w, 502, "PC disconnected or the operation timed out. Check run status before retrying a test launch.")
		return
	}
	if result.Status < 200 || result.Status > 599 || len(result.Data) > 8<<20 {
		writeError(w, 502, "Invalid response from PC.")
		return
	}
	if result.Error != "" {
		writeError(w, result.Status, result.Error)
		return
	}
	contentType := "application/json"
	if strings.HasPrefix(result.ContentType, "text/html") {
		contentType = "text/html; charset=utf-8"
		// Local reports cannot execute scripts against the authenticated control origin.
		w.Header().Set("Content-Security-Policy", "sandbox; default-src 'none'; img-src data:; style-src 'unsafe-inline'")
	}
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(result.Status)
	_, _ = w.Write(result.Data)
}
