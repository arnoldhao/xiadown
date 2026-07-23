package service

import (
	"context"
	"errors"
	"log"
	"runtime"
	"slices"
	"sort"
	"strings"
	"time"

	"xiadown/internal/application/appsessions/dto"
	appcookies "xiadown/internal/application/cookies"
	"xiadown/internal/domain/appsessions"
)

const (
	browserProfileMode              = "browser_profile"
	currentBrowserMode              = "current_browser"
	currentBrowserProfileID         = "current-browser"
	browserScanStatusNew            = "new"
	browserScanStatusReplace        = "replace"
	browserScanStatusSame           = "unchanged"
	browserScanStatusMissing        = "unavailable"
	browserScanReasonNoAuth         = "no_auth_cookies"
	browserScanReasonNoSource       = "source_unavailable"
	browserScanReasonAccessRequired = "browser_cookie_access_required"
	browserScanReasonProtected      = "protected_cookies_unsupported"
)

func (service *AppSessionsService) ScanBrowserProfile(
	ctx context.Context,
	request dto.BrowserProfileSelection,
) (dto.AppSessionBrowserScanResult, error) {
	selectionMode := normalizedBrowserProfileMode(request.Mode)
	browserID, profileID, err := normalizeBrowserProfileSelection(request.Mode, request.BrowserID, request.ProfileID)
	if err != nil {
		return dto.AppSessionBrowserScanResult{}, err
	}
	result := dto.AppSessionBrowserScanResult{
		BrowserID: browserID,
		ProfileID: externalBrowserProfileID(selectionMode, profileID),
		Items:     []dto.AppSessionBrowserScanItem{},
	}
	if service == nil || service.repo == nil || service.browserProfileReader == nil {
		return result, appsessions.ErrUnsupported
	}
	if err := service.EnsureDefaults(ctx); err != nil {
		return result, err
	}
	sessions, err := service.repo.List(ctx)
	if err != nil {
		return result, err
	}
	credentialEpochs := make(map[string]uint64, len(sessions))
	for _, session := range sessions {
		if isSupportedSiteKey(session.SiteKey) {
			credentialEpochs[session.ID] = service.credentialOperationEpoch(session.ID)
		}
	}
	allowedDomains := allAppSessionCookieDomains()
	var records []appcookies.Record
	if selectionMode == currentBrowserMode {
		currentReader, ok := service.browserProfileReader.(AppSessionCurrentBrowserReader)
		if !ok {
			return result, appsessions.ErrUnsupported
		}
		records, err = currentReader.ReadCurrentBrowserCookies(ctx, browserID, allowedDomains)
	} else {
		records, err = service.browserProfileReader.ReadBrowserProfileCookies(
			ctx, browserID, profileID, allowedDomains,
		)
	}
	sourceReason := ""
	switch {
	case err == nil:
	case errors.Is(err, appsessions.ErrNoCookies):
		records = nil
	case errors.Is(err, appsessions.ErrBrowserCookieAccessRequired):
		records = nil
		sourceReason = browserScanReasonAccessRequired
	case errors.Is(err, appsessions.ErrBrowserCookieProtected):
		records = nil
		sourceReason = browserScanReasonProtected
	default:
		return result, err
	}
	// The reader is expected to filter, but enforce the boundary here as well:
	// the in-memory snapshot must never retain unrelated browser cookies.
	records = canonicalAppSessionCookies(appcookies.FilterByDomains(records, allowedDomains))
	bySite := make(map[string]appsessions.Session, len(sessions))
	allowedIDs := make([]string, 0, len(sessions))
	for _, session := range sessions {
		bySite[session.SiteKey] = session
	}
	for _, siteKey := range supportedSiteKeys() {
		session, ok := bySite[siteKey]
		if !ok {
			continue
		}
		candidate := canonicalAppSessionCookies(filterAppSessionCookies(siteKey, records))
		item := dto.AppSessionBrowserScanItem{
			AppSessionID: session.ID,
			SiteKey:      siteKey,
			Label:        appSessionSiteLabel(siteKey),
			AccountLabel: appSessionAccountLabel(session),
			Status:       browserScanStatusMissing,
			Reason:       browserScanReasonNoAuth,
		}
		if sourceReason != "" {
			item.Reason = sourceReason
		}
		if appSessionHasAuthenticationCookies(siteKey, candidate) {
			item.Status = browserScanStatusNew
			item.Selectable = true
			item.Reason = ""
			if session.Status == appsessions.StatusConnected {
				item.Status = browserScanStatusReplace
			}
			if service.provider != nil {
				current, loadErr := service.provider.LoadAppSessionCookies(ctx, siteKey)
				if loadErr == nil && equalAppSessionCookies(candidate, filterAppSessionCookies(siteKey, current)) {
					item.Status = browserScanStatusSame
					item.Selectable = false
				}
			}
		}
		result.Items = append(result.Items, item)
		if item.Selectable {
			allowedIDs = append(allowedIDs, item.AppSessionID)
		}
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	result.SnapshotToken, err = service.storeBrowserScanSnapshot(
		browserID,
		profileID,
		records,
		credentialEpochs,
		allowedIDs,
	)
	if err != nil {
		return result, err
	}
	return result, nil
}

func (service *AppSessionsService) ImportBrowserProfile(
	ctx context.Context,
	request dto.AppSessionBrowserImportRequest,
) (dto.AppSessionBrowserImportResult, error) {
	selectionMode := normalizedBrowserProfileMode(request.Mode)
	browserID, profileID, err := normalizeBrowserProfileSelection(request.Mode, request.BrowserID, request.ProfileID)
	if err != nil {
		return dto.AppSessionBrowserImportResult{}, err
	}
	result := dto.AppSessionBrowserImportResult{ImportedIDs: []string{}, SkippedIDs: []string{}}
	if service == nil || service.repo == nil || service.provider == nil ||
		service.importCommitter == nil {
		return result, appsessions.ErrUnsupported
	}
	cache, ok := service.provider.(AppSessionImportedCookieCache)
	if !ok {
		return result, appsessions.ErrUnsupported
	}
	requested := uniqueAppSessionIDs(request.AppSessionIDs)
	if len(requested) == 0 || len(requested) > len(supportedSiteKeys()) {
		return result, appsessions.ErrInvalidSession
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	snapshot, err := service.consumeBrowserScanSnapshot(
		request.SnapshotToken,
		browserID,
		profileID,
	)
	if err != nil {
		return result, err
	}
	for _, appSessionID := range requested {
		if _, allowed := snapshot.allowedIDs[appSessionID]; !allowed {
			return result, appsessions.ErrInvalidSession
		}
	}
	if err := service.EnsureDefaults(ctx); err != nil {
		return result, err
	}
	sessions, err := service.repo.List(ctx)
	if err != nil {
		return result, err
	}
	byID := make(map[string]appsessions.Session, len(sessions))
	for _, session := range sessions {
		if isSupportedSiteKey(session.SiteKey) {
			byID[session.ID] = session
		}
	}
	records := snapshot.records

	var lastCommitErr error
	for _, appSessionID := range requested {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		current, exists := byID[appSessionID]
		if !exists {
			result.SkippedIDs = append(result.SkippedIDs, appSessionID)
			continue
		}
		selectedSiteKey := current.SiteKey
		candidate := canonicalAppSessionCookies(filterAppSessionCookies(selectedSiteKey, records))
		if !appSessionHasAuthenticationCookies(selectedSiteKey, candidate) {
			result.SkippedIDs = append(result.SkippedIDs, appSessionID)
			continue
		}
		expectedCredentialEpoch, existedAtScan := snapshot.credentialEpochs[appSessionID]
		if !existedAtScan {
			result.SkippedIDs = append(result.SkippedIDs, appSessionID)
			continue
		}

		// Browser I/O completed during Scan, outside the mutation gate. Once a
		// frozen candidate is ready to replace, re-read its metadata and serialize
		// the secret+metadata commit with Clear, manual finalize and delayed
		// account verification callbacks.
		service.credentialMutationMu.Lock()
		if !service.credentialOperationCurrent(appSessionID, expectedCredentialEpoch) {
			service.credentialMutationMu.Unlock()
			result.SkippedIDs = append(result.SkippedIDs, appSessionID)
			continue
		}
		current, getErr := service.repo.Get(ctx, appSessionID)
		if getErr != nil || current.SiteKey != selectedSiteKey || !isSupportedSiteKey(current.SiteKey) {
			service.credentialMutationMu.Unlock()
			result.SkippedIDs = append(result.SkippedIDs, appSessionID)
			if getErr != nil {
				lastCommitErr = getErr
			}
			continue
		}
		if existing, loadErr := service.provider.LoadAppSessionCookies(ctx, current.SiteKey); loadErr == nil &&
			equalAppSessionCookies(candidate, filterAppSessionCookies(current.SiteKey, existing)) {
			service.credentialMutationMu.Unlock()
			result.SkippedIDs = append(result.SkippedIDs, appSessionID)
			continue
		}

		now := service.now().UTC().Round(0)
		verificationStatus := appsessions.AccountVerificationUnsupported
		verificationError := ""
		var verificationStartedAt *time.Time
		if appSessionRequiresAccountVerification(current.SiteKey) {
			if service.accountFetcher != nil {
				verificationStatus = appsessions.AccountVerificationVerifying
				startedAt := now
				verificationStartedAt = &startedAt
			} else {
				verificationStatus = appsessions.AccountVerificationUnverified
				verificationError = appSessionVerificationErrorMessage(appsessions.ErrUnsupported)
			}
		}
		updated, buildErr := appsessions.NewSession(appsessions.SessionParams{
			ID:                           current.ID,
			SiteKey:                      current.SiteKey,
			Status:                       string(appsessions.StatusConnected),
			AccountVerificationStatus:    string(verificationStatus),
			AccountVerificationError:     verificationError,
			AccountVerificationStartedAt: verificationStartedAt,
			SourceType:                   string(appsessions.SourceTypeBrowserProfile),
			SourceBrowser:                browserID,
			SourceProfile:                importedBrowserSourceProfile(selectionMode, profileID),
			LastSyncedAt:                 &now,
			CreatedAt:                    &current.CreatedAt,
			UpdatedAt:                    &now,
		})
		if buildErr != nil {
			service.credentialMutationMu.Unlock()
			result.SkippedIDs = append(result.SkippedIDs, appSessionID)
			lastCommitErr = buildErr
			continue
		}
		encoded, encodeErr := appcookies.EncodeJSON(candidate)
		if encodeErr != nil || encoded == "" {
			service.credentialMutationMu.Unlock()
			result.SkippedIDs = append(result.SkippedIDs, appSessionID)
			lastCommitErr = encodeErr
			continue
		}
		if commitErr := service.importCommitter.CommitImportedAppSession(ctx, updated, []byte(encoded)); commitErr != nil {
			service.credentialMutationMu.Unlock()
			log.Printf("app sessions: browser profile import failed site=%s error=%v", current.SiteKey, commitErr)
			result.SkippedIDs = append(result.SkippedIDs, appSessionID)
			lastCommitErr = commitErr
			continue
		}
		cache.CacheImportedAppSessionCookies(current.SiteKey, candidate)
		verificationEpoch := service.nextAccountVerificationEpoch(updated.ID)
		service.credentialMutationMu.Unlock()
		result.ImportedIDs = append(result.ImportedIDs, appSessionID)
		service.notifyAppSessionChanged(ctx, AppSessionChangeEvent{
			Action:     "import",
			AppSession: service.mapSessionDTOWithCookies(updated, candidate),
			Saved:      true,
			Reason:     selectionMode,
		})
		if verificationStatus == appsessions.AccountVerificationVerifying && verificationStartedAt != nil {
			service.startAppSessionAccountVerification(updated, candidate, *verificationStartedAt, verificationEpoch)
		}
	}
	if len(result.ImportedIDs) == 0 && lastCommitErr != nil {
		return result, lastCommitErr
	}
	return result, nil
}

func normalizeBrowserProfileSelection(mode string, browserID string, profileID string) (string, string, error) {
	mode = normalizedBrowserProfileMode(mode)
	browserID = strings.ToLower(strings.TrimSpace(browserID))
	profileID = strings.TrimSpace(profileID)
	if mode == currentBrowserMode {
		if browserID == "chrome" && profileID == "" {
			return browserID, currentBrowserProfileID, nil
		}
		return "", "", appsessions.ErrUnsupported
	}
	if mode != browserProfileMode {
		return "", "", appsessions.ErrUnsupported
	}
	if !strings.HasPrefix(profileID, "profile-") || len(profileID) > 256 || strings.ContainsAny(profileID, "/\\\x00") {
		return "", "", appsessions.ErrInvalidSession
	}
	if SupportsBrowserProfileImport(browserID) {
		return browserID, profileID, nil
	}
	return "", "", appsessions.ErrUnsupported
}

func normalizedBrowserProfileMode(mode string) string {
	mode = strings.TrimSpace(mode)
	if mode == "" {
		return browserProfileMode
	}
	return mode
}

func importedBrowserSourceProfile(mode string, profileID string) string {
	return externalBrowserProfileID(mode, profileID)
}

func externalBrowserProfileID(mode string, profileID string) string {
	if mode == currentBrowserMode {
		return ""
	}
	return profileID
}

// SupportsBrowserProfileImport is the App Session import boundary. Sniff may
// support a wider set of CDP browsers, but those candidates must not leak into
// this flow until their on-disk profile layout has been adapted and tested.
func SupportsBrowserProfileImport(browserID string) bool {
	switch strings.ToLower(strings.TrimSpace(browserID)) {
	case "chrome", "edge", "brave", "arc", "vivaldi", "opera":
		return true
	case "safari":
		return runtime.GOOS == "darwin"
	default:
		return false
	}
}

func allAppSessionCookieDomains() []string {
	set := make(map[string]struct{})
	for _, siteKey := range supportedSiteKeys() {
		for _, domain := range appSessionCookieDomains(siteKey) {
			domain = strings.Trim(strings.ToLower(strings.TrimSpace(domain)), ".")
			if domain != "" {
				set[domain] = struct{}{}
			}
		}
	}
	result := make([]string, 0, len(set))
	for domain := range set {
		result = append(result, domain)
	}
	sort.Strings(result)
	return result
}

// AppSessionBrowserCookieDomains returns a fresh, normalized copy of the
// domains that browser-profile discovery may inspect. The presentation layer
// uses this exact allowlist for lazy protection discovery, keeping it aligned
// with the domains that ScanBrowserProfile is permitted to retain.
func AppSessionBrowserCookieDomains() []string {
	return allAppSessionCookieDomains()
}

func appSessionAccountLabel(session appsessions.Session) string {
	if label := strings.TrimSpace(session.AccountDisplayName); label != "" {
		return label
	}
	return strings.TrimSpace(session.AccountHandle)
}

func uniqueAppSessionIDs(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func canonicalAppSessionCookies(records []appcookies.Record) []appcookies.Record {
	byIdentity := make(map[string]appcookies.Record, len(records))
	for _, record := range records {
		identity := strings.ToLower(strings.TrimSpace(record.Domain)) + "\x00" +
			strings.TrimSpace(record.Path) + "\x00" + strings.TrimSpace(record.Name)
		if strings.Trim(identity, "\x00") == "" {
			continue
		}
		byIdentity[identity] = record
	}
	result := make([]appcookies.Record, 0, len(byIdentity))
	for _, record := range byIdentity {
		result = append(result, record)
	}
	sort.Slice(result, func(i, j int) bool {
		left := strings.ToLower(result[i].Domain) + "\x00" + result[i].Path + "\x00" + result[i].Name
		right := strings.ToLower(result[j].Domain) + "\x00" + result[j].Path + "\x00" + result[j].Name
		return left < right
	})
	return result
}

func equalAppSessionCookies(left []appcookies.Record, right []appcookies.Record) bool {
	return slices.Equal(canonicalAppSessionCookies(left), canonicalAppSessionCookies(right))
}
