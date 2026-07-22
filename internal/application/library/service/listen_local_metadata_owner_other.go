//go:build !darwin && !linux

package service

import "os"

func copyListenLocalMetadataOwnership(string, os.FileInfo) error {
	return nil
}
