package utils

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// NewLogger creates a new structured logger based on configuration
func NewLogger(level, format, output string) (*zap.Logger, error) {
	// Parse log level
	logLevel := parseLogLevel(level)

	// Configure encoder
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.TimeKey = "timestamp"
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	var encoder zapcore.Encoder
	if format == "console" {
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	} else {
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	}

	// Configure output
	var writeSyncer zapcore.WriteSyncer
	if output == "stdout" {
		writeSyncer = zapcore.Lock(os.Stdout)
	} else if output == "stderr" {
		writeSyncer = zapcore.Lock(os.Stderr)
	} else {
		// File output
		file, err := os.OpenFile(output, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return nil, err
		}
		writeSyncer = zapcore.Lock(file)
	}

	core := zapcore.NewCore(encoder, writeSyncer, logLevel)
	logger := zap.New(core, zap.AddCaller(), zap.AddStacktrace(zap.ErrorLevel))

	return logger, nil
}

// parseLogLevel converts string level to zap log level
func parseLogLevel(level string) zapcore.Level {
	switch level {
	case "debug":
		return zapcore.DebugLevel
	case "info":
		return zapcore.InfoLevel
	case "warn":
		return zapcore.WarnLevel
	case "error":
		return zapcore.ErrorLevel
	case "fatal":
		return zapcore.FatalLevel
	case "panic":
		return zapcore.PanicLevel
	default:
		return zapcore.InfoLevel
	}
}

// LoggerFields creates common logger fields for SMTS operations
func LoggerFields(operation, deployment, messageID, topic string) []zap.Field {
	fields := []zap.Field{
		zap.String("operation", operation),
		zap.String("deployment", deployment),
	}

	if messageID != "" {
		fields = append(fields, zap.String("message_id", messageID))
	}

	if topic != "" {
		fields = append(fields, zap.String("topic", topic))
	}

	return fields
}

// WithError adds error information to logger fields
func WithError(err error) zap.Field {
	return zap.Error(err)
}

// WithRetryCount adds retry count to logger fields
func WithRetryCount(count int) zap.Field {
	return zap.Int("retry_count", count)
}

// WithDuration adds duration to logger fields
func WithDuration(duration int64) zap.Field {
	return zap.Int64("duration_ms", duration)
}