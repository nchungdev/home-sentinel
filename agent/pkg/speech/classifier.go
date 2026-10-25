package speech

import (
	"math"
)

type SoundCategory string

const (
	CategorySpeech SoundCategory = "speech" // Lời nói con người (hài âm, năng lượng liên tục, ZCR 0.05-0.35)
	CategoryClap   SoundCategory = "clap"   // Tiếng vỗ tay (xung cực nhọn, attack time cực dốc < 20ms, crest factor rất cao)
	CategoryNoise  SoundCategory = "noise"  // Tiếng động/va quẹt, cạch cửa, rơi vỡ, ho, tiếng ồn cơ học
	CategoryQuiet  SoundCategory = "quiet"  // Không gian yên tĩnh
)

type AudioFeatures struct {
	RMS          float64
	Peak         int
	ZeroCrossing float64
	DurationSec  float64
	AttackRatio  float64
	ClapCount    int
	Category     SoundCategory
	Description  string
}

// AnalyzeAudioSegment phân tích đặc trưng phổ âm thanh PCM 16-bit mono 16kHz
func AnalyzeAudioSegment(pcm []byte) AudioFeatures {
	samples := len(pcm) / 2
	if samples == 0 {
		return AudioFeatures{Category: CategoryQuiet, Description: "Yên tĩnh"}
	}

	var sumSq float64
	var zeroCrossings int
	maxPeak := 0
	prevSign := 0

	// Phát hiện xung đỉnh cho tiếng vỗ tay: vỗ tay có attack dốc đứng và suy giảm nhanh trong 50ms-150ms
	// Một buffer có thể chứa 1 hoặc 2 tiếng vỗ tay (đôi)
	peakWindowSize := 16000 * 20 / 1000 // 20ms = 320 samples
	peakThreshold := 3500               // biên độ tối thiểu của tiếng vỗ tay
	clapPeaks := 0
	lastClapSample := -10000

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

		if absPeak > peakThreshold && (i-lastClapSample) > (16000*80/1000) { // cách nhau ít nhất 80ms
			// Kiểm tra năng lượng trước đỉnh 20ms xem có phải attack đột ngột không
			prevRMS := calcLocalRMS(pcm, i-peakWindowSize, i)
			currRMS := calcLocalRMS(pcm, i, i+peakWindowSize)
			if prevRMS < 800 && currRMS > 2500 {
				clapPeaks++
				lastClapSample = i
			}
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
		ClapCount:    clapPeaks,
		Category:     CategoryQuiet,
		Description:  "Yên tĩnh",
	}

	// 0. Lọc không gian yên tĩnh và tiếng tạp âm bên ngoài nhà vọng vào:
	// Tạp âm ngoài đường, xe chạy qua, tiếng gió thường có RMS < 500 hoặc Peak < 3000
	// Giọng nói của chủ nhà trong phòng khách khi ra lệnh thường đạt RMS 600 - 2500, Peak > 3500
	if rms < 500.0 && maxPeak < 3000 {
		return feat
	}

	crestFactor := 0.0
	if rms > 0 {
		crestFactor = float64(maxPeak) / rms
	}

	// 1. Nhận diện Tiếng Vỗ Tay (Hand Clap):
	if clapPeaks > 0 && crestFactor > 6.0 && dur < 1.8 && zcr > 0.06 && zcr < 0.38 {
		feat.Category = CategoryClap
		if clapPeaks >= 2 {
			feat.Description = "Tiếng vỗ tay đôi (Double Clap)"
		} else {
			feat.Description = "Tiếng vỗ tay (Clap)"
		}
		return feat
	}

	// 2. Tiếng động dạng xung ngắn / cơ học (Impulse Noise): cạch cửa, va quẹt, rơi đồ
	// Hoặc âm thanh có ZCR quá thấp (ù gió, quạt, máy bay, xe cộ) hay quá cao (tiếng rít)
	isImpulseNoise := (crestFactor > 12.0 && dur < 0.6) || (crestFactor > 15.0)
	isHighFreqNoise := zcr > 0.38
	isLowFreqRumble := zcr < 0.05 // loại trừ tạp âm xe cộ gầm rú ngoài đường

	if isImpulseNoise || isHighFreqNoise || isLowFreqRumble {
		feat.Category = CategoryNoise
		feat.Description = "Tiếng động / va quẹt cơ học"
	} else {
		// Giọng nói thực sự của chủ nhà: RMS tối thiểu 500, dải ZCR 0.05 - 0.38, thời lượng ít nhất 0.8s
		if dur >= 0.8 {
			feat.Category = CategorySpeech
			feat.Description = "Lời nói con người"
		} else {
			feat.Category = CategoryNoise
			feat.Description = "Âm thanh thoáng qua"
		}
	}

	return feat
}

func calcLocalRMS(pcm []byte, startSample, endSample int) float64 {
	totalSamples := len(pcm) / 2
	if startSample < 0 {
		startSample = 0
	}
	if endSample > totalSamples {
		endSample = totalSamples
	}
	if endSample <= startSample {
		return 0
	}

	var sumSq float64
	count := endSample - startSample
	for i := startSample; i < endSample; i++ {
		val := int16(pcm[2*i]) | (int16(pcm[2*i+1]) << 8)
		v := float64(val)
		sumSq += v * v
	}
	return math.Sqrt(sumSq / float64(count))
}
