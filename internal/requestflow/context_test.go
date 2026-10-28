package requestflow

import (
	"context"
	"testing"
)

func TestExplicitSourceRestrictsPlanAndNormalizesTitle(t *testing.T) {
	understanding, _ := (RuleClassifier{}).Understand(context.Background(), Input{
		Text: "Tìm phim Interstellar trên Plex thôi",
	})
	plan := BuildPlan(understanding)
	if len(plan.Tasks) != 1 || plan.Tasks[0].Source != "plex" ||
		plan.Tasks[0].Query != "Interstellar" {
		t.Fatalf("ignored source or sent user command as title: %+v", plan)
	}
}

func TestSelectedTitleAndAllowedSourcesArePreserved(t *testing.T) {
	understanding, _ := (RuleClassifier{}).Understand(context.Background(), Input{
		Text: "Thông tin phim này", Context: InputContext{Title: "Dune", Sources: []string{"tmdb"}},
	})
	plan := BuildPlan(understanding)
	if len(plan.Tasks) != 1 || plan.Tasks[0].Source != "tmdb" || plan.Tasks[0].Query != "Dune" {
		t.Fatalf("lost selected title: %+v", plan)
	}
	if ValidContext(InputContext{Sources: []string{"unknown"}}) {
		t.Fatal("invalid source selection can silently search all sources")
	}
}

func TestTechnicalCameraQuestionIsNotControl(t *testing.T) {
	understanding, _ := (RuleClassifier{}).Understand(context.Background(), Input{
		Text: "Tìm cách sửa lỗi camera trên Windows",
	})
	if understanding.Domain != "technical" || understanding.Intent != "lookup" {
		t.Fatalf("information question became action: %+v", understanding)
	}
}

func TestMetadataTitleNormalization(t *testing.T) {
	for _, text := range []string{
		"Ai đạo diễn Interstellar?", "Tìm thông tin về phim Interstellar",
	} {
		understanding, _ := (RuleClassifier{}).Understand(context.Background(), Input{Text: text})
		if understanding.Entities["title"] != "Interstellar" {
			t.Fatalf("bad metadata query for %s: %+v", text, understanding)
		}
	}
}

func TestMissingSelectedTitleAsksInsteadOfGuessing(t *testing.T) {
	understanding, _ := (RuleClassifier{}).Understand(context.Background(), Input{
		Text: "Thông tin phim này",
	})
	if !understanding.NeedsInput || len(BuildPlan(understanding).Tasks) != 0 {
		t.Fatalf("missing title silently searched literal pronoun: %+v", understanding)
	}
}

func TestExplicitPlaybackIsNotDowngradedToReadOnlySearch(t *testing.T) {
	understanding, _ := (RuleClassifier{}).Understand(context.Background(), Input{
		Text: "Mở phim Interstellar trên TV",
	})
	if understanding.Intent != "play" {
		t.Fatalf("play intent became search: %+v", understanding)
	}
}
