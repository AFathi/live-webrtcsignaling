package main

import (
	"context"
)

/*
 * Fully event based pipeline.
 */
type Pipeline struct {
	Nodes   map[string]IPipelineNode
	Bus     chan interface{}
	Running bool
}

func NewPipeline() *Pipeline {
	p := new(Pipeline)
	p.Nodes = make(map[string]IPipelineNode)
	p.Bus = make(chan interface{}, 1000)
	return p
}

func (p *Pipeline) Register(name string, node IPipelineNode) {
	node.SetName(name)
	node.SetBus(p.Bus)
	p.Nodes[name] = node
}

func (p *Pipeline) Get(name string) IPipelineNode {
	return p.Nodes[name]
}

/*
 * start every node & update self state
 */
func (p *Pipeline) Run(ctx context.Context) {
	for _, node := range p.Nodes {
		go node.Run(ctx)
	}
	p.Running = true
}
