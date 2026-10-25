package camera

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"strconv"
	"strings"
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

func (session *TapoControlSession) MoveMotor(x int, y int) error {
	movePayload := map[string]interface{}{
		"method": "do",
		"motor": map[string]interface{}{
			"move": map[string]interface{}{
				"x_coord": strconv.Itoa(x),
				"y_coord": strconv.Itoa(y),
			},
		},
	}
	_, sendErr := session.SendSecure(movePayload)
	return sendErr
}

func (session *TapoControlSession) MoveDirection(direction string, stepSize int) error {
	delta := stepSize
	if delta <= 0 {
		delta = 20
	}
	xCoord := 0
	yCoord := 0
	switch strings.ToLower(direction) {
	case "left":
		xCoord = -delta
	case "right":
		xCoord = delta
	case "up":
		yCoord = delta
	case "down":
		yCoord = -delta
	case "stop":
		return session.StopMotor()
	default:
		return fmt.Errorf("unsupported direction: %s", direction)
	}
	return session.MoveMotor(xCoord, yCoord)
}

func (session *TapoControlSession) StopMotor() error {
	stopPayload := map[string]interface{}{
		"method": "do",
		"motor": map[string]interface{}{
			"stop": map[string]interface{}{},
		},
	}
	_, sendErr := session.SendSecure(stopPayload)
	return sendErr
}

func (session *TapoControlSession) SetPrivacyMode(enabled bool) error {
	enabledString := "off"
	if enabled {
		enabledString = "on"
	}
	maskPayload := map[string]interface{}{
		"method": "do",
		"lens_mask": map[string]interface{}{
			"lens_mask_info": map[string]interface{}{
				"enabled": enabledString,
			},
		},
	}
	_, sendErr := session.SendSecure(maskPayload)
	if sendErr == nil {
		session.privacyEnabled = enabled
	}
	return sendErr
}

func (session *TapoControlSession) GetPrivacyMode() bool {
	return session.privacyEnabled
}
