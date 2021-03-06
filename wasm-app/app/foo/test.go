package foo

import (
	"strconv"
	"time"
)

func StartReturns() chan string {
	var starter = make(chan string, 100)
	ticker := time.NewTicker(5 * time.Second)

	go func() {
		counter := 0
		for {
			select {
			case <-ticker.C:
				counter = counter + 1
				starter <- strconv.Itoa(counter)
			}
		}
	}()
	return starter
}
