package messagebus

import (
	"encoding/json"
	"log"

	"github.com/nats-io/go-nats"
	stan "github.com/nats-io/go-nats-streaming"
	uuid "github.com/satori/go.uuid"
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

	u1 := uuid.NewV4()

	isConnected = false
	log.Println("Conntected to NATS Streaming server: ", host)
	for {
		var sc stan.Conn
		if isConnected {
			select {
			case message := <-pubs:
				err := sc.Publish(message.Topic, message.Data)
				if err != nil {
					log.Println(err)
				}
			}
		} else {
			log.Println("Connecting to NATS at: ", host)
			nc, err := nats.Connect(host)
			if err != nil {
				log.Fatal(err)
			}
			log.Println("Connecting to Streaming")
			sc, err = stan.Connect("nats-streaming", u1.String(), stan.NatsConn(nc))
			if err != nil {
				log.Fatal(err)
			}
		}

	}
}

func Publish(message *Message) {
	pubs <- message
}

func Subscribe(topic string) {

}
