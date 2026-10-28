package arbitration

const (
	DeviceTV     = "tv"
	DeviceCamera = "camera"

	ReasonRemotePushToTalk = "remote_mic_active"
	ReasonTVOn             = "tv_is_on"
	ReasonTVOff            = "tv_is_off"
	ReasonUserAway         = "user_away_from_tv"
	ReasonEnergyComparison = "higher_voice_energy"
)

type Decision struct {
	ListeningDevice string `json:"listening_device"`
	SpeakingDevice  string `json:"speaking_device"`
	Reason          string `json:"reason"`
}

type Context struct {
	RemoteMicActive   bool    `json:"remote_mic_active"`
	TVScreenOn        bool    `json:"tv_screen_on"`
	UserOnSofa        bool    `json:"user_on_sofa"`
	TVAudioEnergy     float64 `json:"tv_audio_energy"`
	CameraAudioEnergy float64 `json:"camera_audio_energy"`
}

type AudioArbitrator struct {
	EnergyHysteresisThreshold float64
}

func NewAudioArbitrator(threshold float64) *AudioArbitrator {
	if threshold <= 0 {
		threshold = 0.15
	}
	return &AudioArbitrator{
		EnergyHysteresisThreshold: threshold,
	}
}

func (a *AudioArbitrator) Arbitrate(ctx Context) Decision {
	listeningDevice, reason := a.resolveListeningDevice(ctx)
	speakingDevice := a.resolveSpeakingDevice(ctx.TVScreenOn, listeningDevice)

	return Decision{
		ListeningDevice: listeningDevice,
		SpeakingDevice:  speakingDevice,
		Reason:          reason,
	}
}

func (a *AudioArbitrator) resolveListeningDevice(ctx Context) (string, string) {
	if ctx.RemoteMicActive {
		return DeviceTV, ReasonRemotePushToTalk
	}

	if ctx.TVScreenOn {
		return DeviceTV, ReasonTVOn
	}

	if ctx.TVAudioEnergy > 0.0 && ctx.CameraAudioEnergy > 0.0 {
		diff := ctx.TVAudioEnergy - ctx.CameraAudioEnergy
		if diff > a.EnergyHysteresisThreshold {
			return DeviceTV, ReasonEnergyComparison
		}
		if -diff > a.EnergyHysteresisThreshold {
			return DeviceCamera, ReasonEnergyComparison
		}
	}

	if ctx.UserOnSofa {
		return DeviceCamera, ReasonTVOff
	}

	return DeviceCamera, ReasonUserAway
}

func (a *AudioArbitrator) resolveSpeakingDevice(tvScreenOn bool, listeningDevice string) string {
	if tvScreenOn {
		return DeviceTV
	}
	return listeningDevice
}
