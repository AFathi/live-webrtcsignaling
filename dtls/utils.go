package dtls

import (
	"sync"

	"github.com/heytribe/live-webrtcsignaling/my"
)

type Future struct {
	mutex    *my.Mutex
	cond     *sync.Cond
	received bool
	val      interface{}
	err      error
}

// NewFuture returns an initialized and ready Future.
func NewFuture() *Future {
	mutex := &my.Mutex{}
	return &Future{
		mutex:    mutex,
		cond:     sync.NewCond(mutex),
		received: false,
		val:      nil,
		err:      nil,
	}
}

// Get blocks until the Future has a value set.
func (self *Future) Get() (interface{}, error) {
	self.mutex.Lock()
	defer self.mutex.Unlock()
	for {
		if self.received {
			return self.val, self.err
		}
		self.cond.Wait()
	}
}

// Fired returns whether or not a value has been set. If Fired is true, Get
// won't block.
func (self *Future) Fired() bool {
	self.mutex.Lock()
	defer self.mutex.Unlock()
	return self.received
}

// Set provides the value to present and future Get calls. If Set has already
// been called, this is a no-op.
func (self *Future) Set(val interface{}, err error) {
	self.mutex.Lock()
	defer self.mutex.Unlock()
	if self.received {
		return
	}
	self.received = true
	self.val = val
	self.err = err
	self.cond.Broadcast()
}
