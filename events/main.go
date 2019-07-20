package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"html/template"
	"log"
	"math/rand"
	"net/http"
	"os"
	"time"

	uuid "github.com/google/uuid"
	"github.com/jeffchao/backoff"
	nats "github.com/nats-io/go-nats"
	stan "github.com/nats-io/go-nats-streaming"
)

var usageStr = `
Usage: pismoker [options]
Options:
	-nh, --nats-host       <NATSHost>     Start the controller connecting to the defined NATS Streaming server
	-pt, --publish-topic   <Topic>        Topic to publish messages to in NATS
	-st, --subscribe-topic   <Topic>        Topic to listen for upate messages
`

// A single Broker will be created in this program. It is responsible
// for keeping a list of which clients (browsers) are currently attached
// and broadcasting events (messages) to those clients.
//
type Broker struct {
	publishTopic   string
	subscribeTopic string
	natsHost       string

	// Create a map of clients, the keys of the map are the channels
	// over which we can push messages to attached clients.  (The values
	// are just booleans and are meaningless.)
	//
	clients map[chan string]bool

	// Channel into which new clients can be pushed
	//
	newClients chan chan string

	// Channel into which disconnected clients should be pushed
	//
	defunctClients chan chan string

	// Channel into which messages are pushed to be broadcast out
	// to attahed clients.
	//
	messages chan string
}

func main() {
	log.Println("Starting Grillbernetes Event Source")

	var natsHost string
	var publishTopic string
	var subscribeTopic string
	var mockGen = false
	flag.StringVar(&natsHost, "nh", "", "Start the controller connecting to the defined NATS Streaming server")
	flag.StringVar(&natsHost, "nats-host", "", "Start the controller connecting to the defined NATS Streaming server")
	flag.StringVar(&publishTopic, "pt", "smoker-controls", "Topic to publish readings to in NATS")
	flag.StringVar(&publishTopic, "publish-topic", "smoker-controls", "Topic to publish readings to in NATS")
	flag.StringVar(&subscribeTopic, "st", "smoker-readings", "Topic to listen for control messages")
	flag.StringVar(&subscribeTopic, "subscribe-topic", "smoker-readings", "Topic to listen for control messages")
	flag.BoolVar(&mockGen, "mock", true, "Generate mock data")

	flag.Parse()
	if natsHost == "" {
		natsHost = os.Getenv("NATS_HOST")
		if natsHost == "" && !mockGen {
			log.Fatal(usageStr)
		}
	}

	// Make a new Broker instance
	b := &Broker{
		publishTopic,
		subscribeTopic,
		natsHost,
		make(map[chan string]bool),
		make(chan (chan string)),
		make(chan (chan string)),
		make(chan string),
	}
	if mockGen {
		go b.MockGen()
	} else {
		go b.NATSConnect()
	}

	// Start processing events
	b.Start()

	// Make b the HTTP handler for "/events/".  It can do
	// this because it has a ServeHTTP method.  That method
	// is called in a separate goroutine for each
	// request to "/events/".
	http.Handle("/events/", b)

	// Generate a constant stream of events that get pushed
	// into the Broker's messages channel and are then broadcast
	// out to any clients that are attached.

	// When we get a request at "/", call `handler`
	// in a new goroutine.
	http.Handle("/", http.HandlerFunc(handler))

	// Start the server and listen forever on port 8000.
	http.ListenAndServe(":7777", nil)

}

//NATSConnect  Connect to the NATS streaming server and start pushing updates to clients
func (b *Broker) NATSConnect() {
	f := backoff.Fibonacci()
	f.Interval = 1 * time.Millisecond
	f.MaxRetries = 20
	cleanup := make(chan bool, 1)
	guid, err := uuid.NewRandom()
	if err != nil {
		log.Println(err)
		return
	}
	for {
		connect := func() error {
			log.Println("Connecting to NATS at: ", b.natsHost)
			nc, err := nats.Connect(b.natsHost)
			if err != nil {
				log.Fatal(err)
			}
			log.Println("UUID: ", guid.String())
			sc, err := stan.Connect("nats-streaming", guid.String(), stan.NatsConn(nc),
				stan.SetConnectionLostHandler(func(_ stan.Conn, reason error) {
					log.Println("Client Disconnected, sending cleanup signal")
					log.Println(reason)
					cleanup <- true
				}))
			if err != nil {
				log.Fatal(err)
				return err
			}
			log.Println("NATS Connected")
			log.Println("Initializing callback")
			var datum = make(map[string]interface{}, 2)
			sub, err := sc.Subscribe(b.subscribeTopic, func(m *stan.Msg) {
				datum["timestamp"] = m.Timestamp
				datum["data"] = json.RawMessage(m.Data)
				data, err := json.Marshal(datum)
				if err != nil {
					log.Println(err)
				}
				select {
				case b.messages <- string(data):
				default:
				}
			}, stan.StartWithLastReceived())
			if err != nil {
				return err
			}

			log.Println("Listening for messages on topic: ", b.subscribeTopic)
			for range cleanup {
				log.Println("Client disconnected, cleaning up before retry")
				sub.Unsubscribe()
				return errors.New("Client Disconnnected")
			}
			return nil
		}
		err := f.Retry(connect)
		if err != nil {
			log.Println(err)
		}
		f.Reset()
	}
}

func (b *Broker) MockGen() {
	log.Println("Mock Generator started")
	var id = "3b-6cfc0958d2fb"
	ticker := time.NewTicker(1 * time.Second)
	var datum = make(map[string]interface{}, 2)
	//var data = make(map[string]interface{}, 1)
	var temps = make(map[string]interface{}, 3)
	for range ticker.C {
		rand.Seed(time.Now().UnixNano())
		datum["timestamp"] = time.Now().UnixNano() / int64(time.Millisecond)
		temps["id"] = id
		temps["f"] = rand.Intn(300-50) + 50
		temps["c"] = rand.Intn(150-20) + 20
		datum["data"] = temps
		jsondata, err := json.Marshal(datum)
		log.Println("Generated message", string(jsondata))
		if err != nil {
			log.Println(err)
		}
		select {
		case b.messages <- string(jsondata):
		default:
		}
	}
}

// This Broker method starts a new goroutine.  It handles
// the addition & removal of clients, as well as the broadcasting
// of messages out to clients that are currently attached.
//
func (b *Broker) Start() {

	// Start a goroutine
	//
	go func() {

		// Loop endlessly
		//
		for {

			// Block until we receive from one of the
			// three following channels.
			select {

			case s := <-b.newClients:

				// There is a new client attached and we
				// want to start sending them messages.
				b.clients[s] = true
				log.Println("Added new client")

			case s := <-b.defunctClients:

				// A client has dettached and we want to
				// stop sending them messages.
				delete(b.clients, s)
				close(s)

				log.Println("Removed client")

			case msg := <-b.messages:

				// There is a new message to send.  For each
				// attached client, push the new message
				// into the client's message channel.
				for s := range b.clients {
					s <- msg
				}
				//log.Printf("Broadcast message to %d clients", len(b.clients))
			}
		}
	}()
}

// This Broker method handles and HTTP request at the "/events/" URL.
//
func (b *Broker) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	// Make sure that the writer supports flushing.
	//
	f, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
		return
	}

	// Create a new channel, over which the broker can
	// send this client messages.
	messageChan := make(chan string)

	// Add this client to the map of those that should
	// receive updates
	b.newClients <- messageChan

	// Listen to the closing of the http connection via the CloseNotifier
	notify := w.(http.CloseNotifier).CloseNotify()
	go func() {
		<-notify
		// Remove this client from the map of attached clients
		// when `EventHandler` exits.
		b.defunctClients <- messageChan
		log.Println("HTTP connection just closed.")
	}()

	// Set the headers related to event streaming.
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Transfer-Encoding", "chunked")

	// Don't close the connection, instead loop endlessly.
	for {

		// Read from our messageChan.
		msg, open := <-messageChan

		if !open {
			// If our messageChan was closed, this means that the client has
			// disconnected.
			break
		}

		// Write to the ResponseWriter, `w`.
		fmt.Fprintf(w, msg+"\n")

		// Flush the response.  This is only possible if
		// the repsonse supports streaming.
		f.Flush()
	}

	// Done.
	log.Println("Finished HTTP request at ", r.URL.Path)
}

// Handler for the main page, which we wire up to the
// route at "/" below n `main`.
//
func handler(w http.ResponseWriter, r *http.Request) {

	// Did you know Golang's ServeMux matches only the
	// prefix of the request URL?  It's true.  Here we
	// insist the path is just "/".
	if r.URL.Path != "/" {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	// Read in the template with our SSE JavaScript code.
	t, err := template.ParseFiles("templates/index.html")
	if err != nil {
		log.Fatal("Error parsing your template.")

	}

	// Render the template, writing to `w`.
	t.Execute(w, "friend")

	// Done.
	log.Println("Finished HTTP request at", r.URL.Path)
}
