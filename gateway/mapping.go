package main

import (
	"fmt"
	"strings"
)

const (
	kindToggle  = "toggle"
	kindRange   = "range"
	kindSelect  = "select"
	kindTrigger = "trigger"
	kindCover   = "cover"
	kindReadout = "readout"
	kindStream  = "stream"

	featureFanSetSpeed        = 1
	featureClimateTargetTemp  = 1
	featureCoverSetPosition   = 4
	featureMediaVolumeSet     = 4
	featureMediaSelectSource  = 2048
	stateUnavailable          = "unavailable"
	stateUnknown              = "unknown"
	errorUnsupported          = "unsupported"
	errorInvalidValue         = "invalid_value"
	errorReadOnly             = "read_only"
	errorNotFound             = "not_found"
	defaultMinColorTempKelvin = 2000
	defaultMaxColorTempKelvin = 6500
	defaultMinClimateTemp     = 16
	defaultMaxClimateTemp     = 30
	maxBrightness             = 255
	percentMax                = 100
	colorTempStepKelvin       = 50
	streamPathFormat          = "/api/v1/devices/%s/stream"
)

var (
	coverCommands    = []string{"open", "close", "stop"}
	playbackCommands = []string{"play", "pause", "stop", "next", "previous"}
	playbackServices = map[string]string{
		"play":     "media_play",
		"pause":    "media_pause",
		"stop":     "media_stop",
		"next":     "media_next_track",
		"previous": "media_previous_track",
	}
	offStates = []string{"off", "standby", stateUnavailable, stateUnknown}
)

type ActionError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (failure *ActionError) Error() string {
	return failure.Message
}

func actionError(code string, format string, arguments ...any) *ActionError {
	return &ActionError{Code: code, Message: fmt.Sprintf(format, arguments...)}
}

type Attributes map[string]any

func (attributes Attributes) Has(name string) bool {
	_, found := attributes[name]
	return found
}

func (attributes Attributes) Number(name string, fallback float64) float64 {
	number, isNumber := attributes[name].(float64)
	if !isNumber {
		return fallback
	}
	return number
}

func (attributes Attributes) Text(name string) string {
	text, _ := attributes[name].(string)
	return text
}

func (attributes Attributes) List(name string) []string {
	items, _ := attributes[name].([]any)
	texts := make([]string, 0, len(items))
	for _, item := range items {
		if text, isText := item.(string); isText {
			texts = append(texts, text)
		}
	}
	return texts
}

func (attributes Attributes) Supports(feature int) bool {
	return int(attributes.Number("supported_features", 0))&feature != 0
}

type Traits struct {
	Min     *float64 `json:"min,omitempty"`
	Max     *float64 `json:"max,omitempty"`
	Step    *float64 `json:"step,omitempty"`
	Unit    string   `json:"unit,omitempty"`
	Options []string `json:"options,omitempty"`
	URL     string   `json:"url,omitempty"`
}

type CapabilityInfo struct {
	ID       string `json:"id"`
	Kind     string `json:"kind"`
	Writable bool   `json:"writable"`
	Traits
}

type Device struct {
	ID           string           `json:"id"`
	Name         string           `json:"name"`
	Type         string           `json:"type"`
	Icon         string           `json:"icon,omitempty"`
	Area         string           `json:"area,omitempty"`
	Available    bool             `json:"available"`
	Capabilities []CapabilityInfo `json:"capabilities"`
	State        map[string]any   `json:"state"`
}

type Action struct {
	Service string
	Data    map[string]any
}

type ServiceCall struct {
	Domain  string
	Service string
	Data    map[string]any
}

type Capability struct {
	ID       string
	Kind     string
	Describe func(entityID string, attributes Attributes) *Traits
	Read     func(state string, attributes Attributes) any
	Act      func(value any, attributes Attributes) (Action, error)
}

type DomainEntry struct {
	DeviceType   string
	Capabilities []Capability
}

func number(value float64) *float64 {
	return &value
}

func rangeTraits(minimum, maximum, step float64, unit string) *Traits {
	return &Traits{Min: number(minimum), Max: number(maximum), Step: number(step), Unit: unit}
}

func noTraits(string, Attributes) *Traits {
	return &Traits{}
}

func readAttribute(name string) func(string, Attributes) any {
	return func(_ string, attributes Attributes) any { return attributes[name] }
}

func readState(state string, _ Attributes) any {
	return state
}

func toBool(value any) (bool, error) {
	flag, isBool := value.(bool)
	if !isBool {
		return false, actionError(errorInvalidValue, "expected true or false")
	}
	return flag, nil
}

func toNumber(value any, minimum, maximum float64) (float64, error) {
	parsed, isNumber := value.(float64)
	if !isNumber {
		return 0, actionError(errorInvalidValue, "expected a number")
	}
	if parsed < minimum || parsed > maximum {
		return 0, actionError(errorInvalidValue, "value must be within %g and %g", minimum, maximum)
	}
	return parsed, nil
}

func toOption(value any, options []string) (string, error) {
	text, _ := value.(string)
	for _, option := range options {
		if option == text {
			return text, nil
		}
	}
	allowed := strings.Join(options, ", ")
	return "", actionError(errorInvalidValue, "value must be one of %s", allowed)
}

func contains(items []string, wanted string) bool {
	for _, item := range items {
		if item == wanted {
			return true
		}
	}
	return false
}

func toggleCapability(id, onService, offService string, isOn func(string) bool) Capability {
	return Capability{
		ID:       id,
		Kind:     kindToggle,
		Describe: noTraits,
		Read:     func(state string, _ Attributes) any { return isOn(state) },
		Act: func(value any, _ Attributes) (Action, error) {
			flag, flagErr := toBool(value)
			if flagErr != nil {
				return Action{}, flagErr
			}
			if flag {
				return Action{Service: onService}, nil
			}
			return Action{Service: offService}, nil
		},
	}
}

func powerToggle() Capability {
	return toggleCapability("power", "turn_on", "turn_off", func(state string) bool {
		return !contains(offStates, state)
	})
}

func switchToggle() Capability {
	return toggleCapability("power", "turn_on", "turn_off", func(state string) bool {
		return state == "on"
	})
}

func lockToggle() Capability {
	return toggleCapability("locked", "lock", "unlock", func(state string) bool {
		return state == "locked"
	})
}

func triggerCapability(service string) Capability {
	return Capability{
		ID:       "run",
		Kind:     kindTrigger,
		Describe: noTraits,
		Read:     func(string, Attributes) any { return nil },
		Act: func(any, Attributes) (Action, error) {
			return Action{Service: service}, nil
		},
	}
}

func sensorReadout() Capability {
	return Capability{
		ID:   "value",
		Kind: kindReadout,
		Describe: func(_ string, attributes Attributes) *Traits {
			return &Traits{Unit: attributes.Text("unit_of_measurement")}
		},
		Read: readState,
	}
}

func binaryReadout() Capability {
	return Capability{
		ID:       "value",
		Kind:     kindReadout,
		Describe: noTraits,
		Read:     func(state string, _ Attributes) any { return state == "on" },
	}
}

func lightBrightness() Capability {
	return Capability{
		ID:   "brightness",
		Kind: kindRange,
		Describe: func(_ string, attributes Attributes) *Traits {
			for _, mode := range attributes.List("supported_color_modes") {
				if mode != "onoff" {
					return rangeTraits(0, maxBrightness, 1, "")
				}
			}
			return nil
		},
		Read: readAttribute("brightness"),
		Act: func(value any, _ Attributes) (Action, error) {
			level, levelErr := toNumber(value, 0, maxBrightness)
			if levelErr != nil {
				return Action{}, levelErr
			}
			if level == 0 {
				return Action{Service: "turn_off"}, nil
			}
			return Action{"turn_on", map[string]any{"brightness": int(level)}}, nil
		},
	}
}

func lightColorTemperature() Capability {
	bounds := func(attributes Attributes) (float64, float64) {
		return attributes.Number("min_color_temp_kelvin", defaultMinColorTempKelvin),
			attributes.Number("max_color_temp_kelvin", defaultMaxColorTempKelvin)
	}

	return Capability{
		ID:   "color_temp",
		Kind: kindRange,
		Describe: func(_ string, attributes Attributes) *Traits {
			if !contains(attributes.List("supported_color_modes"), "color_temp") {
				return nil
			}
			minimum, maximum := bounds(attributes)
			return rangeTraits(minimum, maximum, colorTempStepKelvin, "K")
		},
		Read: readAttribute("color_temp_kelvin"),
		Act: func(value any, attributes Attributes) (Action, error) {
			minimum, maximum := bounds(attributes)
			kelvin, kelvinErr := toNumber(value, minimum, maximum)
			if kelvinErr != nil {
				return Action{}, kelvinErr
			}
			return Action{"turn_on", map[string]any{"color_temp_kelvin": int(kelvin)}}, nil
		},
	}
}

func fanSpeed() Capability {
	return Capability{
		ID:   "speed",
		Kind: kindRange,
		Describe: func(_ string, attributes Attributes) *Traits {
			if !attributes.Supports(featureFanSetSpeed) {
				return nil
			}
			return rangeTraits(0, percentMax, attributes.Number("percentage_step", 1), "%")
		},
		Read: readAttribute("percentage"),
		Act: func(value any, _ Attributes) (Action, error) {
			percentage, percentageErr := toNumber(value, 0, percentMax)
			if percentageErr != nil {
				return Action{}, percentageErr
			}
			return Action{"set_percentage", map[string]any{"percentage": int(percentage)}}, nil
		},
	}
}

func climateBounds(attributes Attributes) (float64, float64) {
	return attributes.Number("min_temp", defaultMinClimateTemp),
		attributes.Number("max_temp", defaultMaxClimateTemp)
}

func climateTargetTemperature() Capability {
	return Capability{
		ID:   "target_temp",
		Kind: kindRange,
		Describe: func(_ string, attributes Attributes) *Traits {
			if !attributes.Supports(featureClimateTargetTemp) {
				return nil
			}
			minimum, maximum := climateBounds(attributes)
			return rangeTraits(minimum, maximum, attributes.Number("target_temp_step", 1), "°C")
		},
		Read: readAttribute("temperature"),
		Act: func(value any, attributes Attributes) (Action, error) {
			minimum, maximum := climateBounds(attributes)
			target, targetErr := toNumber(value, minimum, maximum)
			if targetErr != nil {
				return Action{}, targetErr
			}
			return Action{"set_temperature", map[string]any{"temperature": target}}, nil
		},
	}
}

func climateMode() Capability {
	return Capability{
		ID:   "mode",
		Kind: kindSelect,
		Describe: func(_ string, attributes Attributes) *Traits {
			modes := []string{}
			for _, mode := range attributes.List("hvac_modes") {
				if mode != "off" {
					modes = append(modes, mode)
				}
			}
			if len(modes) == 0 {
				return nil
			}
			return &Traits{Options: modes}
		},
		Read: readState,
		Act: func(value any, attributes Attributes) (Action, error) {
			mode, modeErr := toOption(value, attributes.List("hvac_modes"))
			if modeErr != nil {
				return Action{}, modeErr
			}
			return Action{"set_hvac_mode", map[string]any{"hvac_mode": mode}}, nil
		},
	}
}

func climateCurrentTemperature() Capability {
	return Capability{
		ID:   "current_temp",
		Kind: kindReadout,
		Describe: func(_ string, attributes Attributes) *Traits {
			if !attributes.Has("current_temperature") {
				return nil
			}
			return &Traits{Unit: "°C"}
		},
		Read: readAttribute("current_temperature"),
	}
}

func coverCommand() Capability {
	return Capability{
		ID:   "command",
		Kind: kindCover,
		Describe: func(string, Attributes) *Traits {
			return &Traits{Options: coverCommands}
		},
		Read: readState,
		Act: func(value any, _ Attributes) (Action, error) {
			command, commandErr := toOption(value, coverCommands)
			if commandErr != nil {
				return Action{}, commandErr
			}
			return Action{Service: command + "_cover"}, nil
		},
	}
}

func coverPosition() Capability {
	return Capability{
		ID:   "position",
		Kind: kindRange,
		Describe: func(_ string, attributes Attributes) *Traits {
			knowsPosition := attributes.Has("current_position")
			if !knowsPosition && !attributes.Supports(featureCoverSetPosition) {
				return nil
			}
			return rangeTraits(0, percentMax, 1, "%")
		},
		Read: readAttribute("current_position"),
		Act: func(value any, _ Attributes) (Action, error) {
			position, positionErr := toNumber(value, 0, percentMax)
			if positionErr != nil {
				return Action{}, positionErr
			}
			return Action{"set_cover_position", map[string]any{"position": int(position)}}, nil
		},
	}
}

func mediaVolume() Capability {
	return Capability{
		ID:   "volume",
		Kind: kindRange,
		Describe: func(_ string, attributes Attributes) *Traits {
			if !attributes.Supports(featureMediaVolumeSet) {
				return nil
			}
			return rangeTraits(0, percentMax, 1, "%")
		},
		Read: func(_ string, attributes Attributes) any {
			level, isNumber := attributes["volume_level"].(float64)
			if !isNumber {
				return nil
			}
			return int(level*percentMax + 0.5)
		},
		Act: func(value any, _ Attributes) (Action, error) {
			volume, volumeErr := toNumber(value, 0, percentMax)
			if volumeErr != nil {
				return Action{}, volumeErr
			}
			return Action{"volume_set", map[string]any{"volume_level": volume / percentMax}}, nil
		},
	}
}

func mediaPlayback() Capability {
	return Capability{
		ID:   "playback",
		Kind: kindSelect,
		Describe: func(string, Attributes) *Traits {
			return &Traits{Options: playbackCommands}
		},
		Read: readState,
		Act: func(value any, _ Attributes) (Action, error) {
			command, commandErr := toOption(value, playbackCommands)
			if commandErr != nil {
				return Action{}, commandErr
			}
			return Action{Service: playbackServices[command]}, nil
		},
	}
}

func mediaSource() Capability {
	return Capability{
		ID:   "source",
		Kind: kindSelect,
		Describe: func(_ string, attributes Attributes) *Traits {
			sources := attributes.List("source_list")
			if !attributes.Supports(featureMediaSelectSource) || len(sources) == 0 {
				return nil
			}
			return &Traits{Options: sources}
		},
		Read: readAttribute("source"),
		Act: func(value any, attributes Attributes) (Action, error) {
			source, sourceErr := toOption(value, attributes.List("source_list"))
			if sourceErr != nil {
				return Action{}, sourceErr
			}
			return Action{"select_source", map[string]any{"source": source}}, nil
		},
	}
}

func numberValue() Capability {
	return Capability{
		ID:   "value",
		Kind: kindRange,
		Describe: func(_ string, attributes Attributes) *Traits {
			return rangeTraits(
				attributes.Number("min", 0),
				attributes.Number("max", percentMax),
				attributes.Number("step", 1),
				attributes.Text("unit_of_measurement"),
			)
		},
		Read: func(state string, _ Attributes) any {
			var parsed float64
			if _, scanErr := fmt.Sscanf(state, "%g", &parsed); scanErr != nil {
				return nil
			}
			return parsed
		},
		Act: func(value any, attributes Attributes) (Action, error) {
			parsed, parsedErr := toNumber(value, attributes.Number("min", 0),
				attributes.Number("max", percentMax))
			if parsedErr != nil {
				return Action{}, parsedErr
			}
			return Action{"set_value", map[string]any{"value": parsed}}, nil
		},
	}
}

func selectOption() Capability {
	return Capability{
		ID:   "option",
		Kind: kindSelect,
		Describe: func(_ string, attributes Attributes) *Traits {
			return &Traits{Options: attributes.List("options")}
		},
		Read: readState,
		Act: func(value any, attributes Attributes) (Action, error) {
			option, optionErr := toOption(value, attributes.List("options"))
			if optionErr != nil {
				return Action{}, optionErr
			}
			return Action{"select_option", map[string]any{"option": option}}, nil
		},
	}
}

func cameraStream() Capability {
	return Capability{
		ID:   "stream",
		Kind: kindStream,
		Describe: func(entityID string, _ Attributes) *Traits {
			return &Traits{URL: fmt.Sprintf(streamPathFormat, entityID)}
		},
		Read: readState,
	}
}

var domainCatalog = map[string]DomainEntry{
	"light": {"light", []Capability{
		powerToggle(), lightBrightness(), lightColorTemperature(),
	}},
	"switch":        {"switch", []Capability{switchToggle()}},
	"input_boolean": {"switch", []Capability{switchToggle()}},
	"automation":    {"switch", []Capability{switchToggle()}},
	"fan":           {"fan", []Capability{powerToggle(), fanSpeed()}},
	"climate": {"climate", []Capability{
		powerToggle(), climateTargetTemperature(), climateMode(), climateCurrentTemperature(),
	}},
	"cover": {"cover", []Capability{coverCommand(), coverPosition()}},
	"lock":  {"lock", []Capability{lockToggle()}},
	"media_player": {"media_player", []Capability{
		powerToggle(), mediaVolume(), mediaPlayback(), mediaSource(),
	}},
	"camera":        {"camera", []Capability{cameraStream()}},
	"scene":         {"scene", []Capability{triggerCapability("turn_on")}},
	"script":        {"script", []Capability{triggerCapability("turn_on")}},
	"button":        {"button", []Capability{triggerCapability("press")}},
	"number":        {"number", []Capability{numberValue()}},
	"input_number":  {"number", []Capability{numberValue()}},
	"select":        {"select", []Capability{selectOption()}},
	"input_select":  {"select", []Capability{selectOption()}},
	"sensor":        {"sensor", []Capability{sensorReadout()}},
	"binary_sensor": {"binary_sensor", []Capability{binaryReadout()}},
}

func domainOf(entityID string) string {
	domain, _, _ := strings.Cut(entityID, ".")
	return domain
}

func isSupported(entityID string) bool {
	_, found := domainCatalog[domainOf(entityID)]
	return found
}

func catalogEntry(entityID string) (DomainEntry, error) {
	entry, found := domainCatalog[domainOf(entityID)]
	if !found {
		return DomainEntry{}, actionError(errorUnsupported, "device type is not supported")
	}
	return entry, nil
}

type describedCapability struct {
	capability Capability
	traits     *Traits
}

func availableCapabilities(
	entry DomainEntry,
	entityID string,
	attributes Attributes,
) []describedCapability {
	described := []describedCapability{}
	for _, capability := range entry.Capabilities {
		if traits := capability.Describe(entityID, attributes); traits != nil {
			described = append(described, describedCapability{capability, traits})
		}
	}
	return described
}

func DescribeDevice(entityID, state string, attributes Attributes) (Device, error) {
	entry, entryErr := catalogEntry(entityID)
	if entryErr != nil {
		return Device{}, entryErr
	}

	name := attributes.Text("friendly_name")
	if name == "" {
		name = entityID
	}

	device := Device{
		ID:           entityID,
		Name:         name,
		Type:         entry.DeviceType,
		Icon:         attributes.Text("icon"),
		Available:    state != stateUnavailable && state != stateUnknown,
		Capabilities: []CapabilityInfo{},
		State:        map[string]any{},
	}
	for _, described := range availableCapabilities(entry, entityID, attributes) {
		capability := described.capability
		device.Capabilities = append(device.Capabilities, CapabilityInfo{
			ID:       capability.ID,
			Kind:     capability.Kind,
			Writable: capability.Act != nil,
			Traits:   *described.traits,
		})
		device.State[capability.ID] = capability.Read(state, attributes)
	}
	return device, nil
}

func ResolveAction(
	entityID, capabilityID string,
	value any,
	attributes Attributes,
) (ServiceCall, error) {
	entry, entryErr := catalogEntry(entityID)
	if entryErr != nil {
		return ServiceCall{}, entryErr
	}

	for _, described := range availableCapabilities(entry, entityID, attributes) {
		capability := described.capability
		if capability.ID != capabilityID {
			continue
		}
		if capability.Act == nil {
			return ServiceCall{}, actionError(errorReadOnly, "%s is read only", capabilityID)
		}

		action, actionErr := capability.Act(value, attributes)
		if actionErr != nil {
			return ServiceCall{}, actionErr
		}

		data := map[string]any{"entity_id": entityID}
		for key, item := range action.Data {
			data[key] = item
		}
		return ServiceCall{Domain: domainOf(entityID), Service: action.Service, Data: data}, nil
	}
	return ServiceCall{}, actionError(
		errorUnsupported, "%s is not available on this device", capabilityID,
	)
}
