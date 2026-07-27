package service

import (
	"math"
	"sort"
	"strings"

	"xiadown/internal/application/library/dto"
)

type catalogStorageVolume struct {
	ID             string
	Name           string
	MountPath      string
	FileSystem     string
	Kind           string
	ReadOnly       bool
	TotalBytes     int64
	AvailableBytes int64
}

func storageVolumeBytes(blocks uint64, blockSize uint64) int64 {
	if blocks == 0 || blockSize == 0 {
		return 0
	}
	if blocks > uint64(math.MaxInt64)/blockSize {
		return math.MaxInt64
	}
	return int64(blocks * blockSize)
}

func catalogStorageVolumeDTOs(items []catalogStorageVolume) []dto.CatalogStorageVolumeDTO {
	byID := make(map[string]catalogStorageVolume, len(items))
	for _, item := range items {
		item.ID = strings.TrimSpace(item.ID)
		item.Name = strings.TrimSpace(item.Name)
		item.MountPath = strings.TrimSpace(item.MountPath)
		item.FileSystem = strings.TrimSpace(item.FileSystem)
		item.Kind = strings.TrimSpace(item.Kind)
		if item.ID == "" || item.MountPath == "" || item.TotalBytes <= 0 {
			continue
		}
		item.AvailableBytes = max(0, min(item.TotalBytes, item.AvailableBytes))
		if current, exists := byID[item.ID]; exists &&
			len(current.MountPath) <= len(item.MountPath) {
			continue
		}
		byID[item.ID] = item
	}
	volumes := make([]catalogStorageVolume, 0, len(byID))
	for _, item := range byID {
		volumes = append(volumes, item)
	}
	sort.Slice(volumes, func(left, right int) bool {
		leftSystem := volumes[left].MountPath == "/" ||
			strings.EqualFold(volumes[left].MountPath, `C:\`)
		rightSystem := volumes[right].MountPath == "/" ||
			strings.EqualFold(volumes[right].MountPath, `C:\`)
		if leftSystem != rightSystem {
			return leftSystem
		}
		return strings.ToLower(volumes[left].MountPath) <
			strings.ToLower(volumes[right].MountPath)
	})
	result := make([]dto.CatalogStorageVolumeDTO, 0, len(volumes))
	for _, item := range volumes {
		result = append(result, dto.CatalogStorageVolumeDTO{
			ID: item.ID, Name: item.Name, MountPath: item.MountPath,
			FileSystem: item.FileSystem, Kind: item.Kind, ReadOnly: item.ReadOnly,
			TotalBytes: item.TotalBytes, AvailableBytes: item.AvailableBytes,
		})
	}
	return result
}
