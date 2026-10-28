package tvmodes

import (
	"errors"
	"path/filepath"
	"testing"
)

type call struct {
	domain, service string
	data            map[string]any
}

type fakeHome struct {
	calls []call
	fail  map[string]bool
}

func (f *fakeHome) CallService(domain, service string, data map[string]any) error {
	f.calls = append(f.calls, call{domain, service, data})
	if f.fail[domain+"."+service] {
		return errors.New("refused")
	}
	return nil
}

func testConfig() Config {
	return Config{Modes: []Mode{
		{ID: "movie", Name: "Xem phim", Actions: []Action{
			{Entity: "light.living", Service: ServiceOff},
			{Entity: "fan.living", Service: ServiceOn, Data: map[string]any{"percentage": 60}},
		}},
		{ID: "dinner", Name: "Phim + ăn cơm", Actions: []Action{
			{Entity: "fan.living", Service: ServiceOn},
		}},
	}}
}

func newEngine(t *testing.T, home *fakeHome) (*Engine, *[]State) {
	t.Helper()
	store := NewStore(filepath.Join(t.TempDir(), "modes.json"))
	if err := store.Replace(testConfig()); err != nil {
		t.Fatal(err)
	}
	var changes []State
	return NewEngine(store, home, func(s State) { changes = append(changes, s) }), &changes
}

func TestActivateSnapshotsOnceThenApplies(t *testing.T) {
	home := &fakeHome{}
	engine, changes := newEngine(t, home)

	if _, err := engine.Activate("movie"); err != nil {
		t.Fatal(err)
	}
	if home.calls[0].service != "create" || home.calls[0].domain != "scene" {
		t.Fatalf("first call must snapshot, got %+v", home.calls[0])
	}
	snapshot := home.calls[0].data["snapshot_entities"].([]string)
	if len(snapshot) != 2 {
		t.Fatalf("snapshot should cover every entity of every mode, got %v", snapshot)
	}
	fan := home.calls[2]
	if fan.data["percentage"] != float64(60) || fan.data["entity_id"] != "fan.living" {
		t.Fatalf("fan action lost its data: %+v", fan)
	}

	home.calls = nil
	if _, err := engine.Activate("dinner"); err != nil {
		t.Fatal(err)
	}
	if home.calls[0].service == "create" {
		t.Fatal("switching between modes must keep the original snapshot")
	}
	if last := (*changes)[len(*changes)-1]; last.Active != "dinner" {
		t.Fatalf("want active dinner, got %+v", last)
	}
}

func TestNormalRestoresSnapshot(t *testing.T) {
	home := &fakeHome{}
	engine, _ := newEngine(t, home)
	if _, err := engine.Activate("movie"); err != nil {
		t.Fatal(err)
	}

	home.calls = nil
	state, err := engine.Activate(NormalID)
	if err != nil {
		t.Fatal(err)
	}
	restore := home.calls[0]
	if restore.domain != "scene" || restore.data["entity_id"] != "scene."+restoreScene {
		t.Fatalf("want snapshot scene restored, got %+v", restore)
	}
	if state.Active != "" {
		t.Fatalf("want no active mode, got %q", state.Active)
	}
}

func TestUnknownModeIsRejected(t *testing.T) {
	engine, _ := newEngine(t, &fakeHome{})
	if _, err := engine.Activate("party"); !errors.Is(err, ErrUnknownMode) {
		t.Fatalf("want ErrUnknownMode, got %v", err)
	}
}

func TestValidateRejectsBadActions(t *testing.T) {
	config := testConfig()
	broken := Action{Entity: "light", Service: "explode"}
	config.Modes[0].Actions = append(config.Modes[0].Actions, broken)
	if Validate(&config) == nil {
		t.Fatal("invalid action accepted")
	}
}
