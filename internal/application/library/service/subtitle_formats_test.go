package service

import "testing"

func TestParseFCPXMLSubtitleCuesReadsNestedTitles(t *testing.T) {
	t.Parallel()

	content := `<?xml version="1.0" encoding="UTF-8"?>
<fcpxml version="1.10">
  <library>
    <event name="Fixture Event">
      <project name="Fixture Project">
        <sequence>
          <spine>
            <title start="0s" duration="2s" name="First cue" />
            <title start="2s" duration="3s"><text><text-style>Second cue</text-style></text></title>
          </spine>
        </sequence>
      </project>
    </event>
  </library>
</fcpxml>`

	cues := parseFCPXMLSubtitleCues(content)
	if len(cues) != 2 {
		t.Fatalf("expected 2 cues, got %d", len(cues))
	}
	if cues[0].Start != "0s" || cues[0].End != "2s" || cues[0].Text != "First cue" {
		t.Fatalf("unexpected first cue: %#v", cues[0])
	}
	if cues[1].Start != "2s" || cues[1].End != "3s" || cues[1].Text != "Second cue" {
		t.Fatalf("unexpected second cue: %#v", cues[1])
	}
}
