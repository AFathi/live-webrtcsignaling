package main

import (
	"errors"

	"github.com/heytribe/live-webrtcsignaling/my"
)

/*
 * SentinelList is a fifo list struct:
 *
 *        +---+
 *        |   |
 * nil <- | O |-> nil                        s := new(SentinelList)
 *        +---+
 *
 *        +---+   +---+
 *        |   |   |   |
 *  X  <->| O |<->| X |<-> 0                 s.Init()
 *        +---+   +---+
 *
 *        +---+   +---+   +---+
 *        |   |   |   |   |   |
 *  X  <->| O |<->| 1 |<->| X |<-> 0         s.Push(1)
 *        +---+   +---+   +---+
 *
 *        +---+   +---+   +---+   +---+
 *        |   |   |   |   |   |   |   |
 *  X  <- | O |<->| 2 |<->| 1 |<->| X |<-> 0 s.Push(2)
 *        +---+   +---+   +---+   +---+
 *
 *        +---+   +---+   +---+
 *        |   |   |   |   |   |
 *  X  <- | O |<->| 2 |<->| X |-> 0          s.Pop() // => 1
 *        +---+   +---+   +---+
 *
 * Pop() on empty list => err
 */
type SentinelList struct {
	my.RWMutex
	data interface{}
	next *SentinelList
	prev *SentinelList
}

func (l *SentinelList) Init() {
	l.Lock()
	defer l.Unlock()

	last := new(SentinelList)
	l.next = last
	l.prev = last
	last.next = l
	last.prev = l
}

func (l *SentinelList) Push(data interface{}) {
	l.Lock()
	defer l.Unlock()

	lElement := new(SentinelList)
	lElement.data = data
	lElement.prev = l
	lElement.next = l.next
	l.next.prev = lElement
	l.next = lElement
}

func (l *SentinelList) Pop() (data interface{}, err error) {
	l.Lock()
	defer l.Unlock()

	if l.prev == l.next {
		err = errors.New("no data to pop")
		return
	}
	lElement := l.prev.prev
	lElement.next.prev = lElement.prev
	lElement.prev.next = lElement.next
	data = lElement.data

	return
}
