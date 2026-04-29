package cookies

import (
	"encoding/json"
	"net/url"
	"strings"

	"xiadown/internal/application/sitepolicy"
)

type Record struct {
	Name     string `json:"name"`
	Value    string `json:"value"`
	Domain   string `json:"domain"`
	Path     string `json:"path"`
	Expires  int64  `json:"expires"`
	HttpOnly bool   `json:"httpOnly"`
	Secure   bool   `json:"secure"`
	SameSite string `json:"sameSite,omitempty"`
}

func EncodeJSON(records []Record) (string, error) {
	if len(records) == 0 {
		return "", nil
	}
	data, err := json.Marshal(records)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func DecodeJSON(data string) []Record {
	trimmed := strings.TrimSpace(data)
	if trimmed == "" {
		return nil
	}
	var records []Record
	if err := json.Unmarshal([]byte(trimmed), &records); err != nil {
		return nil
	}
	return records
}

func FilterByDomains(records []Record, domains []string) []Record {
	if len(records) == 0 || len(domains) == 0 {
		return nil
	}
	result := make([]Record, 0, len(records))
	for _, record := range records {
		domain := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(record.Domain)), ".")
		if domain == "" {
			continue
		}
		for _, allowed := range domains {
			if sitepolicy.HostMatchesDomain(domain, allowed) {
				result = append(result, normalizeRecord(record))
				break
			}
		}
	}
	return result
}

func MatchURL(records []Record, rawURL string) []Record {
	if len(records) == 0 {
		return nil
	}
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return nil
	}
	host := strings.ToLower(strings.TrimSpace(parsed.Hostname()))
	path := parsed.EscapedPath()
	if path == "" {
		path = "/"
	}
	result := make([]Record, 0, len(records))
	for _, record := range records {
		domain := strings.TrimSpace(record.Domain)
		if domain != "" && !sitepolicy.HostMatchesDomain(host, domain) {
			continue
		}
		cookiePath := strings.TrimSpace(record.Path)
		if cookiePath == "" {
			cookiePath = "/"
		}
		if !strings.HasPrefix(path, cookiePath) {
			continue
		}
		result = append(result, normalizeRecord(record))
	}
	return result
}

func normalizeRecord(record Record) Record {
	record.Name = strings.TrimSpace(record.Name)
	record.Domain = strings.TrimSpace(record.Domain)
	record.Path = strings.TrimSpace(record.Path)
	record.SameSite = strings.ToLower(strings.TrimSpace(record.SameSite))
	if record.Path == "" {
		record.Path = "/"
	}
	return record
}
