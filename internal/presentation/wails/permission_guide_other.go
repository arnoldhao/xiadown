//go:build !darwin || !cgo || ios

package wails

func openPermissionGuide(_ permissionGuideRequest) error {
	return nil
}
