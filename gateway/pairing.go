package main

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"sync"
	"time"
)

const (
	pairingPending  = "pending"
	pairingApproved = "approved"
	pairingRejected = "rejected"

	pairingTTL         = 10 * time.Minute
	maxPendingPairings = 16
	maxDeviceNameRunes = 40
	pairingCodeDigits  = 6
	defaultDeviceName  = "TV"
)

var (
	errPairingNotFound   = errors.New("pairing request not found or expired")
	errPairingDecided    = errors.New("pairing request already decided")
	errTooManyPairings   = errors.New("too many pending pairing requests")
	errPairingNonceUnset = errors.New("pairing approval not started")
)

type PairingRequest struct {
	ID          string
	Name        string
	Code        string
	Status      string
	CreatedAt   time.Time
	AccessToken string
}

type PairingStore struct {
	mutex    sync.Mutex
	clock    func() time.Time
	requests map[string]*PairingRequest
	nonces   map[string]string
}

func NewPairingStore(clock func() time.Time) *PairingStore {
	return &PairingStore{
		clock:    clock,
		requests: map[string]*PairingRequest{},
		nonces:   map[string]string{},
	}
}

func randomToken(byteCount int) string {
	buffer := make([]byte, byteCount)
	_, _ = rand.Read(buffer)
	return base64.RawURLEncoding.EncodeToString(buffer)
}

func randomDigits(count int) string {
	buffer := make([]byte, count)
	_, _ = rand.Read(buffer)
	for index, value := range buffer {
		buffer[index] = '0' + value%10
	}
	return string(buffer)
}

func (store *PairingStore) dropExpired() {
	deadline := store.clock().Add(-pairingTTL)
	for id, request := range store.requests {
		if request.CreatedAt.Before(deadline) {
			delete(store.requests, id)
		}
	}
}

func (store *PairingStore) pendingLocked() []PairingRequest {
	store.dropExpired()
	waiting := []PairingRequest{}
	for _, request := range store.requests {
		if request.Status == pairingPending {
			waiting = append(waiting, *request)
		}
	}
	return waiting
}

func (store *PairingStore) Pending() []PairingRequest {
	store.mutex.Lock()
	defer store.mutex.Unlock()

	return store.pendingLocked()
}

func cleanDeviceName(raw string) string {
	runes := []rune(raw)
	if len(runes) > maxDeviceNameRunes {
		runes = runes[:maxDeviceNameRunes]
	}
	name := string(runes)
	for _, character := range name {
		if character != ' ' {
			return name
		}
	}
	return defaultDeviceName
}

func (store *PairingStore) Create(deviceName string) (PairingRequest, error) {
	store.mutex.Lock()
	defer store.mutex.Unlock()

	if len(store.pendingLocked()) >= maxPendingPairings {
		return PairingRequest{}, errTooManyPairings
	}

	request := &PairingRequest{
		ID:        randomToken(12),
		Name:      cleanDeviceName(deviceName),
		Code:      randomDigits(pairingCodeDigits),
		Status:    pairingPending,
		CreatedAt: store.clock(),
	}
	store.requests[request.ID] = request
	return *request, nil
}

func (store *PairingStore) Collect(requestID string) (PairingRequest, error) {
	store.mutex.Lock()
	defer store.mutex.Unlock()

	store.dropExpired()
	request, found := store.requests[requestID]
	if !found {
		return PairingRequest{}, errPairingNotFound
	}
	if request.Status != pairingPending {
		delete(store.requests, requestID)
	}
	return *request, nil
}

func (store *PairingStore) BeginApproval(requestID string) (string, error) {
	store.mutex.Lock()
	defer store.mutex.Unlock()

	store.dropExpired()
	request, found := store.requests[requestID]
	if !found {
		return "", errPairingNotFound
	}
	if request.Status != pairingPending {
		return "", errPairingDecided
	}

	nonce := randomToken(16)
	store.nonces[nonce] = requestID
	return nonce, nil
}

func (store *PairingStore) CompleteApproval(nonce, accessToken string) (PairingRequest, error) {
	store.mutex.Lock()
	defer store.mutex.Unlock()

	requestID, started := store.nonces[nonce]
	if !started {
		return PairingRequest{}, errPairingNonceUnset
	}
	delete(store.nonces, nonce)

	store.dropExpired()
	request, found := store.requests[requestID]
	if !found {
		return PairingRequest{}, errPairingNotFound
	}
	if request.Status != pairingPending {
		return PairingRequest{}, errPairingDecided
	}

	request.Status = pairingApproved
	request.AccessToken = accessToken
	return *request, nil
}

func (store *PairingStore) Reject(requestID string) error {
	store.mutex.Lock()
	defer store.mutex.Unlock()

	store.dropExpired()
	request, found := store.requests[requestID]
	if !found {
		return errPairingNotFound
	}
	if request.Status != pairingPending {
		return errPairingDecided
	}

	request.Status = pairingRejected
	return nil
}

func (store *PairingStore) PendingNameFor(nonce string) (string, error) {
	store.mutex.Lock()
	defer store.mutex.Unlock()

	requestID, started := store.nonces[nonce]
	if !started {
		return "", errPairingNonceUnset
	}

	store.dropExpired()
	request, found := store.requests[requestID]
	if !found {
		return "", errPairingNotFound
	}
	return request.Name, nil
}
