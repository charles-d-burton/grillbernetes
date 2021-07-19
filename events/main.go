package main

import (
	"encoding/json"
	"errors"
	"flag"
	"io"
	"math/rand"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/jeffchao/backoff"
	nats "github.com/nats-io/nats.go"
	"github.com/sirupsen/logrus"
)

const (
	queuelen   = 100
	streamName = "EVENTS"
)

var (
	usageStr = `
Usage: pismoker [options]
Options:
	-nh, --nats-host       <NATSHost>     Start the controller connecting to the defined NATS Streaming server
	-pt, --publish-topic   <Topic>        Topic to publish messages to in NATS
	-st, --subscribe-topic   <Topic>      Topic to listen for upate messages
	-d,  --debug             <Nothing>    Debug flag, enables CORS
`
	log = logrus.New()
)

func init() {
	log.SetFormatter(&logrus.JSONFormatter{})
}

//Env place to hold a reference to the NATSConnection
type Env struct {
	natsConn *NATSConnection
}

//NATSConnection holds the connection and status information of the NATS backend
type NATSConnection struct {
	sync.RWMutex
	Conn        *nats.Conn
	NatsHost    string
	subscribers map[string]*Subscriber
}

//Subscriber non-blocking broker of NATS messages to HTTP clients
type Subscriber struct {
	topic           string
	sub             *nats.Subscription
	connEstablished chan bool
	clients         map[chan []byte]bool
	newClients      chan chan []byte
	defunctClients  chan chan []byte
	messages        chan []byte
	errors          chan error
}

//Message message object to send back to subsriber
type Message struct {
	Timestamp int64           `json:"timestamp"`
	Datum     json.RawMessage `json:"data"`
}

func main() {
	log.Info("Starting Grillbernetes Event Source")

	var natsHost string
	var publishTopic string
	var mockGen = false
	var debug = false
	flag.StringVar(&natsHost, "nh", "", "Start the controller connecting to the defined NATS Streaming server")
	flag.StringVar(&natsHost, "nats-host", "", "Start the controller connecting to the defined NATS Streaming server")
	flag.StringVar(&publishTopic, "pt", "smoker-controls", "Topic to publish readings to in NATS")
	flag.StringVar(&publishTopic, "publish-topic", "smoker-controls", "Topic to publish readings to in NATS")
	//flag.BoolVar(&mockGen, "mock", false, "Generate mock data")
	flag.BoolVar(&debug, "d", false, "Turn on Debugging/Cors")
	flag.BoolVar(&debug, "debug", false, "Turn on Debugging/Cors")

	flag.Parse()
	if natsHost == "" {
		natsHost = os.Getenv("NATS_HOST")
		if natsHost == "" && !mockGen {
			log.Fatal(usageStr)
		}
	}

	router := gin.Default()
	if debug {
		router.Use(cors.New(cors.Config{
			AllowAllOrigins:  true,
			AllowHeaders:     []string{"Origin"},
			AllowMethods:     []string{"PUT", "PATCH", "GET"},
			AllowCredentials: true,
			ExposeHeaders:    []string{"Content-Length"},
		}))
	}
	if mockGen {
		router.GET("/events/:device/:channel", MockGen)

	} else {
		// Make a new Broker instance
		nc := &NATSConnection{
			NatsHost:    natsHost,
			subscribers: make(map[string]*Subscriber, 10),
		}
		nc.Connect()
		env := &Env{nc}
		router.GET("/events/:group/:device/:channel", env.SubscribeSSE)
		router.GET("/stream/:group/:device/:channel", env.SubscribeSSE)
		router.GET("/ws/:group/:device/:channel", env.SubscribeWSS)
		router.GET("/healthz", env.HealthCheck)

	}
	router.Run(":7777")
}

func (env *Env) HealthCheck(c *gin.Context) {
	//TODO: monitor the NATS connection
	c.JSON(http.StatusOK, gin.H{"status": "alive"})
}

//Connect to the NATS remote host with backoff
func (conn *NATSConnection) Connect() {
	log.Info("Starting NATS Connection handler")
	go func() {
		f := backoff.Fibonacci()
		f.Interval = 100 * time.Millisecond
		f.MaxRetries = 60
		connect := func() error {
			cleanup := make(chan bool, 1)
			log.Info("Connecting to NATS at: ", conn.NatsHost)
			nc, err := nats.Connect(conn.NatsHost)
			if err != nil {
				log.Fatal(err)
			}
			conn.Lock()
			conn.Conn = nc //Save the connection
			conn.Unlock()
			log.Info("NATS Connected")
			if len(conn.subscribers) > 0 {
				for _, sub := range conn.subscribers {
					sub.connEstablished <- true //Let the subscriptions know the connections was established
				}
			}
			js, err := conn.Conn.JetStream()
			if err != nil {
				log.Fatal(err)
			}
			stream, err := js.StreamInfo(streamName)
			if err != nil {
				log.Error(err)
			}
			config := &nats.StreamConfig{
				Name:     streamName,
				Subjects: []string{streamName + ".>"},
			}
			if stream == nil {
				log.Infof("creating stream %v", streamName)
				_, err := js.AddStream(config)
				if err != nil {
					log.Fatal(err)
				}
			} else {
				log.Infof("updating stream %v", streamName)
				_, err := js.UpdateStream(config)
				if err != nil {
					log.Fatal(err)
				}
			}
			<-cleanup //Wait for cleanup signal
			return errors.New("connection lost")
		}
		err := f.Retry(connect)
		if err != nil {
			log.Fatal(err) //Unable to reconnect, dying
		}
	}()
}

//GetSubscriber checks for a subscriber, if none is found it creates a new one
func (conn *NATSConnection) GetSubscriber(topic string) (*Subscriber, error) {
	conn.Lock()
	defer conn.Unlock()
	subscriber, found := conn.subscribers[topic]

	if found && subscriber.sub.IsValid() {
		log.Info("Subscriber found for topic: ", topic)
		return subscriber, nil
	}
	if subscriber != nil && !subscriber.sub.IsValid() {
		log.Infof("Sub for %v topic is invalid, establishing new sub", topic)
		delete(conn.subscribers, subscriber.topic)
	}
	log.Info("No subscriber found for topic: ", topic)
	log.Info("Creating new subscriber")
	subscriber = &Subscriber{
		topic:           topic,
		connEstablished: make(chan bool, 1),
		clients:         make(map[chan []byte]bool, 10),
		newClients:      make(chan (chan []byte)),
		defunctClients:  make(chan (chan []byte)),
		messages:        make(chan []byte, 10),
		errors:          make(chan error, 1),
	}
	err := subscriber.Start(conn)
	if err != nil {
		return nil, err
	}
	conn.subscribers[topic] = subscriber
	return subscriber, nil
}

//DeleteSubscriber cleans up subscribers that have been removed
func (conn *NATSConnection) DeleteSubscriber(subscriber *Subscriber) error {
	log.Info("Locked connection, deleting subscriber")
	conn.Lock()
	defer conn.Unlock()
	delete(conn.subscribers, subscriber.topic)
	err := subscriber.sub.Unsubscribe()
	if err != nil {
		return err
	}
	return nil
}

//Start process messages from the subscription and fan out to listeners, also handles subscription status
func (subscriber *Subscriber) Start(conn *NATSConnection) error {
	log.Info("Starting new subscriber for topic: ", subscriber.topic)
	sub, err := subscriber.Subscribe(conn) //First subscribe to my topic
	if err != nil {
		log.Error(err)
		subscriber.errors <- err
		return err
	}
	subscriber.sub = sub
	go func() {
		for {
			select {
			case s := <-subscriber.newClients:
				subscriber.clients[s] = true
				log.Info("Added new subscriber to: ", subscriber.topic)
			case s := <-subscriber.defunctClients:
				delete(subscriber.clients, s)
				log.Info("Removed subscriber from: ", subscriber.topic)
				if len(subscriber.clients) == 0 { //No more clients to service, fully cleanup
					log.Info("No more clients, removing subscriber")
					if subscriber.sub != nil {
						err := conn.DeleteSubscriber(subscriber)
						if err != nil {
							log.Error(err)
							subscriber.errors <- err
						}
						log.Info("Connection cleaned up, exiting subscriber")
					}
					return
				}
			case msg := <-subscriber.messages:
				for queue := range subscriber.clients {
					if len(queue) < queuelen { //Skip client if their queue is full
						queue <- msg
						continue
					}
					log.Info("Subscriber queue full, dropping message")
				}
			case est := <-subscriber.connEstablished:
				if est {
					sub, err := subscriber.Subscribe(conn) //Connection was re-established, start working again
					if err != nil {
						log.Error(err)
						conn.DeleteSubscriber(subscriber)
						subscriber.errors <- err
						return
					}
					subscriber.sub = sub
				} else {
					err := conn.DeleteSubscriber(subscriber)
					if err != nil {

						log.Errorf("Problem unsubscribing: %v ", err)
						subscriber.errors <- err
						return
					}
				}
			}
		}
	}()
	return nil
}

//Subscribe to a given topic in NATS
func (subscriber *Subscriber) Subscribe(conn *NATSConnection) (*nats.Subscription, error) {
	log.Info("Initializing callback")
	log.Info("Subscription topic is: ", subscriber.topic)
	js, err := conn.Conn.JetStream()
	if err != nil {
		return nil, err
	}
	sub, err := js.Subscribe(streamName+"."+subscriber.topic, func(m *nats.Msg) {
		meta, _ := m.Metadata()
		log.Infof("Stream Sequence  : %v\n", meta.Sequence.Stream)
		log.Infof("Consumer Sequence: %v\n", meta.Sequence.Consumer)
		var msg Message
		msg.Timestamp = meta.Timestamp.Unix()
		msg.Datum = m.Data
		data, err := json.Marshal(&msg)
		if err != nil {
			log.Error(err)
		} else {
			subscriber.messages <- data
		}
	}, nats.DeliverNew())
	if err != nil {
		return nil, err
	}

	return sub, nil
}

//MockGen Generates a mock stream of data
func MockGen(c *gin.Context) {
	log.Info("Mock Generator started")
	var id = "3b-6cfc0958d2fb"
	device := c.Param("device")
	channel := c.Param("channel")
	topic := "/" + device + "/" + channel
	log.Info("Sending messages to topic: ", topic)
	ticker := time.NewTicker(1 * time.Second)
	var datum = make(map[string]interface{}, 2)
	//var data = make(map[string]interface{}, 1)
	var temps = make(map[string]interface{}, 3)

	clientGone := c.Writer.CloseNotify()
	buffer := make(chan string, 100)
	go func() {
		for range ticker.C {
			rand.Seed(time.Now().UnixNano())
			datum["timestamp"] = time.Now().UnixNano() / int64(time.Millisecond)
			temps["id"] = id
			temps["f"] = rand.Intn(300-50) + 50
			temps["c"] = rand.Intn(150-20) + 20
			datum["data"] = temps
			jsondata, err := json.Marshal(datum)
			log.Info("Generated message", string(jsondata))
			if err != nil {
				log.Error(err)
			}
			select {
			case buffer <- string(jsondata):
			default:
			}
		}
	}()
	c.Stream(func(w io.Writer) bool {
		select {
		case <-clientGone:
			log.Info("Stopping generator")
			ticker.Stop()
			return true
		case message := <-buffer:
			c.JSON(200, message)
			c.String(200, "\n")
			//c.SSEvent("", message)
			return true
		}
	})
}
