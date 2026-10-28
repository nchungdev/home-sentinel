package pairing

import (
	"crypto/rand"
	"encoding/hex"
	"math/big"
	"strings"
	"sync"
	"time"
)

const (
	codeAlphabet = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"
	codeLength   = 6
	idBytes      = 16
	maxPending   = 32
)

type Pairing struct {
	ID        string
	Code      string
	ExpiresAt time.Time
	approved  bool
}

type Store struct {
	mutex   sync.Mutex
	ttl     time.Duration
	now     func() time.Time
	pending map[string]*Pairing
}

func NewStore(ttl time.Duration) *Store {
	return &Store{ttl: ttl, now: time.Now, pending: map[string]*Pairing{}}
}

func (s *Store) Start() (Pairing, error) {
	id, err := randomID()
	if err != nil {
		return Pairing{}, err
	}
	code, err := randomCode()
	if err != nil {
		return Pairing{}, err
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.prune()
	if len(s.pending) >= maxPending {
		s.dropOldest()
	}
	entry := &Pairing{ID: id, Code: code, ExpiresAt: s.now().Add(s.ttl)}
	s.pending[id] = entry
	return *entry, nil
}

func (s *Store) Approve(code string) bool {
	wanted := NormalizeCode(code)
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.prune()
	for _, entry := range s.pending {
		if entry.Code == wanted && !entry.approved {
			entry.approved = true
			return true
		}
	}
	return false
}

func (s *Store) Poll(id string) (approved bool, found bool) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.prune()
	entry, ok := s.pending[id]
	if !ok {
		return false, false
	}
	if entry.approved {
		delete(s.pending, id)
	}
	return entry.approved, true
}

func NormalizeCode(code string) string {
	upper := strings.ToUpper(code)
	return strings.Map(func(r rune) rune {
		if strings.ContainsRune(codeAlphabet, r) {
			return r
		}
		return -1
	}, upper)
}

func (s *Store) prune() {
	now := s.now()
	for id, entry := range s.pending {
		if now.After(entry.ExpiresAt) {
			delete(s.pending, id)
		}
	}
}

func (s *Store) dropOldest() {
	var oldest *Pairing
	for _, entry := range s.pending {
		if oldest == nil || entry.ExpiresAt.Before(oldest.ExpiresAt) {
			oldest = entry
		}
	}
	if oldest != nil {
		delete(s.pending, oldest.ID)
	}
}

func randomID() (string, error) {
	raw := make([]byte, idBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw), nil
}

func randomCode() (string, error) {
	var builder strings.Builder
	limit := big.NewInt(int64(len(codeAlphabet)))
	for range codeLength {
		index, err := rand.Int(rand.Reader, limit)
		if err != nil {
			return "", err
		}
		builder.WriteByte(codeAlphabet[index.Int64()])
	}
	return builder.String(), nil
}
