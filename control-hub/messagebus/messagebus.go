package messagebus

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/jeffchao/backoff"
	"github.com/nats-io/go-nats"
	stan "github.com/nats-io/go-nats-streaming"
	"github.com/sirupsen/logrus"
)

var (
	pubs        = make(chan *Message, 100)
	isConnected = false
	log         = logrus.New()
	sc          stan.Conn //Will be used in migration go K8S only
)

//Message data to publish to server
type Message struct {
	Topic string          `json:"topic"`
	Data  json.RawMessage `json:"data"`
}

func init() {
	log.SetFormatter(&logrus.JSONFormatter{})
}

//Connect handles connections to the NATS Streaming server
func Connect(host string) {
	f := backoff.Fibonacci()
	f.Interval = 100 * time.Millisecond
	f.MaxRetries = 20
	cleanup := make(chan error, 1)
	for {
		connect := func() error {
			log.Info("Connecting to NATS at: ", host)
			nc, err := nats.Connect(host)
			if err != nil {
				return err
			}
			sc, err := stan.Connect("nats-streaming", "smoker-client", stan.NatsConn(nc),
				stan.SetConnectionLostHandler(func(_ stan.Conn, reason error) {
					log.Info(reason)
					cleanup <- reason
					return
				}))
			if err != nil {
				return err
			}
			isConnected = true
			log.Info("Conntected to NATS Streaming server: ", host)
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
			log.Error(err)
		}
		f.Reset()
	}
}

//Publish takes a message and puts it on the publish queue
func Publish(message *Message) {
	pubs <- message
}

//Subscribe currently unused
func Subscribe(topic string) {

}
