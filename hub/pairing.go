package main

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	pairingStatusPending  = "pending"
	pairingStatusApproved = "approved"
	pairingStatusRejected = "rejected"
	pairingRequestTTL     = 10 * time.Minute
	maxPendingRequests    = 16
	maxDeviceNameLength   = 40
)

var errPairingNotFound = errors.New("pairing request not found")

type PairedDevice struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Token      string `json:"token"`
	PairedAt   int64  `json:"pairedAt"`
	LastSeenAt int64  `json:"lastSeenAt"`
}

type PairingRequest struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Code      string `json:"code"`
	Status    string `json:"status"`
	CreatedAt int64  `json:"createdAt"`
	token     string
}

type PairingManager struct {
	mutex    sync.Mutex
	filePath string
	pending  map[string]*PairingRequest
	paired   []PairedDevice
}

func NewPairingManager(filePath string) *PairingManager {
	manager := &PairingManager{
		filePath: filePath,
		pending:  make(map[string]*PairingRequest),
	}

	manager.loadFromFile()
	return manager
}

func (manager *PairingManager) loadFromFile() {
	contentBytes, readErr := os.ReadFile(manager.filePath)
	if readErr != nil {
		return
	}

	var parsed []PairedDevice
	if jsonErr := json.Unmarshal(contentBytes, &parsed); jsonErr == nil {
		manager.paired = parsed
	}
}

func (manager *PairingManager) saveToFile() error {
	encoded, marshalErr := json.MarshalIndent(manager.paired, "", "  ")
	if marshalErr != nil {
		return marshalErr
	}

	return os.WriteFile(manager.filePath, encoded, 0600)
}

func randomHex(byteCount int) string {
	buffer := make([]byte, byteCount)
	_, _ = rand.Read(buffer)
	return hex.EncodeToString(buffer)
}

func randomPairingCode() string {
	buffer := make([]byte, 3)
	_, _ = rand.Read(buffer)
	number := int(buffer[0])<<16 | int(buffer[1])<<8 | int(buffer[2])
	digits := []byte("000000")
	for position := len(digits) - 1; position >= 0; position-- {
		digits[position] = byte('0' + number%10)
		number /= 10
	}
	return string(digits)
}

func (manager *PairingManager) dropExpired() {
	deadline := time.Now().Add(-pairingRequestTTL).Unix()
	for requestID, request := range manager.pending {
		if request.CreatedAt < deadline {
			delete(manager.pending, requestID)
		}
	}
}

func (manager *PairingManager) Request(deviceName string) (*PairingRequest, error) {
	manager.mutex.Lock()
	defer manager.mutex.Unlock()

	manager.dropExpired()
	if len(manager.pending) >= maxPendingRequests {
		return nil, errors.New("too many pending pairing requests")
	}

	cleanName := strings.TrimSpace(deviceName)
	if len(cleanName) > maxDeviceNameLength {
		cleanName = cleanName[:maxDeviceNameLength]
	}
	if cleanName == "" {
		cleanName = "TV"
	}

	request := &PairingRequest{
		ID:        randomHex(8),
		Name:      cleanName,
		Code:      randomPairingCode(),
		Status:    pairingStatusPending,
		CreatedAt: time.Now().Unix(),
	}
	manager.pending[request.ID] = request
	return request, nil
}

func (manager *PairingManager) Status(requestID string) (string, string, error) {
	manager.mutex.Lock()
	defer manager.mutex.Unlock()

	manager.dropExpired()
	request, found := manager.pending[requestID]
	if !found {
		return "", "", errPairingNotFound
	}

	if request.Status == pairingStatusPending {
		return request.Status, "", nil
	}

	delete(manager.pending, requestID)
	return request.Status, request.token, nil
}

func (manager *PairingManager) Approve(requestID string) error {
	manager.mutex.Lock()
	defer manager.mutex.Unlock()

	request, found := manager.pending[requestID]
	if !found || request.Status != pairingStatusPending {
		return errPairingNotFound
	}

	request.token = randomHex(24)
	request.Status = pairingStatusApproved
	now := time.Now().Unix()
	manager.paired = append(manager.paired, PairedDevice{
		ID:       request.ID,
		Name:     request.Name,
		Token:    request.token,
		PairedAt: now,
	})
	return manager.saveToFile()
}

func (manager *PairingManager) Reject(requestID string) error {
	manager.mutex.Lock()
	defer manager.mutex.Unlock()

	request, found := manager.pending[requestID]
	if !found {
		return errPairingNotFound
	}

	request.Status = pairingStatusRejected
	return nil
}

func (manager *PairingManager) Revoke(deviceID string) error {
	manager.mutex.Lock()
	defer manager.mutex.Unlock()

	kept := manager.paired[:0]
	for _, device := range manager.paired {
		if device.ID != deviceID {
			kept = append(kept, device)
		}
	}
	manager.paired = kept
	return manager.saveToFile()
}

func (manager *PairingManager) Snapshot() ([]*PairingRequest, []PairedDevice) {
	manager.mutex.Lock()
	defer manager.mutex.Unlock()

	manager.dropExpired()
	waiting := make([]*PairingRequest, 0)
	for _, request := range manager.pending {
		if request.Status == pairingStatusPending {
			waiting = append(waiting, request)
		}
	}

	devices := make([]PairedDevice, len(manager.paired))
	for index, device := range manager.paired {
		device.Token = ""
		devices[index] = device
	}
	return waiting, devices
}

func (manager *PairingManager) Authenticate(token string) bool {
	if token == "" {
		return false
	}

	manager.mutex.Lock()
	defer manager.mutex.Unlock()

	for index := range manager.paired {
		expected := []byte(manager.paired[index].Token)
		if subtle.ConstantTimeCompare(expected, []byte(token)) == 1 {
			manager.paired[index].LastSeenAt = time.Now().Unix()
			return true
		}
	}
	return false
}

func bearerToken(request *http.Request) string {
	header := request.Header.Get("Authorization")
	return strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
}

func isHubWebPage(request *http.Request) bool {
	return request.Header.Get("Sec-Fetch-Site") == "same-origin"
}

func (manager *PairingManager) Require(next http.HandlerFunc) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if isHubWebPage(request) || manager.Authenticate(bearerToken(request)) {
			next(writer, request)
			return
		}

		writer.Header().Set("Content-Type", "application/json; charset=utf-8")
		writer.WriteHeader(http.StatusUnauthorized)
		_, _ = writer.Write([]byte(`{"ok":false,"error":"Thiết bị chưa được Hub cho phép"}`))
	}
}

func (manager *PairingManager) RequireWebPage(next http.HandlerFunc) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if isHubWebPage(request) {
			next(writer, request)
			return
		}

		writer.Header().Set("Content-Type", "application/json; charset=utf-8")
		writer.WriteHeader(http.StatusForbidden)
		_, _ = writer.Write([]byte(`{"ok":false,"error":"Chỉ duyệt trên trang web của Hub"}`))
	}
}
