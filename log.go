package ciel

import (
	"io/ioutil"
	stdlog "log"
	"os"
	"strconv"
)

const logFlags = stdlog.Lshortfile | stdlog.LstdFlags

var (
	errlog  = stdlog.New(os.Stderr, "[\033[31mERR \033[0m] ", logFlags)
	warnlog = stdlog.New(os.Stderr, "[\033[33mWARN\033[0m] ", logFlags)
	infolog = stdlog.New(os.Stderr, "[\033[32mINFO\033[0m] ", logFlags)
	dbglog  = stdlog.New(os.Stderr, "[\033[39mDBG \033[0m] ", logFlags)
)

var LogLevel = 3 // 0 1 2 3

func init() {
	i, _ := strconv.Atoi(os.Getenv("CIEL_LOGLEVEL"))
	switch i {
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
		fallthrough
	case 3:
		// No-op
	}
}
