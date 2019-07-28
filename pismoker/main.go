package main

import (
	"flag"
	"log"
	"os"
	"strings"

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
	var machineName string
	flag.StringVar(&natsHost, "nh", "", "Start the controller connecting to the defined NATS Streaming server")
	flag.StringVar(&natsHost, "nats-host", "", "Start the controller connecting to the defined NATS Streaming server")
	flag.StringVar(&machineName, "n", "", "Name of the machine you're working with, defaults to hostname")
	flag.StringVar(&machineName, "name", "", "Name of the machine you're working with, defaults to hostname")
	flag.Parse()
	if natsHost == "" {
		usage()
	}
	if machineName == "" {
		name, err := os.Hostname()
		if err != nil {
			log.Fatal(err)
		}
		machineName = strings.Replace(name, ".", "-", -1)
	}
	controller.StartServer(natsHost, machineName+"-readings", machineName+"-control")
}
