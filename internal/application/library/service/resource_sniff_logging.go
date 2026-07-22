package service

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const resourceSniffLogReferenceBytes = 8

type zapFieldsObject []zap.Field

func (fields zapFieldsObject) MarshalLogObject(encoder zapcore.ObjectEncoder) error {
	for _, field := range fields {
		field.AddTo(encoder)
	}
	return nil
}

// resourceSniffLogReference preserves only one-way correlation. Resource
// browser values can contain credentials, signed URLs, page titles, executable
// paths, and user file paths, none of which may be written to Desktop logs.
func resourceSniffLogReference(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:resourceSniffLogReferenceBytes])
}

func resourceSniffErrorLogFields(code string, err error) []zap.Field {
	fields := []zap.Field{zap.String("errorCode", strings.TrimSpace(code))}
	if err != nil {
		fields = append(fields, zap.String("errorRef", resourceSniffLogReference(err.Error())))
	}
	return fields
}

func resourceSniffPanicLogFields(code string, recovered any) []zap.Field {
	fields := []zap.Field{zap.String("errorCode", strings.TrimSpace(code))}
	if recovered != nil {
		fields = append(fields, zap.String("errorRef", resourceSniffLogReference(fmt.Sprint(recovered))))
	}
	return fields
}
