//go:build windows

package service

import (
	"fmt"
	"strings"

	"golang.org/x/sys/windows"
)

func inspectCatalogStorageVolume(path string) (catalogStorageVolume, error) {
	pathPointer, err := windows.UTF16PtrFromString(strings.TrimSpace(path))
	if err != nil {
		return catalogStorageVolume{}, err
	}
	mountBuffer := make([]uint16, windows.MAX_PATH+1)
	if err := windows.GetVolumePathName(pathPointer, &mountBuffer[0], uint32(len(mountBuffer))); err != nil {
		return catalogStorageVolume{}, err
	}
	mount := windows.UTF16ToString(mountBuffer)
	return inspectWindowsCatalogStorageVolume(mount)
}

func inspectWindowsCatalogStorageVolume(mount string) (catalogStorageVolume, error) {
	mountPointer, err := windows.UTF16PtrFromString(mount)
	if err != nil {
		return catalogStorageVolume{}, err
	}
	volumeBuffer := make([]uint16, windows.MAX_PATH+1)
	volumeID := ""
	if volumeErr := windows.GetVolumeNameForVolumeMountPoint(
		mountPointer,
		&volumeBuffer[0],
		uint32(len(volumeBuffer)),
	); volumeErr == nil {
		volumeID = "windows-volume:" + strings.ToLower(windows.UTF16ToString(volumeBuffer))
	} else {
		volumeID = "windows-mount:" + strings.ToLower(mount)
	}
	var available, total, free uint64
	if err := windows.GetDiskFreeSpaceEx(mountPointer, &available, &total, &free); err != nil {
		return catalogStorageVolume{}, fmt.Errorf("inspect Windows storage capacity: %w", err)
	}
	nameBuffer := make([]uint16, windows.MAX_PATH+1)
	fileSystemBuffer := make([]uint16, windows.MAX_PATH+1)
	var fileSystemFlags uint32
	_ = windows.GetVolumeInformation(
		mountPointer,
		&nameBuffer[0],
		uint32(len(nameBuffer)),
		nil,
		nil,
		&fileSystemFlags,
		&fileSystemBuffer[0],
		uint32(len(fileSystemBuffer)),
	)
	return catalogStorageVolume{
		ID:             volumeID,
		Name:           windows.UTF16ToString(nameBuffer),
		MountPath:      mount,
		FileSystem:     windows.UTF16ToString(fileSystemBuffer),
		ReadOnly:       fileSystemFlags&windows.FILE_READ_ONLY_VOLUME != 0,
		TotalBytes:     storageVolumeBytes(total, 1),
		AvailableBytes: storageVolumeBytes(available, 1),
	}, nil
}

func listCatalogStorageVolumes() ([]catalogStorageVolume, error) {
	required, err := windows.GetLogicalDriveStrings(0, nil)
	if err != nil {
		return nil, fmt.Errorf("list Windows storage volumes: %w", err)
	}
	buffer := make([]uint16, required+1)
	if _, err := windows.GetLogicalDriveStrings(uint32(len(buffer)), &buffer[0]); err != nil {
		return nil, fmt.Errorf("read Windows storage volumes: %w", err)
	}
	result := []catalogStorageVolume{}
	for start := 0; start < len(buffer); {
		end := start
		for end < len(buffer) && buffer[end] != 0 {
			end++
		}
		if end == start {
			break
		}
		mount := windows.UTF16ToString(buffer[start:end])
		start = end + 1
		mountPointer, pointerErr := windows.UTF16PtrFromString(mount)
		if pointerErr != nil {
			continue
		}
		driveType := windows.GetDriveType(mountPointer)
		kind := ""
		switch driveType {
		case windows.DRIVE_FIXED:
			kind = "local"
		case windows.DRIVE_REMOVABLE:
			kind = "removable"
		case windows.DRIVE_REMOTE:
			kind = "network"
		case windows.DRIVE_RAMDISK:
			kind = "memory"
		default:
			continue
		}
		volume, inspectErr := inspectWindowsCatalogStorageVolume(mount)
		if inspectErr != nil || volume.TotalBytes <= 0 {
			continue
		}
		volume.Kind = kind
		result = append(result, volume)
	}
	return result, nil
}
