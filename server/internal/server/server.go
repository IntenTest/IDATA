package server

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"idata-server/internal/protocol"
)

const maxRequestBody = 64 << 10

var (
	clientIDPattern        = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)
	deviceTokenHashPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)
	pairingIDPattern       = regexp.MustCompile(`^[a-f0-9]{32}$`)
)

type Server struct {
	config           Config
	hub              *Hub
	pairings         *pairingManager
	enrollments      *enrollmentManager
	logger           *slog.Logger
	upgrader         websocket.Upgrader
	terminalUpgrader websocket.Upgrader
}

func New(config Config, logger *slog.Logger) (*Server, error) {
	if config.AdminToken == "" {
		return nil, errors.New("admin token is required")
	}
	if config.AgentToken != "" && secureEqual(config.AgentToken, config.AdminToken) {
		return nil, errors.New("agent token and admin token must be different")
	}
	if config.DefaultTimeout <= 0 {
		config.DefaultTimeout = 30 * time.Second
	}
	if config.DeviceSessionTTL == 0 {
		config.DeviceSessionTTL = 8 * time.Hour
	}
	if config.DeviceSessionTTL < time.Minute || config.DeviceSessionTTL > 24*time.Hour {
		return nil, errors.New("device session TTL must be between 1 minute and 24 hours")
	}
	if config.PairingRequestTTL == 0 {
		config.PairingRequestTTL = 2 * time.Minute
	}
	if config.PairingRequestTTL < 30*time.Second || config.PairingRequestTTL > 10*time.Minute {
		return nil, errors.New("pairing request TTL must be between 30 seconds and 10 minutes")
	}
	if config.MaxCommandTimeout < config.DefaultTimeout {
		return nil, errors.New("max command timeout must be at least the default timeout")
	}
	if logger == nil {
		logger = slog.Default()
	}
	if config.BrowserPairingEnabled {
		logger.Info("legacy Windows confirmation compatibility enabled", "session_ttl", config.DeviceSessionTTL, "request_ttl", config.PairingRequestTTL)
	}
	if config.EnrollmentAutoApprove {
		logger.Warn("automatic device enrollment enabled; every valid native client request will be approved")
	}
	enrollments, err := newEnrollmentManager(config.DeviceCredentialsFile, logger)
	if err != nil {
		return nil, err
	}
	return &Server{
		config:      config,
		hub:         NewHub(),
		pairings:    newPairingManager(config, logger),
		enrollments: enrollments,
		logger:      logger,
		upgrader: websocket.Upgrader{
			HandshakeTimeout: 10 * time.Second,
			CheckOrigin: func(r *http.Request) bool {
				// Native agents do not send Origin. Reject browser cross-origin use.
				return r.Header.Get("Origin") == ""
			},
		},
		terminalUpgrader: websocket.Upgrader{
			HandshakeTimeout: 10 * time.Second,
			CheckOrigin:      sameOriginOrNative,
		},
	}, nil
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	s.registerIDATA(mux)
	mux.HandleFunc("GET /healthz", s.handleHealth)
	mux.HandleFunc("GET /ws/agent", s.handleAgent)
	mux.HandleFunc("POST /api/v1/enrollments", s.handleEnrollmentStart)
	mux.HandleFunc("POST /api/v1/enrollments/{enrollment_id}/status", s.handleEnrollmentStatus)
	mux.HandleFunc("GET /api/v1/self", s.handleSelf)
	mux.HandleFunc("POST /api/v1/ip-login", s.handleIPLogin)
	mux.HandleFunc("POST /api/v1/self/logout", s.handleDeviceLogout)
	mux.HandleFunc("GET /api/v1/pairing-candidates", s.handlePairingCandidates)
	mux.HandleFunc("POST /api/v1/pairings/{client_id}", s.handlePairingStart)
	mux.HandleFunc("POST /api/v1/pairings/{pairing_id}/status", s.handlePairingStatus)
	mux.Handle("GET /api/v1/clients", s.requireAdmin(http.HandlerFunc(s.handleClients)))
	mux.Handle("GET /api/v1/enrollments", s.requireAdmin(http.HandlerFunc(s.handleEnrollments)))
	mux.Handle("POST /api/v1/enrollments/{enrollment_id}/approve", s.requireAdmin(http.HandlerFunc(s.handleEnrollmentApprove)))
	mux.Handle("POST /api/v1/enrollments/{enrollment_id}/deny", s.requireAdmin(http.HandlerFunc(s.handleEnrollmentDeny)))
	mux.Handle("DELETE /api/v1/device-credentials/{credential_id}", s.requireAdmin(http.HandlerFunc(s.handleCredentialRevoke)))
	mux.HandleFunc("GET /api/v1/clients/{client_id}/terminal", s.handleTerminal)
	mux.Handle("POST /api/v1/clients/", s.requireAdmin(http.HandlerFunc(s.handleCommand)))
	return securityHeaders(mux)
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleEnrollmentStart(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Origin") != "" {
		writeError(w, http.StatusForbidden, "native client request required")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var identity enrollmentIdentity
	if err := decoder.Decode(&identity); err != nil || !validEnrollmentIdentity(identity) {
		writeError(w, http.StatusBadRequest, "invalid device enrollment request")
		return
	}
	remoteIP, err := addressIP(r.RemoteAddr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid remote address")
		return
	}
	id, pollToken, expiresAt, err := s.enrollments.start(identity, remoteIP.String())
	if err != nil {
		if errors.Is(err, errEnrollmentCapacity) {
			writeError(w, http.StatusServiceUnavailable, "too many pending device requests")
			return
		}
		if errors.Is(err, errEnrollmentRateLimited) {
			writeError(w, http.StatusTooManyRequests, "too many device requests from this address")
			return
		}
		s.logger.Error("start device enrollment failed", "client_id", identity.ClientID, "error", err)
		writeError(w, http.StatusInternalServerError, "could not create device request")
		return
	}
	status := "pending"
	if s.config.EnrollmentAutoApprove {
		if err := s.enrollments.approve(id); err != nil {
			s.logger.Error("auto-approve device enrollment failed", "enrollment_id", id, "client_id", identity.ClientID, "remote_ip", remoteIP.String(), "error", err)
			writeError(w, http.StatusInternalServerError, "could not approve device request")
			return
		}
		status = "approved"
		s.logger.Info("device enrollment auto-approved", "enrollment_id", id, "client_id", identity.ClientID, "remote_ip", remoteIP.String())
	}
	writeJSON(w, http.StatusAccepted, map[string]any{
		"enrollment_id": id, "poll_token": pollToken,
		"expires_at": expiresAt.Format(time.RFC3339), "status": status,
	})
}

func (s *Server) handleEnrollmentStatus(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("enrollment_id")
	pollToken, ok := enrollmentTokenFromRequest(r)
	if !pairingIDPattern.MatchString(id) || !ok {
		writeError(w, http.StatusUnauthorized, "invalid enrollment authorization")
		return
	}
	status, agentToken, err := s.enrollments.poll(id, pollToken)
	if err != nil {
		switch {
		case errors.Is(err, errEnrollmentUnauthorized):
			writeError(w, http.StatusUnauthorized, "invalid enrollment authorization")
		case errors.Is(err, errEnrollmentExpired):
			writeError(w, http.StatusGone, "device request expired")
		default:
			writeError(w, http.StatusNotFound, "device request not found")
		}
		return
	}
	switch status {
	case "pending":
		writeJSON(w, http.StatusAccepted, map[string]string{"status": "pending"})
	case "approved":
		writeJSON(w, http.StatusOK, map[string]string{"status": "approved", "agent_token": agentToken})
	case "denied":
		writeJSON(w, http.StatusOK, map[string]string{"status": "denied", "reason": "denied_by_admin"})
	default:
		writeError(w, http.StatusInternalServerError, "invalid device request state")
	}
}

func (s *Server) handleEnrollments(w http.ResponseWriter, _ *http.Request) {
	requests, credentials := s.enrollments.list()
	writeJSON(w, http.StatusOK, map[string]any{"requests": requests, "credentials": credentials})
}

func (s *Server) handleEnrollmentApprove(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("enrollment_id")
	if !pairingIDPattern.MatchString(id) {
		writeError(w, http.StatusBadRequest, "invalid enrollment ID")
		return
	}
	if err := s.enrollments.approve(id); err != nil {
		if errors.Is(err, errEnrollmentNotFound) {
			writeError(w, http.StatusNotFound, "device request not found")
			return
		}
		s.logger.Error("approve device enrollment failed", "enrollment_id", id, "error", err)
		writeError(w, http.StatusInternalServerError, "could not approve device request")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "approved"})
}

func (s *Server) handleEnrollmentDeny(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("enrollment_id")
	if !pairingIDPattern.MatchString(id) {
		writeError(w, http.StatusBadRequest, "invalid enrollment ID")
		return
	}
	if err := s.enrollments.deny(id); err != nil {
		writeError(w, http.StatusNotFound, "device request not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "denied"})
}

func (s *Server) handleCredentialRevoke(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("credential_id")
	if !pairingIDPattern.MatchString(id) {
		writeError(w, http.StatusBadRequest, "invalid credential ID")
		return
	}
	credential, err := s.enrollments.revoke(id)
	if err != nil {
		if errors.Is(err, errEnrollmentNotFound) {
			writeError(w, http.StatusNotFound, "device credential not found")
			return
		}
		s.logger.Error("revoke device credential failed", "credential_id", id, "error", err)
		writeError(w, http.StatusInternalServerError, "could not revoke device credential")
		return
	}
	if client := s.hub.get(credential.ClientID); client != nil && client.credentialID == id {
		client.close(websocket.ClosePolicyViolation, "device credential revoked")
	}
	w.WriteHeader(http.StatusNoContent)
}

func validEnrollmentIdentity(identity enrollmentIdentity) bool {
	if !clientIDPattern.MatchString(identity.ClientID) || !deviceTokenHashPattern.MatchString(identity.DeviceTokenHash) {
		return false
	}
	if !validEnrollmentOS(identity.OS) || len(identity.Hostname) == 0 || len(identity.Hostname) > 255 || len(identity.Username) > 255 || len(identity.ClientVersion) > 32 || len(identity.Arch) > 32 {
		return false
	}
	if identity.LocalIP != "" && net.ParseIP(identity.LocalIP) == nil {
		return false
	}
	if identity.MACAddress != "" {
		if _, err := net.ParseMAC(identity.MACAddress); err != nil {
			return false
		}
	}
	return true
}

func validEnrollmentOS(value string) bool {
	switch value {
	case "windows", "darwin", "linux":
		return true
	default:
		return false
	}
}

func enrollmentTokenFromRequest(r *http.Request) (string, bool) {
	const prefix = "Enrollment "
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, prefix) {
		return "", false
	}
	token := strings.TrimPrefix(header, prefix)
	return token, len(token) >= 32 && len(token) <= 128
}

func (s *Server) handleAgent(w http.ResponseWriter, r *http.Request) {
	token, ok := bearerTokenFromRequest(r)
	legacyAuthentication := ok && s.config.AgentToken != "" && secureEqual(token, s.config.AgentToken)
	credential, deviceAuthentication := s.enrollments.authenticate(token)
	if !legacyAuthentication && !deviceAuthentication {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.logger.Warn("websocket upgrade failed", "remote", r.RemoteAddr, "error", err)
		return
	}

	_ = conn.SetReadDeadline(time.Now().Add(15 * time.Second))
	conn.SetReadLimit(12 << 20)
	var hello protocol.Message
	if err := conn.ReadJSON(&hello); err != nil {
		_ = conn.Close()
		return
	}
	if err := validateHello(hello); err != nil {
		_ = conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.ClosePolicyViolation, err.Error()), time.Now().Add(time.Second))
		_ = conn.Close()
		return
	}
	if deviceAuthentication && (hello.ClientID != credential.ClientID || !secureEqual(hello.DeviceTokenHash, credential.DeviceTokenHash)) {
		_ = conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "device credential does not match this client"), time.Now().Add(time.Second))
		_ = conn.Close()
		return
	}

	now := time.Now().UTC()
	client := newClientConn(conn, protocol.ClientInfo{
		ID:            hello.ClientID,
		Hostname:      hello.Hostname,
		OS:            hello.OS,
		Arch:          hello.Arch,
		ClientVersion: hello.ClientVersion,
		Capabilities:  append([]string(nil), hello.Capabilities...),
		ConnectedAt:   now.Format(time.RFC3339),
		RemoteAddress: r.RemoteAddr,
	}, hello.DeviceTokenHash)
	if deviceAuthentication {
		client.credentialID = credential.ID
	}
	_ = conn.SetReadDeadline(time.Now().Add(90 * time.Second))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(90 * time.Second))
	})
	s.hub.register(client)
	s.logger.Info("client connected", "client_id", client.info.ID, "os", client.info.OS, "remote", r.RemoteAddr)

	pingDone := make(chan struct{})
	go func() {
		defer close(pingDone)
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := client.writePing(); err != nil {
					return
				}
			case <-client.done:
				return
			}
		}
	}()

	for {
		var message protocol.Message
		if err := conn.ReadJSON(&message); err != nil {
			break
		}
		if message.ProtocolVersion != protocol.Version {
			continue
		}
		switch message.Type {
		case protocol.TypeResult, protocol.TypeAPIResponse:
			if message.RequestID != "" {
				client.deliver(message)
			}
		case protocol.TypeTerminalOpened, protocol.TypeTerminalOutput, protocol.TypeTerminalClosed:
			if message.SessionID != "" {
				client.deliverTerminal(message)
			}
		case protocol.TypePairingResult:
			s.pairings.resolve(client, message)
		}
	}
	client.close(websocket.CloseNormalClosure, "connection closed")
	<-pingDone
	s.pairings.disconnect(client)
	s.hub.unregister(client)
	s.logger.Info("client disconnected", "client_id", client.info.ID)
}

type terminalAuth struct {
	Type  string `json:"type"`
	Token string `json:"token"`
	Mode  string `json:"mode,omitempty"`
}

func (s *Server) handleTerminal(w http.ResponseWriter, r *http.Request) {
	clientID := r.PathValue("client_id")
	if !clientIDPattern.MatchString(clientID) {
		writeError(w, http.StatusBadRequest, "invalid client ID")
		return
	}
	conn, err := s.terminalUpgrader.Upgrade(w, r, nil)
	if err != nil {
		s.logger.Warn("terminal websocket upgrade failed", "remote", r.RemoteAddr, "error", err)
		return
	}
	defer conn.Close()
	conn.SetReadLimit(128 << 10)
	_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	var auth terminalAuth
	if err := conn.ReadJSON(&auth); err != nil {
		_ = writeTerminalJSON(conn, map[string]string{"type": "terminal_error", "error": "unauthorized"})
		_ = conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "unauthorized"), time.Now().Add(time.Second))
		return
	}
	client, authError := s.terminalClientForAuth(clientID, auth, r)
	authMode := auth.Mode
	if authMode == "" {
		authMode = "admin"
	}
	auth.Token = ""
	if authError != "" {
		_ = writeTerminalJSON(conn, map[string]string{"type": "terminal_error", "error": authError})
		_ = conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.ClosePolicyViolation, authError), time.Now().Add(time.Second))
		return
	}
	if client == nil {
		_ = writeTerminalJSON(conn, map[string]string{"type": "terminal_error", "error": "client is not online"})
		return
	}
	if !hasCapability(client.info.Capabilities, "terminal_v1") {
		_ = writeTerminalJSON(conn, map[string]string{"type": "terminal_error", "error": "client does not support interactive terminal"})
		return
	}
	bridge, err := client.openTerminal()
	if err != nil {
		_ = writeTerminalJSON(conn, map[string]string{"type": "terminal_error", "error": err.Error()})
		return
	}
	s.logger.Info("terminal session requested", "client_id", clientID, "session_id", bridge.sessionID, "remote", r.RemoteAddr, "auth_mode", authMode)
	defer func() {
		_ = client.writeTerminal(protocol.Message{Type: protocol.TypeTerminalClose, SessionID: bridge.sessionID})
		client.removeTerminal(bridge)
		s.logger.Info("terminal session ended", "client_id", clientID, "session_id", bridge.sessionID, "remote", r.RemoteAddr)
	}()

	_ = conn.SetReadDeadline(time.Now().Add(90 * time.Second))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(90 * time.Second))
	})
	browserMessages := make(chan protocol.Message, 8)
	browserDone := make(chan error, 1)
	go func() {
		for {
			var message protocol.Message
			if err := conn.ReadJSON(&message); err != nil {
				browserDone <- err
				return
			}
			select {
			case browserMessages <- message:
			case <-bridge.done:
				return
			}
		}
	}()

	openTimer := time.NewTimer(10 * time.Second)
	defer openTimer.Stop()
	pingTicker := time.NewTicker(30 * time.Second)
	defer pingTicker.Stop()
	opened := false
	for {
		select {
		case message := <-bridge.messages:
			switch message.Type {
			case protocol.TypeTerminalOpened:
				opened = true
				if err := writeTerminalJSON(conn, message); err != nil {
					return
				}
			case protocol.TypeTerminalOutput:
				if opened {
					if err := writeTerminalJSON(conn, message); err != nil {
						return
					}
				}
			case protocol.TypeTerminalClosed:
				_ = writeTerminalJSON(conn, message)
				return
			}
		case message := <-browserMessages:
			if !opened {
				continue
			}
			switch message.Type {
			case protocol.TypeTerminalInput:
				if len(message.Data) == 0 || len(message.Data) > 64<<10 {
					continue
				}
				message.SessionID = bridge.sessionID
				if err := client.writeTerminal(message); err != nil {
					return
				}
			case protocol.TypeTerminalResize:
				if message.Columns < 1 || message.Columns > 500 || message.Rows < 1 || message.Rows > 500 {
					continue
				}
				message.SessionID = bridge.sessionID
				if err := client.writeTerminal(message); err != nil {
					return
				}
			case protocol.TypeTerminalClose:
				return
			}
		case <-openTimer.C:
			if !opened {
				_ = writeTerminalJSON(conn, map[string]string{"type": "terminal_error", "error": "client did not open terminal"})
				return
			}
		case <-pingTicker.C:
			if err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(10*time.Second)); err != nil {
				return
			}
		case <-browserDone:
			return
		case <-bridge.done:
			return
		case <-client.done:
			_ = writeTerminalJSON(conn, map[string]string{"type": "terminal_error", "error": "client connection ended"})
			return
		case <-r.Context().Done():
			return
		}
	}
}

func (s *Server) terminalClientForAuth(clientID string, auth terminalAuth, r *http.Request) (*clientConn, string) {
	if auth.Type != "auth" {
		return nil, "unauthorized"
	}
	if auth.Mode == "" && auth.Token != "" {
		if !secureEqual(auth.Token, s.config.AdminToken) {
			return nil, "unauthorized"
		}
		return s.hub.get(clientID), ""
	}
	if auth.Mode == "session" {
		if auth.Token != "" {
			return nil, "unauthorized"
		}
		client, _, err := s.pairings.clientForRequest(r, s.hub)
		if err != nil {
			return nil, "unauthorized"
		}
		if client.info.ID != clientID {
			return nil, "device_scope_violation"
		}
		return client, ""
	}
	if auth.Mode == "ip_session" {
		if auth.Token != "" {
			return nil, "unauthorized"
		}
		client, err := s.pairings.clientForIPSession(r, s.hub, clientID)
		if err != nil {
			return nil, "device_scope_violation"
		}
		return client, ""
	}
	if auth.Mode != "device" || auth.Token == "" {
		return nil, "unauthorized"
	}
	client, err := s.hub.clientForDeviceToken(auth.Token)
	if err != nil {
		return nil, "unauthorized"
	}
	if client.info.ID != clientID {
		return nil, "device_scope_violation"
	}
	return client, ""
}

func writeTerminalJSON(conn *websocket.Conn, value any) error {
	_ = conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	return conn.WriteJSON(value)
}

func sameOriginOrNative(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	parsed, err := url.Parse(origin)
	if err != nil || !strings.EqualFold(parsed.Host, r.Host) {
		return false
	}
	if r.TLS != nil {
		return parsed.Scheme == "https"
	}
	return parsed.Scheme == "http"
}

func sameOriginRequest(r *http.Request) bool {
	return r.Header.Get("Origin") != "" && sameOriginOrNative(r)
}

func hasCapability(values []string, capability string) bool {
	for _, value := range values {
		if value == capability {
			return true
		}
	}
	return false
}

func (s *Server) handleClients(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"clients": s.hub.List()})
}

func (s *Server) handleSelf(w http.ResponseWriter, r *http.Request) {
	if clients, expiresAt, err := s.pairings.clientsForIPSession(r, s.hub); err == nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"clients": clients, "auth_mode": "ip_session",
			"session_expires_at": expiresAt.UTC().Format(time.RFC3339),
		})
		return
	}
	if client, expiresAt, err := s.pairings.clientForRequest(r, s.hub); err == nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"clients": s.hub.sanitizedList(), "self_client_id": client.info.ID,
			"auth_mode": "session", "session_expires_at": expiresAt.UTC().Format(time.RFC3339),
		})
		return
	}
	deviceToken, ok := deviceTokenFromRequest(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "device authentication required")
		return
	}
	clients, selfClientID, err := s.hub.listForDeviceToken(deviceToken)
	if err != nil {
		switch {
		case errors.Is(err, ErrDeviceTokenAmbiguous):
			writeError(w, http.StatusConflict, "device token is assigned to multiple online clients")
		default:
			writeError(w, http.StatusUnauthorized, "device token is invalid or the client is offline")
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"clients": clients, "self_client_id": selfClientID, "auth_mode": "device",
	})
}

func (s *Server) handleIPLogin(w http.ResponseWriter, r *http.Request) {
	if !sameOriginRequest(r) {
		writeError(w, http.StatusForbidden, "same-origin request required")
		return
	}
	clients, err := s.hub.clientsForIP(r.RemoteAddr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid remote address")
		return
	}
	if len(clients) == 0 {
		writeJSON(w, http.StatusAccepted, map[string]string{"status": "waiting"})
		return
	}
	token, expiresAt, err := s.pairings.createIPSession(r.RemoteAddr)
	if err != nil {
		s.logger.Error("create IP browser session failed", "remote", r.RemoteAddr, "error", err)
		writeError(w, http.StatusInternalServerError, "could not create browser session")
		return
	}
	setDeviceSessionCookie(w, r, token, expiresAt)
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "approved", "client_count": len(clients),
		"session_expires_at": expiresAt.UTC().Format(time.RFC3339),
	})
}

func (s *Server) handlePairingCandidates(w http.ResponseWriter, r *http.Request) {
	if !s.config.BrowserPairingEnabled {
		writeJSON(w, http.StatusOK, map[string]any{"clients": []protocol.ClientInfo{}, "pairing_enabled": false})
		return
	}
	clients, err := s.hub.pairingCandidates(r.RemoteAddr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid remote address")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"clients": clients, "pairing_enabled": true,
		"request_ttl_seconds": int(s.config.PairingRequestTTL.Seconds()),
	})
}

func (s *Server) handlePairingStart(w http.ResponseWriter, r *http.Request) {
	if !sameOriginRequest(r) {
		writeError(w, http.StatusForbidden, "same-origin request required")
		return
	}
	clientID := r.PathValue("client_id")
	if !clientIDPattern.MatchString(clientID) || len(r.Host) > 255 {
		writeError(w, http.StatusBadRequest, "invalid pairing request")
		return
	}
	client, err := s.hub.clientForPairing(clientID, r.RemoteAddr)
	if err != nil {
		writeError(w, http.StatusNotFound, "same-IP pairing client is not available")
		return
	}
	started, err := s.pairings.start(client, r.RemoteAddr, r.Host)
	if err != nil {
		switch {
		case errors.Is(err, ErrPairingDisabled):
			writeError(w, http.StatusNotFound, "browser pairing is disabled")
		case errors.Is(err, ErrPairingRateLimited):
			writeError(w, http.StatusTooManyRequests, "wait before requesting another pairing confirmation")
		case errors.Is(err, ErrPairingCapacity):
			writeError(w, http.StatusServiceUnavailable, "too many pending pairing requests")
		default:
			s.logger.Error("start browser pairing failed", "client_id", clientID, "error", err)
			writeError(w, http.StatusBadGateway, "could not contact the Windows Client")
		}
		return
	}
	writeJSON(w, http.StatusAccepted, started)
}

func (s *Server) handlePairingStatus(w http.ResponseWriter, r *http.Request) {
	if !sameOriginRequest(r) {
		writeError(w, http.StatusForbidden, "same-origin request required")
		return
	}
	pairingID := r.PathValue("pairing_id")
	pollToken, ok := pairingTokenFromRequest(r)
	if !pairingIDPattern.MatchString(pairingID) || !ok {
		writeError(w, http.StatusUnauthorized, "invalid pairing status authorization")
		return
	}
	result, err := s.pairings.poll(pairingID, pollToken, r.RemoteAddr)
	if err != nil {
		switch {
		case errors.Is(err, ErrPairingExpired):
			writeError(w, http.StatusGone, "pairing request expired")
		case errors.Is(err, ErrPairingUnauthorized):
			writeError(w, http.StatusUnauthorized, "invalid pairing status authorization")
		case errors.Is(err, ErrPairingNotFound):
			writeError(w, http.StatusNotFound, "pairing request not found")
		default:
			s.logger.Error("poll browser pairing failed", "pairing_id", pairingID, "error", err)
			writeError(w, http.StatusInternalServerError, "pairing status failed")
		}
		return
	}
	if result.Status == "pending" {
		writeJSON(w, http.StatusAccepted, map[string]string{"status": "pending"})
		return
	}
	if result.Status == "denied" {
		writeJSON(w, http.StatusOK, map[string]string{"status": "denied", "reason": result.Error})
		return
	}
	setDeviceSessionCookie(w, r, result.SessionToken, result.SessionExpiresAt)
	writeJSON(w, http.StatusOK, map[string]string{
		"status": "approved", "session_expires_at": result.SessionExpiresAt.UTC().Format(time.RFC3339),
	})
}

func (s *Server) handleDeviceLogout(w http.ResponseWriter, r *http.Request) {
	if !sameOriginRequest(r) {
		writeError(w, http.StatusForbidden, "same-origin request required")
		return
	}
	_ = s.pairings.revoke(r)
	clearDeviceSessionCookie(w, r)
	w.WriteHeader(http.StatusNoContent)
}

func pairingTokenFromRequest(r *http.Request) (string, bool) {
	const prefix = "Pairing "
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, prefix) {
		return "", false
	}
	token := strings.TrimPrefix(header, prefix)
	return token, len(token) >= 32 && len(token) <= 128
}

func deviceTokenFromRequest(r *http.Request) (string, bool) {
	const prefix = "Device "
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, prefix) {
		return "", false
	}
	token := strings.TrimPrefix(header, prefix)
	return token, len(token) >= 32 && len(token) <= 256
}

type commandRequest struct {
	Command        string `json:"command"`
	TimeoutSeconds int    `json:"timeout_seconds"`
}

func (s *Server) handleCommand(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/clients/")
	if !strings.HasSuffix(path, "/commands") {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	rawID := strings.TrimSuffix(path, "/commands")
	clientID, err := url.PathUnescape(strings.TrimSuffix(rawID, "/"))
	if err != nil || !clientIDPattern.MatchString(clientID) {
		writeError(w, http.StatusBadRequest, "invalid client ID")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var request commandRequest
	if err := decoder.Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON request")
		return
	}
	if strings.TrimSpace(request.Command) == "" {
		writeError(w, http.StatusBadRequest, "command is required")
		return
	}
	if len(request.Command) > 32<<10 {
		writeError(w, http.StatusBadRequest, "command is too long")
		return
	}

	timeout := s.config.DefaultTimeout
	if request.TimeoutSeconds != 0 {
		timeout = time.Duration(request.TimeoutSeconds) * time.Second
	}
	if timeout <= 0 || timeout > s.config.MaxCommandTimeout {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("timeout must be between 1 and %d seconds", int(s.config.MaxCommandTimeout.Seconds())))
		return
	}

	// Add transport leeway after the requested process deadline.
	ctx, cancel := context.WithTimeout(r.Context(), timeout+15*time.Second)
	defer cancel()
	result, err := s.hub.SendCommand(ctx, clientID, request.Command, timeout)
	if err != nil {
		switch {
		case errors.Is(err, ErrClientOffline):
			writeError(w, http.StatusNotFound, "client is not online")
		case errors.Is(err, ErrConnectionEnded):
			writeError(w, http.StatusConflict, "client connection ended")
		case errors.Is(err, context.DeadlineExceeded):
			writeError(w, http.StatusGatewayTimeout, "timed out waiting for client result")
		default:
			s.logger.Error("command dispatch failed", "client_id", clientID, "error", err)
			writeError(w, http.StatusBadGateway, "command dispatch failed")
		}
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) requireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !bearerMatches(r, s.config.AdminToken) {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func validateHello(message protocol.Message) error {
	if message.Type != protocol.TypeHello {
		return errors.New("first message must be hello")
	}
	if message.ProtocolVersion != protocol.Version {
		return errors.New("unsupported protocol version")
	}
	if !clientIDPattern.MatchString(message.ClientID) {
		return errors.New("invalid client ID")
	}
	if message.DeviceTokenHash != "" && !deviceTokenHashPattern.MatchString(message.DeviceTokenHash) {
		return errors.New("invalid device token hash")
	}
	if len(message.Hostname) > 255 || len(message.OS) > 32 || len(message.Arch) > 32 || len(message.ClientVersion) > 32 {
		return errors.New("hello field is too long")
	}
	return nil
}

func bearerMatches(r *http.Request, expected string) bool {
	token, ok := bearerTokenFromRequest(r)
	if !ok {
		return false
	}
	return secureEqual(token, expected)
}

func bearerTokenFromRequest(r *http.Request) (string, bool) {
	const prefix = "Bearer "
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, prefix) {
		return "", false
	}
	token := strings.TrimPrefix(header, prefix)
	return token, token != "" && len(token) <= 256
}

func secureEqual(actual, expected string) bool {
	return len(actual) == len(expected) && subtle.ConstantTimeCompare([]byte(actual), []byte(expected)) == 1
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; connect-src 'self' http://127.0.0.1:17891; img-src 'self' data:; style-src 'self'; script-src 'self'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'")
		w.Header().Set("Permissions-Policy", "local-network=(self), loopback-network=(self), local-network-access=(self)")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
