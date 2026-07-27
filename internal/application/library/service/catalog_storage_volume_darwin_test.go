//go:build darwin

package service

import (
	"testing"

	"golang.org/x/sys/unix"
)

func TestListCatalogStorageVolumesIncludesSystemVolume(t *testing.T) {
	items, err := listCatalogStorageVolumes()
	if err != nil {
		t.Fatalf("list storage volumes: %v", err)
	}
	for _, item := range items {
		if item.MountPath == "/" && item.ID != "" &&
			item.TotalBytes > 0 && item.AvailableBytes >= 0 {
			return
		}
	}
	t.Fatalf("system volume missing from inventory: %#v", items)
}

func TestVisibleDarwinCatalogStorageVolumeFiltersHiddenAPFSRoles(t *testing.T) {
	testCases := []struct {
		name      string
		mountPath string
		flags     uint32
		want      bool
	}{
		{name: "system root", mountPath: "/", flags: unix.MNT_LOCAL, want: true},
		{name: "external volume", mountPath: "/Volumes/Creator", flags: unix.MNT_LOCAL, want: true},
		{name: "network volume", mountPath: "/Volumes/Studio", want: true},
		{
			name: "recovery role", mountPath: "/Volumes/Recovery",
			flags: unix.MNT_LOCAL | unix.MNT_DONTBROWSE,
		},
		{name: "data role", mountPath: "/System/Volumes/Data", flags: unix.MNT_LOCAL},
		{name: "preboot role", mountPath: "/System/Volumes/Preboot", flags: unix.MNT_LOCAL},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := isVisibleDarwinCatalogStorageVolume(
				testCase.mountPath,
				testCase.flags,
			); got != testCase.want {
				t.Fatalf("visible = %v, want %v", got, testCase.want)
			}
		})
	}
}
