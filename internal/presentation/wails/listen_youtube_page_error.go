package wails

import (
	"fmt"
	"strings"
)

const (
	listenYouTubeVerificationRequiredErrorCode = "youtube-verification-required"
	listenYouTubeRegionUnavailableErrorCode    = "youtube-region-unavailable"
)

func applyListenYouTubePageError(payload map[string]any) bool {
	rawPageError := normalizeListenYouTubeErrorText(listenPayloadString(payload, "pageErrorText"))
	rawErrorMessage := normalizeListenYouTubeErrorText(listenPayloadString(payload, "errorMessage"))
	rawMessage := normalizeListenYouTubeErrorText(listenPayloadString(payload, "message"))
	upstreamStatus := strings.ToUpper(normalizeListenYouTubeErrorText(listenPayloadString(payload, "upstreamStatus")))
	upstreamReason := normalizeListenYouTubeErrorText(listenPayloadString(payload, "upstreamReason"))
	upstreamSubreason := normalizeListenYouTubeErrorText(listenPayloadString(payload, "upstreamSubreason"))
	playerErrorCode := normalizeListenYouTubeErrorText(listenPayloadDisplayString(payload, "playerErrorCode"))
	upstreamMessage := joinDistinctListenYouTubeErrorText(upstreamReason, upstreamSubreason)
	classificationText := strings.ToLower(joinDistinctListenYouTubeErrorText(
		rawPageError,
		rawErrorMessage,
		rawMessage,
		upstreamMessage,
	))
	structuredFailure := upstreamStatus != "" && upstreamStatus != "OK"
	if classificationText == "" && !structuredFailure && playerErrorCode == "" {
		// A page-error without a visible player error is normally an unrelated
		// image/script/resource failure. It must not replace playback state.
		return listenPayloadString(payload, "type") != "page-error"
	}
	if listenPayloadBool(payload, "upstreamHasCaptcha") {
		setListenYouTubeKnownPageError(
			payload,
			listenYouTubeVerificationRequiredErrorCode,
			"YouTube verification required",
		)
		return true
	}
	verificationMarkers := []string{
		"确认你不是聊天机器人",
		"確認你不是機器人",
		"確認您不是機器人",
		"confirm you're not a bot",
		"confirm you’re not a bot",
		"confirm you are not a bot",
		"confirm that you're not a bot",
		"ロボットではないことを確認",
		"봇이 아님을 확인",
		"no eres un bot",
		"não é um robô",
		"bukan bot",
		"không phải là bot",
	}
	for _, marker := range verificationMarkers {
		if !strings.Contains(classificationText, marker) {
			continue
		}
		setListenYouTubeKnownPageError(
			payload,
			listenYouTubeVerificationRequiredErrorCode,
			"YouTube verification required",
		)
		return true
	}
	regionUnavailableMarkers := []string{
		"youtube 在你所在区域无法使用",
		"youtube 在您所在区域无法使用",
		"youtube 在你所在區域無法使用",
		"youtube 在您所在區域無法使用",
		"youtube is not available in your area",
		"youtube is unavailable in your region",
		"youtube no está disponible en tu zona",
		"youtube no está disponible en tu región",
		"o youtube não está disponível na sua região",
		"youtube tidak tersedia di wilayah anda",
		"youtube không hoạt động ở khu vực của bạn",
		"お住まいの地域では youtube をご利用いただけません",
		"현재 지역에서는 youtube를 이용할 수 없습니다",
		"the uploader has not made this video available in your country",
		"this video is not available in your country",
	}
	for _, marker := range regionUnavailableMarkers {
		if !strings.Contains(classificationText, marker) {
			continue
		}
		setListenYouTubeKnownPageError(
			payload,
			listenYouTubeRegionUnavailableErrorCode,
			"YouTube is unavailable in this region",
		)
		return true
	}

	detailedMessage := joinDistinctListenYouTubeErrorText(
		upstreamReason,
		upstreamSubreason,
		rawPageError,
		rawErrorMessage,
		rawMessage,
	)
	if detailedMessage == "" && playerErrorCode != "" {
		detailedMessage = fmt.Sprintf("YouTube player error %s", playerErrorCode)
	}
	if detailedMessage == "" && structuredFailure {
		detailedMessage = fmt.Sprintf("YouTube playback error (%s)", upstreamStatus)
	}
	if detailedMessage != "" {
		payload["state"] = "error"
		payload["errorMessage"] = detailedMessage
		payload["message"] = detailedMessage
	}
	if listenPayloadDisplayString(payload, "errorCode") == "" &&
		listenPayloadDisplayString(payload, "code") == "" &&
		playerErrorCode != "" {
		payload["errorCode"] = playerErrorCode
		payload["code"] = playerErrorCode
	}
	return true
}

func normalizeListenYouTubeErrorText(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

func joinDistinctListenYouTubeErrorText(values ...string) string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		normalized := normalizeListenYouTubeErrorText(value)
		if normalized == "" {
			continue
		}
		lower := strings.ToLower(normalized)
		duplicate := false
		for index, existing := range parts {
			existingLower := strings.ToLower(existing)
			switch {
			case existingLower == lower || strings.Contains(existingLower, lower):
				duplicate = true
			case strings.Contains(lower, existingLower):
				parts[index] = normalized
				duplicate = true
			}
			if duplicate {
				break
			}
		}
		if !duplicate {
			parts = append(parts, normalized)
		}
	}
	return strings.Join(parts, "\n")
}

func setListenYouTubeKnownPageError(payload map[string]any, code string, message string) {
	payload["state"] = "error"
	payload["errorCode"] = code
	payload["code"] = code
	payload["errorMessage"] = message
	payload["message"] = message
}
