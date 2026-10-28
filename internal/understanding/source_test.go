package understanding

import (
	"context"
	"testing"

	"github.com/nchungdev/home-sentinel/internal/contracts"
)

func TestLibraryQuestionsTargetLocalLibrary(t *testing.T) {
	engine := NewEngine()
	cases := []string{
		"trên máy có phim nào k?",
		"trong thư viện có phim gì",
		"có phim gì xem không",
		"còn phim nào chưa xem",
		"có phim nào trên nas",
	}

	for _, text := range cases {
		intent, err := engine.Parse(context.Background(),
			&contracts.InputEvent{RequestID: "r", SessionID: "s", Text: text}, nil)
		if err != nil {
			t.Fatalf("parse %q: %v", text, err)
		}
		if intent.Name != IntentMediaSearch || intent.ExplicitSource != SourcePlex {
			t.Errorf("%q: got %s source=%q, want media.search from plex",
				text, intent.Name, intent.ExplicitSource)
		}
	}
}

func TestOtherQuestionsStayGeneralChat(t *testing.T) {
	engine := NewEngine()
	for _, text := range []string{"có gì mới không", "máy lạnh có tốn điện không"} {
		intent, err := engine.Parse(context.Background(),
			&contracts.InputEvent{RequestID: "r", SessionID: "s", Text: text}, nil)
		if err != nil {
			t.Fatalf("parse %q: %v", text, err)
		}
		if intent.Name == IntentMediaSearch {
			t.Errorf("%q should not be a media search", text)
		}
	}
}

func TestAirConditionerStatusIsNotSystemStatus(t *testing.T) {
	intent, err := NewEngine().Parse(context.Background(),
		&contracts.InputEvent{RequestID: "r", SessionID: "s", Text: "trạng thái máy lạnh"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if intent.Name == IntentSystemStatus {
		t.Fatal("air conditioner question was routed to NAS status")
	}
}

func TestTVTargetIsNotPartOfTheQuery(t *testing.T) {
	intent, err := NewEngine().Parse(context.Background(),
		&contracts.InputEvent{RequestID: "r", SessionID: "s", Text: "mở phim Naruto trên TV"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if intent.Slots[SlotQuery] != "Naruto" {
		t.Fatalf("query = %q", intent.Slots[SlotQuery])
	}
}
