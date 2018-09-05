package main

type PipelineMessage struct {
	id   uint64
	from IPipelineNode
}

type PipelineMessageError struct {
	PipelineMessage
	err error
}

type PipelineMessageStart struct {
	PipelineMessage
}

type PipelineMessageStop struct {
	PipelineMessage
}

type PipelineMessageNack struct {
	PipelineMessage
}

type PipelineMessageRRStats struct {
	PipelineMessage
	InterarrivalDifference int64
	InterarrivalJitter     uint32
	SSRC                   uint32
}

type PipelineMessageInBps struct {
	PipelineMessage
	Bps uint64
}

type PipelineMessageOutBps struct {
	PipelineMessage
	Bps uint64
}
