package service

import (
	"strings"

	"xiadown/internal/application/library/dto"
)

const (
	resourceSniffFailureProfileConnectionRequired = "profile_connection_required"
	resourceSniffFailureVerificationRequired      = "verification_required"
	resourceSniffFailureNoMediaDetected           = "no_media_detected"
	resourceSniffFailureUnsupportedDouyinLVDetail = "unsupported_douyin_lvdetail"
	resourceSniffFailureDouyinRecommendLogin      = "douyin_recommend_login_required"

	resourceSniffFailureActionConnectProfile       = "connect_profile"
	resourceSniffFailureActionCompleteVerification = "complete_verification"
	resourceSniffFailureActionPlayPage             = "play_page"
	resourceSniffFailureActionNone                 = "none"
)

type resourceSniffFailure struct {
	Code      string
	Site      string
	Action    string
	Retryable bool
	Detail    string
}

func newResourceSniffFailure(code string, site string, action string, retryable bool, detail string) resourceSniffFailure {
	return resourceSniffFailure{
		Code:      strings.TrimSpace(code),
		Site:      strings.TrimSpace(site),
		Action:    strings.TrimSpace(action),
		Retryable: retryable,
		Detail:    strings.TrimSpace(detail),
	}
}

func resourceSniffProfileConnectionRequiredFailure(site string) resourceSniffFailure {
	return newResourceSniffFailure(resourceSniffFailureProfileConnectionRequired, site, resourceSniffFailureActionConnectProfile, true, "")
}

func resourceSniffVerificationRequiredFailure(site string) resourceSniffFailure {
	return newResourceSniffFailure(resourceSniffFailureVerificationRequired, site, resourceSniffFailureActionCompleteVerification, true, "")
}

func resourceSniffNoMediaDetectedFailure(site string) resourceSniffFailure {
	return newResourceSniffFailure(resourceSniffFailureNoMediaDetected, site, resourceSniffFailureActionPlayPage, true, "")
}

func resourceSniffDouyinRecommendLoginFailure() resourceSniffFailure {
	return newResourceSniffFailure(resourceSniffFailureDouyinRecommendLogin, "douyin", resourceSniffFailureActionCompleteVerification, true, "")
}

func resourceSniffSiteName(rawURL string, extractor resourceExtractor) string {
	if extractor == nil {
		extractor = resourceExtractorForURL(rawURL)
	}
	name := strings.TrimSpace(extractor.Name())
	if name != "" && name != (resourceDefaultSiteRules{}).Name() {
		return name
	}
	return extractRegistrableDomain(rawURL)
}

func (failure resourceSniffFailure) toDTO() *dto.ResourceSniffFailure {
	if strings.TrimSpace(failure.Code) == "" {
		return nil
	}
	return &dto.ResourceSniffFailure{
		Code:      strings.TrimSpace(failure.Code),
		Site:      strings.TrimSpace(failure.Site),
		Action:    strings.TrimSpace(failure.Action),
		Retryable: failure.Retryable,
		Detail:    strings.TrimSpace(failure.Detail),
	}
}

func resourceSniffParseFailureResponse(failure resourceSniffFailure) dto.ParseResourceSniffResponse {
	return dto.ParseResourceSniffResponse{Failure: failure.toDTO()}
}
