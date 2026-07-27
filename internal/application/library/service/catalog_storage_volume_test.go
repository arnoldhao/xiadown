package service

import "testing"

func TestCatalogStorageVolumeDTOsDeduplicateAndKeepSystemFirst(t *testing.T) {
	items := []catalogStorageVolume{
		{
			ID: "external", Name: "Creator", MountPath: "/Volumes/Creator",
			TotalBytes: 2_000, AvailableBytes: 500,
		},
		{
			ID: "system", MountPath: "/", TotalBytes: 1_000,
			AvailableBytes: 400,
		},
		{
			ID: "external", Name: "Nested", MountPath: "/Volumes/Creator/Nested",
			TotalBytes: 2_000, AvailableBytes: 600,
		},
	}

	result := catalogStorageVolumeDTOs(items)
	if len(result) != 2 {
		t.Fatalf("volume count = %d, want 2: %#v", len(result), result)
	}
	if result[0].ID != "system" || result[1].ID != "external" {
		t.Fatalf("volumes were not ordered with system first: %#v", result)
	}
	if result[1].MountPath != "/Volumes/Creator" ||
		result[1].AvailableBytes != 500 {
		t.Fatalf("duplicate volume did not preserve the shortest mount: %#v", result[1])
	}
}

func TestCatalogStorageVolumeDTOsClampCapacity(t *testing.T) {
	result := catalogStorageVolumeDTOs([]catalogStorageVolume{{
		ID: "volume", MountPath: "/volume", TotalBytes: 100, AvailableBytes: 150,
	}})
	if len(result) != 1 || result[0].AvailableBytes != 100 {
		t.Fatalf("available capacity was not clamped: %#v", result)
	}
}
