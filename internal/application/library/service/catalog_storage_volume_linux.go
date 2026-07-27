//go:build linux

package service

import (
	"bufio"
	"fmt"
	"os"
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
		ID:             fmt.Sprintf("linux-fsid:%08x:%08x", uint32(stats.Fsid.Val[0]), uint32(stats.Fsid.Val[1])),
		TotalBytes:     storageVolumeBytes(stats.Blocks, uint64(stats.Bsize)),
		AvailableBytes: storageVolumeBytes(stats.Bavail, uint64(stats.Bsize)),
	}, nil
}

func listCatalogStorageVolumes() ([]catalogStorageVolume, error) {
	file, err := os.Open("/proc/self/mounts")
	if err != nil {
		volume, inspectErr := inspectCatalogStorageVolume("/")
		if inspectErr != nil {
			return nil, err
		}
		volume.Name, volume.MountPath, volume.Kind = "", "/", "local"
		return []catalogStorageVolume{volume}, nil
	}
	defer file.Close()

	result := []catalogStorageVolume{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 4 {
			continue
		}
		mountPath := decodeLinuxMountPath(fields[1])
		if mountPath != "/" &&
			!strings.HasPrefix(mountPath, "/mnt/") &&
			!strings.HasPrefix(mountPath, "/media/") &&
			!strings.HasPrefix(mountPath, "/run/media/") {
			continue
		}
		volume, inspectErr := inspectCatalogStorageVolume(mountPath)
		if inspectErr != nil || volume.TotalBytes <= 0 {
			continue
		}
		volume.Name = ""
		if mountPath != "/" {
			volume.Name = filepath.Base(mountPath)
		}
		volume.MountPath = mountPath
		volume.FileSystem = fields[2]
		volume.Kind = "local"
		volume.ReadOnly = strings.Contains(","+fields[3]+",", ",ro,")
		result = append(result, volume)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read Linux storage volumes: %w", err)
	}
	return result, nil
}

func decodeLinuxMountPath(value string) string {
	return strings.NewReplacer(
		`\040`, " ",
		`\011`, "\t",
		`\012`, "\n",
		`\134`, `\`,
	).Replace(value)
}
