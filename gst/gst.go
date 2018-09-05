package gst

//#include "shim.h"
import "C"

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"runtime"
	"unsafe"

	plogger "github.com/heytribe/go-plogger"
)

var logger plogger.PLogger

func init() {
	logger = plogger.New().Prefix("GST").Tag("gst")
	logger.Infof("[GST] Initialization")
	C.X_gst_shim_init()
}

type GstElement struct {
	gstElement *C.GstElement
}

type GstParseContext struct {
	gstCtx *C.GstParseContext
}

func nonCopyGoBytes(ptr uintptr, length int) []byte {
	var slice []byte
	header := (*reflect.SliceHeader)(unsafe.Pointer(&slice))
	header.Cap = length
	header.Len = length
	header.Data = ptr
	return slice
}

func nonCopyCString(data *C.char, size C.int) []byte {
	return nonCopyGoBytes(uintptr(unsafe.Pointer(data)), int(size))
}

type ParseFlags int

const (
	ParseFlagNone                ParseFlags = C.GST_PARSE_FLAG_NONE
	ParseFlagFatalErrors                    = C.GST_PARSE_FLAG_FATAL_ERRORS
	ParseFlagNoSingleElementBins            = C.GST_PARSE_FLAG_NO_SINGLE_ELEMENT_BINS
	ParseFlagPlaceInBin                     = C.GST_PARSE_FLAG_PLACE_IN_BIN
)

func ParseLaunchFull(pipelineDescription string, context *GstParseContext, flags ParseFlags) (p *GstElement, err error) {
	var gError *C.GError

	pDesc := (*C.gchar)(unsafe.Pointer(C.CString(pipelineDescription)))
	defer C.g_free(C.gpointer(unsafe.Pointer(pDesc)))
	var parseCtx *C.GstParseContext
	if context == nil {
		parseCtx = nil
	} else {
		parseCtx = context.gstCtx
	}
	gstElt := C.gst_parse_launch_full(pDesc, parseCtx, C.GstParseFlags(flags), &gError)
	p = &GstElement{
		gstElement: gstElt,
	}

	// Parse gError and set err XXX

	return
}

func ElementFactoryMake(factoryName string, name string) (e *GstElement, err error) {
	var pName *C.gchar

	pFactoryName := (*C.gchar)(unsafe.Pointer(C.CString(factoryName)))
	defer C.g_free(C.gpointer(unsafe.Pointer(pFactoryName)))
	if name == "" {
		pName = nil
	} else {
		pName = (*C.gchar)(unsafe.Pointer(C.CString(name)))
		defer C.g_free(C.gpointer(unsafe.Pointer(pName)))
	}
	gstElt := C.gst_element_factory_make(pFactoryName, pName)

	if gstElt == nil {
		err = errors.New(fmt.Sprintf("could not create a GStreamer element factoryName %s, name %s", factoryName, name))
		return
	}

	e = &GstElement{
		gstElement: gstElt,
	}

	return
}

func PipelineNew(name string) (e *GstElement, err error) {
	var pName *C.gchar

	if name == "" {
		pName = nil
	} else {
		pName := (*C.gchar)(unsafe.Pointer(C.CString(name)))
		defer C.g_free(C.gpointer(unsafe.Pointer(pName)))
	}

	gstElt := C.gst_pipeline_new(pName)
	if gstElt == nil {
		err = errors.New(fmt.Sprintf("could not create a Gstreamer pipeline name %s", name))
		return
	}

	e = &GstElement{
		gstElement: gstElt,
	}

	runtime.SetFinalizer(e, func(e *GstElement) {
		fmt.Printf("CLEANING PIPELINE")
		C.gst_object_unref(C.gpointer(unsafe.Pointer(e.gstElement)))
	})

	return
}

func PipelineUseClock(element *GstElement, clock *GstClock, baseTime GstClockTime) {
	C.X_gst_pipeline_use_clock(element.gstElement, clock.C)
	C.X_gst_element_set_start_time_none(element.gstElement)
	C.gst_element_set_base_time(element.gstElement, C.GstClockTime(baseTime))
}

func PipelineGetBus(element *GstElement) (bus *GstBus) {
	CBus := C.X_gst_pipeline_get_bus(element.gstElement)

	bus = &GstBus{
		C:           CBus,
		callbackCtx: NewBusCallbackCtx(),
	}

	runtime.SetFinalizer(bus, func(bus *GstBus) {
		C.gst_object_unref(C.gpointer(unsafe.Pointer(bus.C)))
	})

	return
}

func BinNew() (element *GstElement) {
	Celement := C.gst_bin_new(nil)
	element = &GstElement{
		gstElement: Celement,
	}

	runtime.SetFinalizer(element, func(e *GstElement) {
		C.gst_object_unref(C.gpointer(unsafe.Pointer(element.gstElement)))
	})

	return
}

func BinAdd(parent *GstElement, child *GstElement) {
	C.X_gst_bin_add(parent.gstElement, child.gstElement)

	return
}

func BinRemove(parent *GstElement, child *GstElement) {
	C.X_gst_bin_remove(parent.gstElement, child.gstElement)
}

func BinAddMany(p *GstElement, elements ...*GstElement) {
	for _, e := range elements {
		if e != nil {
			C.X_gst_bin_add(p.gstElement, e.gstElement)
		}
	}

	return
}

func ElementGetName(element *GstElement) (name string) {
	n := (*C.char)(unsafe.Pointer(C.gst_object_get_name((*C.GstObject)(unsafe.Pointer(element.gstElement)))))
	if n != nil {
		name = string(nonCopyCString(n, C.int(C.strlen(n))))
	}

	return
}

func ElementLink(src *GstElement, dst *GstElement) bool {
	result := C.gst_element_link(src.gstElement, dst.gstElement)
	if result == C.TRUE {
		return true
	}
	return false
}

func ElementLinkMany(ctx context.Context, elements ...*GstElement) error {
	var prevElement *GstElement

	log, _ := plogger.FromContext(ctx)
	prevElement = nil
	log.Infof("ElementLinkMany called")
	for _, e := range elements {
		if prevElement != nil && e != nil {
			log.Infof("Linking %s -> %s", ElementGetName(prevElement), ElementGetName(e))
			result := ElementLink(prevElement, e)
			if result != true {
				err := errors.New(fmt.Sprintf("could not link elements %s -> %s", ElementGetName(prevElement), ElementGetName(e)))
				log.Errorf(err.Error())
				return err
			}
		}
		prevElement = e
	}
	return nil
}

func ElementGetByName(bin *GstElement, name string) (element *GstElement) {
	n := (*C.gchar)(unsafe.Pointer(C.CString(name)))
	defer C.g_free(C.gpointer(unsafe.Pointer(n)))
	e := C.X_gst_bin_get_by_name(bin.gstElement, n)
	element = &GstElement{
		gstElement: e,
	}

	return
}

type GstPadTemplate struct {
	C *C.GstPadTemplate
}

func ElementClassGetPadTemplate(element *GstElement, name string) (padTemplate *GstPadTemplate) {
	n := (*C.gchar)(unsafe.Pointer(C.CString(name)))
	defer C.g_free(C.gpointer(unsafe.Pointer(n)))
	CPadTemplate := C.gst_element_class_get_pad_template(C.X_GST_ELEMENT_GET_CLASS(element.gstElement), n)
	padTemplate = &GstPadTemplate{
		C: CPadTemplate,
	}

	return
}

func ElementRequestPad(element *GstElement, padTemplate *GstPadTemplate, name string, caps *GstCaps) (pad *GstPad) {
	var n *C.gchar
	var c *C.GstCaps

	if name == "" {
		n = nil
	} else {
		n = (*C.gchar)(unsafe.Pointer(C.CString(name)))
		defer C.g_free(C.gpointer(unsafe.Pointer(n)))
	}
	if caps == nil {
		c = nil
	} else {
		c = caps.caps
	}
	CPad := C.gst_element_request_pad(element.gstElement, padTemplate.C, n, c)
	pad = &GstPad{
		pad: CPad,
	}

	return
}

func ElementAddPad(element *GstElement, pad *GstPad) bool {
	Cret := C.gst_element_add_pad(element.gstElement, pad.pad)
	if Cret == 1 {
		return true
	}

	return false
}

type GstPad struct {
	pad *C.GstPad
}

func ElementGetRequestPad(element *GstElement, name string) (pad *GstPad) {
	n := (*C.gchar)(unsafe.Pointer(C.CString(name)))
	defer C.g_free(C.gpointer(unsafe.Pointer(n)))
	CPad := C.gst_element_get_request_pad(element.gstElement, n)
	pad = &GstPad{
		pad: CPad,
	}

	return
}

func ElementGetStaticPad(element *GstElement, name string) (pad *GstPad) {
	n := (*C.gchar)(unsafe.Pointer(C.CString(name)))
	defer C.g_free(C.gpointer(unsafe.Pointer(n)))
	CPad := C.gst_element_get_static_pad(element.gstElement, n)
	pad = &GstPad{
		pad: CPad,
	}

	return
}

func ElementSendEvent(element *GstElement, event *GstEvent) bool {
	CResult := C.gst_element_send_event(element.gstElement, event.C)
	if CResult == 0 {
		return false
	}

	return true
}

type PadLinkReturn int

const (
	PadLinkOk             PadLinkReturn = C.GST_PAD_LINK_OK
	PadLinkWrongHierarchy               = C.GST_PAD_LINK_WRONG_HIERARCHY
	PadLinkWasLinked                    = C.GST_PAD_LINK_WAS_LINKED
	PadLinkWrongDirection               = C.GST_PAD_LINK_WRONG_DIRECTION
	PadLinkNoFormat                     = C.GST_PAD_LINK_NOFORMAT
	PadLinkNoSched                      = C.GST_PAD_LINK_NOSCHED
	PadLinkRefused                      = C.GST_PAD_LINK_REFUSED
)

func PadLink(src *GstPad, sink *GstPad) (padLinkReturn PadLinkReturn) {
	padLinkReturn = PadLinkReturn(C.gst_pad_link(src.pad, sink.pad))
	return
}

func PadSetOffset(gstPad *GstPad, gstClockTime GstClockTime) {
	C.gst_pad_set_offset(gstPad.pad, C.gint64(gstClockTime))
}

func PadGetCurrentCaps(gstPad *GstPad) (gstCaps *GstCaps) {
	Ccaps := C.gst_pad_get_current_caps(gstPad.pad)

	gstCaps = &GstCaps{
		caps: Ccaps,
	}

	runtime.SetFinalizer(gstCaps, func(gstCaps *GstCaps) {
		C.gst_caps_unref(gstCaps.caps)
	})

	return
}

//export go_callback_pad_event_thunk
func go_callback_pad_event_thunk(CgstPad *C.GstPad, CgstObject *C.GstObject, Cevent *C.GstEvent) {
	logger.Infof("New event received !")
	logger.Infof("Event type is %d", C.X_GST_EVENT_TYPE(Cevent))
	return
}

func PadSetEventFullFunctionFull(pad *GstPad) {
	C.gst_pad_set_event_full_function_full(pad.pad, (*[0]byte)(C.cb_pad_event), nil, nil)
}

func ObjectSet(ctx context.Context, e *GstElement, pName string, pValue interface{}) {
	log, _ := plogger.FromContext(ctx)
	CpName := (*C.gchar)(unsafe.Pointer(C.CString(pName)))
	defer C.g_free(C.gpointer(unsafe.Pointer(CpName)))
	switch pValue.(type) {
	case string:
		log.Infof("Found string %s=%s", pName, pValue.(string))
		str := (*C.gchar)(unsafe.Pointer(C.CString(pValue.(string))))
		defer C.g_free(C.gpointer(unsafe.Pointer(str)))
		C.X_gst_g_object_set_string(e.gstElement, CpName, str)
	case int:
		log.Infof("Found int %s=%d", pName, pValue.(int))
		C.X_gst_g_object_set_int(e.gstElement, CpName, C.gint(pValue.(int)))
	case uint32:
		log.Infof("Found uint32 %s=%d", pName, pValue.(uint32))
		C.X_gst_g_object_set_uint(e.gstElement, CpName, C.guint(pValue.(uint32)))
	case bool:
		var value int
		if pValue.(bool) == true {
			value = 1
		} else {
			value = 0
		}
		log.Infof("Found bool %s=%d", pName, value)
		C.X_gst_g_object_set_bool(e.gstElement, CpName, C.gboolean(value))
	case *GstCaps:
		log.Infof("Found *GstCaps %s=%#v", pName, pValue.(*GstCaps))
		caps := pValue.(*GstCaps)
		C.X_gst_g_object_set_caps(e.gstElement, CpName, caps.caps)
	case *GstStructure:
		log.Infof("Found *GstStructure %s=%#v", pName, pValue.(*GstStructure))
		structure := pValue.(*GstStructure)
		C.X_gst_g_object_set_structure(e.gstElement, CpName, structure.C)
	}

	return
}

type StateOptions int

const (
	StateVoidPending StateOptions = C.GST_STATE_VOID_PENDING
	StateNull        StateOptions = C.GST_STATE_NULL
	StateReady       StateOptions = C.GST_STATE_READY
	StatePaused      StateOptions = C.GST_STATE_PAUSED
	StatePlaying     StateOptions = C.GST_STATE_PLAYING
)

func ElementSetState(element *GstElement, state StateOptions) C.GstStateChangeReturn {
	return C.gst_element_set_state(element.gstElement, C.GstState(state))
}

type GstCaps struct {
	caps *C.GstCaps
}

func CapsFromString(caps string) (gstCaps *GstCaps) {
	c := (*C.gchar)(unsafe.Pointer(C.CString(caps)))
	defer C.g_free(C.gpointer(unsafe.Pointer(c)))
	CCaps := C.gst_caps_from_string(c)
	gstCaps = &GstCaps{
		caps: CCaps,
	}

	runtime.SetFinalizer(gstCaps, func(gstCaps *GstCaps) {
		C.gst_caps_unref(gstCaps.caps)
	})

	return
}

func CapsToString(gstCaps *GstCaps) (str string) {
	CStr := C.gst_caps_to_string(gstCaps.caps)
	defer C.g_free(C.gpointer(unsafe.Pointer(CStr)))
	str = C.GoString((*C.char)(unsafe.Pointer(CStr)))

	return
}

type RequestAuxReceiverCallback func(element *GstElement, session uint, dataId int) *GstElement
type RequestAuxSenderCallback func(element *GstElement, session uint, dataId int) *GstElement
type PadAddedCallback func(name string, element *GstElement, pad *GstPad, dataId int)
type NeedDataCallback func(element *GstElement, size uint, dataId int, sourceId int)
type EnoughDataCallback func(element *GstElement, dataId int, sourceId int)

type GstData struct {
	requestAuxReceiverCallback RequestAuxReceiverCallback
	requestAuxSenderCallback   RequestAuxSenderCallback
	padAddedCallback           PadAddedCallback
	needDataCallback           NeedDataCallback
	enoughDataCallback         EnoughDataCallback
	Id                         C.int
	SourceId                   C.int
}

// Callbacks

//export go_callback_request_aux_receiver_thunk
func go_callback_request_aux_receiver_thunk(CrtpBin *C.GstElement, Csession C.guint, Cdata C.gpointer) *C.GstElement {
	gstData := (*GstData)(unsafe.Pointer(Cdata))
	callback := gstData.requestAuxReceiverCallback
	dataId := int(gstData.Id)
	rtpBin := &GstElement{
		gstElement: CrtpBin,
	}
	logger.Debugf("go_callback_request_aux_receiver_thunk called, gstData is %p, rtpBin.C is %p, session is %d", gstData, rtpBin.gstElement, uint(Csession))

	element := callback(rtpBin, uint(Csession), dataId)

	return element.gstElement
}

//export go_callback_request_aux_sender_thunk
func go_callback_request_aux_sender_thunk(CrtpBin *C.GstElement, Csession C.guint, Cdata C.gpointer) *C.GstElement {
	gstData := (*GstData)(unsafe.Pointer(Cdata))
	callback := gstData.requestAuxSenderCallback
	dataId := int(gstData.Id)
	rtpBin := &GstElement{
		gstElement: CrtpBin,
	}
	logger.Debugf("go_callback_request_aux_sender_thunk called, gstData is %p, rtpBin.C is %p, session is %d", gstData, rtpBin.gstElement, uint(Csession))

	element := callback(rtpBin, uint(Csession), dataId)

	return element.gstElement
}

//export go_callback_new_pad_thunk
func go_callback_new_pad_thunk(Cname *C.gchar, CgstElement *C.GstElement, CgstPad *C.GstPad, Cdata C.gpointer) {
	gstData := (*GstData)(unsafe.Pointer(Cdata))
	callback := gstData.padAddedCallback
	dataId := int(gstData.Id)
	name := C.GoString((*C.char)(unsafe.Pointer(Cname)))
	element := &GstElement{
		gstElement: CgstElement,
	}
	pad := &GstPad{
		pad: CgstPad,
	}

	logger.Debugf("callback thunk called, gstData is %p, name is %#v, element is %#v, pad is %#v", gstData, name, element, pad)

	callback(name, element, pad, dataId)
}

//export go_callback_need_data_thunk
func go_callback_need_data_thunk(CgstElement *C.GstElement, size C.guint, Cdata C.gpointer) {
	gstData := (*GstData)(unsafe.Pointer(Cdata))
	callback := gstData.needDataCallback
	dataId := int(gstData.Id)
	sourceId := int(gstData.SourceId)

	element := &GstElement{
		gstElement: CgstElement,
	}

	callback(element, uint(size), dataId, sourceId)
}

//export go_callback_enough_data_thunk
func go_callback_enough_data_thunk(CgstElement *C.GstElement, Cdata C.gpointer) {
	gstData := (*GstData)(unsafe.Pointer(Cdata))
	callback := gstData.enoughDataCallback
	dataId := int(gstData.Id)
	sourceId := int(gstData.SourceId)

	element := &GstElement{
		gstElement: CgstElement,
	}

	callback(element, dataId, sourceId)
}

type CallbackCtx struct {
	gstData []*GstData
}

func (callbackCtx *CallbackCtx) SetRequestAuxSenderCallback(ctx context.Context, element *GstElement, requestAuxSenderCallback RequestAuxSenderCallback, dataId int) {
	log := plogger.FromContextSafe(ctx)
	gstData := new(GstData)
	gstData.requestAuxSenderCallback = requestAuxSenderCallback
	gstData.Id = C.int(dataId)
	callbackCtx.gstData = append(callbackCtx.gstData, gstData)
	log.Debugf("[ GST ] SetRequestAuxSenderCallback with gstData %#v", gstData)
	detailedSignal := (*C.gchar)(unsafe.Pointer(C.CString("request-aux-sender")))
	defer C.g_free(C.gpointer(unsafe.Pointer(detailedSignal)))
	C.X_g_signal_connect(element.gstElement, detailedSignal, (*[0]byte)(C.cb_request_aux_sender), (C.gpointer)(unsafe.Pointer(gstData)))
}

func (callbackCtx *CallbackCtx) SetRequestAuxReceiverCallback(ctx context.Context, element *GstElement, requestAuxReceiverCallback RequestAuxReceiverCallback, dataId int) {
	log := plogger.FromContextSafe(ctx)
	gstData := new(GstData)
	gstData.requestAuxReceiverCallback = requestAuxReceiverCallback
	gstData.Id = C.int(dataId)
	callbackCtx.gstData = append(callbackCtx.gstData, gstData)
	log.Debugf("[ GST ] SetRequestAuxReceiverCallback with gstData %#v", gstData)
	detailedSignal := (*C.gchar)(unsafe.Pointer(C.CString("request-aux-receiver")))
	defer C.g_free(C.gpointer(unsafe.Pointer(detailedSignal)))
	C.X_g_signal_connect(element.gstElement, detailedSignal, (*[0]byte)(C.cb_request_aux_receiver), (C.gpointer)(unsafe.Pointer(gstData)))
}

func (callbackCtx *CallbackCtx) SetPadAddedCallback(ctx context.Context, element *GstElement, padAddedCallback PadAddedCallback, dataId int) {
	log, _ := plogger.FromContext(ctx)
	gstData := new(GstData)
	gstData.padAddedCallback = padAddedCallback
	gstData.Id = C.int(dataId)
	callbackCtx.gstData = append(callbackCtx.gstData, gstData)
	log.Debugf("[ GST ] SetPadAddedCallback with gstData %#v", gstData)
	detailedSignal := (*C.gchar)(unsafe.Pointer(C.CString("pad-added")))
	defer C.g_free(C.gpointer(unsafe.Pointer(detailedSignal)))
	C.X_g_signal_connect(element.gstElement, detailedSignal, (*[0]byte)(C.cb_new_pad), (C.gpointer)(unsafe.Pointer(gstData)))
}

func (callbackCtx *CallbackCtx) SetNeedDataCallback(ctx context.Context, element *GstElement, needDataCallback NeedDataCallback, dataId int, sourceId int) {
	log, _ := plogger.FromContext(ctx)
	gstData := new(GstData)
	gstData.needDataCallback = needDataCallback
	gstData.Id = C.int(dataId)
	gstData.SourceId = C.int(sourceId)
	callbackCtx.gstData = append(callbackCtx.gstData, gstData)
	log.Debugf("[ GST ] SetNeedDataCallback with gstData %#v", gstData)
	detailedSignal := (*C.gchar)(unsafe.Pointer(C.CString("need-data")))
	defer C.g_free(C.gpointer(unsafe.Pointer(detailedSignal)))
	C.X_g_signal_connect(element.gstElement, detailedSignal, (*[0]byte)(C.cb_need_data), (C.gpointer)(unsafe.Pointer(gstData)))
}

func (callbackCtx *CallbackCtx) SetEnoughDataCallback(ctx context.Context, element *GstElement, enoughDataCallback EnoughDataCallback, dataId int, sourceId int) {
	log, _ := plogger.FromContext(ctx)
	gstData := new(GstData)
	gstData.enoughDataCallback = enoughDataCallback
	gstData.Id = C.int(dataId)
	gstData.SourceId = C.int(sourceId)
	callbackCtx.gstData = append(callbackCtx.gstData, gstData)
	log.Infof("[ GST ] SetEnoughDataCallback with gstData %#v", gstData)
	detailedSignal := (*C.gchar)(unsafe.Pointer(C.CString("enough-data")))
	defer C.g_free(C.gpointer(unsafe.Pointer(detailedSignal)))
	C.X_g_signal_connect(element.gstElement, detailedSignal, (*[0]byte)(C.cb_enough_data), (C.gpointer)(unsafe.Pointer(gstData)))
}

func NewCallbackCtx() *CallbackCtx {
	return new(CallbackCtx)
}

func DebugBinToDotFile(p *GstElement, name string) {
	n := (*C.gchar)(unsafe.Pointer(C.CString(name)))
	defer C.g_free(C.gpointer(unsafe.Pointer(n)))
	C.X_GST_DEBUG_BIN_TO_DOT_FILE(p.gstElement, n)
}

// Gst Buffer

type GstBuffer struct {
	C *C.GstBuffer
}

func BufferNewAndAlloc(size uint, shouldFree bool) (gstBuffer *GstBuffer, err error) {
	CGstBuffer := C.gst_buffer_new_allocate(nil, C.gsize(size), nil)

	if CGstBuffer == nil {
		err = errors.New("could not allocate a new GstBuffer")
		return
	}

	gstBuffer = &GstBuffer{C: CGstBuffer}
	if shouldFree == true {
		runtime.SetFinalizer(gstBuffer, func(gstBuffer *GstBuffer) {
			C.gst_buffer_unref(gstBuffer.C)
		})
	}

	return
}

func BufferNewWrapped(data []byte) (gstBuffer *GstBuffer, err error) {
	Cdata := (*C.gchar)(unsafe.Pointer(C.malloc(C.size_t(len(data)))))
	C.bcopy(unsafe.Pointer(&data[0]), unsafe.Pointer(Cdata), C.size_t(len(data)))
	CGstBuffer := C.X_gst_buffer_new_wrapped(Cdata, C.gsize(len(data)))
	if CGstBuffer == nil {
		err = errors.New("could not allocate and wrap a new GstBuffer")
		return
	}
	gstBuffer = &GstBuffer{C: CGstBuffer}

	return
}

func BufferGetData(gstBuffer *GstBuffer) (data []byte, err error) {
	mapInfo := (*C.GstMapInfo)(unsafe.Pointer(C.malloc(C.sizeof_GstMapInfo)))
	defer C.free(unsafe.Pointer(mapInfo))

	if int(C.X_gst_buffer_map(gstBuffer.C, mapInfo)) == 0 {
		err = errors.New(fmt.Sprintf("could not map gstBuffer %#v", gstBuffer))
		return
	}
	CData := (*[1 << 30]byte)(unsafe.Pointer(mapInfo.data))
	data = make([]byte, int(mapInfo.size))
	copy(data, CData[:])
	C.gst_buffer_unmap(gstBuffer.C, mapInfo)

	return
}

func BufferCopy(gstBuffer *GstBuffer) (gstBufferCopy *GstBuffer) {
	CGstBuffer := C.gst_buffer_copy(gstBuffer.C)
	gstBufferCopy = &GstBuffer{
		C: CGstBuffer,
	}

	/*runtime.SetFinalizer(gstBufferCopy, func(b *GstBuffer) {
		C.gst_buffer_unref(b.C)
	})*/

	return
}

func BufferCopyDeep(gstBuffer *GstBuffer, autoDeallocate bool) (gstBufferCopy *GstBuffer) {
	CGstBuffer := C.gst_buffer_copy_deep(gstBuffer.C)
	gstBufferCopy = &GstBuffer{
		C: CGstBuffer,
	}

	if autoDeallocate == true {
		runtime.SetFinalizer(gstBufferCopy, func(gstBuffer *GstBuffer) {
			C.gst_buffer_unref(gstBuffer.C)
		})
	}

	return
}

func BufferRef(gstBuffer *GstBuffer) (gstBufferReturn *GstBuffer) {
	C.gst_buffer_ref(gstBuffer.C)
	gstBufferReturn = gstBuffer

	return
}

func BufferUnref(gstBuffer *GstBuffer) {
	C.gst_buffer_unref(gstBuffer.C)
}

func BufferGetSize(gstBuffer *GstBuffer) (bufferSize uint) {
	bufferSize = uint(C.gst_buffer_get_size(gstBuffer.C))

	return
}

func SignalEmitByName(element *GstElement, detailedSignal string, buffer *GstBuffer) (err error) {
	var gstReturn *C.GstFlowReturn

	gstReturn = (*C.GstFlowReturn)(unsafe.Pointer(C.malloc(C.sizeof_GstFlowReturn)))
	defer C.free(unsafe.Pointer(gstReturn))
	ds := (*C.gchar)(unsafe.Pointer(C.CString(detailedSignal)))
	defer C.g_free(C.gpointer(unsafe.Pointer(ds)))
	C.X_g_signal_emit_buffer_by_name(element.gstElement, ds, buffer.C, gstReturn)

	if *gstReturn != C.GST_FLOW_OK {
		err = errors.New(fmt.Sprintf("could not emit signal to element %#v with detailed signal %s and buffer %#v", element, detailedSignal, buffer))
		return
	}

	return
}

func AppSrcPushBuffer(element *GstElement, buffer *GstBuffer) (err error) {
	var gstReturn C.GstFlowReturn

	gstReturn = C.gst_app_src_push_buffer((*C.GstAppSrc)(unsafe.Pointer(element.gstElement)), buffer.C)
	if buffer.C == nil {
		logger.Debugf("[ GST ] BUFFER %p PUSHED", buffer.C)
	}
	if gstReturn != C.GST_FLOW_OK {
		err = errors.New("could not push buffer on appsrc element")
		return
	}

	return
}

func AppSrcPushSample(element *GstElement, sample *GstSample) (err error) {
	var gstReturn C.GstFlowReturn

	gstReturn = C.gst_app_src_push_sample((*C.GstAppSrc)(unsafe.Pointer(element.gstElement)), sample.C)
	if sample.C == nil {
		logger.Debugf("[ GST ] SAMPLE %p PUSHED", sample.C)
	}
	if gstReturn != C.GST_FLOW_OK {
		err = errors.New("could not push sample on appsrc element")
		return
	}

	return
}

func SampleGetBuffer(sample *GstSample) (buffer *GstBuffer, err error) {
	CGstBuffer := C.gst_sample_get_buffer(sample.C)
	if CGstBuffer == nil {
		err = errors.New("could not get buffer from sample")
		return
	}
	buffer = &GstBuffer{
		C: CGstBuffer,
	}

	return
}

type GstSample struct {
	C      *C.GstSample
	Width  uint32
	Height uint32
}

func AppSinkPullSample(element *GstElement) (sample *GstSample, err error) {
	CGstSample := C.gst_app_sink_pull_sample((*C.GstAppSink)(unsafe.Pointer(element.gstElement)))
	if CGstSample == nil {
		err = errors.New("could not pull a sample from appsink")
		return
	}
	CGstSampleCopy := C.gst_sample_copy(CGstSample)
	//CGstBuffer := C.gst_sample_get_buffer(CGstSample)
	//C.gst_buffer_unref(CGstBuffer)
	C.gst_sample_unref(CGstSample)
	// try to read width/height of pulled sample.
	var width, height C.gint
	CCaps := C.gst_sample_get_caps(CGstSampleCopy)
	CCStruct := C.gst_caps_get_structure(CCaps, 0)
	C.gst_structure_get_int(CCStruct, (*C.gchar)(unsafe.Pointer(C.CString("width"))), &width)
	C.gst_structure_get_int(CCStruct, (*C.gchar)(unsafe.Pointer(C.CString("height"))), &height)

	sample = &GstSample{
		C:      CGstSampleCopy,
		Width:  uint32(width),
		Height: uint32(height),
	}

	/*runtime.SetFinalizer(sample, func(gstSample *GstSample) {
		C.gst_sample_unref(gstSample.C)
	})*/

	return
}

func AppSinkIsEOS(element *GstElement) bool {
	Cbool := C.gst_app_sink_is_eos((*C.GstAppSink)(unsafe.Pointer(element.gstElement)))
	if Cbool == 1 {
		return true
	}

	return false
}

type GstClock struct {
	C *C.GstClock
}

func SampleRef(sample *GstSample) (sampleReturn *GstSample) {
	C.gst_sample_ref(sample.C)
	sampleReturn = sample

	return
}

func SampleUnref(sample *GstSample) {
	C.gst_sample_unref(sample.C)
}

func ElementGetClock(element *GstElement) (gstClock *GstClock) {
	CElementClock := C.gst_element_get_clock(element.gstElement)

	gstClock = &GstClock{
		C: CElementClock,
	}

	runtime.SetFinalizer(gstClock, func(gstClock *GstClock) {
		C.gst_object_unref(C.gpointer(unsafe.Pointer(gstClock.C)))
	})

	return
}

type GstClockTime C.GstClockTime

func ElementGetBaseTime(element *GstElement) GstClockTime {
	return GstClockTime(C.gst_element_get_base_time(element.gstElement))
}

func ClockGetTime(gstClock *GstClock) GstClockTime {
	return GstClockTime(C.gst_clock_get_time(gstClock.C))
}

type FormatOptions int

const (
	FormatUndefined FormatOptions = C.GST_FORMAT_UNDEFINED
	FormatDefault   FormatOptions = C.GST_FORMAT_DEFAULT
	FormatBytes     FormatOptions = C.GST_FORMAT_BYTES
	FormatTime      FormatOptions = C.GST_FORMAT_TIME
	FormatBuffers   FormatOptions = C.GST_FORMAT_BUFFERS
	FormatPercent   FormatOptions = C.GST_FORMAT_PERCENT
)

// Events

type EventTypeOption int

const (
	EventUnknown                EventTypeOption = C.GST_EVENT_UNKNOWN
	EventFlushStart             EventTypeOption = C.GST_EVENT_FLUSH_START
	EventFlushStop              EventTypeOption = C.GST_EVENT_FLUSH_STOP
	EventStreamStart            EventTypeOption = C.GST_EVENT_STREAM_START
	EventCaps                   EventTypeOption = C.GST_EVENT_CAPS
	EventSegment                EventTypeOption = C.GST_EVENT_SEGMENT
	EventStreamCollection       EventTypeOption = C.GST_EVENT_STREAM_COLLECTION
	EventTag                    EventTypeOption = C.GST_EVENT_TAG
	EventBufferSize             EventTypeOption = C.GST_EVENT_BUFFERSIZE
	EventSinkMessage            EventTypeOption = C.GST_EVENT_SINK_MESSAGE
	EventEos                    EventTypeOption = C.GST_EVENT_EOS
	EventToc                    EventTypeOption = C.GST_EVENT_TOC
	EventSegmentDone            EventTypeOption = C.GST_EVENT_SEGMENT_DONE
	EventGap                    EventTypeOption = C.GST_EVENT_GAP
	EvantQos                    EventTypeOption = C.GST_EVENT_QOS
	EventSeek                   EventTypeOption = C.GST_EVENT_SEEK
	EventNavigation             EventTypeOption = C.GST_EVENT_NAVIGATION
	EventLatency                EventTypeOption = C.GST_EVENT_LATENCY
	EventStep                   EventTypeOption = C.GST_EVENT_STEP
	EventReconfigure            EventTypeOption = C.GST_EVENT_RECONFIGURE
	EventTocSelect              EventTypeOption = C.GST_EVENT_TOC_SELECT
	EventCustomUpstream         EventTypeOption = C.GST_EVENT_CUSTOM_UPSTREAM
	EventCustomDownstream       EventTypeOption = C.GST_EVENT_CUSTOM_DOWNSTREAM
	EventCustomDownstreamOob    EventTypeOption = C.GST_EVENT_CUSTOM_DOWNSTREAM_OOB
	EventCustomDownstreamSticky EventTypeOption = C.GST_EVENT_CUSTOM_DOWNSTREAM_STICKY
	EventCustomBoth             EventTypeOption = C.GST_EVENT_CUSTOM_BOTH
	EventCustomBothOob          EventTypeOption = C.GST_EVENT_CUSTOM_BOTH_OOB
)

type GstEvent struct {
	C *C.GstEvent
}

func EventNewCustom(eventType EventTypeOption, structure *GstStructure) (event *GstEvent) {
	CEvent := C.gst_event_new_custom(C.GstEventType(eventType), structure.C)

	event = &GstEvent{
		C: CEvent,
	}

	return
}

func EventNewFlushStart() (event *GstEvent) {
	CEvent := C.gst_event_new_flush_start()

	event = &GstEvent{
		C: CEvent,
	}

	return
}

func EventNewFlushStop() (event *GstEvent) {
	CEvent := C.gst_event_new_flush_stop(C.gboolean(0))

	event = &GstEvent{
		C: CEvent,
	}

	return
}

func EventRef(event *GstEvent) *GstEvent {
	C.gst_event_ref(event.C)

	return event
}

func EventUnref(event *GstEvent) {
	C.gst_event_unref(event.C)
}

// Structure

type GstStructure struct {
	C *C.GstStructure
}

func StructureNewEmpty(name string, deallocate bool) (structure *GstStructure) {
	CName := (*C.gchar)(unsafe.Pointer(C.CString(name)))
	CGstStructure := C.gst_structure_new_empty(CName)

	structure = &GstStructure{
		C: CGstStructure,
	}

	if deallocate == true {
		runtime.SetFinalizer(structure, func(structure *GstStructure) {
			C.gst_structure_free(structure.C)
		})
	}

	return
}

func StructureSetValue(structure *GstStructure, name string, value interface{}) {
	logger := plogger.New()
	CName := (*C.gchar)(unsafe.Pointer(C.CString(name)))
	defer C.g_free(C.gpointer(unsafe.Pointer(CName)))
	switch value.(type) {
	case string:
		logger.Debugf("[ GST ] Found string %s=%s", name, value.(string))
		str := (*C.gchar)(unsafe.Pointer(C.CString(value.(string))))
		defer C.g_free(C.gpointer(unsafe.Pointer(str)))
		C.X_gst_structure_set_string(structure.C, CName, str)
	case int:
		logger.Debugf("[ GST ] Found int %s=%d", name, value.(int))
		C.X_gst_structure_set_int(structure.C, CName, C.gint(value.(int)))
	case uint32:
		logger.Debugf("[ GST ] Found uint32 %s=%d", name, value.(uint32))
		C.X_gst_structure_set_uint(structure.C, CName, C.guint(value.(uint32)))
	case bool:
		var v int
		if value.(bool) == true {
			v = 1
		} else {
			v = 0
		}
		logger.Debugf("[ GST ] Found bool %s=%d", name, value)
		C.X_gst_structure_set_bool(structure.C, CName, C.gboolean(v))
	}

	return
}

func StructureToString(structure *GstStructure) (str string) {
	Cstr := C.gst_structure_to_string(structure.C)
	str = C.GoString((*C.char)(unsafe.Pointer(Cstr)))
	C.g_free((C.gpointer)(unsafe.Pointer(Cstr)))

	return
}

func StructureNewFromString(str string) (structure *GstStructure) {
	Cstr := (*C.gchar)(unsafe.Pointer(C.CString(str)))
	defer C.g_free(C.gpointer(unsafe.Pointer(Cstr)))
	Cstructure := C.gst_structure_new_from_string(Cstr)

	structure = &GstStructure{
		C: Cstructure,
	}

	runtime.SetFinalizer(structure, func(structure *GstStructure) {
		C.gst_structure_free(structure.C)
	})

	return
}

type GMainLoop struct {
	C *C.GMainLoop
}

func MainLoopNew() (loop *GMainLoop) {
	CLoop := C.g_main_loop_new(nil, C.gboolean(0))
	loop = &GMainLoop{C: CLoop}
	runtime.SetFinalizer(loop, func(loop *GMainLoop) {
		C.g_main_loop_unref(loop.C)
	})

	return
}

func (loop *GMainLoop) Run() {
	C.g_main_loop_run(loop.C)
}

func MainContextIteration() {
	C.g_main_context_iteration(C.g_main_context_default(), 1)
}

// Super Func
func AdjustClockOffset(p1 *GstElement, p2 *GstElement) {
	gstClockP1 := ElementGetClock(p1)
	baseTimeP1 := ElementGetBaseTime(p1)

	PipelineUseClock(p2, gstClockP1, baseTimeP1)
}

// Bus

type GstBus struct {
	C           *C.GstBus
	callbackCtx *BusCallbackCtx
}

func BusNew() (bus *GstBus) {
	CGstBus := C.gst_bus_new()

	bus = &GstBus{
		C: CGstBus,
	}

	return
}

type GstMessage struct {
	C *C.GstMessage
}

type PriorityOption int

const (
	PriorityDefault PriorityOption = C.G_PRIORITY_DEFAULT
)

func (bus *GstBus) AddSignalWatchFull(priority PriorityOption) {
	C.gst_bus_add_signal_watch_full(bus.C, C.gint(priority))
}

type BusMessageCallback func(bus *GstBus, message *GstMessage, dataId int)

type GstBusData struct {
	messageCallback BusMessageCallback
	Id              C.int
}

type BusCallbackCtx struct {
	gstBusData []*GstBusData
}

func NewBusCallbackCtx() (ctx *BusCallbackCtx) {
	ctx = new(BusCallbackCtx)

	return
}

func (bus *GstBus) SetMessageCallback(ctx context.Context, busMessageCallback BusMessageCallback, dataId int) {
	log, _ := plogger.FromContext(ctx)
	gstBusData := new(GstBusData)
	gstBusData.messageCallback = busMessageCallback
	gstBusData.Id = C.int(dataId)
	bus.callbackCtx.gstBusData = append(bus.callbackCtx.gstBusData, gstBusData)
	log.Debugf("[ GST ] SetLessageCallback with gstData %#v", gstBusData)
	detailedSignal := (*C.gchar)(unsafe.Pointer(C.CString("message")))
	defer C.g_free(C.gpointer(unsafe.Pointer(detailedSignal)))
	C.X_g_signal_connect_data(C.gpointer(bus.C), detailedSignal, (*[0]byte)(C.cb_bus_message), (C.gpointer)(unsafe.Pointer(gstBusData)), nil, 0)
}

func (bus *GstBus) Pop() (message *GstMessage) {
	CGstMessage := C.gst_bus_pop(bus.C)

	message = &GstMessage{
		C: CGstMessage,
	}

	runtime.SetFinalizer(message, func(message *GstMessage) {
		C.gst_message_unref(message.C)
	})

	return
}

func (bus *GstBus) Poll(messageType GstMessageTypeOption) (message *GstMessage) {
	CGstMessage := C.gst_bus_poll(bus.C, C.GstMessageType(messageType), 18446744073709551615)
	if CGstMessage == nil {
		return nil
	}

	message = &GstMessage{
		C: CGstMessage,
	}

	runtime.SetFinalizer(message, func(message *GstMessage) {
		C.gst_message_unref(message.C)
	})

	return
}

//export go_callback_bus_message_thunk
func go_callback_bus_message_thunk(Cbus *C.GstBus, Cmessage *C.GstMessage, CpollData C.gpointer) {
	logger := plogger.New()
	logger.Debugf("[ GST ] go_callback_bus_message_thunk called, Cbus is %p, Cmessage is %p, CpollData %p", Cbus, Cmessage, CpollData)

	gstBusData := (*GstBusData)(unsafe.Pointer(CpollData))
	callback := gstBusData.messageCallback
	dataId := int(gstBusData.Id)
	bus := &GstBus{
		C: Cbus,
	}
	message := &GstMessage{
		C: Cmessage,
	}

	logger.Debugf("[ GST ] callback is %p", callback)

	callback(bus, message, dataId)
}

// Messages

func (message *GstMessage) GetType() (messageType GstMessageTypeOption) {
	CMessageType := C.X_GST_MESSAGE_TYPE(message.C)
	messageType = GstMessageTypeOption(CMessageType)

	return
}

func (message *GstMessage) GetName() (name string) {
	messageType := message.GetType()
	Cname := C.gst_message_type_get_name(C.GstMessageType(messageType))
	name = C.GoString((*C.char)(unsafe.Pointer(Cname)))

	return
}

func (message *GstMessage) GetStructure() (structure *GstStructure) {
	Cstructure := C.gst_message_get_structure(message.C)
	structure = &GstStructure{
		C: Cstructure,
	}

	return
}

type GstMessageTypeOption C.GstMessageType

const (
	MessageUnknown          GstMessageTypeOption = C.GST_MESSAGE_UNKNOWN
	MessageEos              GstMessageTypeOption = C.GST_MESSAGE_EOS
	MessageError            GstMessageTypeOption = C.GST_MESSAGE_ERROR
	MessageWarning          GstMessageTypeOption = C.GST_MESSAGE_WARNING
	MessageInfo             GstMessageTypeOption = C.GST_MESSAGE_INFO
	MessageTag              GstMessageTypeOption = C.GST_MESSAGE_TAG
	MessageBuffering        GstMessageTypeOption = C.GST_MESSAGE_BUFFERING
	MessageStateChanged     GstMessageTypeOption = C.GST_MESSAGE_STATE_CHANGED
	MessageStateDirty       GstMessageTypeOption = C.GST_MESSAGE_STATE_DIRTY
	MessageStepDone         GstMessageTypeOption = C.GST_MESSAGE_STEP_DONE
	MessageClockProvide     GstMessageTypeOption = C.GST_MESSAGE_CLOCK_PROVIDE
	MessageClockLost        GstMessageTypeOption = C.GST_MESSAGE_CLOCK_LOST
	MessageStructureChange  GstMessageTypeOption = C.GST_MESSAGE_STREAM_STATUS
	MessageApplication      GstMessageTypeOption = C.GST_MESSAGE_APPLICATION
	MessageElement          GstMessageTypeOption = C.GST_MESSAGE_ELEMENT
	MessageSegmentStart     GstMessageTypeOption = C.GST_MESSAGE_SEGMENT_START
	MessageSegmentDone      GstMessageTypeOption = C.GST_MESSAGE_SEGMENT_DONE
	MessageDurationChanged  GstMessageTypeOption = C.GST_MESSAGE_DURATION_CHANGED
	MessageLatency          GstMessageTypeOption = C.GST_MESSAGE_LATENCY
	MessageAsyncStart       GstMessageTypeOption = C.GST_MESSAGE_ASYNC_START
	MessageAsyncDone        GstMessageTypeOption = C.GST_MESSAGE_ASYNC_DONE
	MessageRequestState     GstMessageTypeOption = C.GST_MESSAGE_REQUEST_STATE
	MessageStepStart        GstMessageTypeOption = C.GST_MESSAGE_STEP_START
	MessageQos              GstMessageTypeOption = C.GST_MESSAGE_QOS
	MessageProgress         GstMessageTypeOption = C.GST_MESSAGE_PROGRESS
	MessageToc              GstMessageTypeOption = C.GST_MESSAGE_TOC
	MessageResetTime        GstMessageTypeOption = C.GST_MESSAGE_RESET_TIME
	MessageStreamStart      GstMessageTypeOption = C.GST_MESSAGE_STREAM_START
	MessageNeedContext      GstMessageTypeOption = C.GST_MESSAGE_NEED_CONTEXT
	MessageHaveContext      GstMessageTypeOption = C.GST_MESSAGE_HAVE_CONTEXT
	MessageExtended         GstMessageTypeOption = C.GST_MESSAGE_EXTENDED
	MessageDeviceAdded      GstMessageTypeOption = C.GST_MESSAGE_DEVICE_ADDED
	MessageDeviceRemoved    GstMessageTypeOption = C.GST_MESSAGE_DEVICE_REMOVED
	MessagePropertyNotify   GstMessageTypeOption = C.GST_MESSAGE_PROPERTY_NOTIFY
	MessageStreamCollection GstMessageTypeOption = C.GST_MESSAGE_STREAM_COLLECTION
	MessageStreamsSelected  GstMessageTypeOption = C.GST_MESSAGE_STREAMS_SELECTED
	MessageRedirect         GstMessageTypeOption = C.GST_MESSAGE_REDIRECT
	MessageAny              GstMessageTypeOption = C.GST_MESSAGE_ANY
)

// Ghost

func GhostPadNew(name string, target *GstPad) (ghostPad *GstPad) {
	Cname := (*C.gchar)(unsafe.Pointer(C.CString(name)))
	defer C.g_free(C.gpointer(unsafe.Pointer(Cname)))
	CghostPad := C.gst_ghost_pad_new(Cname, target.pad)

	ghostPad = &GstPad{
		pad: CghostPad,
	}

	return
}
