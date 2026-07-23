//go:build darwin && !ios

package wails

import (
	"context"
	"errors"
	"testing"

	"xiadown/internal/domain/appsessions"
)

func TestClearConnectorAppSessionNativeRuntimeDataRequiresApp(t *testing.T) {
	err := clearConnectorAppSessionNativeRuntimeData(
		context.Background(),
		nil,
		"youtube",
		[]string{"youtube.com"},
	)
	if !errors.Is(err, appsessions.ErrUnsupported) {
		t.Fatalf("clear without app error = %v, want unsupported", err)
	}
}
