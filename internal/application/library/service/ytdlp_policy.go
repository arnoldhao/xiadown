package service

import (
	"strings"

	"xiadown/internal/application/library/dto"
)

type ytdlpLogPolicy string

const (
	ytdlpLogPolicyAuto   ytdlpLogPolicy = "auto"
	ytdlpLogPolicyAlways ytdlpLogPolicy = "always"
	ytdlpLogPolicyNever  ytdlpLogPolicy = "never"
)

func resolveYTDLPLogPolicy(request dto.CreateYTDLPJobRequest) ytdlpLogPolicy {
	value := strings.ToLower(strings.TrimSpace(request.LogPolicy))
	switch value {
	case "always", "full", "debug":
		return ytdlpLogPolicyAlways
	case "never", "off", "none":
		return ytdlpLogPolicyNever
	case "auto", "":
		return ytdlpLogPolicyAuto
	default:
		return ytdlpLogPolicyAuto
	}
}
