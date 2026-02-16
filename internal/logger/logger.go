package logger

import (
	"go.uber.org/zap"
)

var Log *zap.Logger

func Init(level string) error {
	var err error

	if level == "dev" {
		Log, err = zap.NewDevelopment()
		if err != nil {
			return err
		}
	} else {
		Log, err = zap.NewProduction()
		if err != nil {
			return err
		}
	}

	zap.ReplaceGlobals(Log)

	return nil
}

func Sync() {
	if Log != nil {
		_ = Log.Sync()
	}
}
