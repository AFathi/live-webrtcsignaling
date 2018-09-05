package main

/*
 * Interface Proxy
 *
 * we don't know how to match iface plogger.PLogger with rtcp.IPLogger
 * we create a proxy object, wrapping plogger.PLogger iface into a rtcp.IPLogger
 */

import plogger "github.com/heytribe/go-plogger"
import "github.com/heytribe/live-webrtcsignaling/rtcp"

/*
 * plogger.PLogger => rtcp.IPLogger
 */
type IProxyRtcpIPLogger struct {
	plogger.PLogger
}

func (l *IProxyRtcpIPLogger) Prefix(format string, args ...interface{}) rtcp.IPLogger {
	l.PLogger = l.PLogger.Prefix(format, args...)
	return l
}

func (l *IProxyRtcpIPLogger) Tag(format string) rtcp.IPLogger {
	l.PLogger = l.PLogger.Tag(format)
	return l
}

func ICastPLoggerToRtcpPLogger(log plogger.PLogger) *IProxyRtcpIPLogger {
	o := new(IProxyRtcpIPLogger)
	o.PLogger = log
	return o
}
