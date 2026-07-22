package rss

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"

	"go.uber.org/zap"
)

const rssLogReferenceBytes = 8

// rssSafeLogErrorFields deliberately turns an arbitrary error into a fixed
// category and a one-way correlation reference. Network errors can contain a
// signed feed URL, proxy credentials, or a local path, so their text must never
// be handed to zap directly.
func rssSafeLogErrorFields(err error) []zap.Field {
	if err == nil {
		return nil
	}
	code := "rss_operation_failed"
	switch {
	case errors.Is(err, context.Canceled):
		code = "rss_operation_canceled"
	case errors.Is(err, context.DeadlineExceeded):
		code = "rss_operation_timeout"
	}
	return []zap.Field{
		zap.String("errorCode", code),
		zap.String("errorRef", rssOpaqueLogReference(err.Error())),
	}
}

// rssOpaqueLogReference is safe to place in a log or persisted diagnostic. It
// preserves only equality correlation and never the source URL, host, path,
// query, token, or raw error that produced it.
func rssOpaqueLogReference(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:rssLogReferenceBytes])
}
