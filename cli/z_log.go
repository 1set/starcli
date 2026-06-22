package cli

import (
	"go.uber.org/zap"
)

var log *zap.SugaredLogger

func init() {
	log = zap.NewNop().Sugar()
}

// SetLog sets the logger from outside the package.
func SetLog(l *zap.SugaredLogger) {
	log = l
}
