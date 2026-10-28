package camstream

import (
	"crypto/tls"
	"net/http"
	"sync"
	"time"
)

type TapoControlSession struct {
	mutex          sync.Mutex
	host           string
	user           string
	password       string
	stok           string
	seq            int
	cnonce         string
	serverNonce    string
	hashedPassword string
	lsk            []byte
	ivb            []byte
	authenticated  bool
	privacyEnabled bool
	httpClient     *http.Client
}

var globalTapoSession = &TapoControlSession{
	httpClient: &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			DisableKeepAlives: true,
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	},
}

func GetTapoSession(host string, user string, password string) *TapoControlSession {
	globalTapoSession.mutex.Lock()
	defer globalTapoSession.mutex.Unlock()

	if globalTapoSession.host != host || globalTapoSession.password != password {
		globalTapoSession.host = host
		globalTapoSession.user = user
		globalTapoSession.password = password
		globalTapoSession.authenticated = false
		globalTapoSession.stok = ""
	}

	return globalTapoSession
}
