// +build js,wasm

package main

import "syscall/js"

func setLocal(key, value string) {
	js.Global().Get("localStorage").Call("setItem", key, value)
}

func getLocalString(key string) string {
	return js.Global().Get("localStorage").Call("getItem", key).String()
}
