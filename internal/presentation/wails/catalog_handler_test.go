package wails

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"xiadown/internal/application/library/dto"
	"xiadown/internal/application/libraryrootsync"
)

type catalogStorageRootSyncerStub struct {
	events  []string
	stopErr error
}

func (stub *catalogStorageRootSyncerStub) ListStates(
	context.Context,
) ([]libraryrootsync.StateDTO, error) {
	return nil, nil
}

func (stub *catalogStorageRootSyncerStub) StartRootScan(
	_ context.Context,
	request libraryrootsync.RootRequest,
) (libraryrootsync.StateDTO, error) {
	stub.events = append(stub.events, "start:"+request.RootID)
	return libraryrootsync.StateDTO{}, nil
}

func (stub *catalogStorageRootSyncerStub) CancelRootScan(
	context.Context,
	libraryrootsync.RootRequest,
) (libraryrootsync.StateDTO, error) {
	return libraryrootsync.StateDTO{}, nil
}

func (stub *catalogStorageRootSyncerStub) StopRoot(
	_ context.Context,
	rootID string,
) error {
	stub.events = append(stub.events, "stop:"+rootID)
	return stub.stopErr
}

func TestSelectCatalogStorageRootRequestCannotCarryAPath(t *testing.T) {
	typeOfRequest := reflect.TypeOf(SelectCatalogStorageRootRequest{})
	if typeOfRequest.NumField() != 0 {
		t.Fatalf(
			"storage root picker request fields = %d, want no path or policy fields",
			typeOfRequest.NumField(),
		)
	}
}

func TestCatalogStorageRootRelocationPausesAndRestartsSync(t *testing.T) {
	syncer := &catalogStorageRootSyncerStub{}
	root, err := runCatalogStorageRootRelocation(
		context.Background(),
		syncer,
		"old-root",
		func() (dto.CatalogStorageRootDTO, error) {
			syncer.events = append(syncer.events, "relocate")
			return dto.CatalogStorageRootDTO{ID: "new-root"}, nil
		},
	)
	if err != nil || root.ID != "new-root" {
		t.Fatalf("relocate root=%#v err=%v", root, err)
	}
	want := []string{"stop:old-root", "relocate", "start:new-root"}
	if !reflect.DeepEqual(syncer.events, want) {
		t.Fatalf("sync events = %#v, want %#v", syncer.events, want)
	}
}

func TestCatalogStorageRootRelocationRestoresOldSyncOnFailure(t *testing.T) {
	t.Run("relocation fails", func(t *testing.T) {
		syncer := &catalogStorageRootSyncerStub{}
		relocateErr := errors.New("relocation failed")
		_, err := runCatalogStorageRootRelocation(
			context.Background(),
			syncer,
			"root",
			func() (dto.CatalogStorageRootDTO, error) {
				syncer.events = append(syncer.events, "relocate")
				return dto.CatalogStorageRootDTO{}, relocateErr
			},
		)
		if !errors.Is(err, relocateErr) {
			t.Fatalf("error = %v, want relocation failure", err)
		}
		want := []string{"stop:root", "relocate", "start:root"}
		if !reflect.DeepEqual(syncer.events, want) {
			t.Fatalf("sync events = %#v, want %#v", syncer.events, want)
		}
	})

	t.Run("stop is interrupted", func(t *testing.T) {
		stopErr := errors.New("stop interrupted")
		syncer := &catalogStorageRootSyncerStub{stopErr: stopErr}
		called := false
		_, err := runCatalogStorageRootRelocation(
			context.Background(),
			syncer,
			"root",
			func() (dto.CatalogStorageRootDTO, error) {
				called = true
				return dto.CatalogStorageRootDTO{}, nil
			},
		)
		if !errors.Is(err, stopErr) || called {
			t.Fatalf("error=%v relocate-called=%t", err, called)
		}
		want := []string{"stop:root", "start:root"}
		if !reflect.DeepEqual(syncer.events, want) {
			t.Fatalf("sync events = %#v, want %#v", syncer.events, want)
		}
	})
}
