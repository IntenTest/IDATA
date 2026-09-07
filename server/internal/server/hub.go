package server

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"idata-server/internal/protocol"
)

var (
	ErrClientOffline         = errors.New("client is not online")
	ErrConnectionEnded       = errors.New("client connection ended")
	ErrDeviceTokenNotFound   = errors.New("device token does not match an online client")
	ErrDeviceTokenAmbiguous  = errors.New("device token matches multiple online clients")
	ErrPairingClientNotFound = errors.New("same-IP pairing client is not available")
)

type Hub struct {
	mu      sync.RWMutex
	clients map[string]*clientConn
}

func NewHub() *Hub {
	return &Hub{clients: make(map[string]*clientConn)}
}

func (h *Hub) register(client *clientConn) {
	h.mu.Lock()
	previous := h.clients[client.info.ID]
	h.clients[client.info.ID] = client
	h.mu.Unlock()

	if previous != nil && previous != client {
		previous.close(websocket.ClosePolicyViolation, "replaced by a newer connection")
	}
}

func (h *Hub) unregister(client *clientConn) {
	h.mu.Lock()
	if h.clients[client.info.ID] == client {
		delete(h.clients, client.info.ID)
	}
	h.mu.Unlock()
}

func (h *Hub) List() []protocol.ClientInfo {
	h.mu.RLock()
	clients := make([]protocol.ClientInfo, 0, len(h.clients))
	for _, client := range h.clients {
		clients = append(clients, client.info)
	}
	h.mu.RUnlock()
	sort.Slice(clients, func(i, j int) bool { return clients[i].ID < clients[j].ID })
	return clients
}

func (h *Hub) get(clientID string) *clientConn {
	h.mu.RLock()
	client := h.clients[clientID]
	h.mu.RUnlock()
	return client
}

func (h *Hub) clientForDeviceToken(deviceToken string) (*clientConn, error) {
	if len(deviceToken) < 32 || len(deviceToken) > 256 {
		return nil, ErrDeviceTokenNotFound
	}
	tokenHash := fmt.Sprintf("%x", sha256.Sum256([]byte(deviceToken)))
	h.mu.RLock()
	defer h.mu.RUnlock()
	var match *clientConn
	for _, client := range h.clients {
		if !strings.EqualFold(client.info.OS, "windows") || client.deviceTokenHash == "" || !secureEqual(client.deviceTokenHash, tokenHash) {
			continue
		}
		if match != nil {
			return nil, ErrDeviceTokenAmbiguous
		}
		match = client
	}
	if match == nil {
		return nil, ErrDeviceTokenNotFound
	}
	return match, nil
}

func (h *Hub) listForDeviceToken(deviceToken string) ([]protocol.ClientInfo, string, error) {
	self, err := h.clientForDeviceToken(deviceToken)
	if err != nil {
		return nil, "", err
	}
	return h.sanitizedList(), self.info.ID, nil
}

func (h *Hub) pairingCandidates(remoteAddress string) ([]protocol.ClientInfo, error) {
	remoteIP, err := addressIP(remoteAddress)
	if err != nil {
		return nil, err
	}
	h.mu.RLock()
	candidates := make([]protocol.ClientInfo, 0)
	for _, client := range h.clients {
		if !strings.EqualFold(client.info.OS, "windows") || client.deviceTokenHash == "" || !hasCapability(client.info.Capabilities, "browser_pairing_v1") {
			continue
		}
		clientIP, err := addressIP(client.info.RemoteAddress)
		if err != nil || clientIP != remoteIP {
			continue
		}
		info := client.info
		info.RemoteAddress = ""
		candidates = append(candidates, info)
	}
	h.mu.RUnlock()
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].ID < candidates[j].ID })
	return candidates, nil
}

func (h *Hub) clientsForIP(remoteAddress string) ([]protocol.ClientInfo, error) {
	remoteIP, err := addressIP(remoteAddress)
	if err != nil {
		return nil, err
	}
	h.mu.RLock()
	clients := make([]protocol.ClientInfo, 0)
	for _, client := range h.clients {
		clientIP, err := addressIP(client.info.RemoteAddress)
		if err != nil || clientIP != remoteIP {
			continue
		}
		info := client.info
		info.RemoteAddress = ""
		clients = append(clients, info)
	}
	h.mu.RUnlock()
	sort.Slice(clients, func(i, j int) bool { return clients[i].ID < clients[j].ID })
	return clients, nil
}

func (h *Hub) clientForIP(clientID, remoteAddress string) (*clientConn, error) {
	remoteIP, err := addressIP(remoteAddress)
	if err != nil {
		return nil, ErrClientOffline
	}
	client := h.get(clientID)
	if client == nil {
		return nil, ErrClientOffline
	}
	clientIP, err := addressIP(client.info.RemoteAddress)
	if err != nil || clientIP != remoteIP {
		return nil, ErrClientOffline
	}
	return client, nil
}

func (h *Hub) clientForPairing(clientID, remoteAddress string) (*clientConn, error) {
	remoteIP, err := addressIP(remoteAddress)
	if err != nil {
		return nil, ErrPairingClientNotFound
	}
	client := h.get(clientID)
	if client == nil || !strings.EqualFold(client.info.OS, "windows") || client.deviceTokenHash == "" || !hasCapability(client.info.Capabilities, "browser_pairing_v1") {
		return nil, ErrPairingClientNotFound
	}
	clientIP, err := addressIP(client.info.RemoteAddress)
	if err != nil || clientIP != remoteIP {
		return nil, ErrPairingClientNotFound
	}
	return client, nil
}

func (h *Hub) sanitizedList() []protocol.ClientInfo {
	h.mu.RLock()
	clients := make([]protocol.ClientInfo, 0, len(h.clients))
	for _, client := range h.clients {
		info := client.info
		info.RemoteAddress = ""
		clients = append(clients, info)
	}
	h.mu.RUnlock()
	sort.Slice(clients, func(i, j int) bool { return clients[i].ID < clients[j].ID })
	return clients
}

func addressIP(remoteAddress string) (netip.Addr, error) {
	host, _, err := net.SplitHostPort(remoteAddress)
	if err != nil {
		return netip.Addr{}, err
	}
	address, err := netip.ParseAddr(host)
	if err != nil {
		return netip.Addr{}, err
	}
	return address.Unmap(), nil
}

func (h *Hub) SendCommand(ctx context.Context, clientID, command string, timeout time.Duration) (protocol.Message, error) {
	client := h.get(clientID)
	if client == nil {
		return protocol.Message{}, ErrClientOffline
	}
	return client.sendCommand(ctx, command, timeout)
}

type commandResponse struct {
	message protocol.Message
	err     error
}

type clientConn struct {
	conn            *websocket.Conn
	info            protocol.ClientInfo
	deviceTokenHash string
	credentialID    string
	writeMu         sync.Mutex

	pendingMu  sync.Mutex
	pending    map[string]chan commandResponse
	terminalMu sync.Mutex
	terminals  map[string]*terminalBridge
	done       chan struct{}
	closeOnce  sync.Once
}

func newClientConn(conn *websocket.Conn, info protocol.ClientInfo, deviceTokenHash string) *clientConn {
	return &clientConn{
		conn:            conn,
		info:            info,
		deviceTokenHash: deviceTokenHash,
		pending:         make(map[string]chan commandResponse),
		terminals:       make(map[string]*terminalBridge),
		done:            make(chan struct{}),
	}
}

type terminalBridge struct {
	sessionID string
	messages  chan protocol.Message
	done      chan struct{}
	closeOnce sync.Once
}

func (c *clientConn) openTerminal() (*terminalBridge, error) {
	sessionID, err := newRequestID()
	if err != nil {
		return nil, fmt.Errorf("generate terminal session ID: %w", err)
	}
	bridge := &terminalBridge{
		sessionID: sessionID,
		messages:  make(chan protocol.Message, 32),
		done:      make(chan struct{}),
	}
	c.terminalMu.Lock()
	select {
	case <-c.done:
		c.terminalMu.Unlock()
		return nil, ErrConnectionEnded
	default:
	}
	if len(c.terminals) >= 4 {
		c.terminalMu.Unlock()
		return nil, errors.New("terminal session limit reached")
	}
	c.terminals[sessionID] = bridge
	c.terminalMu.Unlock()

	message := protocol.Message{
		Type: protocol.TypeTerminalOpen, ProtocolVersion: protocol.Version, SessionID: sessionID,
	}
	if err := c.writeJSON(message); err != nil {
		c.removeTerminal(bridge)
		return nil, fmt.Errorf("open terminal: %w", err)
	}
	return bridge, nil
}

func (c *clientConn) writeTerminal(message protocol.Message) error {
	message.ProtocolVersion = protocol.Version
	return c.writeJSON(message)
}

func (c *clientConn) deliverTerminal(message protocol.Message) {
	c.terminalMu.Lock()
	bridge := c.terminals[message.SessionID]
	c.terminalMu.Unlock()
	if bridge == nil {
		return
	}
	select {
	case bridge.messages <- message:
	case <-bridge.done:
	default:
		bridge.close()
	}
}

func (c *clientConn) removeTerminal(bridge *terminalBridge) {
	c.terminalMu.Lock()
	if c.terminals[bridge.sessionID] == bridge {
		delete(c.terminals, bridge.sessionID)
	}
	c.terminalMu.Unlock()
	bridge.close()
}

func (b *terminalBridge) close() {
	b.closeOnce.Do(func() { close(b.done) })
}

func (c *clientConn) sendCommand(ctx context.Context, command string, timeout time.Duration) (protocol.Message, error) {
	return c.sendRequest(ctx, protocol.Message{Type: protocol.TypeCommand, Command: command, TimeoutSeconds: int(timeout.Seconds())})
}

func (c *clientConn) sendRequest(ctx context.Context, message protocol.Message) (protocol.Message, error) {
	requestID, err := newRequestID()
	if err != nil {
		return protocol.Message{}, err
	}
	response := make(chan commandResponse, 1)
	c.pendingMu.Lock()
	select {
	case <-c.done:
		c.pendingMu.Unlock()
		return protocol.Message{}, ErrConnectionEnded
	default:
	}
	c.pending[requestID] = response
	c.pendingMu.Unlock()

	defer func() {
		c.pendingMu.Lock()
		delete(c.pending, requestID)
		c.pendingMu.Unlock()
	}()

	message.ProtocolVersion = protocol.Version
	message.RequestID = requestID
	if err := c.writeJSON(message); err != nil {
		return protocol.Message{}, fmt.Errorf("send command: %w", err)
	}

	select {
	case result := <-response:
		return result.message, result.err
	case <-ctx.Done():
		return protocol.Message{}, ctx.Err()
	case <-c.done:
		return protocol.Message{}, ErrConnectionEnded
	}
}

func (c *clientConn) deliver(message protocol.Message) {
	c.pendingMu.Lock()
	response := c.pending[message.RequestID]
	c.pendingMu.Unlock()
	if response == nil {
		return
	}
	select {
	case response <- commandResponse{message: message}:
	default:
	}
}

func (c *clientConn) writeJSON(value any) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	_ = c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	return c.conn.WriteJSON(value)
}

func (c *clientConn) writePing() error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	return c.conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(10*time.Second))
}

func (c *clientConn) close(code int, reason string) {
	c.closeOnce.Do(func() {
		close(c.done)
		c.writeMu.Lock()
		_ = c.conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(code, reason), time.Now().Add(time.Second))
		_ = c.conn.Close()
		c.writeMu.Unlock()

		c.pendingMu.Lock()
		for _, response := range c.pending {
			select {
			case response <- commandResponse{err: ErrConnectionEnded}:
			default:
			}
		}
		c.pendingMu.Unlock()

		c.terminalMu.Lock()
		for _, bridge := range c.terminals {
			bridge.close()
		}
		c.terminalMu.Unlock()
	})
}

func newRequestID() (string, error) {
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return hex.EncodeToString(buffer), nil
}
