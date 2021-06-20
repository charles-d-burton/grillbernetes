package main

import (
	"fmt"
	"os"
	"time"

	"github.com/nats-io/nats.go"
	log "github.com/sirupsen/logrus"
)

const (
	streamName = "grillbernetes"
)

var (
	subjects = []string{
		"pub-hub",
		"events",
		"control-hub",
	}
)

func main() {
	log.SetFormatter(&log.JSONFormatter{})
	natsHost := os.Getenv("NATS_HOST")
	natsPort := os.Getenv("NATS_PORT")
	if natsHost == "" {
		log.Fatal("nats host not set")
	}
	if natsPort == "" {
		natsPort = "4222"
	}

	conn, err := nats.Connect(fmt.Sprintf("nats://%q:%q", natsHost, natsPort))
	if err != nil {
		log.Fatal(err)
	}
	js, err := conn.JetStream()
	if err != nil {
		log.Fatal(err)
	}

	info, err := js.StreamInfo(streamName)
	if err != nil {
		log.Error(err)
	}
	streamConfig := &nats.StreamConfig{
		Name:     streamName,
		Subjects: subjects,
		MaxAge:   24 * time.Hour,
	}
	if info == nil {
		_, err := js.AddStream(streamConfig)
		if err != nil {
			log.Fatal(err)
		}
		os.Exit(0)
	}
	_, err = js.UpdateStream(streamConfig)
	if err != nil {
		log.Fatal(err)
	}

}
