package logger

import (
	"fmt"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type Logger struct {
	SugaredLogger *zap.SugaredLogger
}

func NewLogger(dev bool) (*Logger, error) {
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "ts",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		FunctionKey:    zapcore.OmitKey,
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.CapitalLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.StringDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	config := zap.Config{
		Level:            zap.NewAtomicLevelAt(zap.InfoLevel),
		Development:      dev,
		Encoding:         "console",
		EncoderConfig:    encoderConfig,
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
	}

	if dev {
		config.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
	}

	logger, err := config.Build()
	if err != nil {
		return nil, err
	}
	sugaredLogger := logger.Sugar()
	return &Logger{SugaredLogger: sugaredLogger}, nil
}

// formatMessage formats the message with key-value pairs using = and spaces
func formatMessage(msg string, keysAndValues ...interface{}) string {
	if len(keysAndValues) == 0 {
		return msg
	}

	result := msg
	for i := 0; i < len(keysAndValues); i += 2 {
		if i+1 < len(keysAndValues) {
			result += fmt.Sprintf(" %v=%v", keysAndValues[i], keysAndValues[i+1])
		}
	}
	return result
}

func (l *Logger) Info(msg string, keysAndValues ...interface{}) {
	l.SugaredLogger.Info(formatMessage(msg, keysAndValues...))
}

func (l *Logger) Error(msg string, keysAndValues ...interface{}) {
	l.SugaredLogger.Error(formatMessage(msg, keysAndValues...))
}

func (l *Logger) Debug(msg string, keysAndValues ...interface{}) {
	l.SugaredLogger.Debug(formatMessage(msg, keysAndValues...))
}

func (l *Logger) Warn(msg string, keysAndValues ...interface{}) {
	l.SugaredLogger.Warn(formatMessage(msg, keysAndValues...))
}

func (l *Logger) Fatal(msg string, keysAndValues ...interface{}) {
	l.SugaredLogger.Fatal(formatMessage(msg, keysAndValues...))
}

func (l *Logger) Fatalf(format string, args ...interface{}) {
	l.SugaredLogger.Fatalf(format, args...)
}
