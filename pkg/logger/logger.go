package logger

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type Logger interface {
	Debugf(string, ...any)
	Debug(...any)
	Infof(string, ...any)
	Info(...any)
	Warnf(string, ...any)
	Warn(...any)
	Errorf(string, ...any)
	Error(...any)
	Fatalf(string, ...any)
	Fatal(...any)
}

type NopLogger struct{}

func (*NopLogger) Debugf(string, ...any) {}
func (*NopLogger) Debug(...any)          {}
func (*NopLogger) Infof(string, ...any)  {}
func (*NopLogger) Info(...any)           {}
func (*NopLogger) Warnf(string, ...any)  {}
func (*NopLogger) Warn(...any)           {}
func (*NopLogger) Errorf(string, ...any) {}
func (*NopLogger) Error(...any)          {}
func (*NopLogger) Fatalf(string, ...any) {}
func (*NopLogger) Fatal(...any)          {}

var (
	L        *zap.Logger
	zapLevel = zapcore.DebugLevel
)

func NewZapLogger(verbose bool, color bool) error {
	zapcfg := zap.NewDevelopmentEncoderConfig()
	if color {
		zapcfg.EncodeLevel = zapcore.CapitalColorLevelEncoder
	} else {
		zapcfg.EncodeLevel = zapcore.CapitalLevelEncoder
	}

	if verbose {
		zapLevel = zapcore.InfoLevel
	}
	L = zap.New(zapcore.NewCore(
		zapcore.NewConsoleEncoder(zapcfg),
		zapcore.Lock(os.Stdout),
		zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
			return lvl >= zapLevel
		}),
	), zap.AddCaller())
	return nil
}

func SetZapLoggerVerbose(verbose bool) {
	if verbose {
		zapLevel = zapcore.DebugLevel
	} else {
		zapLevel = zapcore.InfoLevel
	}
}

func CloseZapLogger() {
	L.Sync()
}
