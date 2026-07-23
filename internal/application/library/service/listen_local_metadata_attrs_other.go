//go:build !darwin && !linux

package service

func copyListenLocalMetadataExtendedAttributes(string, string) error {
	return nil
}
