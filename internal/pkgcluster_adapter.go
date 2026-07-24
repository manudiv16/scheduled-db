package internal

import (
	"scheduled-db/internal/logger"

	"github.com/manudiv16/pkgcluster"
)

func init() {
	pkgcluster.SetLogger(pkgcluster.Logger{
		Debug: logger.Debug,
		Info:  logger.Info,
		Warn:  logger.Warn,
		Error: logger.Error,
	})
}
