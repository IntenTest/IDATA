package server

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"idata-server/internal/protocol"
)

func TestDeviceEnrollmentApprovalPersistenceAndRevocation(t *testing.T) {
	storePath := filepath.Join(t.TempDir(), "credentials.json")
	app, err := New(Config{
		AdminToken: "admin-test-token", DeviceCredentialsFile: storePath,
		DefaultTimeout: time.Second, MaxCommandTimeout: 2 * time.Second,
	}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	identity := enrollmentIdentity{
		ClientID: "office-pc", Hostname: "OFFICE-PC", Username: `CORP\alice`,
		LocalIP: "10.0.0.8", MACAddress: "00:11:22:33:44:55", OS: "windows", Arch: "amd64",
		ClientVersion: "0.6.0", DeviceTokenHash: hashDeviceToken(testDeviceToken),
	}
	body, _ := json.Marshal(identity)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/enrollments", bytes.NewReader(body))
	request.RemoteAddr = "203.0.113.10:50000"
	response := httptest.NewRecorder()
	app.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusAccepted {
		t.Fatalf("start status = %d, body = %s", response.Code, response.Body.String())
	}
	var started struct {
		EnrollmentID string `json:"enrollment_id"`
		PollToken    string `json:"poll_token"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &started); err != nil || started.EnrollmentID == "" || started.PollToken == "" {
		t.Fatalf("invalid start response: %s", response.Body.String())
	}

	listRequest := httptest.NewRequest(http.MethodGet, "/api/v1/enrollments", nil)
	listRequest.Header.Set("Authorization", "Bearer admin-test-token")
	listResponse := httptest.NewRecorder()
	app.Handler().ServeHTTP(listResponse, listRequest)
	if listResponse.Code != http.StatusOK || strings.Contains(listResponse.Body.String(), started.PollToken) || strings.Contains(listResponse.Body.String(), `"token_hash"`) {
		t.Fatalf("admin list leaks a credential or failed: %s", listResponse.Body.String())
	}

	approveRequest := httptest.NewRequest(http.MethodPost, "/api/v1/enrollments/"+started.EnrollmentID+"/approve", nil)
	approveRequest.Header.Set("Authorization", "Bearer admin-test-token")
	approveResponse := httptest.NewRecorder()
	app.Handler().ServeHTTP(approveResponse, approveRequest)
	if approveResponse.Code != http.StatusOK {
		t.Fatalf("approve status = %d, body = %s", approveResponse.Code, approveResponse.Body.String())
	}

	pollRequest := httptest.NewRequest(http.MethodPost, "/api/v1/enrollments/"+started.EnrollmentID+"/status", nil)
	pollRequest.Header.Set("Authorization", "Enrollment "+started.PollToken)
	pollResponse := httptest.NewRecorder()
	app.Handler().ServeHTTP(pollResponse, pollRequest)
	var approved struct {
		Status     string `json:"status"`
		AgentToken string `json:"agent_token"`
	}
	_ = json.Unmarshal(pollResponse.Body.Bytes(), &approved)
	if pollResponse.Code != http.StatusOK || approved.Status != "approved" || len(approved.AgentToken) != 64 {
		t.Fatalf("invalid approval response: %s", pollResponse.Body.String())
	}
	credential, ok := app.enrollments.authenticate(approved.AgentToken)
	if !ok || credential.ClientID != identity.ClientID || credential.DeviceTokenHash != identity.DeviceTokenHash {
		t.Fatal("issued credential is not bound to the requested device")
	}

	reloaded, err := newEnrollmentManager(storePath, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := reloaded.authenticate(approved.AgentToken); !ok {
		t.Fatal("approved credential did not survive store reload")
	}

	httpServer := httptest.NewServer(app.Handler())
	defer httpServer.Close()
	header := http.Header{}
	header.Set("Authorization", "Bearer "+approved.AgentToken)
	wrongConnection, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(httpServer.URL, "http")+"/ws/agent", header)
	if err != nil {
		t.Fatal(err)
	}
	if err := wrongConnection.WriteJSON(protocol.Message{
		Type: protocol.TypeHello, ProtocolVersion: protocol.Version, ClientID: "wrong-pc",
		Hostname: identity.Hostname, OS: "windows", Arch: "amd64", DeviceTokenHash: identity.DeviceTokenHash,
	}); err != nil {
		t.Fatal(err)
	}
	_ = wrongConnection.SetReadDeadline(time.Now().Add(time.Second))
	if _, _, err := wrongConnection.ReadMessage(); err == nil {
		t.Fatal("device credential was not bound to its approved client ID")
	}
	wrongConnection.Close()

	connection, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(httpServer.URL, "http")+"/ws/agent", header)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	if err := connection.WriteJSON(protocol.Message{
		Type: protocol.TypeHello, ProtocolVersion: protocol.Version, ClientID: identity.ClientID,
		Hostname: identity.Hostname, OS: "windows", Arch: "amd64", DeviceTokenHash: identity.DeviceTokenHash,
	}); err != nil {
		t.Fatal(err)
	}
	waitForClient(t, app, identity.ClientID)
	revokeRequest := httptest.NewRequest(http.MethodDelete, "/api/v1/device-credentials/"+credential.ID, nil)
	revokeRequest.Header.Set("Authorization", "Bearer admin-test-token")
	revokeResponse := httptest.NewRecorder()
	app.Handler().ServeHTTP(revokeResponse, revokeRequest)
	if revokeResponse.Code != http.StatusNoContent {
		t.Fatalf("revoke status = %d, body = %s", revokeResponse.Code, revokeResponse.Body.String())
	}
	_ = connection.SetReadDeadline(time.Now().Add(time.Second))
	if _, _, err := connection.ReadMessage(); err == nil {
		t.Fatal("revoked online device connection remained open")
	}
	reloaded, err = newEnrollmentManager(storePath, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := reloaded.authenticate(approved.AgentToken); ok {
		t.Fatal("revoked credential still authenticates")
	}
}

func TestEnrollmentAutoApprovalDoesNotRequireHumanApproval(t *testing.T) {
	app, err := New(Config{
		AdminToken: "admin-test-token", DeviceCredentialsFile: filepath.Join(t.TempDir(), "credentials.json"),
		EnrollmentAutoApprove: true,
		DefaultTimeout:        time.Second, MaxCommandTimeout: 2 * time.Second,
	}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	identity := enrollmentIdentity{
		ClientID: "trusted-pc", Hostname: "TRUSTED-PC", OS: "windows", Arch: "amd64",
		DeviceTokenHash: hashDeviceToken(testDeviceToken),
	}
	start := func(remoteAddr string, value enrollmentIdentity) (int, string, string) {
		body, _ := json.Marshal(value)
		request := httptest.NewRequest(http.MethodPost, "/api/v1/enrollments", bytes.NewReader(body))
		request.RemoteAddr = remoteAddr
		response := httptest.NewRecorder()
		app.Handler().ServeHTTP(response, request)
		var result struct {
			EnrollmentID string `json:"enrollment_id"`
			PollToken    string `json:"poll_token"`
		}
		_ = json.Unmarshal(response.Body.Bytes(), &result)
		return response.Code, result.EnrollmentID, result.PollToken
	}
	poll := func(id, token string) (int, string, string) {
		request := httptest.NewRequest(http.MethodPost, "/api/v1/enrollments/"+id+"/status", nil)
		request.Header.Set("Authorization", "Enrollment "+token)
		response := httptest.NewRecorder()
		app.Handler().ServeHTTP(response, request)
		var result struct {
			Status     string `json:"status"`
			AgentToken string `json:"agent_token"`
		}
		_ = json.Unmarshal(response.Body.Bytes(), &result)
		return response.Code, result.Status, result.AgentToken
	}

	code, id, token := start("203.0.113.42:50000", identity)
	statusCode, status, agentToken := poll(id, token)
	if code != http.StatusAccepted || statusCode != http.StatusOK || status != "approved" || len(agentToken) != 64 {
		t.Fatalf("enrollment was not auto-approved: start=%d poll=%d status=%q", code, statusCode, status)
	}
}

func TestEnrollmentCanRequireHumanApprovalWhenAutoApprovalDisabled(t *testing.T) {
	app, err := New(Config{
		AdminToken: "admin-test-token", EnrollmentAutoApprove: false,
		DefaultTimeout: time.Second, MaxCommandTimeout: 2 * time.Second,
	}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	identity := enrollmentIdentity{
		ClientID: "manual-pc", Hostname: "MANUAL-PC", OS: "windows", Arch: "amd64",
		DeviceTokenHash: hashDeviceToken(testDeviceToken),
	}
	body, _ := json.Marshal(identity)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/enrollments", bytes.NewReader(body))
	response := httptest.NewRecorder()
	app.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusAccepted || !strings.Contains(response.Body.String(), `"status":"pending"`) {
		t.Fatalf("manual enrollment was not left pending: %d %s", response.Code, response.Body.String())
	}
}

func TestEnrollmentRequiresNativeRequestAndAdminAuthorization(t *testing.T) {
	app := newTestServer(t)
	identity := enrollmentIdentity{
		ClientID: "office-pc", Hostname: "OFFICE-PC", OS: "windows", Arch: "amd64",
		DeviceTokenHash: hashDeviceToken(testDeviceToken),
	}
	body, _ := json.Marshal(identity)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/enrollments", bytes.NewReader(body))
	request.Header.Set("Origin", "http://malicious.example")
	response := httptest.NewRecorder()
	app.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("browser enrollment status = %d, want 403", response.Code)
	}

	approveRequest := httptest.NewRequest(http.MethodPost, "/api/v1/enrollments/"+strings.Repeat("a", 32)+"/approve", nil)
	approveResponse := httptest.NewRecorder()
	app.Handler().ServeHTTP(approveResponse, approveRequest)
	if approveResponse.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous approval status = %d, want 401", approveResponse.Code)
	}
}

func TestEnrollmentIdentitySupportsManagedDesktopPlatforms(t *testing.T) {
	base := enrollmentIdentity{
		ClientID: "office-device", Hostname: "office-host", Arch: "arm64",
		DeviceTokenHash: hashDeviceToken(testDeviceToken),
	}
	for _, platform := range []string{"windows", "darwin", "linux"} {
		identity := base
		identity.OS = platform
		if !validEnrollmentIdentity(identity) {
			t.Fatalf("valid %s enrollment identity was rejected", platform)
		}
	}
	identity := base
	identity.OS = "freebsd"
	if validEnrollmentIdentity(identity) {
		t.Fatal("unsupported enrollment platform was accepted")
	}
}
