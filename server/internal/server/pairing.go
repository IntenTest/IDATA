package server

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/netip"
	"strings"
	"sync"
	"time"

	"idata-server/internal/protocol"
)

const (
	deviceSessionCookie = "idata_device_session"
	pairingCooldown     = 15 * time.Second
	maxPendingPairings  = 128
)

var (
	ErrPairingDisabled     = errors.New("browser pairing is disabled")
	ErrPairingRateLimited  = errors.New("pairing request rate limited")
	ErrPairingCapacity     = errors.New("too many pending pairing requests")
	ErrPairingNotFound     = errors.New("pairing request not found")
	ErrPairingUnauthorized = errors.New("pairing request authorization failed")
	ErrPairingExpired      = errors.New("pairing request expired")
	ErrDeviceSession       = errors.New("device session is invalid")
)

type pairingManager struct {
	mu       sync.Mutex
	config   Config
	logger   *slog.Logger
	pending  map[string]*pendingPairing
	sessions map[string]deviceSession
	recent   map[string]time.Time
	now      func() time.Time
}

type pendingPairing struct {
	id              string
	client          *clientConn
	clientID        string
	deviceTokenHash string
	browserIP       netip.Addr
	pollTokenHash   string
	expiresAt       time.Time
	status          string
	error           string
}

type deviceSession struct {
	clientID        string
	deviceTokenHash string
	browserIP       netip.Addr
	expiresAt       time.Time
	ipScope         bool
}

type pairingStart struct {
	PairingID string `json:"pairing_id"`
	PollToken string `json:"poll_token"`
	Challenge string `json:"challenge"`
	ExpiresAt string `json:"expires_at"`
}

type pairingPoll struct {
	Status           string
	Error            string
	SessionToken     string
	SessionExpiresAt time.Time
}

func newPairingManager(config Config, logger *slog.Logger) *pairingManager {
	return &pairingManager{
		config: config, logger: logger, now: time.Now,
		pending: make(map[string]*pendingPairing), sessions: make(map[string]deviceSession),
		recent: make(map[string]time.Time),
	}
}

func (p *pairingManager) start(client *clientConn, remoteAddress, serverHost string) (pairingStart, error) {
	if !p.config.BrowserPairingEnabled {
		return pairingStart{}, ErrPairingDisabled
	}
	browserIP, err := addressIP(remoteAddress)
	if err != nil {
		return pairingStart{}, ErrPairingUnauthorized
	}
	pairingID, err := newRequestID()
	if err != nil {
		return pairingStart{}, fmt.Errorf("generate pairing ID: %w", err)
	}
	pollToken, err := randomBrowserToken()
	if err != nil {
		return pairingStart{}, fmt.Errorf("generate pairing poll token: %w", err)
	}
	challenge, err := pairingChallenge()
	if err != nil {
		return pairingStart{}, fmt.Errorf("generate pairing challenge: %w", err)
	}
	now := p.now()
	expiresAt := now.Add(p.config.PairingRequestTTL)
	key := browserIP.String() + "\x00" + client.info.ID

	p.mu.Lock()
	p.cleanupLocked(now)
	if len(p.pending) >= maxPendingPairings {
		p.mu.Unlock()
		return pairingStart{}, ErrPairingCapacity
	}
	if last := p.recent[key]; !last.IsZero() && now.Sub(last) < pairingCooldown {
		p.mu.Unlock()
		return pairingStart{}, ErrPairingRateLimited
	}
	for _, existing := range p.pending {
		if existing.clientID == client.info.ID && existing.browserIP == browserIP && existing.status == "pending" {
			p.mu.Unlock()
			return pairingStart{}, ErrPairingRateLimited
		}
	}
	p.pending[pairingID] = &pendingPairing{
		id: pairingID, client: client, clientID: client.info.ID,
		deviceTokenHash: client.deviceTokenHash, browserIP: browserIP,
		pollTokenHash: tokenHash(pollToken), expiresAt: expiresAt, status: "pending",
	}
	p.recent[key] = now
	p.mu.Unlock()

	message := protocol.Message{
		Type: protocol.TypePairingRequest, ProtocolVersion: protocol.Version,
		PairingID: pairingID, Challenge: challenge, BrowserIP: browserIP.String(),
		ServerHost: serverHost, ExpiresAt: expiresAt.UTC().Format(time.RFC3339),
		SessionTTL: int(p.config.DeviceSessionTTL.Seconds()),
	}
	if err := client.writeJSON(message); err != nil {
		p.mu.Lock()
		delete(p.pending, pairingID)
		p.mu.Unlock()
		return pairingStart{}, fmt.Errorf("send pairing request: %w", err)
	}
	p.logger.Info("browser pairing requested", "pairing_id", pairingID, "client_id", client.info.ID, "remote_ip", browserIP.String())
	return pairingStart{
		PairingID: pairingID, PollToken: pollToken, Challenge: challenge,
		ExpiresAt: expiresAt.UTC().Format(time.RFC3339),
	}, nil
}

func (p *pairingManager) resolve(client *clientConn, message protocol.Message) {
	now := p.now()
	p.mu.Lock()
	request := p.pending[message.PairingID]
	if request == nil || request.client != client || request.status != "pending" {
		p.mu.Unlock()
		return
	}
	if !request.expiresAt.After(now) {
		delete(p.pending, message.PairingID)
		p.mu.Unlock()
		return
	}
	if message.Approved {
		request.status = "approved"
		request.error = ""
	} else {
		request.status = "denied"
		request.error = safePairingError(message.Error)
	}
	status := request.status
	errorText := request.error
	p.mu.Unlock()
	p.logger.Info("browser pairing resolved", "pairing_id", message.PairingID, "client_id", client.info.ID, "status", status, "reason", errorText)
}

func (p *pairingManager) poll(pairingID, pollToken, remoteAddress string) (pairingPoll, error) {
	browserIP, err := addressIP(remoteAddress)
	if err != nil {
		return pairingPoll{}, ErrPairingUnauthorized
	}
	now := p.now()
	p.mu.Lock()
	request := p.pending[pairingID]
	if request == nil {
		p.mu.Unlock()
		return pairingPoll{}, ErrPairingNotFound
	}
	if !request.expiresAt.After(now) {
		delete(p.pending, pairingID)
		p.mu.Unlock()
		return pairingPoll{}, ErrPairingExpired
	}
	if request.browserIP != browserIP || !secureEqual(request.pollTokenHash, tokenHash(pollToken)) {
		p.mu.Unlock()
		return pairingPoll{}, ErrPairingUnauthorized
	}
	switch request.status {
	case "pending":
		p.mu.Unlock()
		return pairingPoll{Status: "pending"}, nil
	case "denied":
		result := pairingPoll{Status: "denied", Error: request.error}
		delete(p.pending, pairingID)
		p.mu.Unlock()
		return result, nil
	case "approved":
		sessionToken, err := randomBrowserToken()
		if err != nil {
			p.mu.Unlock()
			return pairingPoll{}, fmt.Errorf("generate device session: %w", err)
		}
		sessionExpiresAt := now.Add(p.config.DeviceSessionTTL)
		p.sessions[tokenHash(sessionToken)] = deviceSession{
			clientID: request.clientID, deviceTokenHash: request.deviceTokenHash,
			browserIP: request.browserIP, expiresAt: sessionExpiresAt,
		}
		delete(p.pending, pairingID)
		p.mu.Unlock()
		p.logger.Info("browser device session created", "client_id", request.clientID, "remote_ip", browserIP.String(), "expires_at", sessionExpiresAt.UTC().Format(time.RFC3339))
		return pairingPoll{Status: "approved", SessionToken: sessionToken, SessionExpiresAt: sessionExpiresAt}, nil
	default:
		delete(p.pending, pairingID)
		p.mu.Unlock()
		return pairingPoll{}, ErrPairingNotFound
	}
}

func (p *pairingManager) clientForRequest(r *http.Request, hub *Hub) (*clientConn, time.Time, error) {
	session, err := p.sessionForRequest(r)
	if err != nil || session.ipScope {
		return nil, time.Time{}, ErrDeviceSession
	}
	client := hub.get(session.clientID)
	if client == nil || !strings.EqualFold(client.info.OS, "windows") || client.deviceTokenHash == "" || !secureEqual(client.deviceTokenHash, session.deviceTokenHash) {
		return nil, time.Time{}, ErrDeviceSession
	}
	return client, session.expiresAt, nil
}

func (p *pairingManager) createIPSession(remoteAddress string) (string, time.Time, error) {
	browserIP, err := addressIP(remoteAddress)
	if err != nil {
		return "", time.Time{}, ErrDeviceSession
	}
	token, err := randomBrowserToken()
	if err != nil {
		return "", time.Time{}, fmt.Errorf("generate IP browser session: %w", err)
	}
	expiresAt := p.now().Add(p.config.DeviceSessionTTL)
	p.mu.Lock()
	p.cleanupLocked(p.now())
	p.sessions[tokenHash(token)] = deviceSession{browserIP: browserIP, expiresAt: expiresAt, ipScope: true}
	p.mu.Unlock()
	p.logger.Info("IP browser session created", "remote_ip", browserIP.String(), "expires_at", expiresAt.UTC().Format(time.RFC3339))
	return token, expiresAt, nil
}

func (p *pairingManager) clientsForIPSession(r *http.Request, hub *Hub) ([]protocol.ClientInfo, time.Time, error) {
	session, err := p.sessionForRequest(r)
	if err != nil || !session.ipScope {
		return nil, time.Time{}, ErrDeviceSession
	}
	clients, err := hub.clientsForIP(r.RemoteAddr)
	if err != nil {
		return nil, time.Time{}, ErrDeviceSession
	}
	return clients, session.expiresAt, nil
}

func (p *pairingManager) clientForIPSession(r *http.Request, hub *Hub, clientID string) (*clientConn, error) {
	session, err := p.sessionForRequest(r)
	if err != nil || !session.ipScope {
		return nil, ErrDeviceSession
	}
	client, err := hub.clientForIP(clientID, r.RemoteAddr)
	if err != nil {
		return nil, ErrDeviceSession
	}
	return client, nil
}

func (p *pairingManager) sessionForRequest(r *http.Request) (deviceSession, error) {
	cookie, err := r.Cookie(deviceSessionCookie)
	if err != nil || cookie.Value == "" {
		return deviceSession{}, ErrDeviceSession
	}
	browserIP, err := addressIP(r.RemoteAddr)
	if err != nil {
		return deviceSession{}, ErrDeviceSession
	}
	now := p.now()
	p.mu.Lock()
	p.cleanupLocked(now)
	session, ok := p.sessions[tokenHash(cookie.Value)]
	p.mu.Unlock()
	if !ok || session.browserIP != browserIP || !session.expiresAt.After(now) {
		return deviceSession{}, ErrDeviceSession
	}
	return session, nil
}

func (p *pairingManager) revoke(r *http.Request) bool {
	cookie, err := r.Cookie(deviceSessionCookie)
	if err != nil || cookie.Value == "" {
		return false
	}
	browserIP, err := addressIP(r.RemoteAddr)
	if err != nil {
		return false
	}
	hash := tokenHash(cookie.Value)
	p.mu.Lock()
	session, ok := p.sessions[hash]
	if ok && session.browserIP == browserIP {
		delete(p.sessions, hash)
	}
	p.mu.Unlock()
	return ok && session.browserIP == browserIP
}

func (p *pairingManager) disconnect(client *clientConn) {
	p.mu.Lock()
	for _, request := range p.pending {
		if request.client == client && request.status == "pending" {
			request.status = "denied"
			request.error = "client_disconnected"
		}
	}
	p.mu.Unlock()
}

func (p *pairingManager) cleanupLocked(now time.Time) {
	for id, request := range p.pending {
		if !request.expiresAt.After(now) {
			delete(p.pending, id)
		}
	}
	for hash, session := range p.sessions {
		if !session.expiresAt.After(now) {
			delete(p.sessions, hash)
		}
	}
	for key, last := range p.recent {
		if now.Sub(last) >= pairingCooldown {
			delete(p.recent, key)
		}
	}
}

func setDeviceSessionCookie(w http.ResponseWriter, r *http.Request, token string, expiresAt time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name: deviceSessionCookie, Value: token, Path: "/", HttpOnly: true,
		Secure: r.TLS != nil, SameSite: http.SameSiteStrictMode,
		Expires: expiresAt.UTC(), MaxAge: max(1, int(time.Until(expiresAt).Seconds())),
	})
}

func clearDeviceSessionCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name: deviceSessionCookie, Value: "", Path: "/", HttpOnly: true,
		Secure: r.TLS != nil, SameSite: http.SameSiteStrictMode,
		Expires: time.Unix(1, 0).UTC(), MaxAge: -1,
	})
}

func randomBrowserToken() (string, error) {
	buffer := make([]byte, 32)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}

func pairingChallenge() (string, error) {
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	buffer := make([]byte, 8)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	for index := range buffer {
		buffer[index] = alphabet[int(buffer[index])&31]
	}
	return "PAIR IDATA " + string(buffer[:4]) + "-" + string(buffer[4:]), nil
}

func tokenHash(token string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(token)))
}

func safePairingError(value string) string {
	switch value {
	case "denied", "busy", "expired", "prompt_failed", "unsupported_or_invalid", "client_disconnected":
		return value
	default:
		return "denied"
	}
}
