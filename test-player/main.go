package main

import (
	"io"
	"log"
	"os"

	"github.com/anisse/alsa"
	"github.com/hajimehoshi/go-mp3"
)

func main() {
	const seekSeconds = 26
	const channels = 2
	const bytesPerSample = 2 // 16-bit PCM

	f, err := os.Open(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	decoder, err := mp3.NewDecoder(f)
	if err != nil {
		log.Fatal(err)
	}

	sampleRate := decoder.SampleRate()
	bytesPerSecond := sampleRate * channels * bytesPerSample
	skipBytes := int64(seekSeconds * bytesPerSecond)

	// Skip PCM bytes (decoder starts at PCM output, not MP3 compressed stream)
	_, err = decoder.Seek(skipBytes, io.SeekStart)
	if err != nil {
		log.Fatalf("seek error: %v", err)
	}

	player, err := alsa.NewPlayer(sampleRate, channels, bytesPerSample, 4096)
	if err != nil {
		log.Fatal(err)
	}
	defer player.Close()

	buf := make([]byte, 4096)
	for {
		n, err := decoder.Read(buf)
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatal(err)
		}
		if _, err := player.Write(buf[:n]); err != nil {
			log.Fatal(err)
		}
	}
}

