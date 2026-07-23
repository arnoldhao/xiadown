//go:build !darwin && !linux && !windows

package service

import "os"

func listenLocalFileChangeToken(os.FileInfo) string {
	return ""
}
