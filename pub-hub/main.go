package main

import (
	"encoding/json"
	"flag"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis"
	"github.com/google/uuid"
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
	rc  *redis.Client
)

//Message data to publish to server
type Message struct {
	Data json.RawMessage `json:"data"`
}

//Device represents a device with timestamp for ttl
type Device struct {
	ID          string `json:"id"`
	LastContact int64  `json:"last_contact"`
}

func init() {
	log.SetFormatter(&logrus.JSONFormatter{})
	var natsHost string
	var redisHost string
	flag.StringVar(&natsHost, "nh", "", "Start the controller connecting to the defined NATS Streaming server")
	flag.StringVar(&natsHost, "nats-host", "", "Start the controller connecting to the defined NATS Streaming server")
	flag.StringVar(&redisHost, "rd", "", "Start the controller connecting to the redis cluster")
	flag.StringVar(&redisHost, "redis-host", "", "Start the controller connecting to the redis cluster")
	flag.Parse()
	nc, err := nats.Connect(natsHost)
	if err != nil {
		log.Fatal(err)
	}
	guid, err := uuid.NewRandom() //Create a new random unique identifier
	if err != nil {
		log.Fatal(err)
	}
	log.Info("UUID: ", guid.String())
	sc, err = stan.Connect("nats-streaming", guid.String(), stan.NatsConn(nc),
		stan.SetConnectionLostHandler(func(_ stan.Conn, reason error) {
			log.Fatal(reason)
		}))
	if err != nil {
		log.Fatal(err)
	}
	rc = redis.NewClient(&redis.Options{
		Addr:         redisHost,
		Password:     "",
		DB:           0,
		MinIdleConns: 1,
		MaxRetries:   5,
	})
	ticker := time.NewTicker(1000 * time.Millisecond)
	go func() {
		for range ticker.C {
			res, err := rc.Ping().Result()
			if err != nil {
				log.Fatal(err)
			}
			if res == "PONG" {
				log.Info(res)
			}
		}
	}()
}

func usage() {
	log.Fatalf(usageStr)
}

func main() {
	Sweep()
	router := gin.Default()
	router.GET("/healthz", HealthCheck)
	router.POST("/:group/:device/:channel", PostData)
	router.Run(":7777")
}

func HealthCheck(c *gin.Context) {
	res, err := rc.Ping().Result()
	if err != nil || res != "PONG" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "redis died"})
	}
	c.JSON(http.StatusOK, gin.H{"status": "alive"})
}

//PostData post message data to NATS Streaming for event processing
func PostData(c *gin.Context) {
	var msg Message
	if err := c.ShouldBindJSON(&msg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	log.Info("Publishing to: ", c.Param("group")+"-"+c.Param("device")+"-"+c.Param("channel"))
	err := sc.Publish(c.Param("group")+"-"+c.Param("device")+"-"+c.Param("channel"), msg.Data)
	if err != nil {
		log.Error(err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	err = RegisterDevice(c.Param("group"), c.Param("device"))
	if err != nil {
		log.Error(err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "accepted"})
}

//RegisterDevice addes a device to the set for connected device tracking
func RegisterDevice(group, device string) error {
	var dev Device
	dev.ID = device
	dev.LastContact = time.Now().Unix()
	data, err := json.Marshal(&dev)
	if err != nil {
		return err
	}
	err = rc.SAdd(group+"-"+device, data).Err()
	if err != nil {
		return err
	}
	log.Println("Successfully added device: ", device)
	return nil
}

//Sweep Periodically clean up the set
func Sweep() {
	log.Info("Starting ttl cleanup")
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		var cursor uint64
		for {
			select {
			case <-ticker.C:
				log.Info("Running sweep")
				for {
					var keys []string
					var err error
					keys, cursor, err = rc.SScan("*", cursor, "*", 10).Result()
					if err != nil {
						panic(err)
					}
					for _, key := range keys { //TODO: Implement logic to cleanup old set entries
						log.Info(key)
					}
					if cursor == 0 {
						break
					}
				}
			}
		}
	}()
}
