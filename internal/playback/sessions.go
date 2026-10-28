package playback

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type session struct {
	id            string
	root          string
	title         string
	headers       http.Header
	trustedOrigin string
	client        *http.Client
	encryption    cipher.AEAD
	expires       time.Time
}

type Service struct {
	Resolver *Resolver
	mutex    sync.Mutex
	sessions map[string]*session
}

func New() *Service { return &Service{Resolver: NewResolver(), sessions: make(map[string]*session)} }

func (s *Service) Create(
	endpoint, title, publicURL string, headers http.Header, trusted bool,
) (Descriptor, error) {
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.User != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return Descriptor{}, fmt.Errorf("invalid_stream_url")
	}
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return Descriptor{}, err
	}
	block, err := aes.NewCipher(secret)
	if err != nil {
		return Descriptor{}, err
	}
	encryption, err := cipher.NewGCM(block)
	if err != nil {
		return Descriptor{}, err
	}
	rawID := make([]byte, 24)
	if _, err := rand.Read(rawID); err != nil {
		return Descriptor{}, err
	}
	playback := &session{id: hex.EncodeToString(rawID), root: endpoint, title: title,
		headers: headers.Clone(), encryption: encryption, expires: time.Now().Add(6 * time.Hour)}
	if trusted {
		playback.trustedOrigin = parsed.Scheme + "://" + parsed.Host
	}
	playback.client = streamClient(playback.trustedOrigin, playback.headers)
	s.mutex.Lock()
	for id, entry := range s.sessions {
		if time.Now().After(entry.expires) {
			entry.client.CloseIdleConnections()
			delete(s.sessions, id)
		}
	}
	if len(s.sessions) >= 32 {
		s.mutex.Unlock()
		return Descriptor{}, fmt.Errorf("quá nhiều phiên phát")
	}
	s.sessions[playback.id] = playback
	s.mutex.Unlock()
	return Descriptor{URL: strings.TrimRight(publicURL, "/") + playback.reference(endpoint),
		MIMEType: streamMIME(endpoint), Title: title, PreferredHandler: "native",
		PreferredPlayers: []string{"org.videolan.vlc", "system"}, ExpiresAt: playback.expires}, nil
}

func (s *session) reference(endpoint string) string {
	nonce := make([]byte, s.encryption.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return ""
	}
	encrypted := s.encryption.Seal(nonce, nonce, []byte(endpoint), nil)
	suffix := "media"
	if strings.Contains(strings.Split(endpoint, "?")[0], ".m3u8") {
		suffix = "master.m3u8"
	}
	return "/media/stream/" + s.id + "/" + base64.RawURLEncoding.EncodeToString(encrypted) + "/" + suffix
}

func (s *Service) lookup(path string) (*session, string, error) {
	parts := strings.Split(strings.TrimPrefix(path, "/media/stream/"), "/")
	if len(parts) != 3 {
		return nil, "", fmt.Errorf("invalid_stream_reference")
	}
	s.mutex.Lock()
	playback := s.sessions[parts[0]]
	s.mutex.Unlock()
	if playback == nil || time.Now().After(playback.expires) {
		return nil, "", fmt.Errorf("stream_expired")
	}
	data, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || len(data) < playback.encryption.NonceSize() {
		return nil, "", fmt.Errorf("invalid_stream_reference")
	}
	nonce := data[:playback.encryption.NonceSize()]
	plaintext, err := playback.encryption.Open(nil, nonce, data[len(nonce):], nil)
	if err != nil {
		return nil, "", fmt.Errorf("invalid_stream_reference")
	}
	return playback, string(plaintext), nil
}
