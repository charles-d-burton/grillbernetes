package main

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"net/http"

	log "github.com/sirupsen/logrus"

	"github.com/charles-d-burton/grillbernetes/control-hub/messagebus"
	"github.com/gin-gonic/gin"
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
	router := gin.Default()
	router.POST("/", func(c *gin.Context) {
		var msg messagebus.Message
		if err := c.ShouldBindJSON(&msg); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "accepted"})
		messagebus.Publish(&msg)
	})
	router.Run(":7777")
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
		log.Error(err)
	}
	messagebus.Publish(&msg)
}
