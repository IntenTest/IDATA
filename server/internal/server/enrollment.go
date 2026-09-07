package server

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

const (
	enrollmentRequestTTL  = 10 * time.Minute
	maxPendingEnrollments = 100
)

var (
	errEnrollmentNotFound     = errors.New("enrollment request not found")
	errEnrollmentUnauthorized = errors.New("enrollment request authorization failed")
	errEnrollmentExpired      = errors.New("enrollment request expired")
	errEnrollmentCapacity     = errors.New("too many pending enrollment requests")
	errEnrollmentRateLimited  = errors.New("too many enrollment requests from this address")
)

type enrollmentIdentity struct {
	ClientID        string `json:"client_id"`
	Hostname        string `json:"hostname"`
	Username        string `json:"username"`
	LocalIP         string `json:"local_ip"`
	MACAddress      string `json:"mac_address"`
	OS              string `json:"os"`
	Arch            string `json:"arch"`
	ClientVersion   string `json:"client_version"`
	DeviceTokenHash string `json:"device_token_hash"`
}

type enrollmentRequest struct {
	ID           string             `json:"id"`
	Identity     enrollmentIdentity `json:"identity"`
	RemoteIP     string             `json:"remote_ip"`
	RequestedAt  time.Time          `json:"requested_at"`
	ExpiresAt    time.Time          `json:"expires_at"`
	Status       string             `json:"status"`
	pollToken    string
	agentToken   string
	denialReason string
}

type deviceCredential struct {
	ID              string    `json:"id"`
	ClientID        string    `json:"client_id"`
	Hostname        string    `json:"hostname"`
	Username        string    `json:"username"`
	LocalIP         string    `json:"local_ip"`
	MACAddress      string    `json:"mac_address"`
	DeviceTokenHash string    `json:"device_token_hash"`
	TokenHash       string    `json:"token_hash,omitempty"`
	ApprovedAt      time.Time `json:"approved_at"`
}

type enrollmentManager struct {
	mu          sync.Mutex
	storePath   string
	pending     map[string]*enrollmentRequest
	credentials map[string]deviceCredential
	logger      logger
}

type logger interface {
	Info(string, ...any)
	Warn(string, ...any)
	Error(string, ...any)
}

func newEnrollmentManager(storePath string, log logger) (*enrollmentManager, error) {
	m := &enrollmentManager{
		storePath: storePath, pending: make(map[string]*enrollmentRequest),
		credentials: make(map[string]deviceCredential), logger: log,
	}
	if storePath == "" {
		return m, nil
	}
	contents, err := os.ReadFile(storePath)
	if errors.Is(err, os.ErrNotExist) {
		return m, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read device credential store: %w", err)
	}
	var stored []deviceCredential
	if err := json.Unmarshal(contents, &stored); err != nil {
		return nil, fmt.Errorf("parse device credential store: %w", err)
	}
	for _, credential := range stored {
		if credential.ID == "" || !clientIDPattern.MatchString(credential.ClientID) || !deviceTokenHashPattern.MatchString(credential.DeviceTokenHash) || !deviceTokenHashPattern.MatchString(credential.TokenHash) {
			return nil, errors.New("device credential store contains an invalid record")
		}
		m.credentials[credential.ID] = credential
	}
	return m, nil
}

func (m *enrollmentManager) start(identity enrollmentIdentity, remoteIP string) (string, string, time.Time, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cleanupLocked(time.Now())
	requestsFromIP := 0
	for id, request := range m.pending {
		if request.RemoteIP != remoteIP {
			continue
		}
		if request.Identity.ClientID == identity.ClientID && request.Identity.DeviceTokenHash == identity.DeviceTokenHash {
			delete(m.pending, id)
			continue
		}
		if request.Status == "pending" {
			requestsFromIP++
		}
	}
	if requestsFromIP >= 5 {
		return "", "", time.Time{}, errEnrollmentRateLimited
	}
	if len(m.pending) >= maxPendingEnrollments {
		return "", "", time.Time{}, errEnrollmentCapacity
	}
	id, err := randomHex(16)
	if err != nil {
		return "", "", time.Time{}, err
	}
	pollToken, err := randomHex(32)
	if err != nil {
		return "", "", time.Time{}, err
	}
	now := time.Now().UTC()
	request := &enrollmentRequest{
		ID: id, Identity: identity, RemoteIP: remoteIP, RequestedAt: now,
		ExpiresAt: now.Add(enrollmentRequestTTL), Status: "pending", pollToken: pollToken,
	}
	m.pending[id] = request
	m.logger.Info("device enrollment requested", "enrollment_id", id, "client_id", identity.ClientID, "remote_ip", remoteIP)
	return id, pollToken, request.ExpiresAt, nil
}

func (m *enrollmentManager) list() ([]enrollmentRequest, []deviceCredential) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cleanupLocked(time.Now())
	requests := make([]enrollmentRequest, 0, len(m.pending))
	for _, request := range m.pending {
		copy := *request
		copy.pollToken = ""
		copy.agentToken = ""
		requests = append(requests, copy)
	}
	credentials := make([]deviceCredential, 0, len(m.credentials))
	for _, credential := range m.credentials {
		credential.TokenHash = ""
		credentials = append(credentials, credential)
	}
	sort.Slice(requests, func(i, j int) bool { return requests[i].RequestedAt.After(requests[j].RequestedAt) })
	sort.Slice(credentials, func(i, j int) bool { return credentials[i].ApprovedAt.After(credentials[j].ApprovedAt) })
	return requests, credentials
}

func (m *enrollmentManager) approve(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cleanupLocked(time.Now())
	request := m.pending[id]
	if request == nil || request.Status != "pending" {
		return errEnrollmentNotFound
	}
	token, err := randomHex(32)
	if err != nil {
		return err
	}
	credentialID, err := randomHex(16)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	credential := deviceCredential{
		ID: credentialID, ClientID: request.Identity.ClientID, Hostname: request.Identity.Hostname,
		Username: request.Identity.Username, LocalIP: request.Identity.LocalIP, MACAddress: request.Identity.MACAddress,
		DeviceTokenHash: request.Identity.DeviceTokenHash, TokenHash: hashToken(token), ApprovedAt: now,
	}
	for existingID, existing := range m.credentials {
		if existing.ClientID == credential.ClientID || existing.DeviceTokenHash == credential.DeviceTokenHash {
			delete(m.credentials, existingID)
		}
	}
	m.credentials[credentialID] = credential
	if err := m.saveLocked(); err != nil {
		delete(m.credentials, credentialID)
		return err
	}
	request.Status = "approved"
	request.agentToken = token
	m.logger.Info("device enrollment approved", "enrollment_id", id, "credential_id", credentialID, "client_id", credential.ClientID)
	return nil
}

func (m *enrollmentManager) deny(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cleanupLocked(time.Now())
	request := m.pending[id]
	if request == nil || request.Status != "pending" {
		return errEnrollmentNotFound
	}
	request.Status = "denied"
	request.denialReason = "denied_by_admin"
	m.logger.Info("device enrollment denied", "enrollment_id", id, "client_id", request.Identity.ClientID)
	return nil
}

func (m *enrollmentManager) poll(id, pollToken string) (string, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cleanupLocked(time.Now())
	request := m.pending[id]
	if request == nil {
		return "", "", errEnrollmentNotFound
	}
	if !secureEqual(request.pollToken, pollToken) {
		return "", "", errEnrollmentUnauthorized
	}
	if time.Now().After(request.ExpiresAt) {
		delete(m.pending, id)
		return "", "", errEnrollmentExpired
	}
	return request.Status, request.agentToken, nil
}

func (m *enrollmentManager) authenticate(token string) (deviceCredential, bool) {
	if len(token) < 32 || len(token) > 256 {
		return deviceCredential{}, false
	}
	tokenHash := hashToken(token)
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, credential := range m.credentials {
		if secureEqual(credential.TokenHash, tokenHash) {
			return credential, true
		}
	}
	return deviceCredential{}, false
}

func (m *enrollmentManager) revoke(id string) (deviceCredential, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.credentials[id]; !ok {
		return deviceCredential{}, errEnrollmentNotFound
	}
	credential := m.credentials[id]
	delete(m.credentials, id)
	if err := m.saveLocked(); err != nil {
		m.credentials[id] = credential
		return deviceCredential{}, err
	}
	m.logger.Info("device credential revoked", "credential_id", id, "client_id", credential.ClientID)
	return credential, nil
}

func (m *enrollmentManager) saveLocked() error {
	if m.storePath == "" {
		return nil
	}
	stored := make([]deviceCredential, 0, len(m.credentials))
	for _, credential := range m.credentials {
		stored = append(stored, credential)
	}
	contents, err := json.MarshalIndent(stored, "", "  ")
	if err != nil {
		return fmt.Errorf("encode device credential store: %w", err)
	}
	contents = append(contents, '\n')
	if err := os.MkdirAll(filepath.Dir(m.storePath), 0o700); err != nil {
		return fmt.Errorf("create device credential directory: %w", err)
	}
	temporary, err := os.CreateTemp(filepath.Dir(m.storePath), ".device-credentials-*")
	if err != nil {
		return fmt.Errorf("create temporary device credential store: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return err
	}
	if _, err := temporary.Write(contents); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, m.storePath); err != nil {
		return fmt.Errorf("replace device credential store: %w", err)
	}
	return nil
}

func (m *enrollmentManager) cleanupLocked(now time.Time) {
	for id, request := range m.pending {
		if now.After(request.ExpiresAt) {
			delete(m.pending, id)
		}
	}
}

func randomHex(bytes int) (string, error) {
	buffer := make([]byte, bytes)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return hex.EncodeToString(buffer), nil
}

func hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}
