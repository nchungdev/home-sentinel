package camstream

import (
	"bytes"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

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
