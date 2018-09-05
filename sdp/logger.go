package sdp

import (
	"log"
	"os"
)

// logger interface
type Logger interface {
	Debugf(format string, args ...interface{})
	Infof(format string, args ...interface{})
	Warnf(format string, args ...interface{})
	Errorf(format string, args ...interface{})
	Fatalf(format string, args ...interface{})
}

//
// simple console logger
// based on https://www.captaincodeman.com/2015/03/05/dependency-injection-in-go-golang
//
const (
	CLR_0 = "\x1b[30;1m"
	CLR_R = "\x1b[31;1m"
	CLR_G = "\x1b[32;1m"
	CLR_Y = "\x1b[33;1m"
	CLR_B = "\x1b[34;1m"
	CLR_M = "\x1b[35;1m"
	CLR_C = "\x1b[36;1m"
	CLR_W = "\x1b[37;1m"
	CLR_N = "\x1b[0m"
)

type consoleLogger struct {
	debug *log.Logger
	info  *log.Logger
	warn  *log.Logger
	error *log.Logger
	fatal *log.Logger
}

func NewConsoleLogger() Logger {
	return &consoleLogger{
		log.New(os.Stdout, CLR_0, log.Ldate|log.Ltime|log.Lmicroseconds|log.Lshortfile),
		log.New(os.Stdout, CLR_G, log.Ldate|log.Ltime|log.Lmicroseconds|log.Lshortfile),
		log.New(os.Stdout, CLR_Y, log.Ldate|log.Ltime|log.Lmicroseconds|log.Lshortfile),
		log.New(os.Stdout, CLR_R, log.Ldate|log.Ltime|log.Lmicroseconds|log.Lshortfile),
		log.New(os.Stdout, CLR_C, log.Ldate|log.Ltime|log.Lmicroseconds|log.Lshortfile),
	}
}

func (logger *consoleLogger) Debugf(format string, args ...interface{}) {
	logger.debug.Printf("[ DEBUG ] "+format, args...)
}

func (logger *consoleLogger) Infof(format string, args ...interface{}) {
	logger.info.Printf("[ INFO ] "+format, args...)
}

func (logger *consoleLogger) Warnf(format string, args ...interface{}) {
	logger.warn.Printf("[ WARN ] "+format, args...)
}

func (logger *consoleLogger) Errorf(format string, args ...interface{}) {
	logger.error.Printf("[ ERROR ] "+format, args...)
}

func (logger *consoleLogger) Fatalf(format string, args ...interface{}) {
	logger.fatal.Printf("[ FATAL ] "+format, args...)
}
