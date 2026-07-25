package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"xiadown/internal/application/appsessions/dto"
	appcookies "xiadown/internal/application/cookies"
	"xiadown/internal/domain/appsessions"
)

// VerifyAppSession validates the complete read-only runtime snapshot currently
// owned by XiaDown. Runtime hydration may read the shared native cookie store,
// but it never imports or persists browser mutations into the App Session
// credential backup.
func (service *AppSessionsService) VerifyAppSession(ctx context.Context, request dto.VerifyAppSessionRequest) (dto.AppSession, error) {
	if service == nil || service.repo == nil || service.provider == nil {
		return dto.AppSession{}, appsessions.ErrUnsupported
	}
	session, err := service.sessionByID(ctx, request.ID)
	if err != nil {
		return dto.AppSession{}, err
	}
	if service.accountFetcher == nil || !appSessionRequiresAccountVerification(session.SiteKey) {
		return dto.AppSession{}, appsessions.ErrUnsupported
	}

	service.credentialMutationMu.Lock()
	current, err := service.repo.Get(ctx, session.ID)
	if err != nil {
		service.credentialMutationMu.Unlock()
		return dto.AppSession{}, err
	}
	records, err := service.RecordsForSiteKey(ctx, current.SiteKey)
	if err != nil {
		service.credentialMutationMu.Unlock()
		return dto.AppSession{}, err
	}
	records = canonicalAppSessionCookies(filterAppSessionCookies(current.SiteKey, records))
	observedAt := service.now()
	if !appSessionHasAuthenticationCookies(current.SiteKey, records) {
		service.credentialMutationMu.Unlock()
		return dto.AppSession{}, appsessions.ErrNoCookies
	}

	startedAt := observedAt.UTC().Round(0)
	params := appSessionParamsPreservingState(current, startedAt)
	params.AccountVerificationStatus = string(appsessions.AccountVerificationVerifying)
	params.AccountVerificationError = ""
	params.AccountVerificationStartedAt = &startedAt
	updated, err := appsessions.NewSession(params)
	if err == nil {
		err = service.repo.Save(ctx, updated)
	}
	verificationEpoch := uint64(0)
	if err == nil {
		verificationEpoch = service.nextAccountVerificationEpoch(updated.ID)
	}
	service.credentialMutationMu.Unlock()
	if err != nil {
		return dto.AppSession{}, err
	}

	mapped := service.mapSessionDTOWithCookies(updated, records)
	service.notifyAppSessionChanged(ctx, AppSessionChangeEvent{
		Action:     "verify-started",
		AppSession: mapped,
		Saved:      false,
		Reason:     "read_only",
	})
	service.startAppSessionAccountVerification(updated, records, startedAt, verificationEpoch)
	return mapped, nil
}

func (service *AppSessionsService) startAppSessionAccountVerification(session appsessions.Session, records []appcookies.Record, startedAt time.Time, verificationEpoch uint64) {
	if service == nil || service.accountFetcher == nil || verificationEpoch == 0 || !appSessionRequiresAccountVerification(session.SiteKey) {
		return
	}
	timeout := service.accountVerificationTimeoutDuration()
	records = append([]appcookies.Record(nil), records...)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		resultCh := make(chan appSessionAccountVerificationResult, 1)
		go func() {
			account, err := service.accountFetcher(ctx, session.SiteKey, records)
			if err == nil && !appSessionAccountVerified(account) {
				err = appsessions.ErrNoCookies
			}
			resultCh <- appSessionAccountVerificationResult{account: account, err: err}
		}()

		timer := time.NewTimer(timeout)
		defer timer.Stop()
		select {
		case result := <-resultCh:
			service.finishAppSessionAccountVerification(session, records, startedAt, verificationEpoch, result.account, result.err)
		case <-timer.C:
			cancel()
			service.finishAppSessionAccountVerification(session, records, startedAt, verificationEpoch, dto.AppSessionAccount{}, context.DeadlineExceeded)
		}
	}()
}

type appSessionAccountVerificationResult struct {
	account dto.AppSessionAccount
	err     error
}

func (service *AppSessionsService) finishAppSessionAccountVerification(session appsessions.Session, records []appcookies.Record, startedAt time.Time, verificationEpoch uint64, account dto.AppSessionAccount, cause error) {
	if service == nil || service.repo == nil {
		return
	}
	// Verification is a delayed continuation of the credential commit that
	// started it. Serialize its read-check-write sequence with Clear, manual
	// finalize and browser-profile import so a callback holding an older
	// "verifying" snapshot cannot restore connected metadata after the user
	// has cleared the Session.
	service.credentialMutationMu.Lock()
	defer service.credentialMutationMu.Unlock()
	if !service.accountVerificationEpochCurrent(session.ID, verificationEpoch) {
		return
	}
	ctx := context.Background()
	current, err := service.repo.Get(ctx, session.ID)
	if err != nil {
		return
	}
	if current.SiteKey != session.SiteKey ||
		current.AccountVerificationStatus != appsessions.AccountVerificationVerifying {
		return
	}
	if newerVerificationStartedAt(current.AccountVerificationStartedAt, startedAt) {
		return
	}

	now := service.now().UTC().Round(0)
	params := appSessionParamsPreservingState(current, now)
	params.AccountVerificationStartedAt = nil
	reason := "verified"
	if cause != nil {
		params.AccountVerificationStatus = string(appsessions.AccountVerificationUnverified)
		params.AccountVerificationError = appSessionVerificationErrorMessage(cause)
		reason = "unverified"
	} else {
		params.AccountDisplayName = account.DisplayName
		params.AccountHandle = account.Handle
		params.AccountAvatarURL = account.AvatarURL
		params.AccountTierKey = account.TierKey
		params.AccountTierLabel = account.TierLabel
		params.AccountBadgesJSON = encodeBadges(account.Badges)
		params.AccountMetadataJSON = encodeMetadata(accountMetadataWithExpiresAt(account.Metadata, session.SiteKey, records, now))
		params.AccountVerificationStatus = string(appsessions.AccountVerificationVerified)
		params.LastVerifiedAt = &now
	}

	updated, err := appsessions.NewSession(params)
	if err != nil {
		return
	}
	if err := service.repo.Save(ctx, updated); err != nil {
		return
	}
	service.notifyAppSessionChanged(ctx, AppSessionChangeEvent{
		Action:     "verify-finished",
		AppSession: service.mapSessionDTOWithCookies(updated, records),
		// Verification only reads the existing credential snapshot. Saved must
		// remain false so listeners never report this as a credential write.
		Saved:  false,
		Reason: reason,
	})
}

func appSessionParamsPreservingState(current appsessions.Session, updatedAt time.Time) appsessions.SessionParams {
	return appsessions.SessionParams{
		ID:                           current.ID,
		SiteKey:                      current.SiteKey,
		Status:                       string(current.Status),
		AccountDisplayName:           current.AccountDisplayName,
		AccountHandle:                current.AccountHandle,
		AccountAvatarURL:             current.AccountAvatarURL,
		AccountTierKey:               current.AccountTierKey,
		AccountTierLabel:             current.AccountTierLabel,
		AccountBadgesJSON:            current.AccountBadgesJSON,
		AccountMetadataJSON:          current.AccountMetadataJSON,
		AccountVerificationStatus:    string(current.AccountVerificationStatus),
		AccountVerificationError:     current.AccountVerificationError,
		AccountVerificationStartedAt: current.AccountVerificationStartedAt,
		LastVerifiedAt:               current.LastVerifiedAt,
		SourceType:                   string(current.SourceType),
		SourceBrowser:                current.SourceBrowser,
		SourceProfile:                current.SourceProfile,
		LastSyncedAt:                 current.LastSyncedAt,
		CreatedAt:                    &current.CreatedAt,
		UpdatedAt:                    &updatedAt,
	}
}

func newerVerificationStartedAt(current *time.Time, expected time.Time) bool {
	if current == nil || expected.IsZero() {
		return false
	}
	return current.After(expected)
}

func (service *AppSessionsService) accountVerificationTimeoutDuration() time.Duration {
	if service == nil || service.accountVerificationTimeout <= 0 {
		return 30 * time.Second
	}
	return service.accountVerificationTimeout
}

func appSessionVerificationErrorMessage(err error) string {
	if err == nil {
		return ""
	}
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return "verification timed out"
	case errors.Is(err, appsessions.ErrNoCookies):
		return "account could not be verified"
	case errors.Is(err, appsessions.ErrUnsupported):
		return "account verification unsupported"
	}
	message := strings.TrimSpace(err.Error())
	if len(message) > 240 {
		message = message[:240]
	}
	return message
}
