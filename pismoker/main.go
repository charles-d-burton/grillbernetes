package main

import (
	"flag"
	"log"

	"github.com/charles-d-burton/grillbernetes/pismoker/controller"
)

var usageStr = `
Usage: pismoker [options]
Options:
	-nh, --nats-host       <NATSHost>     Start the controller connecting to the defined NATS Streaming server
	-pt, --publish-topic   <Topic>        Topic to publish messages to in NATS
	-ct, --control-topic   <Topic>        Topic to listen for control messages
`

func usage() {
	log.Fatalf(usageStr)
}

// NOTE: Use tls scheme for TLS, e.g. stan-sub -s tls://demo.nats.io:4443 foo
func main() {
	var natsHost string
	var publishTopic string
	var controlTopic string
	flag.StringVar(&natsHost, "nh", "", "Start the controller connecting to the defined NATS Streaming server")
	flag.StringVar(&natsHost, "nats-host", "", "Start the controller connecting to the defined NATS Streaming server")
	flag.StringVar(&publishTopic, "pt", "smoker-readings", "Topic to publish readings to in NATS")
	flag.StringVar(&publishTopic, "publish-topic", "smoker-readings", "Topic to publish readings to in NATS")
	flag.StringVar(&controlTopic, "ct", "smoker-controls", "Topic to listen for control messages")
	flag.StringVar(&controlTopic, "control-topic", "smoker-controls", "Topic to listen for control messages")

	flag.Parse()
	if natsHost == "" {
		usage()
	}
	controller.StartServer(natsHost, publishTopic, controlTopic)
}
