package xgb

import (
	"log"
	"os"
)

// Log controls whether XGB emits errors to stderr. By default, it is enabled.
var PrintLog = true

// log is a wrapper around a log.PrintLogger so we can control whether it should
// output anything.
type xgblog struct {
	*log.Logger
}

func newLogger() xgblog {
	return xgblog{log.New(os.Stderr, "XGB: ", log.Lshortfile)}
}

func (lg xgblog) Print(v ...interface{}) {
	if PrintLog {
		lg.Logger.Print(v...)
	}
}

func (lg xgblog) Printf(format string, v ...interface{}) {
	if PrintLog {
		lg.Logger.Printf(format, v...)
	}
}

func (lg xgblog) Println(v ...interface{}) {
	if PrintLog {
		lg.Logger.Println(v...)
	}
}

func (lg xgblog) Fatal(v ...interface{}) {
	if PrintLog {
		lg.Logger.Fatal(v...)
	} else {
		os.Exit(1)
	}
}

func (lg xgblog) Fatalf(format string, v ...interface{}) {
	if PrintLog {
		lg.Logger.Fatalf(format, v...)
	} else {
		os.Exit(1)
	}
}

func (lg xgblog) Fatalln(v ...interface{}) {
	if PrintLog {
		lg.Logger.Fatalln(v...)
	} else {
		os.Exit(1)
	}
}

func (lg xgblog) Panic(v ...interface{}) {
	if PrintLog {
		lg.Logger.Panic(v...)
	} else {
		panic("")
	}
}

func (lg xgblog) Panicf(format string, v ...interface{}) {
	if PrintLog {
		lg.Logger.Panicf(format, v...)
	} else {
		panic("")
	}
}

func (lg xgblog) Panicln(v ...interface{}) {
	if PrintLog {
		lg.Logger.Panicln(v...)
	} else {
		panic("")
	}
}
