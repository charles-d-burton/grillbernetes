package main

import (
	"encoding/json"
	"io"
	"strings"

	"github.com/gin-gonic/gin"
)

//SubscribeSSE gin context to subscribe to an event stream returning json
func (env *Env) SubscribeSSE(c *gin.Context) {
	realSSE := strings.Contains(c.FullPath(), "stream") //Check if we're looking for true SSE per the spec or streaming JSON
	device := c.Param("device")
	channel := c.Param("channel")
	group := c.Param("group")
	topic := group + "." + device + "." + channel
	log.Info("Subscribing to topic: ", topic)
	subscriber, err := env.natsConn.GetSubscriber(topic)
	if err != nil {
		c.AbortWithError(404, err)
		return
	}
	log.Info("Got subscriber from NATS Connection")
	queue := make(chan []byte, queuelen)
	errs := make(chan error, 1)
	subscriber.newClients <- queue //Add our new client to the recipient list
	clientGone := c.Writer.CloseNotify()
	c.Stream(func(w io.Writer) bool {
		select {
		case <-clientGone:
			subscriber.defunctClients <- queue //Remove our client from the client list
			return false
		case message := <-queue:
			c.Writer.Header().Set("Content-Type", "text/event-stream")
			if realSSE {
				c.SSEvent("message", json.RawMessage(message))
				return true
			}
			c.JSON(200, json.RawMessage(message))
			c.String(200, "\n")
			return true
		case err := <-errs:
			subscriber.defunctClients <- queue //Remove our client from the client list
			c.SSEvent("ERROR:", err.Error())
			return false
		case <-subscriber.errors:
			return false
		}
	})
}
