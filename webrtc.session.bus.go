package main

import (
	"context"
	plogger "github.com/heytribe/go-plogger"
	"github.com/heytribe/live-webrtcsignaling/gst"
)

func messageBusCallback(bus *gst.GstBus, message *gst.GstMessage, dataId int) {
	log := plogger.New() // FIXME: grab context
	log.Debugf("[ WEBRTC ] messageBusCallback called")
	messageType := message.GetType()
	if messageType&gst.MessageElement == gst.MessageElement {
		log.Debugf("[ WEBRTC ] Received a GST MESSAGE ELEMENT event")
	} else {
		log.Debugf("[ WEBRTC ] Received a GST MESSAGE event")
	}
	messageName := message.GetName()
	log.Debugf("[ WEBRTC ] message name is %s", messageName)
	structure := message.GetStructure()
	structureStr := gst.StructureToString(structure)
	log.Debugf("[ WEBRTC ] structure is %s", structureStr)
}

func (w *WebRTCSession) getBusMessages(ctx context.Context, element *gst.GstElement, id int) {
	log, _ := plogger.FromContext(ctx)
	w.bus = gst.PipelineGetBus(element)
	log.Debugf("[ WEBRTC ] bus is %p", w.bus.C)
	w.bus.SetMessageCallback(ctx, messageBusCallback, id)
	w.bus.AddSignalWatchFull(gst.PriorityDefault)
}
