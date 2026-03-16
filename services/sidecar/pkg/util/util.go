package util

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"os"
	"strings"
)

func GetLogger() *zap.Logger {
	atom := zap.NewAtomicLevel()
	encoderCfg := zap.NewProductionEncoderConfig()
	encoderCfg.TimeKey = "timestamp"
	encoderCfg.EncodeTime = zapcore.ISO8601TimeEncoder

	logger := zap.New(zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderCfg),
		zapcore.Lock(os.Stdout),
		atom,
	))
	defer logger.Sync()
	loglvl := GetEnv("LOGLEVEL", "info")
	if strings.ToLower(loglvl) == "debug" {
		atom.SetLevel(zap.DebugLevel)
	}
	return logger
}

func GetEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}

	return fallback
}

func IsTlsEnabled() bool {
	tlsEnabled := os.Getenv("TLS_ENABLED")
	if tlsEnabled != "" && strings.ToLower(tlsEnabled) == "true" {
		return true
	}
	return false
}
