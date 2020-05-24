package max31850

import (
	"errors"
	"log"
	"sync"
	"time"

	"github.com/yryz/ds18b20"
)

var (
	mu             sync.RWMutex
	sensorErr      error
	currentReading float32
	initialized    bool
)

//TODO: Figure out multiple devices using the port parameter
//NewMax31850 Initialize the driver and start publishing data in ms
func InitMax31850(resolution int) error {
	if resolution < 1000 {
		return errors.New("Time resolution less than 1000ms, the maximum rate for a DS18b20")
	}
	ticker := time.NewTicker(time.Duration(resolution) * time.Millisecond)
	log.Println("Initializing sensors")
	sensors, err := ds18b20.Sensors()
	if err != nil {
		log.Fatal(err)
		return err
	}
	go func() {
		for {
			select {
			case <-ticker.C:
				for _, sensor := range sensors {
					t, err := ds18b20.Temperature(sensor)
					if err != nil {
						sensorErr = err
						continue
					}
					sensorErr = nil
					mu.Lock()
					currentReading = float32(t)
					mu.Unlock()
				}
			}
		}
	}()
	initialized = true
	return nil
}

func GetReadingCelsius() (float32, error) {
	if sensorErr != nil {
		return 0, sensorErr
	}
	mu.RLock()
	defer mu.RUnlock()
	return currentReading / 1000, nil
}

func GetReadingFarenheit() (float32, error) {
	if sensorErr != nil {
		return 0, sensorErr
	}
	mu.RLock()
	defer mu.RUnlock()
	return (currentReading/1000)*9/5 + 32, nil
}
