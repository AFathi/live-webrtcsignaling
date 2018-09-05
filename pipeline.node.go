package main

import (
	"context"

	plogger "github.com/heytribe/go-plogger"
)

type IPipelineNode interface {
	SetBus(chan interface{})
	SetName(string)
	Run(context.Context)
}

type PipelineNode struct {
	Bus     chan interface{}
	Name    string
	Running bool
}

func (n *PipelineNode) SetBus(bus chan interface{}) {
	n.Bus = bus
}

func (n *PipelineNode) SetName(name string) {
	n.Name = name
}

func (n *PipelineNode) Run(ctx context.Context) {
	n.Running = true
	n.emitStart()
}

func (n *PipelineNode) onStop(ctx context.Context) {
	n.Running = false
	n.emitStop()
}

// helpers
func (n *PipelineNode) emitStart() {
	msg := new(PipelineMessageStart)
	msg.from = n
	select {
	case n.Bus <- msg:
	default:
		plogger.Warnf("Bus is full, dropping event emitStart")
	}
}

func (n *PipelineNode) emitStop() {
	msg := new(PipelineMessageStop)
	msg.from = n
	select {
	case n.Bus <- msg:
	default:
		plogger.Warnf("Bus is full, dropping event emitStop")
	}
}
