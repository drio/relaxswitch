package main

import (
	"bytes"
	_ "embed"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/anisse/alsa"
	"github.com/hajimehoshi/go-mp3"
	mqtt "github.com/eclipse/paho.mqtt.golang"
)

//go:embed enigma.mp3
var enigmaMP3 []byte

const (
	defaultSkipSeconds = 26   // seconds to skip at start of audio playback
	bufferSize         = 4096 // buffer size for audio data chunks
	channels           = 2    // stereo audio (left + right channels)
	bytesPerSample     = 2    // 16-bit PCM audio samples
)

type Config struct {
	MQTTUser  string
	MQTTPass  string
	MQTTURL   string
	MQTTTopic string
}

func loadConfig() Config {
	config := Config{
		MQTTUser:  "shelly",
		MQTTURL:   "tcp://192.168.8.180:1883",
		MQTTTopic: "shellies/shelly1l-test/relay/0",
	}

	if pass := os.Getenv("MQTT_PASS"); pass != "" {
		config.MQTTPass = pass
	}

	if user := os.Getenv("MQTT_USER"); user != "" {
		config.MQTTUser = user
	}

	if url := os.Getenv("MQTT_URL"); url != "" {
		config.MQTTURL = url
	}

	if topic := os.Getenv("MQTT_TOPIC"); topic != "" {
		config.MQTTTopic = topic
	}

	return config
}

var (
	player *alsa.Player
)

func main() {
	config := loadConfig()
	if config.MQTTPass == "" {
		log.Fatalf("no env var MQTT_PASS set")
	}

	log.Printf("starting service")
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	log.Printf("starting mqtt client")
	startMQTT(config)

	log.Printf("waiting")
	<-c
	log.Println("Bye")
	os.Exit(0)
}

func stopAudio() {
	if player != nil {
		player.Close()
		player = nil
		log.Printf("stopped audio playback")
	}
}

func playEmbeddedMP3(skipSeconds int) error {
	log.Printf("playEmbeddedMP3: using embedded MP3 data")
	fileBytesReader := bytes.NewReader(enigmaMP3)
	decoder, err := mp3.NewDecoder(fileBytesReader)
	if err != nil {
		return fmt.Errorf("failed to create MP3 decoder: %w", err)
	}
	log.Printf("playMP3: decoder created, sample rate: %d", decoder.SampleRate())

	sampleRate := decoder.SampleRate()

	// Skip audio if needed
	if skipSeconds > 0 {
		// Calculate bytes to skip: seconds * samples_per_second * channels * bytes_per_sample
		// For stereo 16-bit audio: skipSeconds * sampleRate * 2 channels * 2 bytes
		skipBytes := int64(skipSeconds) * int64(sampleRate) * int64(channels) * int64(bytesPerSample)
		_, err = decoder.Seek(skipBytes, io.SeekStart)
		if err != nil {
			return fmt.Errorf("failed to skip audio: %w", err)
		}
	}

	log.Printf("playMP3: creating ALSA player")
	player, err = alsa.NewPlayer(sampleRate, channels, bytesPerSample, bufferSize)
	if err != nil {
		return fmt.Errorf("failed to create ALSA player: %w", err)
	}

	log.Printf("playMP3: starting playback")
	go func() {
		buf := make([]byte, bufferSize)
		for {
			n, err := decoder.Read(buf)
			if err == io.EOF {
				break
			}
			if err != nil {
				log.Printf("decoder read error: %v", err)
				break
			}
			if _, err := player.Write(buf[:n]); err != nil {
				log.Printf("player write error: %v", err)
				break
			}
		}
	}()

	return nil
}

func handleMQTTMessage(client mqtt.Client, msg mqtt.Message) {
	pl := string(msg.Payload())
	log.Printf("MQTT message received: topic=%s payload='%s'", msg.Topic(), pl)
	switch pl {
	case "on":
		log.Println("msg: on")
		stopAudio()
		if err := playEmbeddedMP3(defaultSkipSeconds); err != nil {
			log.Printf("error playing song: %s", err)
		}
	case "off":
		log.Println("msg: off")
		stopAudio()
	default:
		log.Printf("unknown message payload: '%s'", pl)
	}
}

func startMQTT(config Config) {
	//mqtt.DEBUG = log.New(os.Stdout, "", 0)
	mqtt.ERROR = log.New(os.Stdout, "", 0)
	hostname, _ := os.Hostname()

	id := hostname + strconv.Itoa(time.Now().Second())
	opts := mqtt.NewClientOptions().AddBroker(config.MQTTURL).SetClientID(id).SetCleanSession(true)
	opts.SetUsername(config.MQTTUser)
	opts.SetPassword(config.MQTTPass)

	opts.OnConnect = func(c mqtt.Client) {
		if token := c.Subscribe(config.MQTTTopic, 0, handleMQTTMessage); token.Wait() && token.Error() != nil {
			panic(token.Error())
		}
	}

	c := mqtt.NewClient(opts)
	if token := c.Connect(); token.Wait() && token.Error() != nil {
		panic(token.Error())
	} else {
		fmt.Printf("Connected to %s\n", config.MQTTURL)
	}
}
