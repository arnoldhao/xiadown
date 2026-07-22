package app

import "context"

type libraryAdmissionGate interface {
	BeginShutdown()
}

type libraryIngressServer interface {
	Shutdown(context.Context) error
}

// quiesceLibraryIngress establishes the shutdown ordering contract shared by
// LAN, Tailscale Serve and the desktop task scheduler: close admission first,
// drain/close all public listeners second, then cancel transport reconcilers.
// Active task cancellation and database shutdown happen only after this
// function returns.
func quiesceLibraryIngress(
	ctx context.Context,
	gate libraryAdmissionGate,
	server libraryIngressServer,
	cancelTransports context.CancelFunc,
) error {
	if gate != nil {
		gate.BeginShutdown()
	}
	var err error
	if server != nil {
		err = server.Shutdown(ctx)
	}
	if cancelTransports != nil {
		cancelTransports()
	}
	return err
}
