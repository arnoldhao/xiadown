package wails

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	settingsdto "xiadown/internal/application/settings/dto"
	"xiadown/internal/application/sniffprofile"
)

func TestSettingsHandlerCreateSniffProfileEnsuresOneDefaultPerBrowser(t *testing.T) {
	const (
		profileID   = "5b5bbf12-6542-4c1e-a576-260f129f6c55"
		displayName = "XiaDown Chrome"
	)
	var ensuredBrowsers []string
	handler := &SettingsHandler{
		sniffProfileEnsurer: func(browser string) (sniffprofile.Manifest, string, error) {
			ensuredBrowsers = append(ensuredBrowsers, browser)
			return sniffprofile.Manifest{
				ProfileID:   profileID,
				DisplayName: displayName,
				BrowserID:   "chrome",
				IsDefault:   true,
			}, "/unused", nil
		},
		sniffProfileLister: func() ([]sniffprofile.Info, error) {
			return []sniffprofile.Info{{
				ProfileID:   profileID,
				DisplayName: displayName,
				Browser:     "chrome",
				IsDefault:   true,
				Exists:      true,
			}}, nil
		},
	}
	request := settingsdto.SniffProfileRequest{Browser: "chrome", DisplayName: "First custom name"}
	first, err := handler.CreateSniffProfile(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	request.DisplayName = "Second custom name"
	second, err := handler.CreateSniffProfile(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}

	if first.ProfileID != profileID || first.ProfileID != second.ProfileID {
		t.Fatalf("repeated create returned different Profiles: first=%#v second=%#v", first, second)
	}
	if !first.IsDefault || !second.IsDefault {
		t.Fatalf("legacy create endpoint did not return the managed default: first=%#v second=%#v", first, second)
	}
	if first.DisplayName != displayName || second.DisplayName != displayName {
		t.Fatalf("legacy display names leaked into the managed default: first=%#v second=%#v", first, second)
	}
	if len(ensuredBrowsers) != 2 || ensuredBrowsers[0] != "chrome" || ensuredBrowsers[1] != "chrome" {
		t.Fatalf("EnsureDefault calls = %#v; want [chrome chrome]", ensuredBrowsers)
	}
}

func TestSettingsHandlerListSniffProfilesPropagatesStrictListingError(t *testing.T) {
	wantErr := errors.New("managed Profile root unreadable")
	handler := &SettingsHandler{
		sniffProfileLister: func() ([]sniffprofile.Info, error) {
			return nil, wantErr
		},
	}

	profiles, err := handler.ListSniffProfiles(context.Background())
	if profiles != nil || !errors.Is(err, wantErr) {
		t.Fatalf("ListSniffProfiles() = %#v, %v; want nil, %v", profiles, err, wantErr)
	}
}

func TestSettingsHandlerListSniffProfilesPreservesDefaultIdentity(t *testing.T) {
	handler := &SettingsHandler{
		sniffProfileLister: func() ([]sniffprofile.Info, error) {
			return []sniffprofile.Info{
				{
					ProfileID:   "5b5bbf12-6542-4c1e-a576-260f129f6c55",
					DisplayName: "XiaDown Chrome",
					Browser:     "chrome",
					IsDefault:   true,
					Exists:      true,
				},
				{
					ProfileID:   "c72f04dd-7d97-46bc-84e1-eb48b9e403b5",
					DisplayName: "XiaDown Chrome",
					Browser:     "chrome",
					Redundant:   true,
					Exists:      true,
				},
			}, nil
		},
	}

	profiles, err := handler.ListSniffProfiles(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(profiles) != 2 || !profiles[0].IsDefault || !profiles[1].Redundant {
		t.Fatalf("default identity was lost at the Wails boundary: %#v", profiles)
	}
}

type settingsSniffActivitySchedulerStub struct{ profileIDs []string }

func (settingsSniffActivitySchedulerStub) NotifyDownloadScheduler() {}

func (stub settingsSniffActivitySchedulerStub) ActiveResourceSniffProfileIDs() []string {
	return append([]string(nil), stub.profileIDs...)
}

func TestSettingsHandlerBlocksActiveSniffProfileMutation(t *testing.T) {
	const profileID = "5b5bbf12-6542-4c1e-a576-260f129f6c55"
	handler := &SettingsHandler{downloadScheduler: settingsSniffActivitySchedulerStub{profileIDs: []string{profileID}}}
	request := settingsdto.SniffProfileRequest{ProfileID: profileID}

	if err := handler.ClearSniffProfile(context.Background(), request); err == nil {
		t.Fatal("active sniff profile clear must be rejected")
	}
	if err := handler.DeleteSniffProfile(context.Background(), request); err == nil {
		t.Fatal("active sniff profile delete must be rejected")
	}
	if _, err := handler.RenameSniffProfile(context.Background(), request); err == nil {
		t.Fatal("active sniff profile rename must be rejected")
	}
}

type concurrentSniffActivityScheduler struct {
	mu         sync.Mutex
	profileIDs []string
}

func (*concurrentSniffActivityScheduler) NotifyDownloadScheduler() {}

func (stub *concurrentSniffActivityScheduler) ActiveResourceSniffProfileIDs() []string {
	stub.mu.Lock()
	defer stub.mu.Unlock()
	return append([]string(nil), stub.profileIDs...)
}

func (stub *concurrentSniffActivityScheduler) setActive(profileID string) {
	stub.mu.Lock()
	stub.profileIDs = []string{profileID}
	stub.mu.Unlock()
}

func TestSettingsHandlerMutationCannotPassConcurrentSniffStart(t *testing.T) {
	const profileID = "5b5bbf12-6542-4c1e-a576-260f129f6c55"
	activity := new(concurrentSniffActivityScheduler)
	handler := &SettingsHandler{downloadScheduler: activity}
	request := settingsdto.SniffProfileRequest{ProfileID: profileID, DisplayName: "Renamed"}

	releaseStart := sniffprofile.LockForRuntimeStart()
	mutationResult := make(chan error, 1)
	go func() {
		_, err := handler.RenameSniffProfile(context.Background(), request)
		mutationResult <- err
	}()

	select {
	case err := <-mutationResult:
		t.Fatalf("Profile mutation escaped the concurrent start boundary: %v", err)
	case <-time.After(50 * time.Millisecond):
	}

	// StartResourceSniff publishes the active session before releasing the read
	// boundary. The waiting mutation must observe that published activity.
	activity.setActive(profileID)
	releaseStart()
	select {
	case err := <-mutationResult:
		if err == nil {
			t.Fatal("Profile mutation did not reject the newly active Sniff Profile")
		}
	case <-time.After(time.Second):
		t.Fatal("Profile mutation remained blocked after runtime start completed")
	}
}
