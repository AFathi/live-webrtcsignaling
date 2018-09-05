package main

import (
	"context"
	"os"
	"strconv"
	"time"

	plogger "github.com/heytribe/go-plogger"
	"github.com/heytribe/live-webrtcsignaling/my"
)

const (
	ENV_DEVELOPPEMENT int = iota
)

const (
	DEV_MIN_UDP_PORT int = 42042
	DEV_MAX_UDP_PORT int = 42082 // excluded
)

type Bitrate struct {
	Start int
	Min   int
	Max   int
	Step  int
}

type ModeOptions int

const (
	ModeSFU = iota
	ModeMCU
)

type Config struct {
	Env         int
	Mode				ModeOptions
	StaticPorts bool
	PLogger     string
	Feature     struct {
		Profiling bool
	}
	Network struct {
		PortNumber string // fixme
		PublicIPV4 string
		Ws         struct {
			WriteWait        time.Duration
			PongWait         time.Duration
			PingPeriod       time.Duration
			WebRTCPingPeriod time.Duration // fixme: should be here ?
			MaxMessageSize   int64
		}
	}
	Instance struct {
		Uuid         string
		FullUnitName string
	}
	Cert struct {
		FilePath    string
		KeyFilePath string
	}
	JWTSecret    string
	GraphiteIPV4 string
	RabbitMqURL  string
	//
	Pwd string
	//
	Rtcp struct {
		RembHistory int
	}
	// Bitrates
	Bitrates struct {
		Audio Bitrate
		Video Bitrate
	}
	CpuCores int
	Vp8      struct {
		EndUsage        int
		CpuUsed         int
		TokenPartitions int
		Deadline        int
		ErrorResilient  int
	}
}

func NewConfig() *Config {
	return new(Config)
}

func (c *Config) Init(ctx context.Context) (err error) {
	ctx = plogger.NewContextAddPrefix(ctx, "Config")
	log, _ := plogger.FromContext(ctx)

	c.Mode = ModeSFU

	staticPorts := os.Getenv("STATIC_PORTS")
	if staticPorts == "1" {
		c.StaticPorts = true
	}

	// FIXME: use var MCU_ENV
	c.Env = ENV_DEVELOPPEMENT
	// logger config
	c.PLogger = my.Getenv("MCU_DEBUG", "*:warn,tag*:warn")
	/*
	 * const
	 */
	// Time allowed to write a message to the peer.
	c.Network.Ws.WriteWait = 10 * time.Second
	// Time allowed to read the next pong message from the peer.
	c.Network.Ws.PongWait = 15 * time.Second
	// Send pings to peer with this period. Must be less than pongWait.
	c.Network.Ws.PingPeriod = 5 * time.Second
	// Webrtc ping period
	c.Network.Ws.WebRTCPingPeriod = 60 * time.Second
	// Maximum message size allowed from peer.
	c.Network.Ws.MaxMessageSize = 8192
	// must be positive
	c.Rtcp.RembHistory = 30
	//
	c.Bitrates.Audio.Start, err = strconv.Atoi(os.Getenv("BITRATE_AUDIO_START"))
	if log.OnError(err, "invalid env BITRATE_AUDIO_START") {
		return
	}
	c.Bitrates.Audio.Min, err = strconv.Atoi(os.Getenv("BITRATE_AUDIO_MIN"))
	if log.OnError(err, "invalid env BITRATE_AUDIO_MIN") {
		return
	}
	c.Bitrates.Audio.Max, err = strconv.Atoi(os.Getenv("BITRATE_AUDIO_MAX"))
	if log.OnError(err, "invalid env BITRATE_AUDIO_MAX") {
		return
	}
	c.Bitrates.Audio.Step, err = strconv.Atoi(os.Getenv("BITRATE_AUDIO_STEP"))
	if log.OnError(err, "invalid env BITRATE_AUDIO_STEP") {
		return
	}
	c.Bitrates.Video.Start, err = strconv.Atoi(os.Getenv("BITRATE_VIDEO_START"))
	if log.OnError(err, "invalid env BITRATE_AUDIO_START") {
		return
	}
	c.Bitrates.Video.Min, err = strconv.Atoi(os.Getenv("BITRATE_VIDEO_MIN"))
	if log.OnError(err, "invalid env BITRATE_AUDIO_MIN") {
		return
	}
	c.Bitrates.Video.Max, err = strconv.Atoi(os.Getenv("BITRATE_VIDEO_MAX"))
	if log.OnError(err, "invalid env BITRATE_VIDEO_MAX") {
		return
	}
	c.Bitrates.Video.Step, err = strconv.Atoi(os.Getenv("BITRATE_VIDEO_STEP"))
	if log.OnError(err, "invalid env BITRATE_VIDEO_STEP") {
		return
	}

	/*
	 * ENV
	 */
	c.Network.PortNumber = os.Getenv("PORT_NUMBER")
	c.Network.PublicIPV4 = os.Getenv("PUBLIC_IPV4")
	//
	c.Instance.Uuid = os.Getenv("INSTANCE_UUID")
	c.Instance.FullUnitName = os.Getenv("FULL_UNIT_NAME")
	//
	c.Cert.FilePath = os.Getenv("CERT_FILE_PATH")
	c.Cert.KeyFilePath = os.Getenv("KEY_FILE_PATH")
	//
	c.JWTSecret = os.Getenv("JWT_SECRET")
	c.GraphiteIPV4 = os.Getenv("GRAPHITE_IPV4")
	c.RabbitMqURL = os.Getenv("RABBITMQ_URL")
	c.CpuCores, err = strconv.Atoi(os.Getenv("CPU_CORES"))
	if log.OnError(err, "invalid env CPU_CORES") {
		return
	}
	// VP8
	c.Vp8.EndUsage, err = strconv.Atoi(os.Getenv("VP8_END_USAGE"))
	if log.OnError(err, "invalid env VP8_END_USAGE") {
		return
	}
	c.Vp8.CpuUsed, err = strconv.Atoi(os.Getenv("VP8_CPU_USED"))
	if log.OnError(err, "invalid env VP8_CPU_USED") {
		return
	}
	c.Vp8.TokenPartitions, err = strconv.Atoi(os.Getenv("VP8_TOKEN_PARTITIONS"))
	if log.OnError(err, "invalid env VP8_TOKEN_PARTITIONS") {
		return
	}
	c.Vp8.Deadline, err = strconv.Atoi(os.Getenv("VP8_DEADLINE"))
	if log.OnError(err, "invalid env VP8_DEADLINE") {
		return
	}
	c.Vp8.ErrorResilient, err = strconv.Atoi(os.Getenv("VP8_ERROR_RESILIENT"))
	if log.OnError(err, "invalid env VP8_ERROR_RESILIENT") {
		return
	}
	/*
	 * fetched/computed
	 */

	c.Pwd, err = os.Getwd()
	if err != nil {
		plogger.New().Fatalf("cannot get current directory: %s", err.Error())
		return
	}

	return
}
