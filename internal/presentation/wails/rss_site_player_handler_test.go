package wails

import (
	"context"
	"errors"
	"testing"
)

type rssSitePlayerStub struct {
	showSession  string
	showRect     ListenEmbeddedVideoRect
	hideSession  string
	hideSeq      uint64
	closeSession string
}

func (stub *rssSitePlayerStub) Prepare(_ context.Context, _ RSSSitePlayerPrepareRequest) (RSSSitePlayerPrepareResponse, error) {
	return RSSSitePlayerPrepareResponse{}, nil
}
func (stub *rssSitePlayerStub) AcceptPrepare(_ uint64) error { return nil }
func (stub *rssSitePlayerStub) CancelPrepare(_ uint64) error { return nil }
func (stub *rssSitePlayerStub) Show(sessionID string, rect ListenEmbeddedVideoRect) (bool, error) {
	stub.showSession, stub.showRect = sessionID, rect
	return true, nil
}
func (stub *rssSitePlayerStub) Hide(sessionID string, sequence uint64) (bool, error) {
	stub.hideSession, stub.hideSeq = sessionID, sequence
	return true, nil
}
func (stub *rssSitePlayerStub) Close(sessionID string) error {
	stub.closeSession = sessionID
	return nil
}

func TestRSSSitePlayerHandlerUsesNestedSessionRectAndForcesInteraction(t *testing.T) {
	stub := &rssSitePlayerStub{}
	handler := &RSSSitePlayerHandler{player: stub}
	shown, err := handler.Show(context.Background(), RSSSitePlayerShowRequest{
		SessionID: "site-session",
		Rect:      ListenEmbeddedVideoRect{Width: 640, Height: 360, Interactive: false},
	})
	if err != nil || !shown {
		t.Fatalf("Show() = %v, %v", shown, err)
	}
	if stub.showSession != "site-session" || !stub.showRect.Interactive {
		t.Fatalf("forwarded Show = session %q rect %#v", stub.showSession, stub.showRect)
	}
	if _, err := handler.Hide(context.Background(), RSSSitePlayerHideRequest{SessionID: "site-session", Sequence: 9}); err != nil {
		t.Fatal(err)
	}
	if stub.hideSession != "site-session" || stub.hideSeq != 9 {
		t.Fatalf("forwarded Hide = %q %d", stub.hideSession, stub.hideSeq)
	}
	if err := handler.Close(context.Background(), RSSSitePlayerSessionRequest{SessionID: "site-session"}); err != nil {
		t.Fatal(err)
	}
	if stub.closeSession != "site-session" {
		t.Fatalf("forwarded Close session = %q", stub.closeSession)
	}
}

func TestRSSSitePrepareCoordinatorCancelMayOvertakePrepare(t *testing.T) {
	var coordinator rssSitePrepareCoordinator
	if sessionID, err := coordinator.cancel(4); err != nil || sessionID != "" {
		t.Fatalf("cancel before begin = %q, %v", sessionID, err)
	}
	if _, err := coordinator.begin(4); !errors.Is(err, context.Canceled) {
		t.Fatalf("begin after cancel error = %v", err)
	}
}

func TestRSSSitePrepareCoordinatorNewerCancelInvalidatesOlderPrepare(t *testing.T) {
	var coordinator rssSitePrepareCoordinator
	older, err := coordinator.begin(10)
	if err != nil {
		t.Fatal(err)
	}
	if sessionID, err := coordinator.cancel(11); err != nil || sessionID != "" {
		t.Fatalf("newer cancel before begin = %q, %v", sessionID, err)
	}
	called := false
	_, err = coordinator.commit(older, func() (RSSSitePlayerPrepareResponse, error) {
		called = true
		return RSSSitePlayerPrepareResponse{SessionID: "stale"}, nil
	})
	if !errors.Is(err, context.Canceled) || called {
		t.Fatalf("older commit after newer cancel = called %v err %v", called, err)
	}
}

func TestRSSSitePrepareCoordinatorOnlyLatestMayCommit(t *testing.T) {
	var coordinator rssSitePrepareCoordinator
	oldTicket, err := coordinator.begin(10)
	if err != nil {
		t.Fatal(err)
	}
	newTicket, err := coordinator.begin(11)
	if err != nil {
		t.Fatal(err)
	}
	oldCalled := false
	_, err = coordinator.commit(oldTicket, func() (RSSSitePlayerPrepareResponse, error) {
		oldCalled = true
		return RSSSitePlayerPrepareResponse{SessionID: "old"}, nil
	})
	if !errors.Is(err, context.Canceled) || oldCalled {
		t.Fatalf("stale commit = called %v err %v", oldCalled, err)
	}
	response, err := coordinator.commit(newTicket, func() (RSSSitePlayerPrepareResponse, error) {
		return RSSSitePlayerPrepareResponse{SessionID: "new"}, nil
	})
	if err != nil || response.SessionID != "new" {
		t.Fatalf("latest commit = %#v, %v", response, err)
	}
	sessionID, err := coordinator.cancel(11)
	if err != nil || sessionID != "new" {
		t.Fatalf("cancel committed = %q, %v", sessionID, err)
	}
}
