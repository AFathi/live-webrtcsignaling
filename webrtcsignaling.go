package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"runtime"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	plogger "github.com/heytribe/go-plogger"
	"github.com/heytribe/live-rabbitmqlib"
	"github.com/heytribe/live-webrtcsignaling/dtls"
	"github.com/heytribe/live-webrtcsignaling/gst"
	"github.com/heytribe/live-webrtcsignaling/my"

	_ "net/http/pprof"
)

var config *Config
var rooms *Rooms
var hub *Hub
var rmq liverabbitmq.Rmq
var udpStats net.Conn
var stunTransactions *StunTransactionsMap
var dtlsCtx *dtls.Ctx
var gWsId uint64 = 0
var upgrader = websocket.Upgrader{
	CheckOrigin:     checkWsOrigin,
	ReadBufferSize:  8192,
	WriteBufferSize: 8192,
}
var features *Features

func serveRoot(w http.ResponseWriter, r *http.Request) {
	var urlPath string
	if r.URL.Path == `/` {
		urlPath = `/www/index.html`
	} else {
		urlPath = `/www/` + r.URL.Path
	}
	b, err := ioutil.ReadFile(config.Pwd + urlPath)
	if err != nil {
		return
	}
	w.Write(b)

	return
}

func checkWsOrigin(r *http.Request) bool {
	//if r.Header["Origin"][0] != `https://wssdev.tribe.pm` &&
	//   r.Header["Origin"][0] != `https://192.168.0.22` {
	//  return false
	// }
	return true
}

func serveApi(w http.ResponseWriter, r *http.Request) {
	logger := plogger.New()
	ws, err := upgrader.Upgrade(w, r, w.Header())
	if err != nil {
		logger.Infof("XX Cannot upgrade the ws connection: %s", err)
		return
	}
	// we can create a logger & save it into context
	wsId := atomic.AddUint64(&gWsId, 1)
	log := logger.Prefix("ws:%d", wsId)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx = plogger.NewContext(ctx, log)
	//
	log.Infof("origin is %s", r.Header["Origin"])
	// Create and register new Websocket connection
	c := NewConnection(wsId, ws)
	log.Infof(`Headers are %#v`, r.Header)
	if r.Header[`X-Real-Ip`] != nil {
		c.ip = r.Header[`X-Real-Ip`][0]
	} else {
		c.ip = `0.0.0.0`
	}

	log.Infof("Register Websocket connection %#v in hub h.register %#v", c, hub.register)
	hub.register <- c
	log.Infof("Register Websocket connection in hub OK")

	c.state = `connected`

	go c.writePump(ctx)
	log.Infof("writePump initiated, calling readPump()")
	err = c.readPump(ctx)
	log.Infof("websocket closed or error with %s", err)
	if c.exit == false && strings.Contains(err.Error(), "websocket: close") == false {
		log.OnError(err, "websocket not closed correctly, waiting 30 seconds before destroying connection")
		if c.state == `connected` {
			c.state = `timeout`
		}
		c.manageTimeout(ctx, time.Second*30)
		return
	}

	// Socket has been closed correctly, unregister
	// Acquire joinMutex to avoid unregister during join
	c.joinMutex.Lock(ctx)
	hub.unregister <- c
	c.joinMutex.Unlock(ctx)
}

func main() {
	var err error

	// Create logger context with prefix
	log := plogger.New()
	ctx := plogger.NewContext(context.Background(), log)

	// Read the conf (envs)
	config = NewConfig()
	err = config.Init(ctx)
	if log.OnError(err, "could not config, exiting...") {
		os.Exit(1)
	}

	// setup libraries
	plogger.FilterOutputs(config.PLogger)
	if config.Env == ENV_DEVELOPPEMENT {
		// tweaking my package for dev env.
		my.EnableAssert()
	}

	// init global vars
	hub = NewHub()
	rooms = NewRooms()
	stunTransactions = NewStunTransactionsMap(ctx)
	features = NewFeatures()

	// init features
	features.Register(ctx, "forcecodec", "VP8")      // val=VP8,H264
	features.Register(ctx, "facedetect", "false") // val=true,false

	// Initialize DTLS package
	dtlsCtx, err = dtls.Init(config.Cert.FilePath, config.Cert.KeyFilePath)
	if log.OnError(err, "could not initialize OpenSSL in DTLS mode") {
		os.Exit(1)
	}
	// Opening statsd connection
	raddr, err := net.ResolveUDPAddr("udp", config.GraphiteIPV4+":8125")
	log.OnError(err, "could not resolve UDP address")
	udpStats, err = net.DialUDP("udp", nil, raddr)
	log.OnError(err, "could not open udp socket to graphite server, nothing will be logged")
	defer udpStats.Close()

	// init RabbitMQ
	rmq.ConnectTLS(config.RabbitMqURL, config.Cert.FilePath, config.Cert.FilePath, config.Cert.KeyFilePath)

	// Run RabbitMQ connection + RPC repy queue
	rmq.CreateServerRpcReply(config.Instance.FullUnitName + `-rpcreplyqueue`)
	go rmq.RunServerRpcReply()

	// set up RabbitMQ event listeners
	eventsRoutingKeysMap := make(map[string]interface{})
	eventsRoutingKeysMap[liverabbitmq.LiveAdminEventLogFiltersUpdateRK] = EVUpdateLogFilters
	rmq.CreateServerEvents(liverabbitmq.LiveBackendEvents, config.Instance.FullUnitName+`-`+liverabbitmq.LiveEvents, eventsRoutingKeysMap)
	go rmq.RunAllServerEvents()

	// initializing objects handlers
	go hub.run()
	hUdp.init()
	go hUdp.run(ctx)
	go sendHeartbeatRoomsOnline(ctx, 60)

	// Running GMainLoop for receiving bus messages
	loop := gst.MainLoopNew()
	go loop.Run()

	// start sending state to mq
	go SendStatePeriodicallyToAMQP(ctx, rmq)

	http.HandleFunc("/memstats", func(w http.ResponseWriter, r *http.Request) {
		var mem runtime.MemStats

		runtime.GC()
		runtime.ReadMemStats(&mem)
		fmt.Fprintf(w,
			`GcNum: %d

Heap: %d/%d by %d objects (reachable or unreachable/amount of virtual address space reserved, maybe not used)
HeapAllTime: %d

StackInuse: %d/%d


@see https://golang.org/pkg/runtime/#MemStats
`,
			mem.NumGC, mem.HeapAlloc, mem.HeapSys, mem.HeapObjects, mem.TotalAlloc, mem.StackInuse, mem.StackSys)
	})

	http.HandleFunc("/globals", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w,
			`
config: %#v

udpStats: %#v

dtlsCtx: %#v

--------

rooms.Data: %#v

hub.socketIds.Data: %#v

stunTransactions.Data: %#v
`,
			config, udpStats, dtlsCtx, rooms.Data, hub.socketIds.Data, stunTransactions.Data)
	})

	http.HandleFunc("/state", httpStateController)
	http.HandleFunc("/", serveRoot)
	http.HandleFunc("/api", serveApi)
	err = http.ListenAndServeTLS(":"+config.Network.PortNumber, config.Cert.FilePath, config.Cert.KeyFilePath, nil)
	if err != nil {
		log.Fatalf("ListenAndServe: %s", err.Error())
	}
}
