//auth.go
package main

import (
	"fmt"
	"time"
)

//AuthManager Struct to hold the auth tokens
type AuthManager struct {
	AuthenticationResult struct {
		AccessToken  string `json:"AccessToken"`
		ExpiresIn    int    `json:"ExpiresIn"`
		RefreshToken string `json:"RefreshToken"`
	} `json:"AuthenticationResult"`
}

//SetExpire  sets the key/value for token expiration
func (mgr *AuthManager) SetExpire(seconds int) {
	now := time.Now()
	expires := now.Add(time.Duration(seconds-30) * time.Second)
	setLocal("expires", expires.Format(time.RFC3339))
}

//CheckExpire check if the token is expired
func (mgr *AuthManager) CheckExpire() bool {
	expStr := getLocalString("expires")
	now := time.Now()
	expires, err := time.Parse(time.RFC3339, expStr)
	if err != nil {
		fmt.Println(err)
		return true
	}
	if now.After(expires) {
		return true
	}
	return false
}

//Start start the loop to verify the tokens
func (mgr *AuthManager) Start() {
	ticker := time.NewTicker(5 * time.Second)
	done := make(chan bool, 1)
	go func() {
		for {
			select {
			case <-done:
				return
			case t := <-ticker.C:
				fmt.Println("Refreshing at: ", t)
				fmt.Println("Expired: ", mgr.CheckExpire())
			}
		}
	}()
}