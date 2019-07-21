package controller

import (
	"errors"
	"log"
	"time"

	"periph.io/x/periph/conn/gpio"
	"periph.io/x/periph/conn/gpio/gpioreg"
)

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
