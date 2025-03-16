package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type Logger struct {
	SugaredLogger *zap.SugaredLogger
}

func NewLogger(dev bool) (*Logger, error) {
	config := zap.NewProductionConfig()
	if dev {
		config = zap.NewDevelopmentConfig()
	}
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	logger, err := config.Build()
	if err != nil {
		return nil, err
	}
	sugaredLogger := logger.Sugar()
	return &Logger{SugaredLogger: sugaredLogger}, nil
}

func (l *Logger) Info(args ...interface{}) {
	l.SugaredLogger.Info(args...)
}

func (l *Logger) Error(args ...interface{}) {
	l.SugaredLogger.Error(args...)
}

func (l *Logger) Debug(args ...interface{}) {
	l.SugaredLogger.Debug(args...)
}

func (l *Logger) Warn(args ...interface{}) {
	l.SugaredLogger.Warn(args...)
}

func (l *Logger) Fatal(args ...interface{}) {
	l.SugaredLogger.Fatal(args...)
}

func (l *Logger) Fatalf(format string, args ...interface{}) {
	l.SugaredLogger.Fatalf(format, args...)
}
