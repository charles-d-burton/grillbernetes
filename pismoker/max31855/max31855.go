package max31855

import (
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"periph.io/x/periph/conn/physic"
	"periph.io/x/periph/conn/spi"
	"periph.io/x/periph/conn/spi/spireg"
)

var (
	imu             sync.RWMutex
	emu             sync.RWMutex
	sensorErr       error
	currentReading  float32
	internalReading float32
	initialized     bool
)

//TODO: Figure out multiple devices using the port parameter
//NewMax31855 Initialize the driver and start publishing data in ms
func InitMax31855(resolution int, port string) error {
	log.Println("Starting MAX31855 Sensor Initialization")
	var wg sync.WaitGroup
	var ierr error
	wg.Add(1) //Force it to wait for initialization
	go func(wg *sync.WaitGroup) {

		if resolution < 50 {
			ierr = errors.New("Time resolution less than 50ms")
			wg.Done()
			return
		}
		t := time.NewTicker(time.Duration(resolution) * time.Millisecond)
		// Use spireg SPI port registry to find the first available SPI bus.
		sp, err := spireg.Open("")
		if err != nil {
			ierr = err
			wg.Done()
			return
		}

		// Convert the spi.Port into a spi.Conn so it can be used for communication.
		c, err := sp.Connect(physic.MegaHertz, spi.Mode3, 8)
		if err != nil {
			ierr = err
			wg.Done()
			return
		}
		wg.Done()
		defer sp.Close()
		for {
			//TODO: Rethink error handling a bit, make a decision whether or not to handle it here and reinitialize or create a channel that can inform the caller to reinit
			var wBuf, rBuf [4]byte
			if err := c.Tx(wBuf[:], rBuf[:]); err != nil {
				sensorErr = fmt.Errorf("max31855: txn error: %v", err)
				continue
			}

			// Check for various errors.
			if rBuf[3]&1 != 0 {
				sensorErr = fmt.Errorf("max31855: thermocouple open circuit error")
				continue
			}
			if rBuf[3]&2 != 0 {
				fmt.Printf("%#02x %02x %02x %02x\n", rBuf[0], rBuf[1], rBuf[2], rBuf[3])
				sensorErr = fmt.Errorf("max31855: thermocouple shorted to ground")
				continue
			}
			if rBuf[3]&4 != 0 {
				sensorErr = fmt.Errorf("max31855: thermocouple shorted to VCC")
				continue
			}
			sensorErr = nil

			// Calculate internal temperature.
			intT := int32((int16(rBuf[2]) << 8) | int16(rBuf[3]&0xf0)) // sign-extension!
			intT = (intT * 1000) >> 8
			imu.Lock()
			internalReading = float32(intT)
			imu.Unlock()
			// Calculate thermocouple temperature.
			thermT := int32((int16(rBuf[0]) << 8) | int16(rBuf[1]&0xfc))
			thermT = (thermT * 1000) >> 4
			emu.Lock()
			currentReading = float32(thermT)
			emu.Unlock()
			<-t.C
		}
	}(&wg)
	wg.Wait()
	initialized = true
	log.Println("MAX31855 Sensor Initialized")
	return ierr
}

func GetReadingCelsius() (float32, error) {
	if sensorErr != nil {
		return 0, sensorErr
	}
	emu.RLock()
	defer emu.RUnlock()
	return currentReading / 1000, nil
}

func GetInternalReadingCelsius() (float32, error) {
	if sensorErr != nil {
		return 0, sensorErr
	}
	imu.RLock()
	defer imu.RUnlock()
	return internalReading / 1000, nil
}

func GetReadingFarenheit() (float32, error) {
	if sensorErr != nil {
		return 0, sensorErr
	}
	emu.RLock()
	defer emu.RUnlock()
	return (currentReading/1000)*9/5 + 32, nil
}

func GetInternalReadingFarenheit() (float32, error) {
	if sensorErr != nil {
		return 0, sensorErr
	}
	imu.RLock()
	defer imu.RUnlock()
	return (internalReading/1000)*9/5 + 32, nil
}
