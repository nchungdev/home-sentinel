package tools

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sync"
	"time"
)

const (
	confirmationField = "confirmation_id"
	confirmationTTL   = 5 * time.Minute
)

var (
	errConfirmationInvalid = errors.New("mã xác nhận không hợp lệ hoặc đã hết hạn")
	errNotConfirmedByUser  = errors.New("người dùng chưa đồng ý ở lượt nói ngay sau")

	affirmativePattern = regexp.MustCompile(`(?i)^\s*(ok(ay)?|oke|ừ|ừm|uh|ờ|vâng|dạ|có|đúng|` +
		`đồng\s+ý|được|đc|làm\s+đi|thêm\s+đi|chắc\s+chắn|yes|yep|sure)(?:$|[^\p{L}\p{N}])`)
	negativePattern = regexp.MustCompile(`(?i)(?:^|[^\p{L}\p{N}])` +
		`(không|ko|khỏi|đừng|thôi|hủy|huỷ|no)(?:$|[^\p{L}\p{N}])`)
)

type pendingConfirmation struct {
	tool       string
	argsKey    string
	issuedTurn int
	expires    time.Time
}

type conversationTurn struct {
	turn     int
	userText string
}

type confirmationStore struct {
	mutex   sync.Mutex
	turns   map[string]conversationTurn
	pending map[string]map[string]pendingConfirmation
	now     func() time.Time
}

func newConfirmationStore() *confirmationStore {
	return &confirmationStore{
		turns:   make(map[string]conversationTurn),
		pending: make(map[string]map[string]pendingConfirmation),
		now:     time.Now,
	}
}

func (s *confirmationStore) beginTurn(conversationID, userText string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	current := s.turns[conversationID]
	s.turns[conversationID] = conversationTurn{turn: current.turn + 1, userText: userText}
}

func (s *confirmationStore) request(conversationID string, tool Tool, args map[string]any) string {
	id := randomID()
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if s.pending[conversationID] == nil {
		s.pending[conversationID] = make(map[string]pendingConfirmation)
	}
	s.pending[conversationID][id] = pendingConfirmation{
		tool: tool.Name, argsKey: argsKey(args),
		issuedTurn: s.turns[conversationID].turn, expires: s.now().Add(confirmationTTL),
	}

	summary := tool.Name
	if tool.Summarize != nil {
		summary = tool.Summarize(args)
	}
	return fmt.Sprintf("CẦN XÁC NHẬN: %s. Hãy hỏi người dùng có đồng ý không rồi dừng lại. "+
		"Chỉ khi người dùng đồng ý ở lượt nói tiếp theo mới gọi lại công cụ này với "+
		"cùng tham số và %s=%q.", summary, confirmationField, id)
}

func (s *confirmationStore) consume(conversationID, id, toolName string, args map[string]any) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	pending, found := s.pending[conversationID][id]
	if !found || pending.tool != toolName || pending.argsKey != argsKey(args) ||
		s.now().After(pending.expires) {
		return errConfirmationInvalid
	}

	current := s.turns[conversationID]
	if current.turn != pending.issuedTurn+1 || !affirmative(current.userText) {
		return errNotConfirmedByUser
	}
	delete(s.pending[conversationID], id)
	return nil
}

func affirmative(text string) bool {
	return affirmativePattern.MatchString(text) && !negativePattern.MatchString(text)
}

func argsKey(args map[string]any) string {
	encoded, _ := json.Marshal(args)
	return string(encoded)
}

func randomID() string {
	raw := make([]byte, 8)
	_, _ = rand.Read(raw)
	return hex.EncodeToString(raw)
}
