package camera

import (
	"fmt"
	"net/url"
	"time"

	"github.com/AlexxIT/go2rtc/pkg/mpegts"
	"github.com/AlexxIT/go2rtc/pkg/tapo"
)

func dialTapoCamera(host string, user string, password string) (*tapo.Client, error) {
	effectivePassword := password
	if effectivePassword == "" {
		effectivePassword = user
	}

	if effectivePassword != "" {
		cloudEndpoint := fmt.Sprintf(
			"tapo://%s@%s:8800",
			url.QueryEscape(effectivePassword), host,
		)
		client, cloudError := tapo.Dial(cloudEndpoint)
		if cloudError == nil {
			return client, nil
		}
	}

	if user != "" && password != "" {
		userEndpoint := fmt.Sprintf(
			"tapo://%s:%s@%s:8800",
			url.QueryEscape(user), url.QueryEscape(password), host,
		)
		client, userError := tapo.Dial(userEndpoint)
		if userError == nil {
			return client, nil
		}
	}

	adminEndpoint := fmt.Sprintf(
		"tapo://admin:%s@%s:8800",
		url.QueryEscape(effectivePassword), host,
	)
	return tapo.Dial(adminEndpoint)
}

func BroadcastAudio(
	cameraHost string,
	cameraUser string,
	cameraPassword string,
	audioPayload []byte,
) error {
	client, dialError := dialTapoCamera(cameraHost, cameraUser, cameraPassword)
	if dialError != nil {
		return fmt.Errorf("dial error: %w", dialError)
	}
	defer client.Close()

	if backchannelError := client.SetupBackchannel(); backchannelError != nil {
		return fmt.Errorf("setup backchannel error: %w", backchannelError)
	}

	streamMuxer := mpegts.NewMuxer()
	trackIdentifier := streamMuxer.AddTrack(mpegts.StreamTypePCMATapo)
	if headerError := client.WriteBackchannel(streamMuxer.GetHeader()); headerError != nil {
		return fmt.Errorf("write header error: %w", headerError)
	}

	rawPcmData := audioPayload
	if len(rawPcmData) > 44 && string(rawPcmData[0:4]) == "RIFF" {
		rawPcmData = rawPcmData[44:]
	}

	chunkSize := 160
	var timestampOffset uint32

	for offset := 0; offset < len(rawPcmData); offset += chunkSize {
		endOffset := offset + chunkSize
		if endOffset > len(rawPcmData) {
			endOffset = len(rawPcmData)
		}

		currentChunk := rawPcmData[offset:endOffset]
		chunkPacket := streamMuxer.GetPayload(
			trackIdentifier, timestampOffset, currentChunk,
		)
		if writeError := client.WriteBackchannel(chunkPacket); writeError != nil {
			return fmt.Errorf("write backchannel chunk error: %w", writeError)
		}

		timestampOffset += uint32(len(currentChunk))
		time.Sleep(20 * time.Millisecond)
	}

	return nil
}
