package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"xiadown/internal/application/connectors/dto"
	appcookies "xiadown/internal/application/cookies"
	"xiadown/internal/application/sitepolicy"
	"xiadown/internal/domain/connectors"
)

const (
	connectorProfileInfoEntryLimit     = 20000
	connectorProfileInfoComponentLimit = 12
)

var errConnectorProfileInfoLimit = errors.New("connector profile info limit reached")

func mapConnectorDTO(item connectors.Connector) dto.Connector {
	cookies := decodeCookies(item.CookiesJSON)
	status := item.Status
	if item.CredentialMode == "" {
		item.CredentialMode = connectors.DefaultCredentialMode(item.Type)
	}
	if item.CredentialMode == connectors.CredentialModeProfile {
		cookies = nil
	}
	if status == "" {
		status = connectors.StatusDisconnected
	}
	if item.CredentialMode == connectors.CredentialModeCookies {
		if len(cookies) == 0 {
			status = connectors.StatusDisconnected
		} else if status == connectors.StatusDisconnected {
			status = connectors.StatusConnected
		}
	}
	lastVerified := ""
	if item.LastVerifiedAt != nil {
		lastVerified = item.LastVerifiedAt.Format(time.RFC3339)
	}
	policy, _ := sitepolicy.ForConnectorType(string(item.Type))
	return dto.Connector{
		ID:              item.ID,
		Type:            string(item.Type),
		Group:           connectorGroup(item.Type),
		Desc:            connectorDesc(item.Type),
		Status:          string(status),
		CredentialMode:  string(item.CredentialMode),
		CredentialState: connectorCredentialState(item, status, len(cookies)),
		CookiesCount:    len(cookies),
		Cookies:         mapCookiesDTO(cookies),
		ProfileKey:      item.ProfileKey,
		ProfilePath:     item.ProfilePath,
		ProfileBrowser:  item.ProfileBrowser,
		ProfileInfo:     connectorProfileInfo(item),
		Domains:         append([]string(nil), policy.Domains...),
		ProfileSites:    mapConnectorSitesDTO(policy.ProfileSites),
		PolicyKey:       policy.Key,
		Capabilities:    append([]string(nil), policy.Capabilities...),
		LastVerifiedAt:  lastVerified,
	}
}

func mapConnectorSitesDTO(sites []sitepolicy.ProfileSite) []dto.ConnectorSite {
	if len(sites) == 0 {
		return nil
	}
	result := make([]dto.ConnectorSite, 0, len(sites))
	for _, site := range sites {
		trimmedURL := strings.TrimSpace(site.URL)
		if trimmedURL == "" {
			continue
		}
		result = append(result, dto.ConnectorSite{
			Key:   strings.TrimSpace(site.Key),
			Label: strings.TrimSpace(site.Label),
			URL:   trimmedURL,
		})
	}
	return result
}

func connectorCredentialState(item connectors.Connector, status connectors.ConnectorStatus, cookiesCount int) string {
	mode := item.CredentialMode
	if mode == "" {
		mode = connectors.DefaultCredentialMode(item.Type)
	}
	if mode == connectors.CredentialModeProfile {
		profilePath := strings.TrimSpace(item.ProfilePath)
		if profilePath == "" {
			return string(connectors.StatusDisconnected)
		}
		if stat, err := os.Stat(profilePath); err == nil && stat.IsDir() {
			return "profile"
		}
		return string(connectors.StatusDisconnected)
	}
	if status == "" {
		status = connectors.StatusDisconnected
	}
	if status == connectors.StatusConnected && cookiesCount == 0 {
		return string(connectors.StatusDisconnected)
	}
	return string(status)
}

func connectorProfileInfo(item connectors.Connector) *dto.ConnectorProfile {
	mode := item.CredentialMode
	if mode == "" {
		mode = connectors.DefaultCredentialMode(item.Type)
	}
	if mode != connectors.CredentialModeProfile {
		return nil
	}
	profilePath := strings.TrimSpace(item.ProfilePath)
	info := &dto.ConnectorProfile{
		Path:     profilePath,
		Browser:  strings.TrimSpace(item.ProfileBrowser),
		Bindings: connectorProfileBindings(item),
	}
	if profilePath == "" {
		return info
	}
	stat, err := os.Stat(profilePath)
	if err != nil {
		if !os.IsNotExist(err) {
			info.Error = err.Error()
		}
		return info
	}
	info.Exists = true
	if !stat.IsDir() {
		info.SizeBytes = stat.Size()
		info.FileCount = 1
		return info
	}

	components := map[string]*dto.ConnectorProfileComponent{}
	visited := 0
	walkErr := filepath.WalkDir(profilePath, func(path string, entry os.DirEntry, err error) error {
		if err != nil || path == profilePath {
			return nil
		}
		visited++
		if visited > connectorProfileInfoEntryLimit {
			info.Truncated = true
			return errConnectorProfileInfoLimit
		}
		rel, err := filepath.Rel(profilePath, path)
		if err != nil || rel == "." || strings.TrimSpace(rel) == "" {
			return nil
		}
		componentName := strings.Split(filepath.ToSlash(rel), "/")[0]
		if componentName == "" {
			return nil
		}
		component := components[componentName]
		if component == nil {
			componentPath := filepath.Join(profilePath, componentName)
			component = &dto.ConnectorProfileComponent{
				Name: componentName,
				Path: componentPath,
				Kind: "file",
			}
			if componentStat, statErr := os.Stat(componentPath); statErr == nil && componentStat.IsDir() {
				component.Kind = "directory"
			}
			components[componentName] = component
		}
		if entry.IsDir() {
			info.DirectoryCount++
			component.DirectoryCount++
			return nil
		}
		fileInfo, err := entry.Info()
		if err != nil {
			return nil
		}
		info.FileCount++
		info.SizeBytes += fileInfo.Size()
		component.FileCount++
		component.SizeBytes += fileInfo.Size()
		return nil
	})
	if walkErr != nil && !errors.Is(walkErr, errConnectorProfileInfoLimit) {
		info.Error = walkErr.Error()
	}

	info.Components = make([]dto.ConnectorProfileComponent, 0, len(components))
	for _, component := range components {
		info.Components = append(info.Components, *component)
	}
	sort.Slice(info.Components, func(i, j int) bool {
		left := info.Components[i]
		right := info.Components[j]
		switch {
		case left.SizeBytes != right.SizeBytes:
			return left.SizeBytes > right.SizeBytes
		case left.FileCount != right.FileCount:
			return left.FileCount > right.FileCount
		default:
			return strings.ToLower(left.Name) < strings.ToLower(right.Name)
		}
	})
	if len(info.Components) > connectorProfileInfoComponentLimit {
		info.Components = info.Components[:connectorProfileInfoComponentLimit]
		info.Truncated = true
	}
	return info
}

func connectorProfileBindings(item connectors.Connector) []dto.ConnectorProfileBinding {
	mode := item.CredentialMode
	if mode == "" {
		mode = connectors.DefaultCredentialMode(item.Type)
	}
	if mode != connectors.CredentialModeProfile {
		return nil
	}
	profileKey := strings.TrimSpace(item.ProfileKey)
	if profileKey == "" {
		profileKey = defaultConnectorProfileKey(item)
	}
	rootPath, err := defaultConnectorProfileRootPath(profileKey)
	if err != nil {
		return nil
	}
	currentBrowser := strings.TrimSpace(item.ProfileBrowser)
	currentPath := strings.TrimSpace(item.ProfilePath)
	bindings := make([]dto.ConnectorProfileBinding, 0, 4)
	seen := map[string]struct{}{}
	add := func(browser string, path string, current bool) {
		browser = sanitizeConnectorProfileKey(firstNonEmptyString(browser, connectorDefaultProfileBrowser))
		path = strings.TrimSpace(path)
		if path == "" {
			resolved, err := defaultConnectorProfilePath(profileKey, browser)
			if err == nil {
				path = resolved
			}
		}
		key := strings.ToLower(browser)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		bindings = append(bindings, connectorProfileBinding(browser, path, current))
	}
	if currentBrowser != "" || currentPath != "" {
		add(currentBrowser, currentPath, true)
	}
	if entries, err := os.ReadDir(rootPath); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			add(entry.Name(), filepath.Join(rootPath, entry.Name()), strings.EqualFold(entry.Name(), currentBrowser))
		}
	}
	sort.SliceStable(bindings, func(i, j int) bool {
		left := bindings[i]
		right := bindings[j]
		if left.Current != right.Current {
			return left.Current
		}
		if left.Exists != right.Exists {
			return left.Exists
		}
		return strings.ToLower(left.Browser) < strings.ToLower(right.Browser)
	})
	return bindings
}

func connectorProfileBinding(browser string, path string, current bool) dto.ConnectorProfileBinding {
	binding := dto.ConnectorProfileBinding{
		Browser: strings.TrimSpace(browser),
		Path:    strings.TrimSpace(path),
		Current: current,
	}
	if binding.Path == "" {
		return binding
	}
	stat, err := os.Stat(binding.Path)
	if err != nil {
		return binding
	}
	binding.Exists = true
	if !stat.IsDir() {
		binding.SizeBytes = stat.Size()
		binding.FileCount = 1
		return binding
	}
	_ = filepath.WalkDir(binding.Path, func(path string, entry os.DirEntry, err error) error {
		if err != nil || path == binding.Path {
			return nil
		}
		if entry.IsDir() {
			binding.DirectoryCount++
			return nil
		}
		fileInfo, err := entry.Info()
		if err != nil {
			return nil
		}
		binding.FileCount++
		binding.SizeBytes += fileInfo.Size()
		return nil
	})
	return binding
}

func (service *ConnectorsService) CookiesForConnectorType(ctx context.Context, connectorType connectors.ConnectorType) ([]appcookies.Record, error) {
	if service == nil {
		return nil, connectors.ErrConnectorNotFound
	}
	items, err := service.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		if item.Type != connectorType {
			continue
		}
		mode := item.CredentialMode
		if mode == "" {
			mode = connectors.DefaultCredentialMode(item.Type)
		}
		if mode == connectors.CredentialModeProfile {
			return nil, connectors.ErrNoCookies
		}
		records := decodeCookies(item.CookiesJSON)
		if len(records) == 0 {
			return nil, connectors.ErrNoCookies
		}
		return records, nil
	}
	return nil, connectors.ErrConnectorNotFound
}

func connectorGroup(connectorType connectors.ConnectorType) string {
	switch connectorType {
	case connectors.ConnectorYouTube,
		connectors.ConnectorBilibili,
		connectors.ConnectorTikTok,
		connectors.ConnectorChinaPrivate,
		connectors.ConnectorInstagram,
		connectors.ConnectorX,
		connectors.ConnectorFacebook,
		connectors.ConnectorVimeo,
		connectors.ConnectorTwitch,
		connectors.ConnectorNiconico:
		return "video"
	default:
		return "other"
	}
}

func connectorDesc(connectorType connectors.ConnectorType) string {
	switch connectorType {
	case connectors.ConnectorYouTube:
		return "YouTube videos, playlists, live streams, age-restricted videos, and member-only content when cookies are available."
	case connectors.ConnectorBilibili:
		return "Bilibili videos, bangumi, playlists, higher-quality formats, and login-gated content when cookies are available."
	case connectors.ConnectorTikTok:
		return "TikTok videos and creator pages that often need browser cookies for region, login, or anti-bot checks."
	case connectors.ConnectorChinaPrivate:
		return "Chinese private-domain video sites that need a persistent browser profile for login and anti-bot checks."
	case connectors.ConnectorInstagram:
		return "Instagram reels, posts, stories, and private or login-gated media when cookies are available."
	case connectors.ConnectorX:
		return "X/Twitter videos, broadcasts, spaces, and posts that may need cookies for login-gated content."
	case connectors.ConnectorFacebook:
		return "Facebook videos, reels, and watch pages that commonly require a logged-in browser session."
	case connectors.ConnectorVimeo:
		return "Vimeo videos and private or account-restricted pages when cookies are available."
	case connectors.ConnectorTwitch:
		return "Twitch streams, VODs, clips, and subscriber or mature content when cookies are available."
	case connectors.ConnectorNiconico:
		return "Niconico videos and account-restricted Japanese media when cookies are available."
	default:
		return ""
	}
}

func mapCookiesDTO(records []appcookies.Record) []dto.ConnectorCookie {
	if len(records) == 0 {
		return nil
	}
	result := make([]dto.ConnectorCookie, 0, len(records))
	for _, record := range records {
		result = append(result, dto.ConnectorCookie{
			Name:     record.Name,
			Value:    record.Value,
			Domain:   record.Domain,
			Path:     record.Path,
			Expires:  record.Expires,
			HttpOnly: record.HttpOnly,
			Secure:   record.Secure,
			SameSite: record.SameSite,
		})
	}
	return result
}

func encodeCookies(records []appcookies.Record) (string, error) {
	return appcookies.EncodeJSON(records)
}

func decodeCookies(data string) []appcookies.Record {
	return appcookies.DecodeJSON(data)
}
