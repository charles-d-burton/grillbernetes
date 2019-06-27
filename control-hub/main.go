package main

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"github.com/charles-d-burton/control-hub/messagebus"
)

var usageStr = `
Usage: pismoker [options]
Options:
	-nh, --nats-host       <NATSHost>     Start the controller connecting to the defined NATS Streaming server
	NATS_HOST="<ENV>"
`

func usage() {
	log.Fatalf(usageStr)
}

func main() {
	var natsHost string
	flag.StringVar(&natsHost, "nh", "", "Start the controller connecting to the defined NATS Streaming server")
	flag.StringVar(&natsHost, "nats-host", "", "Start the controller connecting to the defined NATS Streaming server")
	flag.Parse()
	if natsHost == "" {
		natsHost = os.Getenv("NATS_HOST")
		if natsHost == "" {
			usage()
		}
	}
	go messagebus.Connect(natsHost)
	http.HandleFunc("/send", handleSend)
	log.Fatal(http.ListenAndServe(":7777", nil))
}

func handleSend(rw http.ResponseWriter, req *http.Request) {
	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		log.Println(err)
	}
	log.Println(string(body))
	var msg messagebus.Message
	err = json.Unmarshal(body, &msg)
	if err != nil {
		log.Println(err)
	}
	messagebus.Publish(&msg)
}
