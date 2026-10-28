package haclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

type EntityState struct {
	EntityID   string                 `json:"entity_id"`
	State      string                 `json:"state"`
	Attributes map[string]interface{} `json:"attributes"`
}

type Client struct {
	BaseURL    string
	Token      string
	HTTPClient *http.Client
}

func (c *Client) UpdateConfig(baseURL, token string) {
	if baseURL != "" {
		c.BaseURL = strings.TrimRight(baseURL, "/")
	}
	if token != "" {
		c.Token = token
	}
}

func (c *Client) TestConnection() (bool, string) {
	if c.BaseURL == "" {
		return false, "Chưa nhập URL Home Assistant"
	}
	if c.Token == "" {
		return false, "Chưa nhập Long-Lived Access Token"
	}
	var res map[string]interface{}
	err := c.RequestJSON("/api/", nil, &res)
	if err != nil {
		return false, fmt.Sprintf("Không kết nối được: %v", err)
	}
	if msg, ok := res["message"].(string); ok && msg == "API running." {
		return true, "Kết nối Home Assistant thành công (API running)"
	}
	return true, "Kết nối Home Assistant thành công"
}

func NewClient(baseURL, token string) *Client {
	if baseURL == "" {
		baseURL = os.Getenv("AGENT_HA_URL")
	}
	if token == "" {
		token = os.Getenv("AGENT_HA_TOKEN")
	}
	return &Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		Token:   token,
		HTTPClient: &http.Client{
			Timeout: 8 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

func Normalize(value string) string {
	value = strings.ToLower(value)
	value = strings.ReplaceAll(value, "đ", "d")
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	result, _, _ := transform.String(t, value)
	return strings.TrimSpace(result)
}

func (c *Client) RequestJSON(path string, body interface{}, out interface{}) error {
	var bodyReader *bytes.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		bodyReader = bytes.NewReader(data)
	} else {
		bodyReader = bytes.NewReader([]byte{})
	}

	method := http.MethodGet
	if body != nil {
		method = http.MethodPost
	}

	req, err := http.NewRequest(method, c.BaseURL+path, bodyReader)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP error %d", resp.StatusCode)
	}

	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}

func (c *Client) GetStates() ([]EntityState, error) {
	var states []EntityState
	err := c.RequestJSON("/api/states", nil, &states)
	return states, err
}

func (c *Client) HandleCommand(text string) (string, bool) {
	clean := Normalize(text)

	cmdRegex := regexp.MustCompile(`^(bat|tat|mo|dong|dung)\s+(.+)`)
	tempRegex := regexp.MustCompile(`^(?:dat|chinh)\s+(.+?)\s+(\d+(?:\.\d+)?)\s*(?:do|°c)?$`)

	cmdMatch := cmdRegex.FindStringSubmatch(clean)
	tempMatch := tempRegex.FindStringSubmatch(clean)

	if cmdMatch == nil && tempMatch == nil {
		return "", false
	}

	var verb, target string
	if cmdMatch != nil {
		verb = cmdMatch[1]
		target = cmdMatch[2]
	} else {
		verb = "temperature"
		target = tempMatch[1]
	}

	states, err := c.GetStates()
	if err != nil {
		return "Không kết nối được tới Home Assistant.", true
	}

	domains := map[string]bool{
		"light": true, "switch": true, "fan": true, "climate": true, "cover": true,
	}

	var candidates []EntityState
	for _, s := range states {
		parts := strings.Split(s.EntityID, ".")
		if len(parts) == 0 || !domains[parts[0]] {
			continue
		}

		friendlyName := s.EntityID
		if fn, ok := s.Attributes["friendly_name"].(string); ok {
			friendlyName = fn
		}

		normEntity := Normalize(s.EntityID)
		normLabel := Normalize(friendlyName)

		if target == normEntity || target == normLabel {
			candidates = []EntityState{s}
			break
		}
		if strings.Contains(normLabel, target) {
			candidates = append(candidates, s)
		}
	}

	if len(candidates) == 0 {
		return "Không tìm thấy thiết bị HA khớp tên này.", true
	}
	if len(candidates) > 1 {
		var names []string
		for i, s := range candidates {
			if i >= 8 {
				break
			}
			name := s.EntityID
			if fn, ok := s.Attributes["friendly_name"].(string); ok {
				name = fn
			}
			names = append(names, name)
		}
		return "Có nhiều thiết bị khớp. Hãy nói tên đầy đủ: " + strings.Join(names, "; "), true
	}

	state := candidates[0]
	entity := state.EntityID
	if state.State == "unavailable" || state.State == "unknown" {
		return "Thiết bị hiện không khả dụng, chưa gửi lệnh.", true
	}

	blocked := os.Getenv("AGENT_HA_PROTECTED_ENTITIES")
	if blocked != "" {
		for _, b := range strings.Split(blocked, ",") {
			if strings.TrimSpace(b) == entity {
				return "Thiết bị nguồn trọng yếu được bảo vệ, Agent không điều khiển mục này.", true
			}
		}
	}

	domain := strings.Split(entity, ".")[0]
	data := map[string]interface{}{"entity_id": entity}

	var service string
	if tempMatch != nil {
		if domain != "climate" {
			return "Chỉ điều hòa hỗ trợ lệnh đặt nhiệt độ này.", true
		}
		val, _ := strconv.ParseFloat(tempMatch[2], 64)
		data["temperature"] = val
		service = "set_temperature"
	} else if domain == "cover" {
		switch verb {
		case "mo", "bat":
			service = "open_cover"
		case "dong", "tat":
			service = "close_cover"
		case "dung":
			service = "stop_cover"
		}
	} else if verb == "tat" {
		service = "turn_off"
	} else if verb == "bat" || verb == "mo" {
		service = "turn_on"
	} else {
		return "Lệnh này không phù hợp với thiết bị.", true
	}

	_ = c.RequestJSON(fmt.Sprintf("/api/services/%s/%s", domain, service), data, nil)

	var updated EntityState
	_ = c.RequestJSON("/api/states/"+entity, nil, &updated)

	name := entity
	if fn, ok := state.Attributes["friendly_name"].(string); ok {
		name = fn
	}

	return fmt.Sprintf("HA đã nhận lệnh cho %s. Trạng thái đọc lại: %s.", name, updated.State), true
}
