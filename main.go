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
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/hajimehoshi/go-mp3"
)

//go:embed enigma.mp3
var enigmaMP3 []byte

const (
	defaultSkipSeconds = 26   // seconds to skip at start of audio playback
	bufferSize         = 4096 // buffer size for audio data chunks
	channels           = 2    // stereo audio (left + right channels)
	bytesPerSample     = 2    // 16-bit PCM audio samples
	
	// Config defaults
	defaultMQTTUser  = "shelly"
	defaultMQTTURL   = "tcp://192.168.8.180:1883"
	defaultMQTTTopic = "shellies/shelly1l-test/relay/0"
)

type Config struct {
	MQTTUser  string
	MQTTPass  string
	MQTTURL   string
	MQTTTopic string
}

type Player interface {
	Write([]byte) (int, error)
	Close() error
}

type AudioManager struct {
	player        Player
	playerFactory func() (Player, error)
	playing       bool
	doneChan      chan struct{}
	lastState     string
}

func NewAudioManager(playerFactory func() (Player, error)) *AudioManager {
	return &AudioManager{
		playerFactory: playerFactory,
	}
}

func main() {
	config := loadConfig()
	if config.MQTTPass == "" {
		log.Fatalf("no env var MQTT_PASS set")
	}

	// Create factory function for ALSA players
	playerFactory := func() (Player, error) {
		return alsa.NewPlayer(44100, channels, bytesPerSample, bufferSize)
	}

	audioManager := NewAudioManager(playerFactory)

	log.Printf("starting service")
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	log.Printf("starting mqtt client")
	if err := startMQTT(config, audioManager); err != nil {
		log.Fatalf("failed to start MQTT: %v", err)
	}

	log.Printf("waiting")
	<-c
	log.Println("Bye")
	os.Exit(0)
}

func loadConfig() Config {
	config := Config{
		MQTTUser:  defaultMQTTUser,
		MQTTURL:   defaultMQTTURL,
		MQTTTopic: defaultMQTTTopic,
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

func (am *AudioManager) stopAudio() {
	if am.playing {
		am.playing = false
		log.Printf("stopping audio playback")
	}

	// Wait for goroutine to finish
	if am.doneChan != nil {
		<-am.doneChan
		am.doneChan = nil
	}

	// Close player
	if am.player != nil {
		am.player.Close()
		am.player = nil
		log.Printf("closed audio player")
	}
}

func (am *AudioManager) createDecoder(skipSeconds int) (*mp3.Decoder, error) {
	log.Printf("creating MP3 decoder from embedded data")
	fileBytesReader := bytes.NewReader(enigmaMP3)
	decoder, err := mp3.NewDecoder(fileBytesReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create MP3 decoder: %w", err)
	}
	log.Printf("decoder created, sample rate: %d", decoder.SampleRate())

	// Skip audio if needed
	if skipSeconds > 0 {
		sampleRate := decoder.SampleRate()
		// Calculate bytes to skip: seconds * samples_per_second * channels * bytes_per_sample
		// For stereo 16-bit audio: skipSeconds * sampleRate * 2 channels * 2 bytes
		skipBytes := int64(skipSeconds) * int64(sampleRate) * int64(channels) * int64(bytesPerSample)
		_, err = decoder.Seek(skipBytes, io.SeekStart)
		if err != nil {
			return nil, fmt.Errorf("failed to skip audio: %w", err)
		}
	}

	return decoder, nil
}

func (am *AudioManager) startPlaybackGoroutine(decoder *mp3.Decoder) {
	// Set up new playback session
	am.playing = true
	am.doneChan = make(chan struct{})

	log.Printf("starting playback goroutine")
	go func() {
		// Capture references to avoid race with stopAudio()
		currentPlayer := am.player
		currentDoneChan := am.doneChan
		defer func() {
			close(currentDoneChan)
		}()

		buf := make([]byte, bufferSize)
		for am.playing {
			n, err := decoder.Read(buf)
			if err == io.EOF {
				log.Printf("playback finished")
				am.playing = false
				return
			}
			if err != nil {
				log.Printf("decoder read error: %v", err)
				return
			}
			if _, err := currentPlayer.Write(buf[:n]); err != nil {
				log.Printf("player write error: %v", err)
				return
			}
		}
	}()
}

func (am *AudioManager) playEmbeddedMP3(skipSeconds int) error {
	// Stop any existing playback first
	am.stopAudio()

	decoder, err := am.createDecoder(skipSeconds)
	if err != nil {
		return err
	}

	log.Printf("creating new player")
	player, err := am.playerFactory()
	if err != nil {
		return fmt.Errorf("failed to create player: %w", err)
	}
	am.player = player

	am.startPlaybackGoroutine(decoder)
	return nil
}

func createMessageHandler(am *AudioManager) mqtt.MessageHandler {
	return func(client mqtt.Client, msg mqtt.Message) {
		pl := string(msg.Payload())
		log.Printf("MQTT message received: topic=%s payload='%s'", msg.Topic(), pl)

		// The shelly sends heartbeats with the same topic and value
		if pl == am.lastState {
			log.Printf("same state received (%s), ignoring", pl)
			return
		}

		switch pl {
		case "on":
			log.Println("msg: on")
			if err := am.playEmbeddedMP3(defaultSkipSeconds); err != nil {
				log.Printf("error playing song: %s", err)
			} else {
				am.lastState = "on"
			}
		case "off":
			log.Println("msg: off")
			am.stopAudio()
			am.lastState = "off"
		default:
			log.Printf("unknown message payload: '%s'", pl)
		}
	}
}

func createMQTTClient(config Config, messageHandler mqtt.MessageHandler) (mqtt.Client, error) {
	//mqtt.DEBUG = log.New(os.Stdout, "", 0)
	mqtt.ERROR = log.New(os.Stdout, "", 0)
	hostname, _ := os.Hostname()

	id := hostname + strconv.Itoa(time.Now().Second())
	opts := mqtt.NewClientOptions().AddBroker(config.MQTTURL).SetClientID(id).SetCleanSession(true)
	opts.SetUsername(config.MQTTUser)
	opts.SetPassword(config.MQTTPass)

	opts.OnConnect = func(c mqtt.Client) {
		if token := c.Subscribe(config.MQTTTopic, 0, messageHandler); token.Wait() && token.Error() != nil {
			panic(token.Error())
		}
	}

	return mqtt.NewClient(opts), nil
}

func startMQTT(config Config, am *AudioManager) error {
	messageHandler := createMessageHandler(am)
	c, err := createMQTTClient(config, messageHandler)
	if err != nil {
		return fmt.Errorf("failed to create MQTT client: %w", err)
	}

	if token := c.Connect(); token.Wait() && token.Error() != nil {
		return fmt.Errorf("failed to connect to MQTT broker: %w", token.Error())
	}

	fmt.Printf("Connected to %s\n", config.MQTTURL)
	return nil
}
