package ciel

import (
	"io/ioutil"
	stdlog "log"
	"os"
	"strconv"
)

const logFlags = stdlog.Lshortfile | stdlog.Ltime

var (
	errlog  = stdlog.New(os.Stderr, "\033[31;1m[ERR ]\033[0m ", logFlags)
	warnlog = stdlog.New(os.Stderr, "\033[33;1m[WARN]\033[0m ", logFlags)
	infolog = stdlog.New(os.Stderr, "\033[32;1m[INFO]\033[0m ", logFlags)
	dbglog  = stdlog.New(os.Stderr, "\033[39;1m[DBG ]\033[0m ", logFlags)
)

var LogLevel = 0

func init() {
	LogLevel, _ = strconv.Atoi(os.Getenv("CIEL_LOGLEVEL"))
	switch LogLevel {
	case -1:
		errlog.SetOutput(ioutil.Discard)
		fallthrough
	case 0:
		warnlog.SetOutput(ioutil.Discard)
		fallthrough
	case 1:
		infolog.SetOutput(ioutil.Discard)
		fallthrough
	case 2:
		dbglog.SetOutput(ioutil.Discard)
	}
}
