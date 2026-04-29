package service

import "testing"

func TestParseProgressSpeedMetric(t *testing.T) {
	t.Run("download byte rate", func(t *testing.T) {
		metric := parseProgressSpeedMetric("download", "18.4 MiB/s")
		if metric == nil {
			t.Fatalf("expected metric")
		}
		if metric.Kind != speedMetricKindBytesPerSecond {
			t.Fatalf("unexpected kind: %s", metric.Kind)
		}
		if metric.BytesPerSecond == nil || int(*metric.BytesPerSecond) != 19293798 {
			t.Fatalf("unexpected bytes per second: %#v", metric.BytesPerSecond)
		}
	})

	t.Run("transcode fps", func(t *testing.T) {
		metric := parseProgressSpeedMetric("transcode", "92 fps")
		if metric == nil {
			t.Fatalf("expected metric")
		}
		if metric.Kind != speedMetricKindFramesPerSecond {
			t.Fatalf("unexpected kind: %s", metric.Kind)
		}
		if metric.FramesPerSecond == nil || *metric.FramesPerSecond != 92 {
			t.Fatalf("unexpected fps: %#v", metric.FramesPerSecond)
		}
	})

	t.Run("transcode speed factor", func(t *testing.T) {
		metric := parseProgressSpeedMetric("transcode", "1.75x")
		if metric == nil {
			t.Fatalf("expected metric")
		}
		if metric.Kind != speedMetricKindFactor {
			t.Fatalf("unexpected kind: %s", metric.Kind)
		}
		if metric.Factor == nil || *metric.Factor != 1.75 {
			t.Fatalf("unexpected factor: %#v", metric.Factor)
		}
	})
}
