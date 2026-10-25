package speech

import (
	"math"
)

type SoundCategory string

const (
	CategorySpeech SoundCategory = "speech" // Lời nói con người (hài âm, năng lượng liên tục, ZCR 0.05-0.35)
	CategoryNoise  SoundCategory = "noise"  // Tiếng động/va quẹt, cạch cửa, rơi vỡ, ho, vỗ tay (ZCR cực cao hoặc xung cực ngắn)
	CategoryQuiet  SoundCategory = "quiet"  // Không gian yên tĩnh
)

type AudioFeatures struct {
	RMS          float64
	Peak         int
	ZeroCrossing float64
	DurationSec  float64
	Category     SoundCategory
}

// AnalyzeAudioSegment phân tích đặc trưng phổ âm thanh PCM 16-bit mono 16kHz
func AnalyzeAudioSegment(pcm []byte) AudioFeatures {
	samples := len(pcm) / 2
	if samples == 0 {
		return AudioFeatures{Category: CategoryQuiet}
	}

	var sumSq float64
	var zeroCrossings int
	maxPeak := 0
	prevSign := 0

	for i := 0; i < samples; i++ {
		val := int16(pcm[2*i]) | (int16(pcm[2*i+1]) << 8)
		v := float64(val)
		sumSq += v * v

		absPeak := int(val)
		if absPeak < 0 {
			absPeak = -absPeak
		}
		if absPeak > maxPeak {
			maxPeak = absPeak
		}

		// Tính Zero Crossing Rate (ZCR)
		sign := 0
		if val > 100 {
			sign = 1
		} else if val < -100 {
			sign = -1
		}
		if sign != 0 && prevSign != 0 && sign != prevSign {
			zeroCrossings++
		}
		if sign != 0 {
			prevSign = sign
		}
	}

	rms := math.Sqrt(sumSq / float64(samples))
	zcr := float64(zeroCrossings) / float64(samples)
	dur := float64(samples) / 16000.0

	feat := AudioFeatures{
		RMS:          rms,
		Peak:         maxPeak,
		ZeroCrossing: zcr,
		DurationSec:  dur,
		Category:     CategoryQuiet,
	}

	if rms < 100.0 {
		feat.Category = CategoryQuiet
		return feat
	}

	// 1. Tiếng động dạng xung ngắn (Impulse Noise): cạch cửa, va đập, gõ bàn, rơi đồ
	// Thường có đỉnh Peak rất cao so với RMS trung bình (Crest Factor lớn), hoặc thời lượng < 0.6s
	crestFactor := 0.0
	if rms > 0 {
		crestFactor = float64(maxPeak) / rms
	}

	// Tiếng động cơ học (cạch cửa, gõ cửa, đồ đạc va quẹt):
	// - Crest factor > 10 hoặc ZCR > 0.40 (tần số cao lẹt xẹt) hoặc ZCR < 0.02 (tiếng ù/thổi gió)
	// - Hoặc âm thanh ngắn < 0.7s
	isImpulseNoise := crestFactor > 12.0 || dur < 0.7
	isHighFreqNoise := zcr > 0.38
	isLowFreqRumble := zcr < 0.03

	if isImpulseNoise || isHighFreqNoise || isLowFreqRumble {
		feat.Category = CategoryNoise
	} else {
		// Giọng nói con người (tiếng Việt): dải tần 200Hz - 3400Hz, ZCR thường rơi vào 0.05 - 0.32
		feat.Category = CategorySpeech
	}

	return feat
}
