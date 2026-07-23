package browsercdp

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"go.uber.org/zap"
)

func browserLogReference(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:8])
}

func browserErrorLogField(err error) zap.Field {
	if err == nil {
		return zap.String("errorRef", "")
	}
	return zap.String("errorRef", browserLogReference(err.Error()))
}

func browserRecoveredLogField(recovered any) zap.Field {
	return zap.String("errorRef", browserLogReference(fmt.Sprint(recovered)))
}
