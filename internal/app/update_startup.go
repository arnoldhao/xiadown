package app

import (
	"context"
	"os"
	"strconv"
	"strings"

	domainupdate "xiadown/internal/domain/update"
	infrastructureupdate "xiadown/internal/infrastructure/update"
)

type preparedUpdateStartupRunner interface {
	PreparedUpdate(ctx context.Context) (domainupdate.Info, bool, error)
	ClearPreparedUpdate(ctx context.Context) error
	RestartToApply(ctx context.Context) error
}

func TryApplyPreparedUpdateOnLaunch(ctx context.Context, args []string) (bool, error) {
	startup := currentStartupContext(args)
	if startup.skipPreparedUpdate {
		return false, nil
	}

	installer, err := infrastructureupdate.NewInstaller("")
	if err != nil {
		return false, err
	}
	return maybeApplyPreparedUpdateOnLaunch(ctx, resolveVersion(os.Getenv("APP_ENV")), installer)
}

func maybeApplyPreparedUpdateOnLaunch(ctx context.Context, currentVersion string, installer preparedUpdateStartupRunner) (bool, error) {
	if installer == nil {
		return false, nil
	}

	normalizedCurrent := domainupdate.NormalizeVersion(currentVersion)
	if !isComparableReleaseVersion(normalizedCurrent) {
		return false, nil
	}

	prepared, found, err := installer.PreparedUpdate(ctx)
	if err != nil || !found {
		return false, err
	}

	preparedVersion := domainupdate.NormalizeVersion(prepared.PreparedVersion)
	if preparedVersion == "" {
		preparedVersion = domainupdate.NormalizeVersion(prepared.LatestVersion)
	}
	if !isComparableReleaseVersion(preparedVersion) {
		return false, nil
	}

	if domainupdate.CompareVersion(normalizedCurrent, preparedVersion) >= 0 {
		if err := installer.ClearPreparedUpdate(ctx); err != nil {
			return false, err
		}
		return false, nil
	}

	if err := installer.RestartToApply(ctx); err != nil {
		return false, err
	}
	return true, nil
}

func isComparableReleaseVersion(version string) bool {
	normalized := domainupdate.NormalizeVersion(version)
	if normalized == "" {
		return false
	}
	parts := strings.Split(normalized, ".")
	for _, part := range parts {
		if part == "" {
			return false
		}
		if _, err := strconv.Atoi(part); err != nil {
			return false
		}
	}
	return true
}
