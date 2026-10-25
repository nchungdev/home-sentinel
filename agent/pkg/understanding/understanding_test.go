package understanding

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/nchungdev/home-sentinel/agent/pkg/contracts"
)

func TestParseMediaSearchPhoneticAndKind(t *testing.T) {
	engine := NewEngine()
	ctx := context.Background()
	event := &contracts.InputEvent{
		RequestID: "req-media-1",
		SessionID: "sess-1",
		Text:      "mở chương trình anh trai c2",
		Locale:    "vi-VN",
		InputType: "voice",
	}

	intent, err := engine.Parse(ctx, event, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if intent.Name != IntentMediaSearch {
		t.Fatalf("expected intent %s, got %s", IntentMediaSearch, intent.Name)
	}
	if intent.Domain != DomainMedia {
		t.Fatalf("expected domain %s, got %s", DomainMedia, intent.Domain)
	}
	if intent.Slots[SlotKind] != "chương trình" {
		t.Fatalf("expected kind 'chương trình', got %v", intent.Slots[SlotKind])
	}
	if intent.Slots[SlotQuery] != "Anh Trai Say Hi" {
		t.Fatalf("expected query 'Anh Trai Say Hi', got %v", intent.Slots[SlotQuery])
	}
	if intent.ExplicitSource != "" {
		t.Fatalf("expected empty explicit source, got %s", intent.ExplicitSource)
	}
}

func TestParseMediaSearchPhoneticAndExplicitSource(t *testing.T) {
	engine := NewEngine()
	ctx := context.Background()
	event := &contracts.InputEvent{
		RequestID: "req-media-2",
		SessionID: "sess-1",
		Text:      "tìm phim a ven dơ en gêm trên youtube",
		Locale:    "vi-VN",
		InputType: "voice",
	}

	intent, err := engine.Parse(ctx, event, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if intent.Name != IntentMediaSearch {
		t.Fatalf("expected intent %s, got %s", IntentMediaSearch, intent.Name)
	}
	if intent.ExplicitSource != SourceYouTube {
		t.Fatalf("expected explicit source %s, got %s", SourceYouTube, intent.ExplicitSource)
	}
	if intent.Slots[SlotKind] != "phim" {
		t.Fatalf("expected kind 'phim', got %v", intent.Slots[SlotKind])
	}
	if intent.Slots[SlotQuery] != "Avengers Endgame" {
		t.Fatalf("expected query 'Avengers Endgame', got %v", intent.Slots[SlotQuery])
	}
}

func TestPhoneticDictionaryVariants(t *testing.T) {
	engine := NewEngine()
	ctx := context.Background()

	testCases := []struct {
		input         string
		expectedQuery string
		expectedSrc   string
	}{
		{
			input:         "tìm phim anh trai say hai bằng youtube",
			expectedQuery: "Anh Trai Say Hi",
			expectedSrc:   SourceYouTube,
		},
		{
			input:         "phát video anh trai xe hai trên youtube",
			expectedQuery: "Anh Trai Say Hi",
			expectedSrc:   SourceYouTube,
		},
		{
			input:         "chiếu phim ơ ven gơ end gêm từ nas",
			expectedQuery: "Avengers Endgame",
			expectedSrc:   SourcePlex,
		},
		{
			input:         "xem phim aven gơ trên plex",
			expectedQuery: "Avengers",
			expectedSrc:   SourcePlex,
		},
		{
			input:         "tìm phim úp pen hay mơ",
			expectedQuery: "Oppenheimer",
			expectedSrc:   "",
		},
		{
			input:         "mở phim ốp pen hai mơ",
			expectedQuery: "Oppenheimer",
			expectedSrc:   "",
		},
		{
			input:         "tìm phim ai dơn men trên youtube",
			expectedQuery: "Iron Man",
			expectedSrc:   SourceYouTube,
		},
		{
			input:         "chiếu phim xì pai đơ men từ nas",
			expectedQuery: "Spider-Man",
			expectedSrc:   SourcePlex,
		},
		{
			input:         "mở phim đa gôn bôn",
			expectedQuery: "Dragon Ball",
			expectedSrc:   "",
		},
	}

	for _, testCase := range testCases {
		event := &contracts.InputEvent{
			RequestID: "req-test-variant",
			SessionID: "sess-1",
			Text:      testCase.input,
		}

		intent, err := engine.Parse(ctx, event, nil)
		if err != nil {
			t.Fatalf("error for input %q: %v", testCase.input, err)
		}

		if intent.Slots[SlotQuery] != testCase.expectedQuery {
			t.Fatalf(
				"input %q expected query %q, got %q",
				testCase.input,
				testCase.expectedQuery,
				intent.Slots[SlotQuery],
			)
		}

		if intent.ExplicitSource != testCase.expectedSrc {
			t.Fatalf(
				"input %q expected source %q, got %q",
				testCase.input,
				testCase.expectedSrc,
				intent.ExplicitSource,
			)
		}
	}
}

func TestParseHomeAssistantCommands(t *testing.T) {
	engine := NewEngine()
	ctx := context.Background()

	eventLight := &contracts.InputEvent{
		RequestID: "req-ha-1",
		SessionID: "sess-1",
		Text:      "bật đèn phòng khách",
	}

	intentLight, err := engine.Parse(ctx, eventLight, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if intentLight.Name != IntentHomeControl {
		t.Fatalf("expected intent %s, got %s", IntentHomeControl, intentLight.Name)
	}
	if intentLight.Domain != DomainHome {
		t.Fatalf("expected domain %s, got %s", DomainHome, intentLight.Domain)
	}
	if intentLight.Slots[SlotAction] != ActionTurnOn {
		t.Fatalf("expected action turn_on, got %v", intentLight.Slots[SlotAction])
	}
	if intentLight.Slots[SlotTargetLabel] != "đèn phòng khách" {
		t.Fatalf(
			"expected target_label 'đèn phòng khách', got %v",
			intentLight.Slots[SlotTargetLabel],
		)
	}

	eventClimate := &contracts.InputEvent{
		RequestID: "req-ha-2",
		SessionID: "sess-1",
		Text:      "chỉnh điều hòa 24 độ",
	}

	intentClimate, err := engine.Parse(ctx, eventClimate, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if intentClimate.Name != IntentHomeControl {
		t.Fatalf("expected intent %s, got %s", IntentHomeControl, intentClimate.Name)
	}
	if intentClimate.Slots[SlotAction] != ActionSetTemperature {
		t.Fatalf(
			"expected action set_temperature, got %v",
			intentClimate.Slots[SlotAction],
		)
	}
	if intentClimate.Slots[SlotTargetLabel] != "điều hòa" {
		t.Fatalf(
			"expected target_label 'điều hòa', got %v",
			intentClimate.Slots[SlotTargetLabel],
		)
	}
	if intentClimate.Slots[SlotTemperature] != float64(24) {
		t.Fatalf(
			"expected temperature 24, got %v",
			intentClimate.Slots[SlotTemperature],
		)
	}

	eventCurtain := &contracts.InputEvent{
		RequestID: "req-ha-3",
		SessionID: "sess-1",
		Text:      "đóng rèm phòng ngủ",
	}

	intentCurtain, err := engine.Parse(ctx, eventCurtain, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if intentCurtain.Slots[SlotAction] != ActionClose {
		t.Fatalf("expected action close, got %v", intentCurtain.Slots[SlotAction])
	}
}

func TestParseCameraCommands(t *testing.T) {
	engine := NewEngine()
	ctx := context.Background()

	directionCases := []struct {
		input             string
		expectedDirection string
	}{
		{"quay camera sang trái", DirectionLeft},
		{"quay camera sang phải", DirectionRight},
		{"quay camera lên", DirectionUp},
		{"quay camera xuống", DirectionDown},
	}

	for _, testCase := range directionCases {
		event := &contracts.InputEvent{
			RequestID: "req-cam-dir",
			SessionID: "sess-1",
			Text:      testCase.input,
		}

		intent, err := engine.Parse(ctx, event, nil)
		if err != nil {
			t.Fatalf("error parsing %q: %v", testCase.input, err)
		}
		if intent.Name != IntentCameraControl {
			t.Fatalf("expected camera.control for %q, got %s", testCase.input, intent.Name)
		}
		if intent.Slots[SlotDirection] != testCase.expectedDirection {
			t.Fatalf(
				"expected direction %s for %q, got %v",
				testCase.expectedDirection,
				testCase.input,
				intent.Slots[SlotDirection],
			)
		}
	}

	eventPrivacyOn := &contracts.InputEvent{
		RequestID: "req-cam-priv-1",
		SessionID: "sess-1",
		Text:      "bật chế độ riêng tư",
	}
	intentPrivacyOn, err := engine.Parse(ctx, eventPrivacyOn, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if intentPrivacyOn.Slots[SlotPrivacy] != PrivacyOn {
		t.Fatalf("expected privacy on, got %v", intentPrivacyOn.Slots[SlotPrivacy])
	}

	eventPrivacyOff := &contracts.InputEvent{
		RequestID: "req-cam-priv-2",
		SessionID: "sess-1",
		Text:      "tắt privacy camera",
	}
	intentPrivacyOff, err := engine.Parse(ctx, eventPrivacyOff, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if intentPrivacyOff.Slots[SlotPrivacy] != PrivacyOff {
		t.Fatalf("expected privacy off, got %v", intentPrivacyOff.Slots[SlotPrivacy])
	}

	eventSpeak := &contracts.InputEvent{
		RequestID: "req-cam-spk",
		SessionID: "sess-1",
		Text:      "nói qua camera: chào cả nhà",
	}
	intentSpeak, err := engine.Parse(ctx, eventSpeak, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if intentSpeak.Slots[SlotSpeak] != "chào cả nhà" {
		t.Fatalf("expected speak 'chào cả nhà', got %v", intentSpeak.Slots[SlotSpeak])
	}
}

func TestParseSystemAndNASCommands(t *testing.T) {
	engine := NewEngine()
	ctx := context.Background()

	testCases := []struct {
		input          string
		expectedTarget string
	}{
		{"kiểm tra trạng thái nas", "nas"},
		{"thông tin nas", "nas"},
		{"dung lượng ổ cứng nas", "nas"},
		{"trạng thái docker", "docker"},
		{"trạng thái hệ thống", "system"},
	}

	for _, testCase := range testCases {
		event := &contracts.InputEvent{
			RequestID: "req-sys",
			SessionID: "sess-1",
			Text:      testCase.input,
		}

		intent, err := engine.Parse(ctx, event, nil)
		if err != nil {
			t.Fatalf("error parsing %q: %v", testCase.input, err)
		}
		if intent.Name != IntentSystemStatus {
			t.Fatalf("expected system.status for %q, got %s", testCase.input, intent.Name)
		}
		if intent.Domain != DomainSystem {
			t.Fatalf("expected domain system for %q, got %s", testCase.input, intent.Domain)
		}
		if intent.Slots[SlotTarget] != testCase.expectedTarget {
			t.Fatalf(
				"expected target %s for %q, got %v",
				testCase.expectedTarget,
				testCase.input,
				intent.Slots[SlotTarget],
			)
		}
	}
}

func TestParseGeneralChat(t *testing.T) {
	engine := NewEngine()
	ctx := context.Background()

	event := &contracts.InputEvent{
		RequestID: "req-chat",
		SessionID: "sess-1",
		Text:      "thời tiết hôm nay thế nào",
	}

	intent, err := engine.Parse(ctx, event, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if intent.Name != IntentGeneralChat {
		t.Fatalf("expected general.chat, got %s", intent.Name)
	}
	if intent.Domain != DomainGeneral {
		t.Fatalf("expected domain general, got %s", intent.Domain)
	}
	if intent.Slots[SlotText] != "thời tiết hôm nay thế nào" {
		t.Fatalf(
			"expected text 'thời tiết hôm nay thế nào', got %v",
			intent.Slots[SlotText],
		)
	}
}

func TestParseContextCancellationAndTimeout(t *testing.T) {
	engine := NewEngine()
	event := &contracts.InputEvent{
		RequestID: "req-timeout",
		SessionID: "sess-1",
		Text:      "mở chương trình anh trai c2",
	}

	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()

	intent, err := engine.Parse(canceledCtx, event, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if intent != nil {
		t.Fatalf("expected nil intent on cancellation, got %v", intent)
	}

	timeoutCtx, cancelTimeout := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancelTimeout()
	time.Sleep(1 * time.Millisecond)

	intentTimeout, errTimeout := engine.Parse(timeoutCtx, event, nil)
	if !errors.Is(errTimeout, context.DeadlineExceeded) {
		t.Fatalf("expected context.DeadlineExceeded, got %v", errTimeout)
	}
	if intentTimeout != nil {
		t.Fatalf("expected nil intent on timeout, got %v", intentTimeout)
	}
}

func TestParseInvalidInputs(t *testing.T) {
	engine := NewEngine()
	ctx := context.Background()

	intentNil, errNil := engine.Parse(ctx, nil, nil)
	if !errors.Is(errNil, contracts.ErrInvalidInputEvent) {
		t.Fatalf("expected ErrInvalidInputEvent, got %v", errNil)
	}
	if intentNil != nil {
		t.Fatalf("expected nil intent, got %v", intentNil)
	}

	emptyEvent := &contracts.InputEvent{
		RequestID: "req-empty",
		Text:      "   ",
	}
	_, errEmpty := engine.Parse(ctx, emptyEvent, nil)
	if !errors.Is(errEmpty, contracts.ErrInvalidInputEvent) {
		t.Fatalf("expected ErrInvalidInputEvent for empty text, got %v", errEmpty)
	}
}
