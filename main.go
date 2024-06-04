package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

var cmd *exec.Cmd

func main() {
	varName := "MQTT_PASS"
	pass := os.Getenv(varName)
	if pass == "" {
		log.Fatalf("no env var MQTT_PASS set")
	}

	log.Printf("starting service")
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	log.Printf("starting mqtt client")
	startMQTT(pass)

	log.Printf("waiting")
	<-c
	log.Println("Bye")
	os.Exit(0)
}

func kill() {
	if cmd == nil {
		log.Printf("nothing to kill")
		return
	}

	err := cmd.Process.Kill()
	if err != nil {
		log.Printf("error killing process: %s", err)
	}
	log.Printf("killing play ok")
}

func startMQTT(pass string) {
	//mqtt.DEBUG = log.New(os.Stdout, "", 0)
	mqtt.ERROR = log.New(os.Stdout, "", 0)
	hostname, _ := os.Hostname()

	//url := "tcp://192.168.8.180:1883"
	url := "tcp://gopher:1883"
	user := "shelly"
	topic := "shellies/shelly1l-test/relay/0"

	id := hostname + strconv.Itoa(time.Now().Second())
	opts := mqtt.NewClientOptions().AddBroker(url).SetClientID(id).SetCleanSession(true)
	opts.SetUsername(user)
	opts.SetPassword(pass)

	onMessageReceived := (func(client mqtt.Client, msg mqtt.Message) {
		log.Println("msg here")
		pl := string(msg.Payload())
		switch pl {
		case "on":
			//
			log.Println("msg: on")
			kill()
			cmd = exec.Command("play", "./enigma.mp3", "trim", "26")
			if err := cmd.Start(); err != nil {
				log.Printf("error playing song: %s", err)
			}
		case "off":
			log.Println("msg: off")
			kill()
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
