package main

import (
	"net/url"

	"github.com/maxence-charriere/go-app/v7/pkg/app"
	"github.com/tevino/abool"
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
