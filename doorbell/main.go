package main

import (
	"fmt"
	"log"

	"periph.io/x/periph/conn/gpio"
	"periph.io/x/periph/conn/gpio/gpioreg"
	"periph.io/x/periph/host"
)

const (
	backoffTimeout = 60
)

var (
	buttonPresses = make(chan bool, 100)
	lastPress int64
)

func main() {
	log.Println("Hello world")
}

func ButtonPress() {
	go func() {
		// Load all the drivers:
		if _, err := host.Init(); err != nil {
			log.Fatal(err)
		}

		// Lookup a pin by its number:
		p := gpioreg.ByName("GPIO2")
		if p == nil {
			log.Fatal("Failed to find GPIO2")
		}

		fmt.Printf("%s: %s\n", p, p.Function())

		// Set it as input, with an internal pull down resistor:
		if err := p.In(gpio.PullDown, gpio.BothEdges); err != nil {
			log.Fatal(err)
		}

		// Wait for edges as detected by the hardware, and print the value read:
		for {
			p.WaitForEdge(-1)
			fmt.Printf("-> %s\n", p.Read())
			buttonPresses <- true
		}
	}()
}
