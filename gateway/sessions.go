package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"sort"
	"sync"
	"time"
)

const accessTokenSafetyMargin = 30 * time.Second

var errSessionNotFound = errors.New("session not found")

type Session struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	ClientID     string    `json:"clientId"`
	RefreshToken string    `json:"refreshToken"`
	CreatedAt    time.Time `json:"createdAt"`
}

type cachedAccess struct {
	token     string
	expiresAt time.Time
}

type SessionStore struct {
	mutex    sync.Mutex
	filePath string
	clock    func() time.Time
	sessions map[string]Session
	access   map[string]cachedAccess
}

func NewSessionStore(filePath string, clock func() time.Time) *SessionStore {
	store := &SessionStore{
		filePath: filePath,
		clock:    clock,
		sessions: map[string]Session{},
		access:   map[string]cachedAccess{},
	}
	store.load()
	return store
}

func hashToken(token string) string {
	digest := sha256.Sum256([]byte(token))
	return hex.EncodeToString(digest[:])
}

func (store *SessionStore) load() {
	content, readErr := os.ReadFile(store.filePath)
	if readErr != nil {
		return
	}
	_ = json.Unmarshal(content, &store.sessions)
}

func (store *SessionStore) saveLocked() error {
	encoded, marshalErr := json.MarshalIndent(store.sessions, "", "  ")
	if marshalErr != nil {
		return marshalErr
	}
	return os.WriteFile(store.filePath, encoded, 0600)
}

func (store *SessionStore) Add(name, clientID, refreshToken string) (string, error) {
	store.mutex.Lock()
	defer store.mutex.Unlock()

	token := randomToken(32)
	store.sessions[hashToken(token)] = Session{
		ID:           randomToken(6),
		Name:         name,
		ClientID:     clientID,
		RefreshToken: refreshToken,
		CreatedAt:    store.clock(),
	}
	return token, store.saveLocked()
}

func (store *SessionStore) Lookup(token string) (string, Session, bool) {
	store.mutex.Lock()
	defer store.mutex.Unlock()

	key := hashToken(token)
	session, found := store.sessions[key]
	return key, session, found
}

func (store *SessionStore) Remove(key string) (Session, error) {
	store.mutex.Lock()
	defer store.mutex.Unlock()

	session, found := store.sessions[key]
	if !found {
		return Session{}, errSessionNotFound
	}
	delete(store.sessions, key)
	delete(store.access, key)
	return session, store.saveLocked()
}

func (store *SessionStore) RemoveByID(sessionID string) (Session, error) {
	store.mutex.Lock()
	key := ""
	for candidate, session := range store.sessions {
		if session.ID == sessionID {
			key = candidate
		}
	}
	store.mutex.Unlock()

	if key == "" {
		return Session{}, errSessionNotFound
	}
	return store.Remove(key)
}

func (store *SessionStore) List() []Session {
	store.mutex.Lock()
	defer store.mutex.Unlock()

	listed := make([]Session, 0, len(store.sessions))
	for _, session := range store.sessions {
		session.RefreshToken = ""
		listed = append(listed, session)
	}
	sort.Slice(listed, func(left, right int) bool {
		return listed[left].CreatedAt.Before(listed[right].CreatedAt)
	})
	return listed
}

func (store *SessionStore) AccessToken(
	ctx context.Context,
	key string,
	session Session,
	homeAssistant *HomeAssistant,
) (string, error) {
	store.mutex.Lock()
	cached, found := store.access[key]
	store.mutex.Unlock()
	if found && store.clock().Add(accessTokenSafetyMargin).Before(cached.expiresAt) {
		return cached.token, nil
	}

	grant, refreshErr := homeAssistant.RefreshAccess(ctx, session.RefreshToken, session.ClientID)
	if refreshErr != nil {
		return "", refreshErr
	}

	store.mutex.Lock()
	store.access[key] = cachedAccess{
		token:     grant.AccessToken,
		expiresAt: store.clock().Add(time.Duration(grant.ExpiresIn) * time.Second),
	}
	store.mutex.Unlock()
	return grant.AccessToken, nil
}
