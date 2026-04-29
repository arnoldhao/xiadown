//go:build darwin

package wails

import "testing"

func TestParseDarwinDSCLHexValue(t *testing.T) {
	output := "GeneratedUID: test-guid\nJPEGPhoto:\n 89504e47 0d0a1a0a\n 0000000d\nRealName: Test User\n"

	got := parseDarwinDSCLHexValue(output, "JPEGPhoto")
	want := []byte{
		0x89, 0x50, 0x4e, 0x47,
		0x0d, 0x0a, 0x1a, 0x0a,
		0x00, 0x00, 0x00, 0x0d,
	}
	if len(got) != len(want) {
		t.Fatalf("expected %d bytes, got %d", len(want), len(got))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("byte %d mismatch: expected %x, got %x", i, want[i], got[i])
		}
	}
}

func TestResolveDarwinCurrentUserAvatarUsesJPEGPhoto(t *testing.T) {
	output := "GeneratedUID: test-guid\nJPEGPhoto:\n 89504e47 0d0a1a0a 0000000d 49484452\n 00000001 00000001 08040000 00b51c0c\n 02000000 0b494441 5478da63 fcff1f00\n 03030200 efa93e2a 00000000 49454e44\n ae426082\n"

	avatar := resolveDarwinCurrentUserAvatar(output)
	if avatar.Base64 == "" {
		t.Fatal("expected base64 avatar from JPEGPhoto")
	}
	if avatar.Mime == "" {
		t.Fatal("expected non-empty avatar mime from JPEGPhoto")
	}
	if avatar.Path != "" {
		t.Fatalf("expected empty avatar path, got %q", avatar.Path)
	}
}
