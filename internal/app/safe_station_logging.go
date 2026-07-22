package app

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"go.uber.org/zap"
)

func safeStationLogReference(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:8])
}

func safeStationErrorLogFields(code string, err error) []zap.Field {
	fields := []zap.Field{zap.String("errorCode", strings.TrimSpace(code))}
	if err != nil {
		fields = append(fields, zap.String("errorRef", safeStationLogReference(err.Error())))
	}
	return fields
}

func safeStationTextLogFields(code string, values ...string) []zap.Field {
	fields := []zap.Field{zap.String("errorCode", strings.TrimSpace(code))}
	for _, value := range values {
		if reference := safeStationLogReference(value); reference != "" {
			fields = append(fields, zap.String("errorRef", reference))
			break
		}
	}
	return fields
}

func safeApplicationStartedLogFields(
	logDirectory string,
	logLevel string,
	language string,
	appearance string,
	networkMode string,
	networkGeneration uint64,
	networkGateway string,
) []zap.Field {
	return []zap.Field{
		zap.String("logDirectoryRef", safeStationLogReference(logDirectory)),
		zap.String("logLevel", logLevel),
		zap.String("language", language),
		zap.String("appearance", appearance),
		zap.String("networkMode", networkMode),
		zap.Uint64("networkGeneration", networkGeneration),
		zap.String("networkGatewayRef", safeStationLogReference(networkGateway)),
	}
}
