from typing import Any, Dict, Tuple

DEVICE_TV = "tv"
DEVICE_CAMERA = "camera"

REASON_REMOTE_PUSH_TO_TALK = "remote_mic_active"
REASON_TV_ON = "tv_is_on"
REASON_TV_OFF = "tv_is_off"
REASON_USER_AWAY = "user_away_from_tv"
REASON_ENERGY_COMPARISON = "higher_voice_energy"

class AudioArbitrator:
    def __init__(self, energyHysteresisThreshold: float = 0.15):
        self.energyHysteresisThreshold = energyHysteresisThreshold

    def arbitrate(self, context: Dict[str, Any]) -> Dict[str, str]:
        remoteMicActive = bool(context.get("remote_mic_active", False))
        tvScreenOn = bool(context.get("tv_screen_on", False))
        userOnSofa = bool(context.get("user_on_sofa", False))
        tvEnergy = float(context.get("tv_audio_energy", 0.0))
        cameraEnergy = float(context.get("camera_audio_energy", 0.0))

        listeningDevice, reason = self._resolveListeningDevice(
            remoteMicActive,
            tvScreenOn,
            userOnSofa,
            tvEnergy,
            cameraEnergy
        )

        speakingDevice = self._resolveSpeakingDevice(tvScreenOn, listeningDevice)

        return {
            "listening_device": listeningDevice,
            "speaking_device": speakingDevice,
            "reason": reason
        }

    def _resolveListeningDevice(
        self,
        remoteMicActive: bool,
        tvScreenOn: bool,
        userOnSofa: bool,
        tvEnergy: float,
        cameraEnergy: float
    ) -> Tuple[str, str]:
        if remoteMicActive:
            return DEVICE_TV, REASON_REMOTE_PUSH_TO_TALK

        if tvScreenOn:
            return DEVICE_TV, REASON_TV_ON

        if tvEnergy > 0.0 and cameraEnergy > 0.0:
            if tvEnergy - cameraEnergy > self.energyHysteresisThreshold:
                return DEVICE_TV, REASON_ENERGY_COMPARISON
            if cameraEnergy - tvEnergy > self.energyHysteresisThreshold:
                return DEVICE_CAMERA, REASON_ENERGY_COMPARISON

        if userOnSofa:
            return DEVICE_CAMERA, REASON_TV_OFF

        return DEVICE_CAMERA, REASON_USER_AWAY

    def _resolveSpeakingDevice(self, tvScreenOn: bool, listeningDevice: str) -> str:
        if tvScreenOn:
            return DEVICE_TV
        return listeningDevice
