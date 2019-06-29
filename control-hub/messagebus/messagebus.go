package messagebus

import (
	"encoding/json"
	"log"

	"github.com/nats-io/go-nats"
	stan "github.com/nats-io/go-nats-streaming"
)

var (
	pubs        = make(chan *Message, 100)
	isConnected = false
)

type Message struct {
	Topic string          `json:"topic"`
	Data  json.RawMessage `json:"data"`
}

func Connect(host string) {
	log.Println("Connecting to NATS at: ", host)
	nc, err := nats.Connect(host)
	if err != nil {
		log.Fatal(err)
	}
	sc, err := stan.Connect("nats-streaming", "smoker-client", stan.NatsConn(nc))
	if err != nil {
		log.Fatal(err)
	}
	isConnected = true
	log.Println("Conntected to NATS Streaming server: ", host)
	for {
		select {
		case message := <-pubs:
			err := sc.Publish(message.Topic, message.Data)
			if err != nil {
				log.Println(err)
			}
		}
	}
}

func Publish(message *Message) {
	pubs <- message
}

func Subscribe(topic string) {

}
