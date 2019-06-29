package main

import (
	"log"
	"flag"
	"net/http"
	"io/ioutil"
	"encoding/json"
	"github.com/charles-d-burton/grillbernetes/control-hub/messagebus"
)

var usageStr = `
Usage: pismoker [options]
Options:
	-nh, --nats-host       <NATSHost>     Start the controller connecting to the defined NATS Streaming server
`

func usage() {
	log.Fatalf(usageStr)
}

func main() {
	var natsHost string
	flag.StringVar(&natsHost, "nh", "", "Start the controller connecting to the defined NATS Streaming server")
	flag.StringVar(&natsHost, "nats-host", "", "Start the controller connecting to the defined NATS Streaming server")
	flag.Parse()
	go messagebus.Connect(natsHost)
	http.HandleFunc("/", handleSend)
	log.Fatal(http.ListenAndServe(":7777", nil))
}

func handleSend(rw http.ResponseWriter, req *http.Request) {
	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		log.Println(err)
	}
	log.Println(body)
	var msg messagebus.Message
	err = json.Unmarshal(body, &msg)
	if err != nil {
		log.Println(err)
	}
	messagebus.Publish(&msg)
}