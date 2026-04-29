package app

import "testing"

func TestCurrentStartupContext(t *testing.T) {
	tests := []struct {
		name              string
		args              []string
		expectedAutoStart bool
		expectedSkip      bool
	}{
		{name: "no marker", args: []string{"--verbose"}},
		{name: "exact marker", args: []string{"--autostart"}, expectedAutoStart: true},
		{name: "marker with spaces and mixed case", args: []string{"  --AutoStart  "}, expectedAutoStart: true},
		{name: "skip prepared update marker", args: []string{"--skip-prepared-update-once"}, expectedSkip: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := currentStartupContext(tt.args)
			if got.launchedByAutoStart != tt.expectedAutoStart {
				t.Fatalf("launchedByAutoStart = %v, want %v", got.launchedByAutoStart, tt.expectedAutoStart)
			}
			if got.skipPreparedUpdate != tt.expectedSkip {
				t.Fatalf("skipPreparedUpdate = %v, want %v", got.skipPreparedUpdate, tt.expectedSkip)
			}
		})
	}
}
