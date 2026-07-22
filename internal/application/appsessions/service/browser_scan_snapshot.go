package service

import (
	"crypto/rand"
	"encoding/base64"
	"strings"
	"time"

	appcookies "xiadown/internal/application/cookies"
	"xiadown/internal/domain/appsessions"
)

const (
	browserScanSnapshotTTL            = 2 * time.Minute
	browserScanSnapshotCapacity       = 8
	browserScanSnapshotMaxRecords     = 20_000
	browserScanSnapshotMaxBytes       = int64(8 << 20)
	browserScanSnapshotTotalMaxBytes  = int64(16 << 20)
	browserScanSnapshotTokenBytes     = 32
	browserScanSnapshotRecordOverhead = int64(64)
)

type browserScanSnapshot struct {
	browserID        string
	profileID        string
	records          []appcookies.Record
	credentialEpochs map[string]uint64
	allowedIDs       map[string]struct{}
	createdAt        time.Time
	expiresAt        time.Time
	sizeBytes        int64
}

func (service *AppSessionsService) storeBrowserScanSnapshot(
	browserID string,
	profileID string,
	records []appcookies.Record,
	credentialEpochs map[string]uint64,
	allowedIDs []string,
) (string, error) {
	if service == nil {
		return "", appsessions.ErrInvalidSession
	}
	sizeBytes, ok := browserScanSnapshotSize(records, credentialEpochs, allowedIDs)
	if !ok {
		return "", appsessions.ErrInvalidSession
	}
	snapshot := browserScanSnapshot{
		browserID:        browserID,
		profileID:        profileID,
		records:          append([]appcookies.Record(nil), records...),
		credentialEpochs: cloneCredentialEpochs(credentialEpochs),
		allowedIDs:       cloneAllowedAppSessionIDs(allowedIDs),
		sizeBytes:        sizeBytes,
	}

	for attempt := 0; attempt < 4; attempt++ {
		token, err := newBrowserScanSnapshotToken()
		if err != nil {
			return "", err
		}
		now := service.browserScanSnapshotNow()
		snapshot.createdAt = now
		snapshot.expiresAt = now.Add(browserScanSnapshotTTL)

		service.browserScanSnapshotMu.Lock()
		if service.browserScanSnapshots == nil {
			service.browserScanSnapshots = make(map[string]browserScanSnapshot)
		}
		service.purgeExpiredBrowserScanSnapshotsLocked(now)
		if _, exists := service.browserScanSnapshots[token]; exists {
			service.browserScanSnapshotMu.Unlock()
			continue
		}
		for len(service.browserScanSnapshots) >= browserScanSnapshotCapacity ||
			service.browserScanSnapshotBytes+sizeBytes > browserScanSnapshotTotalMaxBytes {
			if !service.evictOldestBrowserScanSnapshotLocked() {
				break
			}
		}
		if len(service.browserScanSnapshots) >= browserScanSnapshotCapacity ||
			service.browserScanSnapshotBytes+sizeBytes > browserScanSnapshotTotalMaxBytes {
			service.browserScanSnapshotMu.Unlock()
			return "", appsessions.ErrInvalidSession
		}
		service.browserScanSnapshots[token] = snapshot
		service.browserScanSnapshotBytes += sizeBytes
		service.browserScanSnapshotMu.Unlock()
		return token, nil
	}
	return "", appsessions.ErrInvalidSession
}

// consumeBrowserScanSnapshot removes a token while holding the store lock.
// Even a browser/profile mismatch burns the token, so every accepted token can
// authorize at most one import attempt.
func (service *AppSessionsService) consumeBrowserScanSnapshot(
	token string,
	browserID string,
	profileID string,
) (browserScanSnapshot, error) {
	if service == nil || !validBrowserScanSnapshotToken(token) {
		return browserScanSnapshot{}, appsessions.ErrInvalidSession
	}
	now := service.browserScanSnapshotNow()
	service.browserScanSnapshotMu.Lock()
	service.purgeExpiredBrowserScanSnapshotsLocked(now)
	snapshot, ok := service.browserScanSnapshots[token]
	if ok {
		service.deleteBrowserScanSnapshotLocked(token, snapshot)
	}
	service.browserScanSnapshotMu.Unlock()
	if !ok || !now.Before(snapshot.expiresAt) ||
		snapshot.browserID != browserID || snapshot.profileID != profileID {
		return browserScanSnapshot{}, appsessions.ErrInvalidSession
	}
	return snapshot, nil
}

func (service *AppSessionsService) clearBrowserScanSnapshots() {
	if service == nil {
		return
	}
	service.browserScanSnapshotMu.Lock()
	service.browserScanSnapshots = make(map[string]browserScanSnapshot)
	service.browserScanSnapshotBytes = 0
	service.browserScanSnapshotMu.Unlock()
}

func (service *AppSessionsService) purgeExpiredBrowserScanSnapshotsLocked(now time.Time) {
	for token, snapshot := range service.browserScanSnapshots {
		if !now.Before(snapshot.expiresAt) {
			service.deleteBrowserScanSnapshotLocked(token, snapshot)
		}
	}
}

func (service *AppSessionsService) evictOldestBrowserScanSnapshotLocked() bool {
	oldestToken := ""
	var oldest browserScanSnapshot
	for token, snapshot := range service.browserScanSnapshots {
		if oldestToken == "" || snapshot.createdAt.Before(oldest.createdAt) {
			oldestToken = token
			oldest = snapshot
		}
	}
	if oldestToken == "" {
		return false
	}
	service.deleteBrowserScanSnapshotLocked(oldestToken, oldest)
	return true
}

func (service *AppSessionsService) deleteBrowserScanSnapshotLocked(
	token string,
	snapshot browserScanSnapshot,
) {
	delete(service.browserScanSnapshots, token)
	service.browserScanSnapshotBytes -= snapshot.sizeBytes
	if service.browserScanSnapshotBytes < 0 {
		service.browserScanSnapshotBytes = 0
	}
}

func (service *AppSessionsService) browserScanSnapshotNow() time.Time {
	if service != nil && service.now != nil {
		return service.now()
	}
	return time.Now()
}

func newBrowserScanSnapshotToken() (string, error) {
	payload := make([]byte, browserScanSnapshotTokenBytes)
	if _, err := rand.Read(payload); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(payload), nil
}

func validBrowserScanSnapshotToken(token string) bool {
	if token == "" || strings.TrimSpace(token) != token {
		return false
	}
	payload, err := base64.RawURLEncoding.DecodeString(token)
	return err == nil && len(payload) == browserScanSnapshotTokenBytes
}

func browserScanSnapshotSize(
	records []appcookies.Record,
	credentialEpochs map[string]uint64,
	allowedIDs []string,
) (int64, bool) {
	if len(records) > browserScanSnapshotMaxRecords {
		return 0, false
	}
	if len(allowedIDs) > len(supportedSiteKeys()) {
		return 0, false
	}
	var sizeBytes int64
	for appSessionID := range credentialEpochs {
		sizeBytes += browserScanSnapshotRecordOverhead + int64(len(appSessionID))
	}
	for _, appSessionID := range allowedIDs {
		sizeBytes += browserScanSnapshotRecordOverhead + int64(len(appSessionID))
	}
	if sizeBytes > browserScanSnapshotMaxBytes {
		return 0, false
	}
	for _, record := range records {
		sizeBytes += browserScanSnapshotRecordOverhead + int64(
			len(record.Name)+len(record.Value)+len(record.Domain)+len(record.Path)+len(record.SameSite),
		)
		if sizeBytes > browserScanSnapshotMaxBytes {
			return 0, false
		}
	}
	return sizeBytes, sizeBytes <= browserScanSnapshotMaxBytes
}

func cloneCredentialEpochs(values map[string]uint64) map[string]uint64 {
	result := make(map[string]uint64, len(values))
	for key, value := range values {
		result[key] = value
	}
	return result
}

func cloneAllowedAppSessionIDs(values []string) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		result[value] = struct{}{}
	}
	return result
}
