//go:build !windows

package libraryimport

func platformPathHidden(string) bool { return false }
