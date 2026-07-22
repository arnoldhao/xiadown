//go:build !windows

package browserprofile

func isBrowserProfileInUseError(error) bool { return false }
