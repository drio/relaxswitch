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

	"github.com/ebitengine/oto/v3"
	"github.com/hajimehoshi/go-mp3"
	mqtt "github.com/eclipse/paho.mqtt.golang"
)

//go:embed enigma.mp3
var enigmaMP3 []byte

var (
	otoCtx *oto.Context
	player *oto.Player
)

func main() {
	pass := os.Getenv("MQTT_PASS")
	if pass == "" {
		log.Fatalf("no env var MQTT_PASS set")
	}

	user := os.Getenv("MQTT_USER")
	if user == "" {
		user = "shelly"
	}

	url := os.Getenv("MQTT_URL")
	if url == "" {
		url = "tcp://192.168.8.180:1883"
	}

	topic := os.Getenv("MQTT_TOPIC")
	if topic == "" {
		topic = "shellies/shelly1l-test/relay/0"
	}

	log.Printf("starting service")
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	log.Printf("starting mqtt client")
	startMQTT(user, pass, url, topic)

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

	if otoCtx == nil {
		log.Printf("playMP3: creating audio context")
		op := &oto.NewContextOptions{
			SampleRate:   44100,
			ChannelCount: 2,
			Format:       oto.FormatSignedInt16LE,
		}
		
		var readyChan chan struct{}
		otoCtx, readyChan, err = oto.NewContext(op)
		if err != nil {
			return fmt.Errorf("failed to create audio context: %w", err)
		}
		log.Printf("playMP3: waiting for audio context to be ready")
		<-readyChan
		log.Printf("playMP3: audio context ready")
	}

	// Skip audio if needed
	if skipSeconds > 0 {
		skipBytes := int64(skipSeconds) * int64(decoder.SampleRate()) * 4 // 2 channels * 2 bytes per sample
		_, err = io.CopyN(io.Discard, decoder, skipBytes)
		if err != nil && err != io.EOF {
			return fmt.Errorf("failed to skip audio: %w", err)
		}
	}

	log.Printf("playMP3: creating player")
	player = otoCtx.NewPlayer(decoder)
	log.Printf("playMP3: starting playback")
	player.Play()

	return nil
}

func startMQTT(user, pass, url, topic string) {
	//mqtt.DEBUG = log.New(os.Stdout, "", 0)
	mqtt.ERROR = log.New(os.Stdout, "", 0)
	hostname, _ := os.Hostname()

	id := hostname + strconv.Itoa(time.Now().Second())
	opts := mqtt.NewClientOptions().AddBroker(url).SetClientID(id).SetCleanSession(true)
	opts.SetUsername(user)
	opts.SetPassword(pass)

	onMessageReceived := (func(client mqtt.Client, msg mqtt.Message) {
		pl := string(msg.Payload())
		log.Printf("MQTT message received: topic=%s payload='%s'", msg.Topic(), pl)
		switch pl {
		case "on":
			log.Println("msg: on")
			stopAudio()
			if err := playEmbeddedMP3(26); err != nil {
				log.Printf("error playing song: %s", err)
			}
		case "off":
			log.Println("msg: off")
			stopAudio()
		default:
			log.Printf("unknown message payload: '%s'", pl)
		}
	})

	opts.OnConnect = func(c mqtt.Client) {
		if token := c.Subscribe(topic, 0, onMessageReceived); token.Wait() && token.Error() != nil {
			panic(token.Error())
		}
	}

	c := mqtt.NewClient(opts)
	if token := c.Connect(); token.Wait() && token.Error() != nil {
		panic(token.Error())
	} else {
		fmt.Printf("Connected to %s\n", url)
	}
}
