package camstream

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
)

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
