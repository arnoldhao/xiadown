//go:build !darwin && !linux && !windows

package service

func inspectCatalogStorageVolume(string) (catalogStorageVolume, error) {
	return catalogStorageVolume{}, nil
}

func listCatalogStorageVolumes() ([]catalogStorageVolume, error) {
	return []catalogStorageVolume{}, nil
}
