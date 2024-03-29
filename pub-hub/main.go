package main

import (
	"encoding/json"
	"flag"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis"
	nats "github.com/nats-io/nats.go"
	"github.com/sirupsen/logrus"
)

const (
	streamName = "EVENTS"
)

var (
	usageStr = `
Usage: pismoker [options]
Options:
	-nh, --nats-host       <NATS_HOST>     Start the controller connecting to the defined NATS Streaming server
	-rd, --redis-host      <REDIS_HOST>    Start the controller connecting to the defined Redis Host
`
	log = logrus.New()
	js  nats.JetStreamContext
	rc  *redis.Client
)

//Message data to publish to server
type Message struct {
	Data json.RawMessage `json:"data"`
}

type HsetValue struct {
	Device      string `json:"device"`
	TimeSeconds int64  `json:"time_seconds"`
	Channel     string `json:"channel"`
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
	if natsHost == "" {
		natsHost = os.Getenv("NATS_HOST")
		if natsHost == "" {
			log.Fatal("NATS_HOST Undefined\n", usageStr)
		}
	}
	if redisHost == "" {
		redisHost = os.Getenv("REDIS_HOST")
		if redisHost == "" {
			log.Fatal("REDIS_HOST Undefined\n", usageStr)
		}
	}
	log.Infof("connecting to nats host: %q", natsHost)
	conn, err := nats.Connect(natsHost,
		nats.DisconnectErrHandler(func(_ *nats.Conn, err error) {
			log.Error(err)
		}),
		nats.DisconnectHandler(func(_ *nats.Conn) {
			log.Error("unexpectedly disconnected from nats")
		}),
	)
	if err != nil {
		log.Fatal(err)
	}

	js, err = conn.JetStream()
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
				//log.Info(res)
			}
		}
	}()
}

func main() {
	//Sweep()
	router := gin.Default()
	router.GET("/healthz", HealthCheck)
	router.POST("/:group/:device/:channel", PostData)
	router.Run(":7777")
}

func HealthCheck(c *gin.Context) {
	res, err := rc.Ping().Result()
	if err != nil || res != "PONG" {
		log.Error("redis connection failed")
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "redis died"})
	}
	c.JSON(http.StatusOK, gin.H{"status": "alive"})
}

//PostData post message data to NATS Streaming for event processing
func PostData(c *gin.Context) {
	var msg Message
	if err := c.ShouldBindJSON(&msg); err != nil {
		log.Error(err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	log.Info("Publishing to: ", streamName+"."+c.Param("group")+"."+c.Param("device")+"."+c.Param("channel"))
	//TODO: Need to thread this probably, a pool of workers would be a good idea here
	log.Info("Msg: ", string(msg.Data))
	_, err := js.Publish(streamName+"."+c.Param("group")+"."+c.Param("device")+"."+c.Param("channel"), msg.Data)
	if err != nil {
		log.Error(err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	var hval HsetValue
	hval.Channel = c.Param("channel")
	hval.Device = c.Param("device")
	hval.TimeSeconds = time.Now().Unix()
	err = hval.Update(c.Param("group"))
	if err != nil {
		log.Error(err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "accepted"})
}

//Update update the device in a redis hashtable
func (hval *HsetValue) Update(group string) error {
	data, err := json.Marshal(&hval)
	if err != nil {
		return err
	}
	set, err := rc.HSet(group, hval.Device, string(data)).Result()
	if err != nil {
		return err
	}
	if set {
		log.Infof("Device %v added to HSET %v", hval.Device, group)
	}
	return nil
}

//Sweep Periodically clean up the set
func Sweep() {
	log.Info("Starting ttl cleanup")
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		var cursor uint64
		for {
			select {
			case <-ticker.C:
				log.Info("Running sweep")
				for {
					//var keys []string
					//var err error
					keys, cursor, err := rc.HScan("*", cursor, "*", 200).Result()
					if err != nil {
						panic(err)
					}
					for _, key := range keys { //TODO: Implement logic to cleanup old set entries
						/*duration := rc.ObjectIdleTime(key)
						idleTime := int64(duration.Val() / time.Second)
						if idleTime > 86400 { //Delete keys that haven't been accessed in 24 hours
							rc.Del(key)
						}*/
						log.Infof("KEY: %v", key)
					}
					if cursor == 0 {
						break
					}
				}
			}
		}
	}()
}
