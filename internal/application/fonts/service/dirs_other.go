//go:build !darwin && !linux && !windows

package service

func platformFontDirectories(_ string) []string {
	return nil
}
