package main

import (
	"net/url"
	"syscall/js"

	"github.com/maxence-charriere/go-app/v7/pkg/app"
	"github.com/tevino/abool"
)

const (
	events   = "https://events.home.rsmachiner.com/stream/home/smoker-pi/readings"
	controls = "https://control-hub.home.rsmachiner.com/config/home/smoker-pi/configs"
	auth     = "https://auth.home.rsmachiner.com/login"
	aToken   = "accessToken"
	rToken   = "refreshToken"
	uname    = "username"
)

var (
	loggedIn = abool.New()
)

type frontpage struct {
	app.Compo
	name string
}

func (f *frontpage) Render() app.UI {
	return app.Text("Routed to frontpage")
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

func setLocal(key, value string) {
	js.Global().Get("localStorage").Call("setItem", key, value)
}

func getLocalString(key string) string {
	return js.Global().Get("localStorage").Call("getItem", key).String()
}
