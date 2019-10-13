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
	-rh, --redis-host       <NATSHost>     Start the controller connecting to the defined NATS Streaming server
`
	log = logrus.New()
	rc  *redis.Client
)

//Message data to publish to server
type Message struct {
	Data json.RawMessage `json:"config"`
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
	router.GET("/:device/:config", GetConfig)
	router.POST("/:device/:config", SetConfig)
	router.Run(":7777")
}

//GetConfig retrieve a config from Redis
func GetConfig(c *gin.Context) {
	val, err := rc.Get(c.Param("device") + "/" + c.Param("config")).Result()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		log.Fatal(err)
		return
	}
	c.Data(http.StatusOK, "application/json", []byte(val))
}

//SetConfig sets the config for a given device
func SetConfig(c *gin.Context) {
	var msg Message
	if err := c.ShouldBindJSON(&msg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	err := rc.Set(c.Param("device")+"/"+c.Param("config"), msg.Data, 0)
	if err != nil {
		c.JSON(http.StatusRequestTimeout, gin.H{"error": err.Err()})
		log.Fatal(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "accepted"})
}

//Register Device
func RegisterDevice(device string) {

}
