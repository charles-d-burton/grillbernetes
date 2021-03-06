package main

import (
	"net/url"
	"sync"

	"github.com/maxence-charriere/go-app/v7/pkg/app"
	"github.com/tevino/abool"
	"github.com/traefik/yaegi/interp"
	"github.com/traefik/yaegi/stdlib"
)

const (
	events   = "https://events.home.rsmachiner.com/stream/home/smoker-pi/readings"
	controls = "https://control-hub.home.rsmachiner.com/config/home/smoker-pi/configs"
	auth     = "https://auth.home.rsmachiner.com/"
	aToken   = "accessToken"
	rToken   = "refreshToken"
	uname    = "username"
)

var (
	loggedIn = abool.New()
	done     = make(chan bool, 1)
)

const src = `package foo
import (
	"strconv"
	"time"
	"fmt"
)

func StartReturns() chan string {
	var starter = make(chan string, 100)
	
	fmt.Println("Building function")
	
	go func() {
		counter := 0
		for {
			counter = counter + 1
			starter <- strconv.Itoa(counter)
			time.Sleep(5 * time.Second)
		}
	}()
	return starter
}`

type frontpage struct {
	app.Compo
	sync.RWMutex
	name    string
	starter chan string
	dynamic string
}

func (f *frontpage) Render() app.UI {
	if f.starter == nil {
		f.dynamic = "Loading dynamic data stream"
		go func() {
			app.Log("Calling interpreter")
			i := interp.New(interp.Options{})
			i.Use(stdlib.Symbols)
			i.Eval(src)
			v, err := i.Eval("foo.StartReturns()")
			if err != nil {
				app.Log(err.Error())
			}
			c := v.Interface().(chan string)
			f.starter = c
			for val := range f.starter {
				f.Lock()
				f.dynamic = val
				app.Log(val)
				f.Unlock()
				app.Dispatch(func() {
					f.Update()
				})
			}
		}()
	}
	return app.Text(f.dynamic)
}

func (f *frontpage) OnNav(ctx app.Context, u *url.URL) {
	//token := getLocalString("token")

	app.Log("Checking token")
	if loggedIn.IsNotSet() {
		app.Navigate("/login")
		return
	}
	f.Update()
}

func main() {
	app.Route("/", &frontpage{})
	app.Route("/login", &login{})
	app.Run()
}
