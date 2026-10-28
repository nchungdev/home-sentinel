package tvmodes

import (
	"errors"
	"fmt"
	"sort"
)

var ErrUnknownMode = errors.New("unknown mode")

type HomeAssistant interface {
	CallService(domain, service string, data map[string]any) error
}

type Engine struct {
	store    *Store
	home     HomeAssistant
	onChange func(State)
}

func NewEngine(store *Store, home HomeAssistant, onChange func(State)) *Engine {
	return &Engine{store: store, home: home, onChange: onChange}
}

func (e *Engine) State() State {
	config := e.store.Get()
	summaries := make([]Summary, 0, len(config.Modes))
	for _, mode := range config.Modes {
		summaries = append(summaries, mode.summary())
	}
	return State{Active: config.Active, Modes: summaries}
}

func (e *Engine) Activate(id string) (State, error) {
	if id == NormalID {
		return e.restore()
	}

	config := e.store.Get()
	mode, found := find(config.Modes, id)
	if !found {
		return State{}, ErrUnknownMode
	}
	snapshotted := config.Snapshotted
	if config.Active == "" {
		taken, err := e.snapshot(config.Modes)
		if err != nil {
			return State{}, fmt.Errorf("snapshot: %w", err)
		}
		snapshotted = taken
	}

	failed := e.apply(mode.Actions)
	if err := e.store.setActive(mode.ID, snapshotted); err != nil {
		return State{}, err
	}
	state := e.notify()
	if failed > 0 {
		return state, fmt.Errorf("%d of %d actions failed", failed, len(mode.Actions))
	}
	return state, nil
}

func (e *Engine) restore() (State, error) {
	config := e.store.Get()
	if config.Active == "" {
		return e.State(), nil
	}
	if config.Snapshotted {
		scene := map[string]any{"entity_id": "scene." + restoreScene}
		if err := e.home.CallService("scene", ServiceOn, scene); err != nil {
			return State{}, fmt.Errorf("restore: %w", err)
		}
	}
	if err := e.store.setActive("", false); err != nil {
		return State{}, err
	}
	return e.notify(), nil
}

func (e *Engine) snapshot(modes []Mode) (bool, error) {
	entities := touchedEntities(modes)
	if len(entities) == 0 {
		return false, nil
	}
	err := e.home.CallService("scene", "create", map[string]any{
		"scene_id":          restoreScene,
		"snapshot_entities": entities,
	})
	return err == nil, err
}

func (e *Engine) apply(actions []Action) int {
	failed := 0
	for _, action := range actions {
		data := map[string]any{"entity_id": action.Entity}
		for key, value := range action.Data {
			data[key] = value
		}
		if e.home.CallService(action.domain(), action.Service, data) != nil {
			failed++
		}
	}
	return failed
}

func (e *Engine) notify() State {
	state := e.State()
	if e.onChange != nil {
		e.onChange(state)
	}
	return state
}

func touchedEntities(modes []Mode) []string {
	unique := map[string]bool{}
	for _, mode := range modes {
		for _, action := range mode.Actions {
			unique[action.Entity] = true
		}
	}
	entities := make([]string, 0, len(unique))
	for entity := range unique {
		entities = append(entities, entity)
	}
	sort.Strings(entities)
	return entities
}
