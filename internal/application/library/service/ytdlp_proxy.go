package service

import "strings"

func (service *LibraryService) resolveYTDLPProxy(targetURL string) string {
	if service == nil || service.proxyClient == nil {
		return ""
	}
	resolver, ok := service.proxyClient.(interface {
		ResolveProxy(string) (string, error)
	})
	if !ok {
		return ""
	}
	proxyURL, err := resolver.ResolveProxy(strings.TrimSpace(targetURL))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(proxyURL)
}
