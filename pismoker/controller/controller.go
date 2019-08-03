package controller

import (
	"encoding/json"
	"errors"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/felixge/pidctrl"
	"github.com/nats-io/go-nats"
	stan "github.com/nats-io/go-nats-streaming"
	"github.com/yryz/ds18b20"
	"periph.io/x/periph/conn/gpio"
	"periph.io/x/periph/conn/gpio/gpioreg"
	"periph.io/x/periph/host"
)

const (
	gpioPwr  = "GPIO8"
	relayPwr = "21"
)

var (
	readingQueue = make(chan Reading, 100)
	signalChan   = make(chan os.Signal)
	stopper      = make(chan struct{})
	receivers    Receivers
	controlState ControlState
	pidState     PIDState
)

//ControlState Represent the runtime state of the smoker
type ControlState struct {
	Pwr     bool    `json:"pwr"`
	Temp    float64 `json:"temp"`
	RunTime int     `json:"run_time"`
}

//Reading data structure to hold sensor data
type Reading struct {
	ID string  `json:"id"`
	F  float64 `json:"f"`
	C  float64 `json:"c"`
}

//PIDState Represent the state of the PID controller
type PIDState struct {
	sync.Mutex
	Started      bool         `json:"started"`
	Kp           float64      `json:"kp"`
	Ki           float64      `json:"ki"`
	Kd           float64      `json:"kd"`
	Window       int          `json:"window"`
	ControlState ControlState `json:"-"`
}

//Receivers Store channels that receive fanout messages
type Receivers struct {
	sync.Mutex
	Receivers []chan Reading
}

//Catch the interrupt and kill signals to clean up
func init() {
	signal.Notify(signalChan, syscall.SIGTERM)
	signal.Notify(signalChan, syscall.SIGINT)
}

//StartServer starts the control server connecting to the defined nats host
func StartServer(natsHost, publishTopic, controlTopic string) error {
	var wg sync.WaitGroup
	wg.Add(3)
	go Stop()
	go PublishToNATS(natsHost, publishTopic, controlTopic, &wg)

	log.Println("Starting GPIO initialization")
	if _, err := host.Init(); err != nil {
		log.Fatal(err)
	}
	go RelayControlLoop(&wg)
	go ReadQueue()
	for {
		err := ReadLoop(&wg)
		if err != nil {
			log.Println(err)
			break
		}
	}
	wg.Wait()
	return nil
}

//Stop receive the stop message signal from the OS and signal all goroutines to stop
func Stop() {
	sig := <-signalChan
	close(stopper)
	log.Println("Exiting", sig)
}

//ReadLoop Read the sensor data in a loop, pass the data to the channel for fanout
func ReadLoop(wg *sync.WaitGroup) error {
	defer wg.Done()
	log.Println("Initializing sensors")
	sensors, err := ds18b20.Sensors()
	if err != nil {
		log.Fatal(err)
	}
	ticker := time.NewTicker(1 * time.Second)
	for {
		select {
		case <-ticker.C:
			for _, sensor := range sensors {
				t, err := ds18b20.Temperature(sensor)
				if err != nil {
					return nil
				}
				var reading Reading
				reading.ID = sensor
				reading.C = t
				reading.F = CtoF(t)
				readingQueue <- reading

			}
		case <-stopper:
			log.Println("Closing read loop")
			close(readingQueue)
			return errors.New("Stopping read loop")
		}
	}
}

//Receive a Reading and then peform the PID control
func RelayControlLoop(wg *sync.WaitGroup) {
	defer wg.Done()
	receiver := make(chan Reading, 10)
	log.Println("Registering relay receiver")
	receivers.Lock()
	receivers.Receivers = append(receivers.Receivers, receiver)
	receivers.Unlock()
	p := gpioreg.ByName(relayPwr)
	if p == nil {
		log.Fatal("Unable to locat relay control pin")
	}
	pidState.Lock()
	pidState.Kp = 5
	pidState.Ki = 3
	pidState.Kd = 3
	//pidState.Window = 1000
	//pidState.ControlState = 24
	pidState.Unlock()

	if p == nil {
		log.Fatal("Relay pin not found")
	}
	pid := pidctrl.NewPIDController(pidState.Kp, pidState.Ki, pidState.Kd)
	pid.SetOutputLimits(0, 1)
	pwrState := false

	for {
		pidState.Lock()
		pid.Set(pidState.ControlState.Temp)
		pwrState = pidState.ControlState.Pwr
		pidState.Unlock()
		select {
		case reading := <-receiver:
			log.Println("Received temperature update")
			update := pid.Update(reading.F)
			log.Println("PID says: ", update)
			if pwrState {
				if update == 0 {

					log.Println("Turning off relay")
					if err := p.Out(gpio.Low); err != nil {
						log.Println(err)
					}
				} else {
					log.Println("Turning on relay")
					if err := p.Out(gpio.High); err != nil {
						log.Println(err)
					}
				}
			} else {
				log.Println("Relay Powered Off")
				if err := p.Out(gpio.Low); err != nil {
					log.Println(err)
				}
			}
		case <-stopper:
			log.Println("Stopping relay control")
			if err := p.Out(gpio.Low); err != nil {
				log.Println(err)
			}
			return
		}
	}
}

//ReadQueue receive a Reading and fan it out
func ReadQueue() {
	for {
		reading := <-readingQueue
		log.Println(reading)
		receivers.Lock()
		for _, receiver := range receivers.Receivers {
			select {
			case receiver <- reading:
			default:
				log.Println("Queue full")
			}
		}
		receivers.Unlock()
	}
}

//PublishToNATS publish Reading to the NATS server
func PublishToNATS(natsHost, publishTopic, controlTopic string, wg *sync.WaitGroup) {
	defer wg.Done()
	receiver := make(chan Reading, 100)
	log.Println("Registering receiver")
	receivers.Lock()
	receivers.Receivers = append(receivers.Receivers, receiver)
	receivers.Unlock()

	log.Println("Connecting to NATS at: ", natsHost)
	nc, err := nats.Connect(natsHost)
	if err != nil {
		log.Fatal(err)
	}
	sc, err := stan.Connect("nats-streaming", "smoker-client", stan.NatsConn(nc))
	if err != nil {
		log.Fatal(err)
	}
	log.Println("NATS Connected")
	log.Println("Initializing callback")
	_, err = sc.Subscribe(controlTopic, func(m *stan.Msg) {
		ProcessNATSMessage(m)
	}, stan.StartWithLastReceived())
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Listening for messages on topic: ", controlTopic)
	for {
		select {
		case reading := <-receiver:
			data, err := json.Marshal(reading)
			if err != nil {
				log.Println(err)
			}
			sc.Publish(publishTopic, data)
		case <-stopper:
			log.Println("Stopping publish")
			return
		}
	}
}

//ProcessNATSMessage process a control message from the NATS server
func ProcessNATSMessage(msg *stan.Msg) {
	defer pidState.Unlock() //Make sure the pidstate is unlocked in case of failures
	log.Println("Received control state update")
	log.Println(msg)
	var controlState ControlState
	err := json.Unmarshal(msg.Data, &controlState)
	if err != nil {
		log.Println(err)
	}
	pidState.Lock()
	//Keeps the machine from powering on right away
	if !pidState.Started {
		log.Println("Initial startup message, defaulting to off")
		controlState.Pwr = false
		pidState.Started = true
	}
	pidState.ControlState = controlState

}

/********************************
Helpers
 ********************************/

//ResetPin reset a passed in GPIO pin
func ResetPin(pin string) error {
	log.Println("Resetting pin, setting to low")
	p := gpioreg.ByName(pin)
	if p == nil {
		return errors.New("Failed to get Pin: " + pin)
	}
	if err := p.Out(gpio.Low); err != nil {
		return err
	}
	time.Sleep(3 * time.Second)
	log.Println("Resetting pin, setting to high")
	if err := p.Out(gpio.High); err != nil {
		return err
	}
	time.Sleep(1 * time.Second)
	return nil
}

//CtoF convert celsius to farenheit
func CtoF(c float64) float64 {
	return (c*9/5 + 32)
}

//FtoC conver farenheit to celsius
func FtoC(f float64) float64 {
	return ((f - 32) * 5 / 9)
}
