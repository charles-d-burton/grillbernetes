package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"github.com/charles-d-burton/grillbernetes/pismoker/controller"
	"github.com/go-redis/redis"
	"github.com/nats-io/go-nats"
)

var (
	usageStr = `
Usage: pismoker [options]
Options:
	-nh, --nats-host       <NATSHost>     Start the controller connecting to the defined NATS Streaming server
	-pt, --publish-topic   <Topic>        Topic to publish messages to in NATS
	-ct, --control-topic   <Topic>        Topic to listen for control messages
	-rc, --redis-host      <RedisHost>    Host for redis to connect to
`
	natsHost    = ""
	machineName = ""
	natsConn    *nats.Conn
	rc          *redis.Client
	signalChan  = make(chan os.Signal, 1)
	workerPool  WorkerPool
)

func usage() {
	log.Fatalf(usageStr)
}

func init() {
	flag.StringVar(&natsHost, "nh", "", "Start the controller connecting to the defined NATS Streaming server")
	flag.StringVar(&natsHost, "nats-host", "", "Start the controller connecting to the defined NATS Streaming server")
	flag.StringVar(&machineName, "n", "", "Name of the machine you're working with, defaults to hostname")
	flag.StringVar(&machineName, "name", "", "Name of the machine you're working with, defaults to hostname")
	flag.Parse()
	if natsHost == "" {
		usage()
	}
	if machineName == "" {
		name, err := os.Hostname()
		if err != nil {
			log.Fatal(err)
		}
		machineName = strings.Replace(name, ".", "-", -1)
	}
	signal.Notify(signalChan, syscall.SIGTERM)
	signal.Notify(signalChan, syscall.SIGINT)
}

//Worker Define worker implementation
type Worker struct {
	f func(reading Reading) error
}

//Reading data structure to hold sensor data
type Reading struct {
	ID string  `json:"id"`
	F  float64 `json:"f"`
	C  float64 `json:"c"`
}

//WorkerPool place holder to fan out to workers
type WorkerPool struct {
	sync.RWMutex
	Workers []*Worker
}

// NOTE: Use tls scheme for TLS, e.g. stan-sub -s tls://demo.nats.io:4443 foo
func main() {
	controller.StartServer(natsHost, machineName+"-readings", machineName+"-control")
}

//Fanout send the reading to all workers
func Fanout(reading Reading) error {
	workerPool.RLock()
	defer workerPool.Unlock()
	for _, worker := range workerPool.Workers {
		worker.f(reading)
	}
	return nil
}
