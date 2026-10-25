package transport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/nchungdev/home-sentinel/agent/pkg/contracts"
	"github.com/nchungdev/home-sentinel/agent/pkg/orchestrator"
)

type LegacyAskRequest struct {
	Text    string `json:"text"`
	Session string `json:"session"`
}

type LegacyAdapter struct {
	config       Config
	orchestrator *orchestrator.Orchestrator
}

func NewLegacyAdapter(config Config, orch *orchestrator.Orchestrator) *LegacyAdapter {
	if config.DefaultTimeout <= 0 {
		config.DefaultTimeout = DefaultTimeoutDuration
	}
	if config.MaxBodyBytes <= 0 {
		config.MaxBodyBytes = DefaultMaxBodyBytes
	}

	return &LegacyAdapter{
		config:       config,
		orchestrator: orch,
	}
}

func (a *LegacyAdapter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !IsIPAllowed(r.RemoteAddr, a.config.AllowedIPs) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	if a.config.Token != "" {
		authHeader := r.Header.Get(HeaderAuthorization)
		if !VerifyBearerToken(authHeader, a.config.Token) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
	}

	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	reqCtx, ctx, cancel := ExtractRequestContext(r, a.config.DefaultTimeout)
	defer cancel()

	limitedReader := http.MaxBytesReader(w, r.Body, a.config.MaxBodyBytes)
	var askReq LegacyAskRequest
	if err := json.NewDecoder(limitedReader).Decode(&askReq); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}

	trimmedText := strings.TrimSpace(askReq.Text)
	if trimmedText == "" {
		http.Error(w, "empty question", http.StatusBadRequest)
		return
	}

	requestID := fmt.Sprintf("req_%d", time.Now().UnixNano())
	inputEvent := &contracts.InputEvent{
		RequestID: requestID,
		SessionID: strings.TrimSpace(askReq.Session),
		Text:      trimmedText,
		Locale:    "vi-VN",
		InputType: "voice",
	}

	response, err := a.orchestrator.Process(ctx, inputEvent, reqCtx)
	if err != nil {
		if errors.Is(ctx.Err(), context.Canceled) ||
			errors.Is(r.Context().Err(), context.Canceled) {
			http.Error(w, "client disconnected", 499)
			return
		}
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			http.Error(w, "request timeout", http.StatusGatewayTimeout)
			return
		}

		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set(HeaderContentType, "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)

	_ = StreamPresentationResponse(w, response)
}

func StreamPresentationResponse(w io.Writer, response *contracts.PresentationResponse) error {
	if response == nil {
		return nil
	}

	flusher, canFlush := w.(http.Flusher)
	speech := strings.TrimSpace(response.SpeechText)

	if speech != "" {
		chunks := splitStreamChunks(speech)
		for _, chunk := range chunks {
			if _, err := w.Write([]byte(chunk)); err != nil {
				return err
			}
			if canFlush {
				flusher.Flush()
			}
		}
	}

	if len(response.Cards) > 0 {
		cardsHTML := RenderCardsHTML(response.Cards)
		if cardsHTML != "" {
			separator := ""
			if speech != "" {
				separator = "\n\n"
			}
			if _, err := w.Write([]byte(separator + cardsHTML)); err != nil {
				return err
			}
			if canFlush {
				flusher.Flush()
			}
		}
	}

	return nil
}

func FormatLegacyResponse(response *contracts.PresentationResponse) string {
	if response == nil {
		return ""
	}

	speech := strings.TrimSpace(response.SpeechText)
	cardsHTML := RenderCardsHTML(response.Cards)

	if speech != "" && cardsHTML != "" {
		return speech + "\n\n" + cardsHTML
	}
	if speech != "" {
		return speech
	}
	return cardsHTML
}

func RenderCardHTML(card map[string]any) string {
	if rawHTML, ok := card["raw_html"].(string); ok && rawHTML != "" {
		return rawHTML
	}
	if html, ok := card["html"].(string); ok && html != "" {
		return html
	}

	title := getCardString(card, "title", "name")
	status := getCardString(card, "status", "state")
	itemType := getCardString(card, "type")
	year := getCardString(card, "year")
	link := getCardString(card, "link", "deep_link")
	poster := getCardString(card, "poster", "thumb")
	subText := getCardString(card, "subtitle", "summary")
	if subText == "" && status != "" {
		subText = status
	}

	posterHTML := `<div class="poster"></div>`
	if poster != "" {
		cleanPoster := strings.ReplaceAll(poster, `"`, `&quot;`)
		posterHTML = fmt.Sprintf(`<img class="poster" src="%s" alt="%s">`, cleanPoster, title)
	}

	cleanTitle := strings.ReplaceAll(title, `"`, `&quot;`)

	return fmt.Sprintf(
		`<div class="card" data-status="%s" data-title="%s" data-type="%s" `+
			`data-year="%s" data-link="%s">%s<div class="meta">`+
			`<h3 class="meta-title">%s</h3><div class="meta-sub">%s</div>`+
			`</div></div>`,
		status, cleanTitle, itemType, year, link, posterHTML, title, subText,
	)
}

func RenderCardsHTML(cards []map[string]any) string {
	if len(cards) == 0 {
		return ""
	}

	var builder strings.Builder
	builder.WriteString(`<div class="grid">`)
	for _, card := range cards {
		builder.WriteString(RenderCardHTML(card))
	}
	builder.WriteString(`</div>`)

	return builder.String()
}

func splitStreamChunks(text string) []string {
	words := strings.Fields(text)
	if len(words) <= 4 {
		return []string{text}
	}

	var chunks []string
	var current strings.Builder
	for wordIndex, word := range words {
		if current.Len() > 0 {
			current.WriteString(" ")
		}
		current.WriteString(word)

		if (wordIndex+1)%4 == 0 || wordIndex == len(words)-1 {
			chunkText := current.String()
			if wordIndex != len(words)-1 {
				chunkText += " "
			}
			chunks = append(chunks, chunkText)
			current.Reset()
		}
	}

	return chunks
}

func getCardString(card map[string]any, keys ...string) string {
	for _, key := range keys {
		if rawValue, exists := card[key]; exists && rawValue != nil {
			switch value := rawValue.(type) {
			case string:
				return value
			case int:
				return strconv.Itoa(value)
			case int64:
				return strconv.FormatInt(value, 10)
			case float64:
				return strconv.FormatFloat(value, 'f', -1, 64)
			default:
				return fmt.Sprintf("%v", value)
			}
		}
	}
	return ""
}
