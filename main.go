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
	defaultSkipSeconds = 26
	bufferSize         = 4096
	channels           = 2
	bytesPerSample     = 2
)

var (
	player *alsa.Player
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

	sampleRate := decoder.SampleRate()

	// Skip audio if needed
	if skipSeconds > 0 {
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
			if err := playEmbeddedMP3(defaultSkipSeconds); err != nil {
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
