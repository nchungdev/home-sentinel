package main

import (
	"flag"
	"fmt"
	"net/url"
	"os"
	"time"

	"github.com/AlexxIT/go2rtc/pkg/mpegts"
	"github.com/AlexxIT/go2rtc/pkg/tapo"
)

func resolveArguments() (string, string) {
	hostFlag := flag.String("host", "", "camera host IP")
	portFlag := flag.String("port", "8800", "camera port")
	userFlag := flag.String("user", "", "username")
	passFlag := flag.String("pass", "", "password")
	wavFlag := flag.String("wav", "", "wav file path")
	flag.Parse()

	if *hostFlag != "" && *wavFlag != "" {
		encodedUser := url.QueryEscape(*userFlag)
		encodedPass := url.QueryEscape(*passFlag)
		targetPort := *portFlag
		if targetPort == "" {
			targetPort = "8800"
		}
		endpoint := fmt.Sprintf(
			"tapo://%s:%s@%s:%s",
			encodedUser, encodedPass, *hostFlag, targetPort,
		)
		return endpoint, *wavFlag
	}

	positional := flag.Args()
	if len(positional) >= 2 {
		return positional[0], positional[1]
	}

	if len(os.Args) >= 3 && os.Args[1] != "-host" {
		return os.Args[1], os.Args[2]
	}

	fmt.Println("Usage: tapo_speaker <tapo_url> <wav_path> or -host <ip> -wav <path>")
	os.Exit(1)
	return "", ""
}

func main() {
	tapoURL, wavPath := resolveArguments()

	client, dialError := tapo.Dial(tapoURL)
	if dialError != nil {
		fmt.Printf("Dial error: %v\n", dialError)
		os.Exit(1)
	}
	defer client.Close()

	if setupError := client.SetupBackchannel(); setupError != nil {
		fmt.Printf("SetupBackchannel error: %v\n", setupError)
		os.Exit(1)
	}

	muxer := mpegts.NewMuxer()
	pid := muxer.AddTrack(mpegts.StreamTypePCMATapo)
	if headerError := client.WriteBackchannel(muxer.GetHeader()); headerError != nil {
		fmt.Printf("Write header error: %v\n", headerError)
		os.Exit(1)
	}

	rawPcm, readError := os.ReadFile(wavPath)
	if readError != nil {
		fmt.Printf("Read wav error: %v\n", readError)
		os.Exit(1)
	}
	if len(rawPcm) > 44 {
		rawPcm = rawPcm[44:]
	}

	chunkSize := 160
	var timestamp uint32 = 0
	for index := 0; index < len(rawPcm); index += chunkSize {
		endIndex := index + chunkSize
		if endIndex > len(rawPcm) {
			endIndex = len(rawPcm)
		}
		chunk := rawPcm[index:endIndex]
		payload := muxer.GetPayload(pid, timestamp, chunk)
		if writeError := client.WriteBackchannel(payload); writeError != nil {
			fmt.Printf("Write payload error: %v\n", writeError)
			break
		}
		timestamp += uint32(len(chunk))
		time.Sleep(20 * time.Millisecond)
	}
	fmt.Println("Audio playback completed successfully!")
}
