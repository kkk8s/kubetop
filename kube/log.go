package kube

import (
	"flag"
	"os"

	"k8s.io/klog/v2"
)

type LogLevel int

const (
	INFO LogLevel = iota
	WARNING
	ERROR
)


func init() {
	klog.InitFlags(nil)
	flag.Parse()

	klog.SetOutput(os.Stdout)
}

func Info(err error, msg string) {
	if err != nil {
		klog.InfoDepth(1, msg, err)
	}
}

func Warning(err error, msg string) {
	if err != nil {
		klog.WarningDepth(1, msg, err)
	}
}

func Error(err error, msg string) {
	if err != nil {
		klog.ErrorDepth(1, msg, err)
		os.Exit(1)
	}
}

func SetLogLevel(level LogLevel) {
	switch level {
	case INFO:
		_ = flag.Set("v", "2")
	case WARNING:
		_ = flag.Set("v", "4")
	case ERROR:
		_ = flag.Set("v", "6")
	default:
		_ = flag.Set("v", "2")
	}
}