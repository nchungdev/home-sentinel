package camera

import (
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type VoiceProfile struct {
	BaseVoice string
	Pitch     float64
	Speed     float64
}

var VoiceProfiles = map[string]VoiceProfile{
	"duinch_cute":    {BaseVoice: "Linh", Pitch: 1.48, Speed: 1.18},
	"duinch_deep":    {BaseVoice: "Linh", Pitch: 0.72, Speed: 0.96},
	"duinch_natural": {BaseVoice: "Linh", Pitch: 1.0, Speed: 1.0},
	"duinch_baby":    {BaseVoice: "Linh", Pitch: 1.62, Speed: 1.1},
	"duinch_fast":    {BaseVoice: "Linh", Pitch: 1.05, Speed: 1.3},
	"samantha":       {BaseVoice: "Samantha", Pitch: 1.0, Speed: 1.0},
	"Linh":           {BaseVoice: "Linh", Pitch: 1.0, Speed: 1.0},
	"Samantha":       {BaseVoice: "Samantha", Pitch: 1.0, Speed: 1.0},
}

func GenerateSpeechAudio(
	speechText string,
	voiceIdentifier string,
	silencePaddingMs int,
	speedMultiplier float64,
) ([]byte, error) {
	temporaryDirectory, createError := os.MkdirTemp("", "tts_generate_*")
	if createError != nil {
		return nil, createError
	}
	defer os.RemoveAll(temporaryDirectory)

	aiffFilePath := filepath.Join(temporaryDirectory, "speech.aiff")
	wavFilePath := filepath.Join(temporaryDirectory, "speech.wav")

	profile, exists := VoiceProfiles[voiceIdentifier]
	if !exists {
		profile = VoiceProfile{BaseVoice: "Linh", Pitch: 1.0, Speed: 1.0}
	}

	targetPitch := profile.Pitch
	targetSpeed := profile.Speed * speedMultiplier
	if targetSpeed <= 0.1 {
		targetSpeed = 1.0
	}

	sayProcess := exec.Command("say", "-v", profile.BaseVoice, "-o", aiffFilePath, speechText)
	if sayError := sayProcess.Run(); sayError != nil {
		return nil, fmt.Errorf("say command error: %w", sayError)
	}

	filterParts := make([]string, 0, 3)
	if math.Abs(targetPitch-1.0) > 0.01 {
		tempoRatio := targetSpeed / targetPitch
		filterParts = append(
			filterParts,
			fmt.Sprintf(
				"asetrate=22050*%.3f,aresample=22050,atempo=%.3f",
				targetPitch, tempoRatio,
			),
		)
	} else if math.Abs(targetSpeed-1.0) > 0.01 {
		filterParts = append(filterParts, fmt.Sprintf("atempo=%.3f", targetSpeed))
	}

	if silencePaddingMs > 0 {
		filterParts = append(
			filterParts,
			fmt.Sprintf(
				"adelay=%d|%d,apad=pad_dur=0.3",
				silencePaddingMs, silencePaddingMs,
			),
		)
	}

	ffmpegArguments := []string{"-y", "-i", aiffFilePath}
	if len(filterParts) > 0 {
		ffmpegArguments = append(ffmpegArguments, "-af", strings.Join(filterParts, ","))
	}
	ffmpegArguments = append(
		ffmpegArguments,
		"-acodec", "pcm_alaw", "-ar", "8000", "-ac", "1",
		wavFilePath,
	)

	ffmpegProcess := exec.Command("ffmpeg", ffmpegArguments...)
	if ffmpegError := ffmpegProcess.Run(); ffmpegError != nil {
		return nil, fmt.Errorf("ffmpeg conversion error: %w", ffmpegError)
	}

	return os.ReadFile(wavFilePath)
}
