package logger

import (
	log "github.com/sirupsen/logrus"
	"os"
	"subscriptionbot/config"
)

func InitLog(cfg *config.Config) {
	log.SetReportCaller(true)
	log.SetOutput(os.Stdout)
	log.SetFormatter(&log.JSONFormatter{})
	logLevel, err := log.ParseLevel(cfg.LogLevel)
	if err != nil {
		logLevel = log.TraceLevel
	}
	log.SetLevel(logLevel)
}
