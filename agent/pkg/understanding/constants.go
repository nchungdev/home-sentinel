package understanding

const (
	IntentMediaSearch   = "media.search"
	IntentHomeControl   = "home.control"
	IntentCameraControl = "camera.control"
	IntentSystemStatus  = "system.status"
	IntentGeneralChat   = "general.chat"

	DomainMedia   = "media"
	DomainHome    = "home_assistant"
	DomainCamera  = "camera"
	DomainSystem  = "system"
	DomainGeneral = "general"

	SourceYouTube = "youtube"
	SourcePlex    = "plex"

	ConfidenceRuleMatched = 0.95
	ConfidenceFallback    = 0.70

	SlotQuery       = "query"
	SlotKind        = "kind"
	SlotAction      = "action"
	SlotTargetLabel = "target_label"
	SlotTemperature = "temperature"
	SlotDirection   = "direction"
	SlotPrivacy     = "privacy"
	SlotSpeak       = "speak"
	SlotTarget      = "target"
	SlotText        = "text"

	ActionTurnOn         = "turn_on"
	ActionTurnOff        = "turn_off"
	ActionSetTemperature = "set_temperature"
	ActionOpen           = "open"
	ActionClose          = "close"
	ActionLock           = "lock"
	ActionUnlock         = "unlock"
	ActionView           = "view"

	PrivacyOn  = "on"
	PrivacyOff = "off"

	DirectionLeft  = "left"
	DirectionRight = "right"
	DirectionUp    = "up"
	DirectionDown  = "down"
)
