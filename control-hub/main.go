package main

import (
	"encoding/json"
	"flag"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	nats "github.com/nats-io/nats.go"
	stan "github.com/nats-io/stan.go"
	"github.com/sirupsen/logrus"
)

var (
	usageStr = `
Usage: pismoker [options]
Options:
	-nh, --nats-host       <NATSHost>     Start the controller connecting to the defined NATS Streaming server
`
	log = logrus.New()
	sc  stan.Conn
)

//Message data to publish to server
type Message struct {
	Topic string          `json:"topic"`
	Data  json.RawMessage `json:"data"`
}

func init() {
	log.SetFormatter(&logrus.JSONFormatter{})
	var natsHost string
	flag.StringVar(&natsHost, "nh", "", "Start the controller connecting to the defined NATS Streaming server")
	flag.StringVar(&natsHost, "nats-host", "", "Start the controller connecting to the defined NATS Streaming server")
	flag.Parse()
	nc, err := nats.Connect(natsHost)
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}
	sc, err = stan.Connect("nats-streaming", "smoker-client", stan.NatsConn(nc),
		stan.SetConnectionLostHandler(func(_ stan.Conn, reason error) {
			log.Info(reason)
			os.Exit(1)
		}))
}

func usage() {
	log.Fatalf(usageStr)
}

func main() {
	router := gin.Default()
	router.POST("/", func(c *gin.Context) {
		var msg Message
		if err := c.ShouldBindJSON(&msg); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "accepted"})
		err := sc.Publish(msg.Topic, msg.Data)
		if err != nil {
			log.Println(err)
		}
	})
	router.Run(":7777")
}
