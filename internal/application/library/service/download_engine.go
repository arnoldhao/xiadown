package service

func isResourceDownloadURL(rawURL string) bool {
	domain := extractRegistrableDomain(rawURL)
	if domain == "" {
		return false
	}
	switch domain {
	case "youtube.com", "youtu.be":
		return false
	default:
		return true
	}
}
