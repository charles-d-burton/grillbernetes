package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/felixge/pidctrl"
	"github.com/jeffchao/backoff"
	"github.com/yryz/ds18b20"
	"periph.io/x/periph/conn/gpio"
	"periph.io/x/periph/conn/gpio/gpioreg"
)

const (
	gpioPwr  = "GPIO8"
	relayPwr = "21"
)

var (
	usageStr = `
Usage: pismoker [options]
Options:
	-pt, --publish-topic   <Topic>        Topic to publish messages to in NATS
	-ct, --control-topic   <Topic>        Topic to listen for control messages
	-ch, --control-host    <ControlHost>  Remote host that maintains control state
	-dh, --data-host       <DataHost>     Remote host that accepts Readings
`
	dataHost     = ""
	controlHost  = ""
	machineName  = ""
	signalChan   = make(chan os.Signal, 1)
	controlChan  = make(chan *ControlState, 5)
	readings     = make(chan Reading, 1000)
	listeners    = make([]chan Reading, 2)
	stoppers     = make([]chan bool, 1)
	controlState ControlState
)

func usage() {
	log.Fatalf(usageStr)
}

func init() {
	flag.StringVar(&dataHost, "dh", "", "Start the controller connecting to the defined event consumer")
	flag.StringVar(&dataHost, "data-host", "", "Start the controller connecting to the defined event consumer")
	flag.StringVar(&machineName, "n", "", "Name of the machine you're working with, defaults to hostname")
	flag.StringVar(&machineName, "name", "", "Name of the machine you're working with, defaults to hostname")
	flag.StringVar(&controlHost, "ch", "", "Hostname:Port of the config enpoint")
	flag.StringVar(&controlHost, "control-host", "", "Hostname:Port of the config enpoint")
	flag.Parse()
	if dataHost == "" || controlHost == "" {
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

//ControlState Represent the runtime state of the smoker
type ControlState struct {
	Pwr     bool    `json:"pwr"`
	Temp    float64 `json:"temp"`
	RunTime int     `json:"run_time"`
}

//PIDState Represent the state of the PID controller
type PIDState struct {
	Kp     float64 `json:"kp"`
	Ki     float64 `json:"ki"`
	Kd     float64 `json:"kd"`
	Window int     `json:"window"`
}

//Reading data structure to hold sensor data
type Reading struct {
	ID string  `json:"id"`
	F  float64 `json:"f"`
	C  float64 `json:"c"`
}

// NOTE: Use tls scheme for TLS, e.g. stan-sub -s tls://demo.nats.io:4443 foo
func main() {
	//controller.StartServer(natsHost, machineName+"-readings", machineName+"-control")
	var wg sync.WaitGroup
	go func() {
		select {
		case <-signalChan:
			for _, stopper := range stoppers {
				stopper <- true
			}
		}
	}()
	wg.Add(1)
	stoppers = append(stoppers, Fanout(&wg)) //Start the Fanout
	wg.Add(1)
	stoppers = append(stoppers, PollRunState(&wg)) //Start watching for runstate updates
	wg.Add(1)
	er, es := PublishEvents(&wg)
	listeners = append(listeners, er)
	stoppers = append(stoppers, es)
	wg.Add(1)
	rp, sp := PidLoop(&wg)
	listeners = append(listeners, rp)
	stoppers = append(stoppers, sp)
	wg.Add(1)
	ReadLoop(&wg)
	wg.Wait()
}

//Fanout send the reading to all workers
func Fanout(wg *sync.WaitGroup) chan bool {
	stopper := make(chan bool, 1)
	go func() {
		select {
		case reading := <-readings:
			for _, listener := range listeners {
				listener <- reading
			}
		case <-stopper:
			wg.Done()
			break
		}
	}()
	return stopper
}

//PollRunState poll for config state updates
func PollRunState(wg *sync.WaitGroup) chan bool {
	ticker := time.NewTicker(5 * time.Second)
	stopper := make(chan bool, 1)
	go func() {
		for {
			select {
			case <-stopper:
				ticker.Stop()
				wg.Done()
				break
			case t := <-ticker.C:
				log.Println(t)
				resp, err := http.Get("https://" + controlHost + "/" + machineName + "/configs")
				if err != nil {
					log.Println(err)
					continue
				}
				defer resp.Body.Close()
				body, err := ioutil.ReadAll(resp.Body)
				if err != nil {
					log.Println(err)
					continue
				}
				log.Println("Got config: ", body)
				err = json.Unmarshal(body, &controlState)
				if err != nil {
					log.Println(err)
					continue
				}
				controlChan <- &controlState

			}
		}
	}()
	return stopper
}

//PublishEvents Push events to the data stream
func PublishEvents(wg *sync.WaitGroup) (chan Reading, chan bool) {
	stopper := make(chan bool, 1)
	readings := make(chan Reading, 1000)
	eventStream := "https://" + dataHost + "/" + machineName + "/readings"
	go func() {
		select {
		case reading := <-readings:
			data, err := json.Marshal(&reading)
			if err != nil {
				log.Println(err)
			} else {
				resp, err := http.Post(eventStream, "application/json", bytes.NewBuffer(data))
				if err != nil {
					log.Println(err)
				} else {
					body, err := ioutil.ReadAll(resp.Body)
					if err != nil {
						log.Println(err)
					} else if resp.StatusCode != http.StatusOK {
						log.Println("Response from server not OK: ", resp.Status)
					} else {
						log.Println(string(body))
					}
				}
			}
		case <-stopper:
			wg.Done()
			break
		}
	}()
	return readings, stopper
}

//PidLoop Watch for changes to run state and execute the PID algorithm to control the software run state
func PidLoop(wg *sync.WaitGroup) (chan Reading, chan bool) {
	readings := make(chan Reading, 100)
	stop := make(chan bool, 1)
	go func() {
		pidState := PIDState{
			Kp: 5,
			Ki: 3,
			Kd: 3,
		}
		controlState := &ControlState{
			Pwr:  false,
			Temp: 0,
		}
		p := gpioreg.ByName(relayPwr)
		if p == nil {
			log.Fatal("Unable to locate relay control pin")
		}
		pid := pidctrl.NewPIDController(pidState.Kp, pidState.Ki, pidState.Kd)
		pid.SetOutputLimits(0, 1)
		for {
			select {
			case state := <-controlChan:
				controlState.Pwr = state.Pwr
				controlState.Temp = state.Temp
			case reading := <-readings:
				log.Println("Received temperature update")
				update := pid.Update(reading.F)
				log.Println("PID says: ", update)
				if controlState.Pwr {
					if update == 0 {
						log.Println("Turning off relay")
						if err := p.Out(gpio.Low); err != nil {
							log.Println(err)
							ResetPin(p)
						}
					} else {
						log.Println("Turning on relay")
						if err := p.Out(gpio.High); err != nil {
							log.Println(err)
							ResetPin(p)
						}
					}
				} else {
					log.Println("Relay Powered Off")
					if err := p.Out(gpio.Low); err != nil {
						log.Println(err)

					}
				}
			case <-stop:
				wg.Done()
				break
			}

		}
	}()
	return readings, stop
}

//ReadLoop Read the sensor data in a loop, pass the data to the channel for fanout
func ReadLoop(wg *sync.WaitGroup) {
	defer wg.Done()
	go func() error {
		f := backoff.Fibonacci()
		f.Interval = 10 * time.Millisecond
		f.MaxRetries = 10
		connect := func() error { //Closure to support backoff/retry
			log.Println("Initializing sensors")
			sensors, err := ds18b20.Sensors()
			if err != nil {
				log.Fatal(err)
			}
			ticker := time.NewTicker(1 * time.Second)
			for {
				select {
				case <-ticker.C:
					//log.Println("Scanning sensors")
					for _, sensor := range sensors {
						t, err := ds18b20.Temperature(sensor)
						if err != nil {
							return err
						}
						var reading Reading
						reading.ID = sensor
						reading.C = t
						reading.F = CtoF(t)
						readings <- reading
					}
				}
			}
		}

		err := f.Retry(connect)
		//Completely failed, send the term signal to program
		if err != nil {
			log.Println(err)
			signalChan <- syscall.SIGTERM
			return err
		}
		return nil
	}()
}

/********************************
Helper Functions
********************************/

//ResetPin reset a passed in GPIO pin
func ResetPin(p gpio.PinIO) error {
	log.Println("Resetting pin, setting to low")
	if p == nil {
		return errors.New("Failed to get Pin: " + p.Name())
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
