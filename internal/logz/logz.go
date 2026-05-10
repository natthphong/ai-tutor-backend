package logz

import (
	"fmt"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"time"
)

var log *zap.Logger
var undoFn func()

func Init(loglevel, name string) {

	ec := zap.NewProductionEncoderConfig()
	ec.EncodeTime = zapcore.ISO8601TimeEncoder

	c := zap.NewProductionConfig()
	c.Level = zap.NewAtomicLevelAt(getZapLevel(loglevel))
	c.EncoderConfig = ec
	c.DisableStacktrace = true

	log, _ = c.Build()
	log.With(zap.String("component", name)).Info("initialized logging ...")
	undoFn = zap.ReplaceGlobals(log)
}

func Drop() {
	defer func() {
		err := log.Sync()
		if err != nil {
			panic(err)
		}
	}()
	undoFn()
}

func NewLogger() *zap.Logger {
	return log
}

const (
	Debug = "debug"
	Warn  = "warn"
	Error = "error"
	Fatal = "fatal"
)

func getZapLevel(level string) zapcore.Level {
	switch level {
	default:
		return zapcore.InfoLevel
	case Warn:
		return zapcore.WarnLevel
	case Debug:
		return zapcore.DebugLevel
	case Error:
		return zapcore.ErrorLevel
	case Fatal:
		return zapcore.FatalLevel

	}
}

func ExecutionTime(start time.Time, name string, l *zap.Logger) {
	elapse := time.Since(start)
	l.With(zap.Int64("duration", elapse.Milliseconds())).Info(fmt.Sprintf("%s took %s", name, elapse), zap.String("tag", "duration"))
}
