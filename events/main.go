package main

import (
	"encoding/json"
	"errors"
	"flag"
	"io"
	"math/rand"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	uuid "github.com/google/uuid"
	"github.com/jeffchao/backoff"
	nats "github.com/nats-io/nats.go"
	stan "github.com/nats-io/stan.go"
	"github.com/sirupsen/logrus"
)

const (
	queuelen = 100
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
	Conn        stan.Conn
	NatsHost    string
	subscribers map[string]*Subscriber
}

//Subscriber non-blocking broker of NATS messages to HTTP clients
type Subscriber struct {
	topic           string
	sub             stan.Subscription
	connEstablished chan bool
	clients         map[chan string]bool
	newClients      chan chan string
	defunctClients  chan chan string
	messages        chan string
	errors          chan error
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
	flag.BoolVar(&mockGen, "mock", false, "Generate mock data")
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
		router.Use(cors.Default())
	}
	if mockGen {
		router.GET("/events/:device/:channel", MockGen)
		router.Run(":7777")

	} else {
		// Make a new Broker instance
		nc := &NATSConnection{
			NatsHost:    natsHost,
			subscribers: make(map[string]*Subscriber, 10),
		}
		nc.Connect()
		env := &Env{nc}
		router.GET("/events/:group/:device/:channel", env.Subscribe)
		router.GET("/stream/:group/:device/:channel", env.Subscribe)
		router.Run(":7777")
	}
}

//Subscribe gin context to subscribe to an event stream returning json
func (env *Env) Subscribe(c *gin.Context) {
	realSSE := strings.Contains(c.FullPath(), "stream") //Check if we're looking for true SSE per the spec or streaming JSON
	device := c.Param("device")
	channel := c.Param("channel")
	group := c.Param("group")
	topic := group + "-" + device + "-" + channel
	log.Info("Subscribing to topic: ", topic)
	subscriber, err := env.natsConn.GetSubscriber(topic)
	if err != nil {
		c.AbortWithError(404, err)
		return
	}
	log.Info("Got subscriber from NATS Connection")
	queue := make(chan string, queuelen)
	errs := make(chan error, 1)
	subscriber.newClients <- queue //Add our new client to the recipient list
	clientGone := c.Writer.CloseNotify()
	c.Stream(func(w io.Writer) bool {
		select {
		case <-clientGone:
			subscriber.defunctClients <- queue //Remove our client from the client list
			return false
		case message := <-queue:
			if realSSE {
				c.SSEvent("message", json.RawMessage(message))
				return true
			}
			c.JSON(200, json.RawMessage(message))
			c.String(200, "\n")
			return true
		case err := <-errs:
			subscriber.defunctClients <- queue //Remove our client from the client list
			c.SSEvent("ERROR:", err.Error())
			return false
		case <-subscriber.errors:
			return false
		}
	})
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
			guid, err := uuid.NewRandom() //Create a new random unique identifier
			if err != nil {
				log.Error(err)
				return err
			}
			log.Info("UUID: ", guid.String())
			sc, err := stan.Connect("nats-streaming", guid.String(), stan.NatsConn(nc),
				stan.SetConnectionLostHandler(func(_ stan.Conn, reason error) {
					log.Info("Client Disconnected, sending cleanup signal")
					log.Info(reason)
					for _, sub := range conn.subscribers {
						sub.connEstablished <- false
					}
					cleanup <- true //Fire the job to throw an error and retry
				}))
			if err != nil {
				return err
			}
			conn.Lock()
			conn.Conn = sc //Save the connection
			conn.Unlock()
			log.Info("NATS Connected")
			if len(conn.subscribers) > 0 {
				for _, sub := range conn.subscribers {
					sub.connEstablished <- true //Let the subscriptions know the connections was established
				}
			}
			select {
			case <-cleanup:
				return errors.New("Connection lost")
			}
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
	if !subscriber.sub.IsValid() {
		log.Infof("Sub for %v topic is invalid, establishing new sub", topic)
		delete(conn.subscribers, subscriber.topic)
	}
	log.Info("No subscriber found for topic: ", topic)
	log.Info("Creating new subscriber")
	subscriber = &Subscriber{
		topic:           topic,
		connEstablished: make(chan bool, 1),
		clients:         make(map[chan string]bool, 10),
		newClients:      make(chan (chan string)),
		defunctClients:  make(chan (chan string)),
		messages:        make(chan string, 10),
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
				if len(subscriber.clients) > 0 {
					for queue := range subscriber.clients {
						if len(queue) < queuelen { //Skip client if their queue is full
							queue <- msg
							continue
						}
						log.Info("Subscriber queue full, dropping message")
					}
				} else if len(subscriber.clients) == 0 {
					log.Info("Received message for defunct subscriber, removing subscriber")
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
func (subscriber *Subscriber) Subscribe(conn *NATSConnection) (stan.Subscription, error) {
	log.Info("Initializing callback")
	log.Info("Subscription topic is: ", subscriber.topic)
	var datum = make(map[string]interface{}, 2)
	sub, err := conn.Conn.Subscribe(subscriber.topic, func(m *stan.Msg) {
		datum["timestamp"] = m.Timestamp
		datum["data"] = json.RawMessage(m.Data)
		data, err := json.Marshal(datum)
		if err != nil {
			log.Error(err)
		} else {
			subscriber.messages <- string(data)
		}
	}, stan.StartWithLastReceived())
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
			c.JSON(200, json.RawMessage(message))
			c.String(200, "\n")
			//c.SSEvent("", message)
			return true
		}
	})
}
