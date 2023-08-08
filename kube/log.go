package kube

import (
	"os"

	"github.com/sirupsen/logrus"
)

var log = logrus.New()

func init() {
	log.Out = os.Stdout
	log.Formatter = &logrus.TextFormatter{
		FullTimestamp: true,
		DisableQuote: true,
	}
	log.SetLevel(logrus.InfoLevel)
}

func HandlerError(err error, msg string) {
	if err != nil {
		log.WithError(err).Fatalln(msg)
	}
}
