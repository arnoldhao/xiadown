//go:build darwin

package service

import (
	"fmt"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
)

func inspectCatalogStorageVolume(path string) (catalogStorageVolume, error) {
	var stats unix.Statfs_t
	if err := unix.Statfs(strings.TrimSpace(path), &stats); err != nil {
		return catalogStorageVolume{}, err
	}
	return catalogStorageVolume{
		ID:             fmt.Sprintf("darwin-fsid:%08x:%08x", uint32(stats.Fsid.Val[0]), uint32(stats.Fsid.Val[1])),
		TotalBytes:     storageVolumeBytes(stats.Blocks, uint64(stats.Bsize)),
		AvailableBytes: storageVolumeBytes(stats.Bavail, uint64(stats.Bsize)),
	}, nil
}

func listCatalogStorageVolumes() ([]catalogStorageVolume, error) {
	count, err := unix.Getfsstat(nil, unix.MNT_NOWAIT)
	if err != nil {
		return nil, fmt.Errorf("list macOS storage volumes: %w", err)
	}
	stats := make([]unix.Statfs_t, count)
	count, err = unix.Getfsstat(stats, unix.MNT_NOWAIT)
	if err != nil {
		return nil, fmt.Errorf("read macOS storage volumes: %w", err)
	}
	if count > len(stats) {
		count = len(stats)
	}
	result := make([]catalogStorageVolume, 0, count)
	for _, item := range stats[:count] {
		mountPath := unix.ByteSliceToString(item.Mntonname[:])
		if !isVisibleDarwinCatalogStorageVolume(mountPath, item.Flags) {
			continue
		}
		totalBytes := storageVolumeBytes(item.Blocks, uint64(item.Bsize))
		if totalBytes <= 0 {
			continue
		}
		name := ""
		if mountPath != "/" {
			name = filepath.Base(mountPath)
		}
		kind := "network"
		if item.Flags&unix.MNT_LOCAL != 0 {
			kind = "local"
		}
		result = append(result, catalogStorageVolume{
			ID:             fmt.Sprintf("darwin-fsid:%08x:%08x", uint32(item.Fsid.Val[0]), uint32(item.Fsid.Val[1])),
			Name:           name,
			MountPath:      mountPath,
			FileSystem:     unix.ByteSliceToString(item.Fstypename[:]),
			Kind:           kind,
			ReadOnly:       item.Flags&unix.MNT_RDONLY != 0,
			TotalBytes:     totalBytes,
			AvailableBytes: storageVolumeBytes(item.Bavail, uint64(item.Bsize)),
		})
	}
	return result, nil
}

// macOS exposes the APFS System, Data, Preboot, Recovery, and VM roles as
// separate mounts even though they share one user-facing container. Only the
// root mount and browsable volumes below /Volumes belong in Library Overview.
// MNT_DONTBROWSE excludes Recovery and other implementation-only mounts while
// preserving ordinary external and network volumes.
func isVisibleDarwinCatalogStorageVolume(mountPath string, flags uint32) bool {
	if flags&unix.MNT_DONTBROWSE != 0 {
		return false
	}
	return mountPath == "/" || strings.HasPrefix(mountPath, "/Volumes/")
}
