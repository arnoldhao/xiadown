//go:build !darwin && !linux && !windows

package service

import "os"

func listenLocalFileLinkCount(string, os.FileInfo) (uint64, error) {
	return 1, nil
}
