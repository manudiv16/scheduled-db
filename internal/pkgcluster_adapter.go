package internal

import (
	"github.com/manudiv16/pkgcluster"
	"scheduled-db/internal/logger"
)

func init() {
	pkgcluster.SetLogger(pkgcluster.Logger{
		Debug: logger.Debug,
		Info:  logger.Info,
		Warn:  logger.Warn,
		Error: logger.Error,
	})
}
