package main

import (
	"encoding/json"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

const (
	readBuffer  = 1024
	writeBuffer = 1024
)

//SubscribeWSS gin context to subscribe to an event stream returning json
func (env *Env) SubscribeWSS(c *gin.Context) {
	device := c.Param("device")
	channel := c.Param("channel")
	group := c.Param("group")
	topic := group + "." + device + "." + channel
	log.Info("Subscribing to topic: ", topic)
	subscriber, err := env.natsConn.GetSubscriber(topic)
	if err != nil {
		log.Error(err)
		c.AbortWithError(404, err)
		return
	}
	log.Info("Got subscriber from NATS Connection")
	queue := make(chan []byte, queuelen)
	errs := make(chan error, 1)
	subscriber.newClients <- queue //Add our new client to the recipient list
	clientGone := c.Writer.CloseNotify()
	conn, err := websocket.Upgrade(c.Writer, c.Request, nil, readBuffer, writeBuffer)
	if err != nil {
		log.Error(err)
		c.AbortWithError(404, err)
		return
	}
	for {
		select {
		case <-clientGone:
			subscriber.defunctClients <- queue //Remove our client from the client list
			return
		case message := <-queue:
			conn.WriteMessage(websocket.TextMessage, json.RawMessage(message))
		case err := <-errs:
			subscriber.defunctClients <- queue //Remove our client from the client list
			c.SSEvent("ERROR:", err.Error())
			return
		case <-subscriber.errors:
			return
		}
	}
}
