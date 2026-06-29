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

func (service *AppSessionsService) startAppSessionAccountVerification(session appsessions.Session, records []appcookies.Record, startedAt time.Time) {
	if service == nil || service.accountFetcher == nil || !appSessionRequiresAccountVerification(session.SiteKey) {
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
			service.finishAppSessionAccountVerification(session, records, startedAt, result.account, result.err)
		case <-timer.C:
			cancel()
			service.finishAppSessionAccountVerification(session, records, startedAt, dto.AppSessionAccount{}, context.DeadlineExceeded)
		}
	}()
}

type appSessionAccountVerificationResult struct {
	account dto.AppSessionAccount
	err     error
}

func (service *AppSessionsService) finishAppSessionAccountVerification(session appsessions.Session, records []appcookies.Record, startedAt time.Time, account dto.AppSessionAccount, cause error) {
	if service == nil || service.repo == nil {
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
	params := appsessions.SessionParams{
		ID:        current.ID,
		SiteKey:   current.SiteKey,
		Status:    string(current.Status),
		CreatedAt: &current.CreatedAt,
		UpdatedAt: &now,
	}
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
		Saved:      len(records) > 0,
		Reason:     reason,
	})
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
