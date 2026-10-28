package resolution

const (
	ActionTypeYouTubeLaunch = "youtube.launch"
	ActionTypePlexOpen      = "plex.open"
	ActionTypeHAServiceCall = "ha.service_call"
	ActionTypeCameraPrivacy = "camera.privacy"
	ActionTypeCameraPTZ     = "camera.ptz"
	ActionTypeCameraSpeak   = "camera.speak"
	ActionTypeCameraView    = "camera.view"
	ActionTypeCameraStatus  = "camera.status"
	ActionTypeSystemStatus  = "system.status"
	ActionTypeLLMChat       = "llm.chat"

	RuntimeTVClient = "tv_client"
	RuntimeBackend  = "backend"

	DefaultTimeoutMs = 5000
)
