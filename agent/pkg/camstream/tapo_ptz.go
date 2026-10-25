package camstream

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
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

func generateRandomHex(length int) (string, error) {
	randomBytes := make([]byte, length)
	if _, readErr := rand.Read(randomBytes); readErr != nil {
		return "", readErr
	}

	return strings.ToUpper(hex.EncodeToString(randomBytes)), nil
}

func sha256Hex(content string) string {
	hasher := sha256.New()
	hasher.Write([]byte(content))
	return strings.ToUpper(hex.EncodeToString(hasher.Sum(nil)))
}

func md5Hex(content string) string {
	hasher := md5.New()
	hasher.Write([]byte(content))
	return strings.ToUpper(hex.EncodeToString(hasher.Sum(nil)))
}

func pkcs7Pad(input []byte, blockSize int) []byte {
	paddingSize := blockSize - len(input)%blockSize
	paddingBytes := bytes.Repeat([]byte{byte(paddingSize)}, paddingSize)
	return append(input, paddingBytes...)
}

func pkcs7Unpad(input []byte) ([]byte, error) {
	length := len(input)
	if length == 0 {
		return nil, errors.New("empty ciphertext buffer")
	}

	paddingSize := int(input[length-1])
	if paddingSize > length || paddingSize == 0 {
		return nil, errors.New("invalid padding format")
	}

	return input[:length-paddingSize], nil
}

func encryptAesCbc(plaintext []byte, key []byte, iv []byte) ([]byte, error) {
	cipherBlock, cipherErr := aes.NewCipher(key)
	if cipherErr != nil {
		return nil, cipherErr
	}

	paddedData := pkcs7Pad(plaintext, aes.BlockSize)
	ciphertext := make([]byte, len(paddedData))
	encrypter := cipher.NewCBCEncrypter(cipherBlock, iv)
	encrypter.CryptBlocks(ciphertext, paddedData)

	return ciphertext, nil
}

func decryptAesCbc(ciphertext []byte, key []byte, iv []byte) ([]byte, error) {
	cipherBlock, cipherErr := aes.NewCipher(key)
	if cipherErr != nil {
		return nil, cipherErr
	}

	if len(ciphertext)%aes.BlockSize != 0 {
		return nil, errors.New("ciphertext is not multiple of block size")
	}

	decryptedPadded := make([]byte, len(ciphertext))
	decrypter := cipher.NewCBCDecrypter(cipherBlock, iv)
	decrypter.CryptBlocks(decryptedPadded, ciphertext)

	return pkcs7Unpad(decryptedPadded)
}

func (session *TapoControlSession) Authenticate() error {
	session.mutex.Lock()
	defer session.mutex.Unlock()
	return session.authenticateLocked()
}

func (session *TapoControlSession) authenticateLocked() error {
	clientNonce, nonceErr := generateRandomHex(8)
	if nonceErr != nil {
		return nonceErr
	}
	session.cnonce = clientNonce

	loginProbePayload := map[string]interface{}{
		"method": "login",
		"params": map[string]interface{}{
			"cnonce":       session.cnonce,
			"encrypt_type": "3",
			"username":     "admin",
		},
	}

	encodedProbe, encodeErr := json.Marshal(loginProbePayload)
	if encodeErr != nil {
		return encodeErr
	}

	probeURL := fmt.Sprintf("https://%s/stok=/ds", session.host)
	probeRequest, reqErr := http.NewRequest("POST", probeURL, bytes.NewReader(encodedProbe))
	if reqErr != nil {
		return reqErr
	}
	probeRequest.Header.Set("Content-Type", "application/json; charset=UTF-8")
	probeRequest.Header.Set("User-Agent", "Tapo CameraClient Android")
	probeRequest.Header.Set("requestByApp", "true")

	probeResponse, probeErr := session.httpClient.Do(probeRequest)
	if probeErr != nil {
		return probeErr
	}
	defer probeResponse.Body.Close()

	probeBodyBytes, readErr := io.ReadAll(probeResponse.Body)
	if readErr != nil {
		return readErr
	}

	var probeResult struct {
		Result struct {
			Data struct {
				Nonce         string `json:"nonce"`
				DeviceConfirm string `json:"device_confirm"`
			} `json:"data"`
		} `json:"result"`
	}
	_ = json.Unmarshal(probeBodyBytes, &probeResult)
	session.serverNonce = probeResult.Result.Data.Nonce

	if session.serverNonce == "" {
		return errors.New("missing server nonce from tapo camera handshake")
	}

	effectivePassword := session.password
	shaHashed := sha256Hex(effectivePassword)
	md5Hashed := md5Hex(effectivePassword)

	deviceConfirmReceived := probeResult.Result.Data.DeviceConfirm
	shaVerification := sha256Hex(
		session.cnonce + shaHashed + session.serverNonce,
	) + session.serverNonce + session.cnonce

	md5Verification := sha256Hex(
		session.cnonce + md5Hashed + session.serverNonce,
	) + session.serverNonce + session.cnonce

	if deviceConfirmReceived == shaVerification {
		session.hashedPassword = shaHashed
	} else if deviceConfirmReceived == md5Verification {
		session.hashedPassword = md5Hashed
	} else {
		session.hashedPassword = shaHashed
	}

	digestPasswdHash := sha256Hex(
		session.hashedPassword + session.cnonce + session.serverNonce,
	)
	digestPasswdParam := digestPasswdHash + session.cnonce + session.serverNonce

	confirmPayload := map[string]interface{}{
		"method": "login",
		"params": map[string]interface{}{
			"cnonce":        session.cnonce,
			"encrypt_type":  "3",
			"digest_passwd": digestPasswdParam,
			"username":      "admin",
		},
	}

	encodedConfirm, _ := json.Marshal(confirmPayload)
	confirmRequest, _ := http.NewRequest("POST", probeURL, bytes.NewReader(encodedConfirm))
	confirmRequest.Header.Set("Content-Type", "application/json; charset=UTF-8")
	confirmRequest.Header.Set("User-Agent", "Tapo CameraClient Android")
	confirmRequest.Header.Set("requestByApp", "true")

	confirmResponse, confirmErr := session.httpClient.Do(confirmRequest)
	if confirmErr != nil {
		return confirmErr
	}
	defer confirmResponse.Body.Close()

	confirmBodyBytes, _ := io.ReadAll(confirmResponse.Body)
	var confirmResult struct {
		Result struct {
			Stok     string `json:"stok"`
			StartSeq int    `json:"start_seq"`
		} `json:"result"`
		ErrorCode int `json:"error_code"`
	}

	if jsonErr := json.Unmarshal(confirmBodyBytes, &confirmResult); jsonErr != nil {
		return jsonErr
	}

	if confirmResult.Result.Stok == "" {
		return fmt.Errorf("camera login failed with error code: %d", confirmResult.ErrorCode)
	}

	session.stok = confirmResult.Result.Stok
	session.seq = confirmResult.Result.StartSeq
	if session.seq == 0 {
		session.seq = 1000
	}

	hashedKey := sha256Hex(session.cnonce + session.hashedPassword + session.serverNonce)
	lskHash := sha256.Sum256(
		[]byte("lsk" + session.cnonce + session.serverNonce + hashedKey),
	)
	ivbHash := sha256.Sum256(
		[]byte("ivb" + session.cnonce + session.serverNonce + hashedKey),
	)

	session.lsk = lskHash[:16]
	session.ivb = ivbHash[:16]
	session.authenticated = true

	return nil
}

func (session *TapoControlSession) SendSecure(
	innerPayload map[string]interface{},
) (map[string]interface{}, error) {
	session.mutex.Lock()
	defer session.mutex.Unlock()

	if !session.authenticated || session.stok == "" {
		if authErr := session.authenticateLocked(); authErr != nil {
			return nil, authErr
		}
	}

	encodedInner, jsonErr := json.Marshal(innerPayload)
	if jsonErr != nil {
		return nil, jsonErr
	}

	cipherBytes, encryptErr := encryptAesCbc(encodedInner, session.lsk, session.ivb)
	if encryptErr != nil {
		return nil, encryptErr
	}

	base64Cipher := base64.StdEncoding.EncodeToString(cipherBytes)
	wrappedPayload := map[string]interface{}{
		"method": "securePassthrough",
		"params": map[string]interface{}{
			"request": base64Cipher,
		},
	}

	wrappedBytes, _ := json.Marshal(wrappedPayload)
	tagPrefix := sha256Hex(session.hashedPassword + session.cnonce)
	tagData := tagPrefix + string(wrappedBytes) + strconv.Itoa(session.seq)
	tapoTag := sha256Hex(tagData)

	requestURL := fmt.Sprintf("https://%s/stok=%s/ds", session.host, session.stok)
	requestObj, reqErr := http.NewRequest("POST", requestURL, bytes.NewReader(wrappedBytes))
	if reqErr != nil {
		return nil, reqErr
	}

	requestObj.Header.Set("Content-Type", "application/json; charset=UTF-8")
	requestObj.Header.Set("User-Agent", "Tapo CameraClient Android")
	requestObj.Header.Set("requestByApp", "true")
	requestObj.Header.Set("Seq", strconv.Itoa(session.seq))
	requestObj.Header.Set("Tapo_tag", tapoTag)
	session.seq++

	response, postErr := session.httpClient.Do(requestObj)
	if postErr != nil {
		session.authenticated = false
		return nil, postErr
	}
	defer response.Body.Close()

	responseBytes, _ := io.ReadAll(response.Body)
	var responseWrapper struct {
		Result struct {
			Response string `json:"response"`
		} `json:"result"`
		ErrorCode int `json:"error_code"`
	}

	_ = json.Unmarshal(responseBytes, &responseWrapper)
	if responseWrapper.Result.Response == "" {
		if responseWrapper.ErrorCode == 0 {
			return map[string]interface{}{"ok": true, "error_code": 0}, nil
		}
		session.authenticated = false
		return nil, fmt.Errorf(
			"camera secure request returned error %d", responseWrapper.ErrorCode,
		)
	}

	cipherRaw, b64Err := base64.StdEncoding.DecodeString(responseWrapper.Result.Response)
	if b64Err != nil {
		return nil, b64Err
	}

	decryptedBytes, decryptErr := decryptAesCbc(cipherRaw, session.lsk, session.ivb)
	if decryptErr != nil {
		session.authenticated = false
		return nil, decryptErr
	}

	var innerResult map[string]interface{}
	if unmarshalErr := json.Unmarshal(decryptedBytes, &innerResult); unmarshalErr != nil {
		return nil, unmarshalErr
	}

	if codeVal, hasCode := innerResult["error_code"]; hasCode {
		if codeNum, isNum := codeVal.(float64); isNum && codeNum != 0 {
			return nil, fmt.Errorf("camera returned error code %d", int(codeNum))
		}
	}

	return innerResult, nil
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

func (session *TapoControlSession) SetDetectionFeature(feature string, enabled bool) error {
	enabledString := "off"
	if enabled {
		enabledString = "on"
	}

	var payload map[string]interface{}
	switch feature {
	case "motion":
		payload = map[string]interface{}{
			"method": "setDetectionConfig",
			"motion_detection": map[string]interface{}{
				"motion_det": map[string]interface{}{
					"enabled": enabledString,
				},
			},
		}
	case "tracking":
		payload = map[string]interface{}{
			"method": "setTargetTrackConfig",
			"target_track": map[string]interface{}{
				"target_track_info": map[string]interface{}{
					"enabled": enabledString,
				},
			},
		}
	case "person":
		payload = map[string]interface{}{
			"method": "setPersonDetectionConfig",
			"people_detection": map[string]interface{}{
				"detection": map[string]interface{}{
					"enabled": enabledString,
				},
			},
		}
	case "baby_cry":
		payload = map[string]interface{}{
			"method": "setBCDConfig",
			"sound_detection": map[string]interface{}{
				"bcd": map[string]interface{}{
					"enabled": enabledString,
				},
			},
		}
	default:
		return fmt.Errorf("tính năng không hợp lệ: %s", feature)
	}

	_, sendErr := session.SendSecure(payload)
	return sendErr
}

func (session *TapoControlSession) GetDetectionStates() (map[string]bool, error) {
	session.mutex.Lock()
	defer session.mutex.Unlock()

	if !session.authenticated || session.stok == "" {
		if authErr := session.authenticateLocked(); authErr != nil {
			return nil, authErr
		}
	}

	payload := map[string]interface{}{
		"method": "get",
		"motion_detection": map[string]interface{}{
			"name": []string{"motion_det"},
		},
		"people_detection": map[string]interface{}{
			"name": []string{"detection"},
		},
		"target_track": map[string]interface{}{
			"name": []string{"target_track_info"},
		},
		"sound_detection": map[string]interface{}{
			"name": []string{"bcd"},
		},
		"lens_mask": map[string]interface{}{
			"name": []string{"lens_mask_info"},
		},
	}

	encoded, _ := json.Marshal(payload)
	reqURL := fmt.Sprintf("https://%s/stok=%s/ds", session.host, session.stok)
	req, err := http.NewRequest("POST", reqURL, bytes.NewReader(encoded))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")

	resp, err := session.httpClient.Do(req)
	if err != nil {
		session.authenticated = false
		return nil, err
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	var result struct {
		ErrorCode       int `json:"error_code"`
		MotionDetection struct {
			MotionDet struct {
				Enabled string `json:"enabled"`
			} `json:"motion_det"`
		} `json:"motion_detection"`
		PeopleDetection struct {
			Detection struct {
				Enabled string `json:"enabled"`
			} `json:"detection"`
		} `json:"people_detection"`
		TargetTrack struct {
			TargetTrackInfo struct {
				Enabled string `json:"enabled"`
			} `json:"target_track_info"`
		} `json:"target_track"`
		SoundDetection struct {
			BCD struct {
				Enabled string `json:"enabled"`
			} `json:"bcd"`
		} `json:"sound_detection"`
		LensMask struct {
			LensMaskInfo struct {
				Enabled string `json:"enabled"`
			} `json:"lens_mask_info"`
		} `json:"lens_mask"`
	}

	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return nil, err
	}

	states := map[string]bool{
		"motion":   result.MotionDetection.MotionDet.Enabled == "on",
		"person":   result.PeopleDetection.Detection.Enabled == "on",
		"tracking": result.TargetTrack.TargetTrackInfo.Enabled == "on",
		"baby_cry": result.SoundDetection.BCD.Enabled == "on",
		"privacy":  result.LensMask.LensMaskInfo.Enabled == "on",
	}

	return states, nil
}

func (session *TapoControlSession) GetPrivacyMode() bool {
	return session.privacyEnabled
}

// PresetInfo represents a camera PTZ preset
type PresetInfo struct {
	Token string `json:"token"`
	Name  string `json:"name"`
}

// GetPresets retrieves list of presets via ONVIF (port 2020)
func (session *TapoControlSession) GetPresets() ([]PresetInfo, error) {
	url := fmt.Sprintf("http://%s:2020/onvif/ptz_service", session.host)
	created := time.Now().UTC().Format(time.RFC3339)
	nonce := "1234567890123456"
	nonceB64 := base64.StdEncoding.EncodeToString([]byte(nonce))
	h := sha1.New()
	h.Write([]byte(nonce))
	h.Write([]byte(created))
	h.Write([]byte(session.password))
	passDigest := base64.StdEncoding.EncodeToString(h.Sum(nil))

	xmlReq := fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>
<soap:Envelope xmlns:soap="http://www.w3.org/2003/05/soap-envelope" xmlns:tptz="http://www.onvif.org/ver20/ptz/wsdl">
  <soap:Header>
    <Security xmlns="http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-wssecurity-secext-1.0.xsd">
      <UsernameToken>
        <Username>%s</Username>
        <Password Type="http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-username-token-profile-1.0#PasswordDigest">%s</Password>
        <Nonce EncodingType="http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-soap-message-security-1.0#Base64Binary">%s</Nonce>
        <Created xmlns="http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-wssecurity-utility-1.0.xsd">%s</Created>
      </UsernameToken>
    </Security>
  </soap:Header>
  <soap:Body>
    <tptz:GetPresets>
      <tptz:ProfileToken>profile_1</tptz:ProfileToken>
    </tptz:GetPresets>
  </soap:Body>
</soap:Envelope>`, session.user, passDigest, nonceB64, created)

	req, err := http.NewRequest("POST", url, bytes.NewBufferString(xmlReq))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/soap+xml; charset=utf-8; action=\"http://www.onvif.org/ver20/ptz/wsdl/GetPresets\"")
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	bodyStr := string(bodyBytes)

	var presets []PresetInfo
	// Parse preset tokens and names using regex
	re := regexp.MustCompile(`<[^:]*:Preset token="([^"]+)">.*?<[^:]*:Name>([^<]+)</[^:]*:Name>`)
	matches := re.FindAllStringSubmatch(bodyStr, -1)
	for _, m := range matches {
		if len(m) >= 3 {
			presets = append(presets, PresetInfo{
				Token: m[1],
				Name:  m[2],
			})
		}
	}
	return presets, nil
}

// GotoPreset moves the camera to a specific preset by token or name via ONVIF
func (session *TapoControlSession) GotoPreset(tokenOrName string) error {
	token := tokenOrName
	// If name passed (e.g. "sofa", "bàn ăn", "cửa chính"), map to token
	lower := strings.ToLower(tokenOrName)
	if lower == "sofa" {
		token = "1"
	} else if lower == "bàn ăn" || lower == "ban an" {
		token = "2"
	} else if lower == "cửa chính" || lower == "cua chinh" {
		token = "3"
	}

	url := fmt.Sprintf("http://%s:2020/onvif/ptz_service", session.host)
	created := time.Now().UTC().Format(time.RFC3339)
	nonce := "1234567890123456"
	nonceB64 := base64.StdEncoding.EncodeToString([]byte(nonce))
	h := sha1.New()
	h.Write([]byte(nonce))
	h.Write([]byte(created))
	h.Write([]byte(session.password))
	passDigest := base64.StdEncoding.EncodeToString(h.Sum(nil))

	xmlReq := fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>
<soap:Envelope xmlns:soap="http://www.w3.org/2003/05/soap-envelope" xmlns:tptz="http://www.onvif.org/ver20/ptz/wsdl">
  <soap:Header>
    <Security xmlns="http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-wssecurity-secext-1.0.xsd">
      <UsernameToken>
        <Username>%s</Username>
        <Password Type="http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-username-token-profile-1.0#PasswordDigest">%s</Password>
        <Nonce EncodingType="http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-soap-message-security-1.0#Base64Binary">%s</Nonce>
        <Created xmlns="http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-wssecurity-utility-1.0.xsd">%s</Created>
      </UsernameToken>
    </Security>
  </soap:Header>
  <soap:Body>
    <tptz:GotoPreset>
      <tptz:ProfileToken>profile_1</tptz:ProfileToken>
      <tptz:PresetToken>%s</tptz:PresetToken>
    </tptz:GotoPreset>
  </soap:Body>
</soap:Envelope>`, session.user, passDigest, nonceB64, created, token)

	req, err := http.NewRequest("POST", url, bytes.NewBufferString(xmlReq))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/soap+xml; charset=utf-8; action=\"http://www.onvif.org/ver20/ptz/wsdl/GotoPreset\"")
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("ONVIF error %s: %s", resp.Status, string(b))
	}
	return nil
}
