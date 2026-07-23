//go:build !windows

package service

import "os"

func restrictResourceCookieJar(file *os.File) error {
	return file.Chmod(0o600)
}
