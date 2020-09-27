package main

import (
	"fmt"
	"net/http"

	"github.com/maxence-charriere/go-app/v7/pkg/app"
)

func main() {
	fmt.Println("starting local server")

	h := &app.Handler{
		Title:       "K8S Kitchen",
		Author:      "Charles Burton",
		Description: "K8S Kitchen WASM App",
		Keywords: []string{
			"IoT",
		},
		Styles: []string{
			"https://code.getmdl.io/1.3.0/material.indigo-pink.min.css",
			"https://fonts.googleapis.com/icon?family=Material+Icons",
		},
	}
	fmt.Println("Setup complete, serving...")
	if err := http.ListenAndServe(":7000", h); err != nil {
		panic(err)
	}
}
