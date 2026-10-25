package resolution

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/nchungdev/home-sentinel/agent/pkg/contracts"
	"github.com/nchungdev/home-sentinel/agent/pkg/orchestrator"
	"github.com/nchungdev/home-sentinel/agent/pkg/understanding"
)

var _ orchestrator.ResolutionEngine = (*Resolver)(nil)

type mockPlexSearcher struct {
	items       map[string]*PlexMediaItem
	searchDelay time.Duration
	searchErr   error
	calls       int
}

func (m *mockPlexSearcher) SearchFirst(
	ctx context.Context,
	query string,
) (*PlexMediaItem, error) {
	m.calls++
	if m.searchDelay > 0 {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(m.searchDelay):
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if m.searchErr != nil {
		return nil, m.searchErr
	}
	return m.items[query], nil
}

func (m *mockPlexSearcher) GetRecent(
	ctx context.Context,
	limit int,
) ([]*PlexMediaItem, error) {
	if m.searchErr != nil {
		return nil, m.searchErr
	}
	var res []*PlexMediaItem
	for _, it := range m.items {
		res = append(res, it)
		if limit > 0 && len(res) >= limit {
			break
		}
	}
	return res, nil
}

type mockHAEntityResolver struct {
	entities   map[string]*HAEntityMatch
	resolveErr error
	calls      int
}

func (m *mockHAEntityResolver) ResolveEntity(
	ctx context.Context,
	targetLabel string,
	action string,
) (*HAEntityMatch, error) {
	m.calls++
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if m.resolveErr != nil {
		return nil, m.resolveErr
	}
	lookupKey := targetLabel + ":" + action
	if match, exists := m.entities[lookupKey]; exists {
		return match, nil
	}
	return m.entities[targetLabel], nil
}

func TestResolve_Media_ShowToYouTube(t *testing.T) {
	plexMock := &mockPlexSearcher{
		items: map[string]*PlexMediaItem{},
	}
	haMock := &mockHAEntityResolver{
		entities: map[string]*HAEntityMatch{},
	}
	engine := NewResolver(plexMock, haMock)

	intent := &contracts.Intent{
		Name:   understanding.IntentMediaSearch,
		Domain: understanding.DomainMedia,
		Slots: map[string]any{
			understanding.SlotQuery: "Anh Trai Say Hi",
			understanding.SlotKind:  "chương trình",
		},
	}

	plan, err := engine.Resolve(context.Background(), intent, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := plan.Validate(); err != nil {
		t.Fatalf("invalid plan: %v", err)
	}

	if len(plan.Steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(plan.Steps))
	}

	step := plan.Steps[0]
	if step.ActionType != ActionTypeYouTubeLaunch {
		t.Errorf("expected %s, got %s", ActionTypeYouTubeLaunch, step.ActionType)
	}
	if step.RunsOn != RuntimeTVClient {
		t.Errorf("expected %s, got %s", RuntimeTVClient, step.RunsOn)
	}
	if step.Parameters["query"] != "Anh Trai Say Hi" {
		t.Errorf("expected query 'Anh Trai Say Hi', got %v", step.Parameters["query"])
	}
	if step.Parameters["kind"] != "chương trình" {
		t.Errorf("expected kind 'chương trình', got %v", step.Parameters["kind"])
	}
	if plexMock.calls != 0 {
		t.Errorf("expected plex search not to be called, got %d calls", plexMock.calls)
	}
}

func TestResolve_Media_Dune2InPlex(t *testing.T) {
	duneItem := &PlexMediaItem{
		Title:     "Dune: Part Two",
		Year:      2024,
		RatingKey: "plex-12345",
		Type:      "movie",
		DeepLink:  "plex://server/play?key=plex-12345",
	}
	plexMock := &mockPlexSearcher{
		items: map[string]*PlexMediaItem{
			"Dune 2": duneItem,
		},
	}
	engine := NewResolver(plexMock, nil)

	intent := &contracts.Intent{
		Name:   understanding.IntentMediaSearch,
		Domain: understanding.DomainMedia,
		Slots: map[string]any{
			understanding.SlotQuery: "Dune 2",
			understanding.SlotKind:  "phim",
		},
	}

	plan, err := engine.Resolve(context.Background(), intent, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := plan.Validate(); err != nil {
		t.Fatalf("invalid plan: %v", err)
	}

	if len(plan.Steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(plan.Steps))
	}

	step := plan.Steps[0]
	if step.ActionType != ActionTypePlexOpen {
		t.Errorf("expected %s, got %s", ActionTypePlexOpen, step.ActionType)
	}
	if step.RunsOn != RuntimeTVClient {
		t.Errorf("expected %s, got %s", RuntimeTVClient, step.RunsOn)
	}
	if step.TargetID != "plex-12345" {
		t.Errorf("expected target plex-12345, got %s", step.TargetID)
	}
	if step.Parameters["deep_link"] != "plex://server/play?key=plex-12345" {
		t.Errorf("expected deep link, got %v", step.Parameters["deep_link"])
	}
	if found, ok := plan.Evidence["found_in_plex"].(bool); !ok || !found {
		t.Errorf("expected found_in_plex true, got %v", plan.Evidence["found_in_plex"])
	}
}

func TestResolve_Media_MovieNotInPlex_FallbackYouTube(t *testing.T) {
	plexMock := &mockPlexSearcher{
		items: map[string]*PlexMediaItem{},
	}
	engine := NewResolver(plexMock, nil)

	intent := &contracts.Intent{
		Name:   understanding.IntentMediaSearch,
		Domain: understanding.DomainMedia,
		Slots: map[string]any{
			understanding.SlotQuery: "Phim lạ",
			understanding.SlotKind:  "phim lẻ",
		},
	}

	plan, err := engine.Resolve(context.Background(), intent, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := plan.Validate(); err != nil {
		t.Fatalf("invalid plan: %v", err)
	}

	if len(plan.Steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(plan.Steps))
	}

	step := plan.Steps[0]
	if step.ActionType != ActionTypeYouTubeLaunch {
		t.Errorf("expected %s, got %s", ActionTypeYouTubeLaunch, step.ActionType)
	}
	if step.RunsOn != RuntimeTVClient {
		t.Errorf("expected %s, got %s", RuntimeTVClient, step.RunsOn)
	}
	if found, ok := plan.Evidence["found_in_plex"].(bool); !ok || found {
		t.Errorf("expected found_in_plex false, got %v", plan.Evidence["found_in_plex"])
	}
	if fallback, ok := plan.Evidence["fallback_source"].(string); !ok ||
		fallback != understanding.SourceYouTube {
		t.Errorf("expected fallback youtube, got %v", plan.Evidence["fallback_source"])
	}
}

func TestResolve_Media_ExplicitYouTube(t *testing.T) {
	plexMock := &mockPlexSearcher{
		items: map[string]*PlexMediaItem{
			"Dune 2": {
				Title:     "Dune: Part Two",
				RatingKey: "123",
				DeepLink:  "plex://play?key=123",
			},
		},
	}
	engine := NewResolver(plexMock, nil)

	intent := &contracts.Intent{
		Name:           understanding.IntentMediaSearch,
		Domain:         understanding.DomainMedia,
		ExplicitSource: understanding.SourceYouTube,
		Slots: map[string]any{
			understanding.SlotQuery: "Dune 2",
		},
	}

	plan, err := engine.Resolve(context.Background(), intent, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(plan.Steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(plan.Steps))
	}

	step := plan.Steps[0]
	if step.ActionType != ActionTypeYouTubeLaunch {
		t.Errorf("expected %s, got %s", ActionTypeYouTubeLaunch, step.ActionType)
	}
	if plexMock.calls != 0 {
		t.Errorf("expected no plex call when explicit youtube, got %d", plexMock.calls)
	}
}

func TestResolve_Media_ExplicitPlex(t *testing.T) {
	plexMock := &mockPlexSearcher{
		items: map[string]*PlexMediaItem{
			"Dune 2": {
				Title:     "Dune: Part Two",
				RatingKey: "dune-key",
				DeepLink:  "plex://play?key=dune-key",
			},
		},
	}
	engine := NewResolver(plexMock, nil)

	intentFound := &contracts.Intent{
		Name:           understanding.IntentMediaSearch,
		Domain:         understanding.DomainMedia,
		ExplicitSource: understanding.SourcePlex,
		Slots: map[string]any{
			understanding.SlotQuery: "Dune 2",
		},
	}

	planFound, err := engine.Resolve(context.Background(), intentFound, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(planFound.Steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(planFound.Steps))
	}
	if planFound.Steps[0].ActionType != ActionTypePlexOpen {
		t.Errorf("expected %s, got %s", ActionTypePlexOpen, planFound.Steps[0].ActionType)
	}
	if planFound.Steps[0].RunsOn != RuntimeTVClient {
		t.Errorf("expected %s, got %s", RuntimeTVClient, planFound.Steps[0].RunsOn)
	}

	intentNotFound := &contracts.Intent{
		Name:           understanding.IntentMediaSearch,
		Domain:         understanding.DomainMedia,
		ExplicitSource: understanding.SourcePlex,
		Slots: map[string]any{
			understanding.SlotQuery: "Phim không có",
		},
	}

	planNotFound, err := engine.Resolve(context.Background(), intentNotFound, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(planNotFound.Steps) != 0 {
		t.Errorf("expected 0 steps for missing explicit plex media, got %d",
			len(planNotFound.Steps))
	}
	if found, ok := planNotFound.Evidence["found_in_plex"].(bool); !ok || found {
		t.Errorf("expected found_in_plex false, got %v", planNotFound.Evidence["found_in_plex"])
	}
}

func TestResolve_HomeControl_EntityMatched(t *testing.T) {
	haMock := &mockHAEntityResolver{
		entities: map[string]*HAEntityMatch{
			"đèn phòng khách": {
				EntityID: "light.living_room",
				Domain:   "light",
				Service:  "turn_on",
			},
		},
	}
	engine := NewResolver(nil, haMock)

	intent := &contracts.Intent{
		Name:   understanding.IntentHomeControl,
		Domain: understanding.DomainHome,
		Slots: map[string]any{
			understanding.SlotTargetLabel: "đèn phòng khách",
			understanding.SlotAction:      understanding.ActionTurnOn,
		},
	}

	plan, err := engine.Resolve(context.Background(), intent, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := plan.Validate(); err != nil {
		t.Fatalf("invalid plan: %v", err)
	}

	if len(plan.Steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(plan.Steps))
	}

	step := plan.Steps[0]
	if step.ActionType != ActionTypeHAServiceCall {
		t.Errorf("expected %s, got %s", ActionTypeHAServiceCall, step.ActionType)
	}
	if step.TargetID != "light.living_room" {
		t.Errorf("expected target light.living_room, got %s", step.TargetID)
	}
	if step.RunsOn != RuntimeBackend {
		t.Errorf("expected %s, got %s", RuntimeBackend, step.RunsOn)
	}
	if step.Parameters["entity_id"] != "light.living_room" {
		t.Errorf("expected entity_id parameter, got %v", step.Parameters["entity_id"])
	}
}

func TestResolve_HomeControl_EntityNotMatched(t *testing.T) {
	haMock := &mockHAEntityResolver{
		entities: map[string]*HAEntityMatch{},
	}
	engine := NewResolver(nil, haMock)

	intent := &contracts.Intent{
		Name:   understanding.IntentHomeControl,
		Domain: understanding.DomainHome,
		Slots: map[string]any{
			understanding.SlotTargetLabel: "thiết bị không tồn tại",
			understanding.SlotAction:      understanding.ActionTurnOn,
		},
	}

	plan, err := engine.Resolve(context.Background(), intent, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(plan.Steps) != 0 {
		t.Fatalf("expected 0 steps for unmatched entity, got %d", len(plan.Steps))
	}

	if matched, ok := plan.Evidence["matched"].(bool); !ok || matched {
		t.Errorf("expected matched false, got %v", plan.Evidence["matched"])
	}
}

func TestResolve_HomeControl_SetTemperature(t *testing.T) {
	haMock := &mockHAEntityResolver{
		entities: map[string]*HAEntityMatch{
			"máy lạnh": {
				EntityID: "climate.bedroom",
				Domain:   "climate",
				Service:  "set_temperature",
			},
		},
	}
	engine := NewResolver(nil, haMock)

	intent := &contracts.Intent{
		Name:   understanding.IntentHomeControl,
		Domain: understanding.DomainHome,
		Slots: map[string]any{
			understanding.SlotTargetLabel: "máy lạnh",
			understanding.SlotAction:      understanding.ActionSetTemperature,
			understanding.SlotTemperature: 24.5,
		},
	}

	plan, err := engine.Resolve(context.Background(), intent, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(plan.Steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(plan.Steps))
	}

	step := plan.Steps[0]
	if step.Parameters["temperature"] != 24.5 {
		t.Errorf("expected temperature 24.5, got %v", step.Parameters["temperature"])
	}
}

func TestResolve_ContextTimeout(t *testing.T) {
	cancelledContext, cancel := context.WithCancel(context.Background())
	cancel()

	engine := NewResolver(nil, nil)
	intent := &contracts.Intent{
		Name:   understanding.IntentMediaSearch,
		Domain: understanding.DomainMedia,
		Slots: map[string]any{
			understanding.SlotQuery: "Dune",
		},
	}

	_, err := engine.Resolve(cancelledContext, intent, nil)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}

	plexMock := &mockPlexSearcher{
		items:       map[string]*PlexMediaItem{},
		searchDelay: 50 * time.Millisecond,
	}
	slowEngine := NewResolver(plexMock, nil)

	timeoutContext, cancelTimeout := context.WithTimeout(
		context.Background(),
		10*time.Millisecond,
	)
	defer cancelTimeout()

	_, timeoutErr := slowEngine.Resolve(timeoutContext, intent, nil)
	if !errors.Is(timeoutErr, context.DeadlineExceeded) {
		t.Errorf("expected context.DeadlineExceeded, got %v", timeoutErr)
	}
}

func TestResolve_CameraControl(t *testing.T) {
	engine := NewResolver(nil, nil)

	privacyIntent := &contracts.Intent{
		Name:   understanding.IntentCameraControl,
		Domain: understanding.DomainCamera,
		Slots: map[string]any{
			understanding.SlotPrivacy: understanding.PrivacyOn,
		},
	}
	privacyPlan, err := engine.Resolve(context.Background(), privacyIntent, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if privacyPlan.Steps[0].ActionType != ActionTypeCameraPrivacy {
		t.Errorf("expected %s, got %s", ActionTypeCameraPrivacy,
			privacyPlan.Steps[0].ActionType)
	}
	if privacyPlan.Steps[0].RunsOn != RuntimeBackend {
		t.Errorf("expected %s, got %s", RuntimeBackend,
			privacyPlan.Steps[0].RunsOn)
	}

	ptzIntent := &contracts.Intent{
		Name:   understanding.IntentCameraControl,
		Domain: understanding.DomainCamera,
		Slots: map[string]any{
			understanding.SlotDirection: understanding.DirectionRight,
		},
	}
	ptzPlan, err := engine.Resolve(context.Background(), ptzIntent, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ptzPlan.Steps[0].ActionType != ActionTypeCameraPTZ {
		t.Errorf("expected %s, got %s", ActionTypeCameraPTZ,
			ptzPlan.Steps[0].ActionType)
	}

	speakIntent := &contracts.Intent{
		Name:   understanding.IntentCameraControl,
		Domain: understanding.DomainCamera,
		Slots: map[string]any{
			understanding.SlotSpeak: "Chào gia đình",
		},
	}
	speakPlan, err := engine.Resolve(context.Background(), speakIntent, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if speakPlan.Steps[0].ActionType != ActionTypeCameraSpeak {
		t.Errorf("expected %s, got %s", ActionTypeCameraSpeak,
			speakPlan.Steps[0].ActionType)
	}

	viewIntent := &contracts.Intent{
		Name:   understanding.IntentCameraControl,
		Domain: understanding.DomainCamera,
		Slots: map[string]any{
			understanding.SlotAction:      understanding.ActionView,
			understanding.SlotTargetLabel: "cổng",
		},
	}
	viewPlan, err := engine.Resolve(context.Background(), viewIntent, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if viewPlan.Steps[0].ActionType != ActionTypeCameraView {
		t.Errorf("expected %s, got %s", ActionTypeCameraView,
			viewPlan.Steps[0].ActionType)
	}
	if viewPlan.Steps[0].RunsOn != RuntimeTVClient {
		t.Errorf("expected %s, got %s", RuntimeTVClient,
			viewPlan.Steps[0].RunsOn)
	}
}

func TestResolve_SystemStatusAndGeneral(t *testing.T) {
	engine := NewResolver(nil, nil)

	systemIntent := &contracts.Intent{
		Name:   understanding.IntentSystemStatus,
		Domain: understanding.DomainSystem,
		Slots: map[string]any{
			understanding.SlotTarget: "nas",
		},
	}
	systemPlan, err := engine.Resolve(context.Background(), systemIntent, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if systemPlan.Steps[0].ActionType != ActionTypeSystemStatus {
		t.Errorf("expected %s, got %s", ActionTypeSystemStatus,
			systemPlan.Steps[0].ActionType)
	}
	if systemPlan.Steps[0].RunsOn != RuntimeBackend {
		t.Errorf("expected %s, got %s", RuntimeBackend,
			systemPlan.Steps[0].RunsOn)
	}

	generalIntent := &contracts.Intent{
		Name:   understanding.IntentGeneralChat,
		Domain: understanding.DomainGeneral,
		Slots: map[string]any{
			understanding.SlotText: "Thời tiết hôm nay thế nào?",
		},
	}
	generalPlan, err := engine.Resolve(context.Background(), generalIntent, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if generalPlan.Steps[0].ActionType != ActionTypeLLMChat {
		t.Errorf("expected %s, got %s", ActionTypeLLMChat,
			generalPlan.Steps[0].ActionType)
	}

	unknownIntent := &contracts.Intent{
		Name:   "custom.unknown",
		Domain: "custom",
	}
	unknownPlan, err := engine.Resolve(context.Background(), unknownIntent, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(unknownPlan.Steps) != 0 {
		t.Errorf("expected 0 steps for unknown intent, got %d", len(unknownPlan.Steps))
	}
}

func TestResolve_InvalidIntent(t *testing.T) {
	engine := NewResolver(nil, nil)

	_, err := engine.Resolve(context.Background(), nil, nil)
	if !errors.Is(err, contracts.ErrInvalidIntent) {
		t.Errorf("expected ErrInvalidIntent, got %v", err)
	}

	emptyIntent := &contracts.Intent{Name: "  "}
	_, errEmpty := engine.Resolve(context.Background(), emptyIntent, nil)
	if !errors.Is(errEmpty, contracts.ErrInvalidIntent) {
		t.Errorf("expected ErrInvalidIntent, got %v", errEmpty)
	}
}

func TestResolve_PlexCatalog(t *testing.T) {
	plexMock := &mockPlexSearcher{
		items: map[string]*PlexMediaItem{
			"doraemon": {
				Title:     "Doraemon Tây Du Kí",
				Year:      1988,
				RatingKey: "123",
				Type:      "movie",
			},
		},
	}
	engine := NewResolver(plexMock, nil)

	intent := &contracts.Intent{
		Name:           understanding.IntentMediaSearch,
		Domain:         understanding.DomainMedia,
		Confidence:     0.9,
		ExplicitSource: understanding.SourcePlex,
		Slots: map[string]any{
			understanding.SlotQuery: "có phim gì",
		},
	}

	plan, err := engine.Resolve(context.Background(), intent, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plan.Evidence["is_catalog"] != true {
		t.Errorf("expected is_catalog true, got %v", plan.Evidence["is_catalog"])
	}
	titles, ok := plan.Evidence["titles"].([]string)
	if !ok || len(titles) == 0 {
		t.Fatalf("expected non-empty titles list")
	}
	if titles[0] != "Doraemon Tây Du Kí" {
		t.Errorf("expected Doraemon Tây Du Kí, got %s", titles[0])
	}
}
