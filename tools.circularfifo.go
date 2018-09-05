package main

import (
	"container/ring"

	"github.com/heytribe/live-webrtcsignaling/my"
)

/*
 * CircularFIFO is a first-in first-out queue with a fixed size
 * that replaces its oldest element if full.
 * Thread Safe
 *
 * CircularFIFO contains a ring of CircularFIFONode
 *
 */
type CircularFIFO struct {
	my.RWMutex
	writeHead *ring.Ring // pointing to last written element
	size      int
	max       int
}

func NewCircularFIFO(max int) *CircularFIFO {
	// overly deffensive code
	if max < 1 {
		// FIXME: logger.Errorf("CircularFIFO.max should be at least >= 1, input max=%d", max)
		max = 1
	}
	c := new(CircularFIFO)
	c.Init(max)
	return c
}

func (c *CircularFIFO) Init(max int) *CircularFIFO {
	c.max = max
	return c
}

func (c *CircularFIFO) PushBack(data interface{}) {
	c.Lock()
	defer c.Unlock()

	if c.writeHead == nil {
		// first element
		c.writeHead = ring.New(1)
		c.writeHead.Value = data
		c.size++
		return
	}
	if c.size < c.max {
		// create a new node, insert, move writeHead to the new node
		e := ring.New(1)
		e.Value = data
		c.writeHead.Link(e)
		c.writeHead = e
		c.size++
		return
	}
	// element
	c.writeHead = c.writeHead.Next()
	c.writeHead.Value = data
}

func (c *CircularFIFO) GetLast() interface{} {
	return c.writeHead.Value
}

// Do calls function f on each element of the ring, from oldest to nearest
func (c *CircularFIFO) Do(f func(interface{})) {
	if c.writeHead == nil {
		return
	}
	c.RLock()
	c.writeHead.Next().Do(f)
	c.RUnlock()
}
