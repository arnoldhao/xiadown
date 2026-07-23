package browserprofile

import (
	"fmt"
	"sort"
	"strings"
)

const (
	snapshotDomainLimit       = 64
	snapshotDomainInputLimit  = 256
	snapshotDomainLengthLimit = 253
	snapshotDomainLabelLimit  = 63
)

func normalizeSnapshotDomains(domains []string) ([]string, error) {
	if len(domains) == 0 || len(domains) > snapshotDomainInputLimit {
		return nil, fmt.Errorf("browser snapshot domain allowlist is invalid")
	}
	set := make(map[string]struct{}, len(domains))
	for _, value := range domains {
		domain := strings.Trim(strings.ToLower(strings.TrimSpace(value)), ".")
		if !validSnapshotDomain(domain) {
			return nil, fmt.Errorf("browser snapshot domain allowlist is invalid")
		}
		set[domain] = struct{}{}
		if len(set) > snapshotDomainLimit {
			return nil, fmt.Errorf("browser snapshot domain allowlist exceeds limit")
		}
	}
	result := make([]string, 0, len(set))
	for domain := range set {
		result = append(result, domain)
	}
	sort.Strings(result)
	if len(result) == 0 {
		return nil, fmt.Errorf("browser snapshot domain allowlist is empty")
	}
	return result, nil
}

func validSnapshotDomain(domain string) bool {
	if domain == "" || len(domain) > snapshotDomainLengthLimit {
		return false
	}
	for _, label := range strings.Split(domain, ".") {
		if label == "" || len(label) > snapshotDomainLabelLimit || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, value := range label {
			if (value < 'a' || value > 'z') && (value < '0' || value > '9') && value != '-' {
				return false
			}
		}
	}
	return true
}

func snapshotDomainAllowed(cookieDomain string, allowedDomains []string) bool {
	domain := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(cookieDomain)), ".")
	if domain == "" {
		return false
	}
	for _, allowed := range allowedDomains {
		if domain == allowed || strings.HasSuffix(domain, "."+allowed) {
			return true
		}
	}
	return false
}
