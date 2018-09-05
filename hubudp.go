package main

import (
	"context"

	plogger "github.com/heytribe/go-plogger"
)

type hubUdp struct {
	// Register requests from the connections.
	register chan *connectionUdp

	// Unregister requests from connections.
	unregister chan *connectionUdp

	// Close requests from connections.
	close chan *connectionUdp
}

var hUdp = hubUdp{
	register:   make(chan *connectionUdp),
	unregister: make(chan *connectionUdp),
	close:      make(chan *connectionUdp),
}

func (hUdp *hubUdp) init() {
}

func (hUdp *hubUdp) destroyConnection(c *connectionUdp) {
	//close(c.send)

	return
}

func (hUdp *hubUdp) run(ctx context.Context) {
	log, _ := plogger.FromContext(ctx)
	for {
		select {
		case c := <-hUdp.register:
			log.Infof("registering connection %#v", c)
		case c := <-hUdp.unregister:
			go hUdp.destroyConnection(c)
		case c := <-hUdp.close:
			close(c.send)
		}
	}
}
