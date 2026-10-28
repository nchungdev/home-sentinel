package tvmodes

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"
)

var modeID = regexp.MustCompile(`^[a-z0-9_]{1,32}$`)

type Store struct {
	mutex  sync.Mutex
	path   string
	config Config
}

func NewStore(path string) *Store {
	store := &Store{path: path, config: DefaultConfig()}
	data, err := os.ReadFile(path)
	if err != nil {
		return store
	}
	var loaded Config
	if json.Unmarshal(data, &loaded) == nil && Validate(&loaded) == nil {
		store.config = loaded
	}
	return store
}

func (s *Store) Get() Config {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return clone(s.config)
}

func (s *Store) Replace(next Config) error {
	if err := Validate(&next); err != nil {
		return err
	}
	s.mutex.Lock()
	defer s.mutex.Unlock()
	next.Active = s.config.Active
	next.Snapshotted = s.config.Snapshotted
	if _, found := find(next.Modes, next.Active); !found {
		next.Active = ""
	}
	return s.save(next)
}

func (s *Store) setActive(active string, snapshotted bool) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	next := clone(s.config)
	next.Active = active
	next.Snapshotted = snapshotted
	return s.save(next)
}

func (s *Store) save(next Config) error {
	data, err := json.MarshalIndent(next, "", "  ")
	if err != nil {
		return err
	}
	temporary := s.path + ".tmp"
	if err := os.WriteFile(temporary, data, 0o600); err != nil {
		return err
	}
	if err := os.Rename(temporary, s.path); err != nil {
		return err
	}
	s.config = next
	return nil
}

func Validate(config *Config) error {
	if len(config.Modes) > maxModes {
		return fmt.Errorf("at most %d modes", maxModes)
	}
	seen := map[string]bool{}
	for index := range config.Modes {
		mode := &config.Modes[index]
		mode.Name = strings.TrimSpace(mode.Name)
		if !modeID.MatchString(mode.ID) || mode.ID == NormalID || seen[mode.ID] {
			return fmt.Errorf("invalid or duplicate mode id %q", mode.ID)
		}
		if mode.Name == "" || len([]rune(mode.Name)) > maxNameLength {
			return fmt.Errorf("mode %q needs a name of 1-%d characters", mode.ID, maxNameLength)
		}
		if len(mode.Actions) > maxActions {
			return fmt.Errorf("mode %q has more than %d actions", mode.ID, maxActions)
		}
		for _, action := range mode.Actions {
			if !action.valid() {
				return fmt.Errorf("mode %q has an invalid action for %q", mode.ID, action.Entity)
			}
		}
		seen[mode.ID] = true
	}
	return nil
}

func find(modes []Mode, id string) (Mode, bool) {
	for _, mode := range modes {
		if mode.ID == id {
			return mode, true
		}
	}
	return Mode{}, false
}

func clone(config Config) Config {
	data, _ := json.Marshal(config)
	var copied Config
	_ = json.Unmarshal(data, &copied)
	return copied
}
