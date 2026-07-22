package wails

import "testing"

func TestListenEmbeddedVideoRevealReadyUsesNativeMountAsAuthority(t *testing.T) {
	tests := []struct {
		name         string
		nativeShown  bool
		resizeReady  bool
		ownerActive  bool
		wantRevealed bool
	}{
		{
			name:         "resize acknowledgement received",
			nativeShown:  true,
			resizeReady:  true,
			ownerActive:  true,
			wantRevealed: true,
		},
		{
			name:         "delayed resize acknowledgement does not hide mounted video",
			nativeShown:  true,
			resizeReady:  false,
			ownerActive:  true,
			wantRevealed: true,
		},
		{
			name:         "native mount failed",
			nativeShown:  false,
			resizeReady:  true,
			ownerActive:  true,
			wantRevealed: false,
		},
		{
			name:         "another player owns the surface",
			nativeShown:  true,
			resizeReady:  true,
			ownerActive:  false,
			wantRevealed: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := listenEmbeddedVideoRevealReady(
				test.nativeShown,
				test.resizeReady,
				test.ownerActive,
			)
			if got != test.wantRevealed {
				t.Fatalf("listenEmbeddedVideoRevealReady() = %v, want %v", got, test.wantRevealed)
			}
		})
	}
}

func TestListenNativeWindowFeatureGuardIsIdempotentAndSupportsRollback(t *testing.T) {
	guard := &listenNativeWindowFeatureGuard{}
	if !guard.Claim(42) {
		t.Fatal("first feature installation claim was rejected")
	}
	if guard.Claim(42) {
		t.Fatal("duplicate feature installation claim was accepted")
	}
	if guard.Claim(0) {
		t.Fatal("zero window ID must not be claimed")
	}
	guard.Release(42)
	if !guard.Claim(42) {
		t.Fatal("released feature installation could not be retried")
	}
}

func TestNormalizeListenEmbeddedVideoRectPreservesHostFullscreenGeometry(t *testing.T) {
	rect := normalizeListenEmbeddedVideoRect(ListenEmbeddedVideoRect{
		X:              18,
		Y:              18,
		Width:          2524,
		Height:         1328,
		CenterX:        1280,
		CenterY:        682,
		ViewportWidth:  2560,
		ViewportHeight: 1440,
		Radius:         14,
		Interactive:    false,
		Sequence:       42,
	})
	if rect.X != 18 || rect.Y != 18 || rect.Width != 2524 || rect.Height != 1328 {
		t.Fatalf("host fullscreen frame changed during normalization: %#v", rect)
	}
	if rect.CenterX != 1280 || rect.CenterY != 682 ||
		rect.ViewportWidth != 2560 || rect.ViewportHeight != 1440 {
		t.Fatalf("host viewport geometry changed during normalization: %#v", rect)
	}
	if rect.Radius != 14 || rect.Interactive || rect.Sequence != 42 {
		t.Fatalf("rounded non-interactive surface contract changed: %#v", rect)
	}
}
