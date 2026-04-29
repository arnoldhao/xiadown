package service

import (
	"regexp"
	"strconv"
	"strings"

	"xiadown/internal/application/library/dto"
)

const (
	speedMetricKindBytesPerSecond  = "bytes_per_second"
	speedMetricKindFramesPerSecond = "frames_per_second"
	speedMetricKindFactor          = "factor"
	speedMetricKindOther           = "other"
)

var (
	speedMetricByteRateRe = regexp.MustCompile(`(?i)^\s*([0-9]+(?:\.[0-9]+)?)\s*([kmgtp]?i?b)\s*/\s*s\s*$`)
	speedMetricFPSRe      = regexp.MustCompile(`(?i)^\s*([0-9]+(?:\.[0-9]+)?)\s*fps\s*$`)
	speedMetricFactorRe   = regexp.MustCompile(`(?i)^\s*([0-9]+(?:\.[0-9]+)?)\s*x\s*$`)
)

func parseProgressSpeedMetric(kind string, speed string) *dto.OperationSpeedMetricDTO {
	trimmedKind := strings.TrimSpace(strings.ToLower(kind))
	trimmedSpeed := strings.TrimSpace(speed)
	if trimmedSpeed == "" {
		return nil
	}

	if match := speedMetricByteRateRe.FindStringSubmatch(trimmedSpeed); len(match) >= 3 {
		bytesPerSecond, ok := parseSpeedMetricByteRate(match[1], match[2])
		if ok && bytesPerSecond > 0 {
			return &dto.OperationSpeedMetricDTO{
				Kind:           speedMetricKindBytesPerSecond,
				Label:          trimmedSpeed,
				BytesPerSecond: float64Ptr(bytesPerSecond),
			}
		}
	}

	if match := speedMetricFPSRe.FindStringSubmatch(trimmedSpeed); len(match) >= 2 {
		framesPerSecond, ok := parseSpeedMetricFloat(match[1])
		if ok && framesPerSecond > 0 {
			return &dto.OperationSpeedMetricDTO{
				Kind:            speedMetricKindFramesPerSecond,
				Label:           trimmedSpeed,
				FramesPerSecond: float64Ptr(framesPerSecond),
			}
		}
	}

	if trimmedKind == "transcode" {
		if match := speedMetricFactorRe.FindStringSubmatch(trimmedSpeed); len(match) >= 2 {
			factor, ok := parseSpeedMetricFloat(match[1])
			if ok && factor > 0 {
				return &dto.OperationSpeedMetricDTO{
					Kind:   speedMetricKindFactor,
					Label:  trimmedSpeed,
					Factor: float64Ptr(factor),
				}
			}
		}
	}

	return &dto.OperationSpeedMetricDTO{
		Kind:  speedMetricKindOther,
		Label: trimmedSpeed,
	}
}

func parseSpeedMetricByteRate(value string, unit string) (float64, bool) {
	parsed, ok := parseSpeedMetricFloat(value)
	if !ok || parsed <= 0 {
		return 0, false
	}
	normalizedUnit := strings.ToUpper(strings.TrimSpace(unit))
	if normalizedUnit == "" || normalizedUnit == "B" {
		return parsed, true
	}

	base := float64(1000)
	if strings.Contains(normalizedUnit, "I") {
		base = 1024
	}
	prefix := normalizedUnit[:1]
	multiplier := float64(1)
	switch prefix {
	case "K":
		multiplier = base
	case "M":
		multiplier = base * base
	case "G":
		multiplier = base * base * base
	case "T":
		multiplier = base * base * base * base
	case "P":
		multiplier = base * base * base * base * base
	}
	return parsed * multiplier, true
}

func parseSpeedMetricFloat(value string) (float64, bool) {
	parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil || parsed <= 0 {
		return 0, false
	}
	return parsed, true
}

func float64Ptr(value float64) *float64 {
	return &value
}
