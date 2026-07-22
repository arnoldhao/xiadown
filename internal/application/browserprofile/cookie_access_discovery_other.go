//go:build !windows

package browserprofile

// App-bound v20 cookie protection is a Windows Chromium mechanism. Other
// platforms keep their existing direct-profile discovery behavior.
func applyCookieProtectionDiscovery(_ *DiscoveryResult, _ []string) error {
	return nil
}
