package messagebus

import (
	"encoding/json"
	"errors"
	"log"
	"time"

	"github.com/jeffchao/backoff"
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
	f := backoff.Fibonacci()
	f.Interval = 1 * time.Millisecond
	f.MaxRetries = 20
	cleanup := make(chan error, 1)
	for {
		connect := func() error {
			log.Println("Connecting to NATS at: ", host)
			nc, err := nats.Connect(host)
			if err != nil {
				return err
			}
			sc, err := stan.Connect("nats-streaming", "smoker-client", stan.NatsConn(nc),
				stan.SetConnectionLostHandler(func(_ stan.Conn, reason error) {
					log.Println(reason)
					cleanup <- reason
				}))
			if err != nil {
				return err
			}
			isConnected = true
			log.Println("Conntected to NATS Streaming server: ", host)
			select {
			case message := <-pubs:
				err := sc.Publish(message.Topic, message.Data)
				if err != nil {
					return err
				}
			case err := <-cleanup:
				return err
			}
			return errors.New("An unknown error occured")
		}
		err := f.Retry(connect)
		if err != nil {
			log.Println(err)
		}
		f.Reset()
	}
}

func Publish(message *Message) {
	pubs <- message
}

func Subscribe(topic string) {

}
