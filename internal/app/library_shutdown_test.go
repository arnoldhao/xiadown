package app

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

type recordingAdmissionGate struct{ events *[]string }

func (gate recordingAdmissionGate) BeginShutdown() {
	*gate.events = append(*gate.events, "gate")
}

type recordingIngressServer struct {
	events *[]string
	err    error
}

func (server recordingIngressServer) Shutdown(context.Context) error {
	*server.events = append(*server.events, "server")
	return server.err
}

func TestQuiesceLibraryIngressOrdersGateServerAndTransportCancellation(t *testing.T) {
	events := make([]string, 0, 3)
	wantErr := errors.New("shutdown timeout")
	err := quiesceLibraryIngress(
		context.Background(),
		recordingAdmissionGate{events: &events},
		recordingIngressServer{events: &events, err: wantErr},
		func() { events = append(events, "cancel") },
	)
	if !errors.Is(err, wantErr) {
		t.Fatalf("quiesceLibraryIngress() error = %v, want %v", err, wantErr)
	}
	if want := []string{"gate", "server", "cancel"}; !reflect.DeepEqual(events, want) {
		t.Fatalf("shutdown order = %v, want %v", events, want)
	}
}
