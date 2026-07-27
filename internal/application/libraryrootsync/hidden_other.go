//go:build !windows

package libraryrootsync

func platformPathHidden(string) bool { return false }
