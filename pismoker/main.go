package main

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/charles-d-burton/grillbernetes/pismoker/max31850"
	"github.com/charles-d-burton/grillbernetes/pismoker/max31855"
	"github.com/felixge/pidctrl"
	"github.com/jeffchao/backoff"
	"github.com/tevino/abool"
	"periph.io/x/periph/conn/gpio"
	"periph.io/x/periph/conn/gpio/gpioreg"
	"periph.io/x/periph/host"
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
	-st, --sensor-type     <Sensor Type>  The kind of sensor that's connected
`
	dataHost    = ""
	controlHost = ""

	machineName      = ""
	id               = ""
	group            = ""
	sensorType       = ""
	sampleRate       int
	sensorSampleRate int
	signalChan       = make(chan os.Signal, 1)
	controlChan      = make(chan *ControlState, 5)
	readings         = make(chan Reading, 1000)
	listeners        []chan Reading
	finalizer        = make(chan bool, 1)
	controlState     ControlState
	powered          = abool.New()
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
	flag.StringVar(&group, "g", "home", "Logical group")
	flag.StringVar(&group, "group", "home", "Logical group")
	flag.IntVar(&sampleRate, "sr", 1, "Frequency in seconds to take a data sample")
	flag.IntVar(&sampleRate, "sample-rate", 1, "Frequency in seconds to take a data sample")
	flag.IntVar(&sensorSampleRate, "ssr", 100, "Frequency in ms to poll the sensor for data")
	flag.IntVar(&sensorSampleRate, "sensor-sample-rate", 100, "Frequence in ms to poll the sensor for data")
	flag.StringVar(&sensorType, "st", "", "Type of sensor to use")
	flag.StringVar(&sensorType, "sensor-type", "", "Type of sensor to use.  Must be one of max31855 or max31850")
	flag.Parse()
	if dataHost == "" || controlHost == "" || group == "" || sensorType == "" {
		usage()
	}
	if machineName == "" {
		name, err := os.Hostname()
		if err != nil {
			log.Fatal(err)
		}
		machineName = strings.Replace(name, ".", "-", -1)
		h := sha1.New()
		h.Write([]byte(machineName))
		id = hex.EncodeToString(h.Sum(nil))
	}
	log.Println("Starting GPIO initialization")
	if _, err := host.Init(); err != nil {
		log.Fatal(err)
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
	F  float32 `json:"f"`
	C  float32 `json:"c"`
}

// NOTE: Use tls scheme for TLS, e.g. stan-sub -s tls://demo.nats.io:4443 foo
func main() {
	//controller.StartServer(natsHost, machineName+"-readings", machineName+"-control")
	Fanout()       //Start the Fanout
	PollRunState() //Start watching for runstate updates
	er := PublishEvents()
	listeners = append(listeners, er)
	rp := PidLoop()
	listeners = append(listeners, rp)
	ReadLoop()
	log.Println("Finished initialization")
	select {
	case <-finalizer:
		log.Println("Program Exiting")
		os.Exit(0)
	}
}

//Fanout send the reading to all workers
func Fanout() {
	go func() {
		for {
			select {
			case reading := <-readings:
				for _, listener := range listeners {
					listener <- reading
				}
			}
		}
	}()
}

//PollRunState poll for config state updates
func PollRunState() {
	ticker := time.NewTicker(5 * time.Second)
	//stopper := make(chan bool, 1)
	go func() {
		for {
			select {
			case <-ticker.C:
				//log.Println(t)
				resp, err := http.Get(controlHost + "/" + "config" + "/" + group + "/" + machineName + "/configs")
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
				log.Println("Got config: ", string(body))
				err = json.Unmarshal(body, &controlState)
				if err != nil {
					log.Println(err)
					continue
				}
				powered.SetTo(controlState.Pwr)
				controlChan <- &controlState

			}
		}
	}()
}

//PublishEvents Push events to the data stream
func PublishEvents() chan Reading {
	log.Println("Starting Publish event loop")
	reads := make(chan Reading, 1000)
	eventStream := dataHost + "/" + group + "/" + machineName + "/readings"
	dataMap := make(map[string]Reading, 1)
	go func() {
		for {
			reading, ok := <-reads
			log.Println("Publish received reading")
			if !ok { //Check if channel closed, leave if it is
				finalizer <- true
				break
			}
			if powered.IsSet() { //Only publish data when the machine is on
				dataMap["data"] = reading
				data, err := json.Marshal(dataMap)
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
							log.Println(resp.Status)
							log.Println(string(body))
						}
						resp.Body.Close()
					}
				}
			}
		}
	}()
	return reads
}

//PidLoop Watch for changes to run state and execute the PID algorithm to control the software run state
func PidLoop() chan Reading {
	log.Println("Starting PID Control loop")
	reads := make(chan Reading, 100)
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
				log.Println("Received control state change")
				controlState.Pwr = state.Pwr
				controlState.Temp = state.Temp
				pid.Set(state.Temp)
			case reading, ok := <-reads:
				if !ok {
					if err := p.Out(gpio.Low); err != nil {
						log.Println(err)
					}
					finalizer <- true
					break
				}
				log.Println("Received temperature update")
				log.Println("Reading: ", reading.F)
				update := pid.Update(float64(reading.F))
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
			case <-signalChan: //Stop reading on SIGTERM, shutdown relay for safety
				log.Println("Turning off Relay due to process stop")
				if err := p.Out(gpio.Low); err != nil {
					log.Println(err)
				}
				finalizer <- true
				break
			}

		}
	}()
	return reads
}

//ReadLoop Read the sensor data in a loop, pass the data to the channel for fanout
func ReadLoop() {
	log.Println("Starting Sensor read loop")
	go func() {
		f := backoff.Fibonacci()
		f.Interval = 10 * time.Millisecond
		f.MaxRetries = 10
		connect := func() error { //Closure to support backoff/retry
			log.Println("Initializing sensors")
			ticker := time.NewTicker(time.Duration(sampleRate) * time.Second)
			for {
				select {
				case <-ticker.C:
					var reading Reading
					switch sensorType {
					case "max31855":
						err := max31855.InitMax31855(sensorSampleRate, "")
						if err != nil {
							return err
						}
						reading.ID = id
						reading.C, err = max31855.GetReadingCelsius()
						reading.F, err = max31855.GetReadingFarenheit()
						if err != nil {
							return err
						}

					case "max31850":
						err := max31850.InitMax31850(sensorSampleRate)
						if err != nil {
							return err
						}
						reading.ID = id
						reading.C, err = max31850.GetReadingCelsius()
						reading.F, err = max31850.GetReadingFarenheit()
						if err != nil {
							return err
						}
					}
				}
			}
		}

		err := f.Retry(connect)
		//Completely failed, send the term signal to program
		if err != nil {
			log.Println(err)
			signalChan <- syscall.SIGTERM
		}
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
