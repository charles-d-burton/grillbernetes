package main

import (
	"encoding/json"
	"flag"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis"
	"github.com/sirupsen/logrus"
)

var (
	usageStr = `
Usage: pismoker [options]
Options:
	-nh, --nats-host       <NATSHost>     Start the controller connecting to the defined NATS Streaming server
`
	log = logrus.New()
	rc  *redis.Client
)

//Message data to publish to server
type Message struct {
	Topic string          `json:"topic"`
	Data  json.RawMessage `json:"data"`
}

func init() {
	log.SetFormatter(&logrus.JSONFormatter{})
	var redisHost string
	flag.StringVar(&redisHost, "rd", "", "Start the controller connecting to the defined NATS Streaming server")
	flag.StringVar(&redisHost, "redis-host", "", "Start the controller connecting to the defined NATS Streaming server")
	flag.Parse()
	rc = redis.NewClient(&redis.Options{
		Addr:         redisHost,
		Password:     "",
		DB:           0,
		MinIdleConns: 1,
		MaxRetries:   5,
	})
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
		err := rc.Set(msg.Topic, msg.Data, 0)
		if err != nil {
			log.Println(err)
			c.JSON(http.StatusRequestTimeout, gin.H{"error": err.Err()})
		}
	})
	router.Run(":7777")
}
