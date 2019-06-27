package messagebus

import (
	stan "github.com/nats-io/go-nats-streaming"
	"github.com/nats-io/go-nats"
	"github.com/satori/go.uuid"
	"log"
	"encoding/json"
)

var (
	pubs        = make(chan *Message, 100)
	isConnected = false
)

type Message struct {
	Topic string `json:"topic"`
	Data  json.RawMessage `json:"data"`
}

func Connect(host string)  {
	log.Println("Connecting to NATS at: ", host)
	nc, err := nats.Connect(host)
	if err != nil {
		log.Fatal(err)
	}
	u1 := uuid.NewV4()
	sc, err := stan.Connect("nats-streaming", u1.String(), stan.NatsConn(nc))
	if err != nil {
		log.Fatal(err)
	}
	isConnected = true
	log.Println("Conntected to NATS Streaming server: ", host)
	for {
		select {
		case message := <- pubs:
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
