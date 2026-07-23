import { describe, expect, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";

import {
  formatPlaybackRateLabel,
  formatYouTubeQualityLabel,
  YouTubeWorkspaceTransportBar,
  youtubeVolumeEditorReducer,
  youtubeVolumeFromPointerDrag,
} from "@/app/youtube/YouTubeWorkspaceTransportBar";
import type { YouTubeWorkspacePlaybackState } from "@/app/youtube/types";

const playback: YouTubeWorkspacePlaybackState = {
  descriptor: {
    source: "youtube",
    mediaKind: "video",
    sessionId: "youtube-session",
    videoId: "AbCdEfGh123",
    title: "Workspace video",
    artist: "Creator",
    webUrl: "https://www.youtube.com/watch?v=AbCdEfGh123",
  },
  status: {
    state: "playing",
    currentTime: 25,
    duration: 100,
    playbackRateOptions: [
      { id: "0.5", label: "0.5x" },
      { id: "1", label: "1x" },
      { id: "1.5", label: "1.5x" },
      { id: "2", label: "2x" },
    ],
    selections: { playbackRateId: "1" },
  },
  currentIndex: 0,
  queue: [],
  muted: false,
  volume: 1,
  capabilities: {
    previous: false,
    next: false,
    playPause: true,
    like: false,
    dislike: false,
    fullscreen: true,
    captions: false,
    audioTrack: false,
    quality: false,
    volume: true,
    playbackRate: true,
  },
};

const labels = {
  player: "Now playing",
  previous: "Previous",
  play: "Play",
  pause: "Pause",
  next: "Next",
  fullscreen: "Fullscreen",
  exitFullscreen: "Exit fullscreen",
  captions: "Captions",
  audioTrack: "Audio track",
  quality: "Quality",
  danmaku: "Danmaku",
  playbackSpeed: "Playback speed",
  volume: "Volume",
  mute: "Mute",
  unmute: "Unmute",
  download: "Download",
  upNext: "Up Next",
  unavailable: "Unavailable",
  off: "Off",
};

const callbacks = {
  onPrevious: () => {},
  onTogglePlayback: () => {},
  onNext: () => {},
  onDownload: () => {},
  onToggleUpNext: () => {},
  onFullscreen: () => {},
  onToggleMute: () => {},
  onToggleCaptions: () => {},
  onSelectCaption: () => {},
  onSelectAudioTrack: () => {},
  onSelectQuality: () => {},
  onSelectPlaybackRate: () => {},
  onVolumeChange: () => {},
  onSeek: () => {},
};

describe("YouTubeWorkspaceTransportBar", () => {
  test("renders a time-first progress surface and the compact playback actions", () => {
    const markup = renderToStaticMarkup(
      <YouTubeWorkspaceTransportBar
        playback={playback}
        labels={labels}
        {...callbacks}
      />,
    );

    expect(markup).toContain('aria-label="Pause"');
    expect(markup).not.toContain('aria-label="Previous"');
    expect(markup).not.toContain('aria-label="Next"');
    expect(markup).toContain('aria-label="Download"');
    expect(markup).toContain('aria-label="Up Next"');
    expect(markup).toContain('aria-label="Volume"');
    expect(markup).toContain('aria-label="Playback speed: 1×"');
    expect(markup).toContain(">1×</span>");
    expect(markup).toContain('aria-label="Fullscreen"');
    expect(markup).not.toContain('aria-label="Danmaku"');
    expect(markup).toContain("youtube-workspace-transport-times");
    expect(markup).toContain("0:25");
    expect(markup).toContain("-1:15");
    expect(markup).toContain("width:25%");
    expect(markup).toMatch(
      /class="youtube-workspace-transport-timeline-input"[^>]*type="range"[^>]*min="0"[^>]*max="100"[^>]*step="1"[^>]*value="25"/,
    );
    expect(markup).not.toContain("youtube-workspace-transport-artwork");
    expect(markup).not.toContain("Workspace video</span>");
    expect(markup).not.toContain('aria-label="More"');
    expect(markup).not.toContain('aria-label="Like"');
    expect(markup).not.toContain('aria-label="Dislike"');
  });

  test("offers an explicit mute action inside the expanded volume editor", async () => {
    const source = await Bun.file(
      new URL("./YouTubeWorkspaceTransportBar.tsx", import.meta.url),
    ).text();

    expect(source).toContain("onClick={onToggleMute}");
    expect(source).toContain("playback.muted ? labels.unmute : labels.mute");
    expect(source).toContain("playback.muted ? <VolumeX /> : <Volume2 />");
  });

  test("keeps the shared transport active with an explicit fullscreen exit", () => {
    const markup = renderToStaticMarkup(
      <YouTubeWorkspaceTransportBar
        playback={playback}
        labels={labels}
        fullscreenActive
        {...callbacks}
      />,
    );

    expect(markup).toContain('aria-label="Exit fullscreen" aria-pressed="true"');
    expect(markup).toContain('data-active="true"');
  });

  test("accepts a focus target for modal fullscreen entry", async () => {
    const source = await Bun.file(
      new URL("./YouTubeWorkspaceTransportBar.tsx", import.meta.url),
    ).text();

    expect(source).toContain(
      "fullscreenButtonRef?: React.Ref<HTMLButtonElement>",
    );
    expect(source).toContain("buttonRef={fullscreenButtonRef}");
  });

  test("exposes active Up Next state in its new position", () => {
    const markup = renderToStaticMarkup(
      <YouTubeWorkspaceTransportBar
        playback={playback}
        labels={labels}
        upNextOpen
        {...callbacks}
      />,
    );

    expect(markup).toContain('aria-label="Up Next" aria-pressed="true"');
    expect(markup).toContain('data-active="true"');
  });

  test("exposes an indeterminate progressbar when duration is unknown", () => {
    const markup = renderToStaticMarkup(
      <YouTubeWorkspaceTransportBar
        playback={{
          ...playback,
          status: { ...playback.status, currentTime: 12, duration: 0 },
          descriptor: { ...playback.descriptor, durationSeconds: 0 },
        }}
        labels={labels}
        {...callbacks}
      />,
    );

    expect(markup).toContain('role="progressbar"');
    expect(markup).toContain('aria-valuemin="0"');
    expect(markup).not.toContain("aria-valuemax");
    expect(markup).not.toContain("aria-valuenow");
    expect(markup).not.toContain("youtube-workspace-transport-timeline-input");
    expect(markup).toContain("0:12");
    expect(markup).toContain("-0:00");
  });

  test("keeps captions and quality in independent controls", async () => {
    const source = await Bun.file(
      new URL("./YouTubeWorkspaceTransportBar.tsx", import.meta.url),
    ).text();

    expect(source).toContain('className="youtube-workspace-settings-menu"');
    expect(source).toContain("function CaptionsControl(");
    expect(source).toContain("function QualityControl(");
    expect(source).toContain("function AudioTrackControl(");
    expect(source).toContain("<DropdownMenuRadioGroup");
    expect(source).toContain("onValueChange={onSelectCaption}");
    expect(source).toContain("onValueChange={onSelectAudioTrack}");
    expect(source).toContain("onValueChange={onSelectQuality}");
    expect(source).toContain("<DropdownMenuRadioItem");
  });

  test("keeps the audio-track control visible while unavailable", () => {
    const markup = renderToStaticMarkup(
      <YouTubeWorkspaceTransportBar
        playback={playback}
        labels={labels}
        {...callbacks}
      />,
    );

    expect(markup).toMatch(/aria-label="Audio track"[^>]*disabled/);
  });

  test("disables captions when the current video has no caption tracks", () => {
    const markup = renderToStaticMarkup(
      <YouTubeWorkspaceTransportBar
        playback={{
          ...playback,
          capabilities: { ...playback.capabilities, captions: true },
          status: {
            ...playback.status,
            captionOptions: [],
            selections: { ...playback.status.selections, captionId: "" },
          },
        }}
        labels={labels}
        {...callbacks}
      />,
    );

    expect(markup).toMatch(/aria-label="Captions"[^>]*disabled/);
  });

  test("keeps captions selectable after the current track is switched Off", () => {
    const markup = renderToStaticMarkup(
      <YouTubeWorkspaceTransportBar
        playback={{
          ...playback,
          capabilities: { ...playback.capabilities, captions: true },
          status: {
            ...playback.status,
            captionOptions: [{ id: "en", label: "English" }],
            selections: { ...playback.status.selections, captionId: "" },
          },
        }}
        labels={labels}
        {...callbacks}
      />,
    );

    expect(markup).toMatch(
      /aria-label="Captions"[^>]*title="Captions"[^>]*data-active="false"/,
    );
    expect(markup).not.toMatch(/aria-label="Captions"[^>]*disabled/);
  });

  test("formats and exposes common playback rates", async () => {
    const source = await Bun.file(
      new URL("./YouTubeWorkspaceTransportBar.tsx", import.meta.url),
    ).text();

    expect(formatPlaybackRateLabel("0.25")).toBe("0.25×");
    expect(formatPlaybackRateLabel("1")).toBe("1×");
    expect(formatPlaybackRateLabel("1.5")).toBe("1.5×");
    expect(formatPlaybackRateLabel("2")).toBe("2×");
    expect(source).toContain("function PlaybackRateControl(");
    expect(source).toContain("onValueChange={onSelectPlaybackRate}");
  });

  test("formats native YouTube quality ids as professional display labels", () => {
    expect(
      formatYouTubeQualityLabel({ id: "hd2160", label: "hd2160" }),
    ).toBe("2160p · 4K");
    expect(
      formatYouTubeQualityLabel({ id: "hd1080", label: "hd1080" }),
    ).toBe("1080p · Full HD");
    expect(
      formatYouTubeQualityLabel({ id: "large", label: "large" }),
    ).toBe("480p · SD");
    expect(
      formatYouTubeQualityLabel({ id: "custom", label: "enhanced" }),
    ).toBe("Enhanced");
  });

  test("uses a click-to-expand volume editor that replaces adjacent actions", async () => {
    const [source, css] = await Promise.all([
      Bun.file(new URL("./YouTubeWorkspaceTransportBar.tsx", import.meta.url)).text(),
      Bun.file(new URL("./youtube-workspace.css", import.meta.url)).text(),
    ]);

    expect(source).toContain("const volumeExpanded = volumeEditor.expanded");
	expect(source).toContain("if (!capabilities.volume) {");
    expect(source).toContain('data-volume-dragging={volumeEditor.dragging ? "true" : "false"}');
    expect(source).not.toContain("setPointerCapture");
    expect(source).toContain("event.preventDefault()");
    expect(source).toContain("event.currentTarget.getBoundingClientRect().width");
    expect(source).toContain("pointerId: event.pointerId");
    expect(source).toContain("volumePointerActiveRef.current?.pointerId !== pointerId");
    expect(source).not.toContain("onPointerUp={finishVolumeDrag}");
    expect(source).not.toContain("onPointerCancel={finishVolumeDrag}");
    expect(source).toContain(
      'window.addEventListener("pointermove", updateOnPointerMove, true)',
    );
    expect(source).toContain(
      'window.addEventListener("pointerup", finishOnPointerRelease, true)',
    );
    expect(source).toContain(
      'window.addEventListener("pointercancel", finishOnPointerRelease, true)',
    );
    expect(source).toContain("if (!volumeKeyboardActiveRef.current) {");
    expect(source).toContain('dispatchVolumeEditor({ type: "open" })');
    expect(source).toContain('dispatchVolumeEditor({ type: "force-close" })');
    expect(source).toContain('document.addEventListener("pointerdown", closeOnOutsidePointer, true)');
    expect(source).toContain('data-volume-expanded={volumeExpanded ? "true" : "false"}');
    expect(source).toContain('className="youtube-workspace-volume-editor"');
    expect(css).toContain(
      '.youtube-workspace-transport-right[data-volume-expanded="true"]',
    );
    expect(css).toContain(".youtube-workspace-transport-right-actions");
    expect(css).toContain("pointer-events: none;");
  });

  test("keeps remote volume sync from moving the slider during a drag", () => {
    const initial = { draft: 0.4, dragging: false, expanded: false };
    const opened = youtubeVolumeEditorReducer(initial, { type: "open" });
    expect(opened).toEqual({ draft: 0.4, dragging: false, expanded: true });

    const dragging = youtubeVolumeEditorReducer(opened, { type: "drag-start" });
    const changed = youtubeVolumeEditorReducer(dragging, {
      type: "input",
      volume: 0.72,
    });

    expect(
      youtubeVolumeEditorReducer(changed, { type: "sync", volume: 0.2 }),
    ).toEqual({ draft: 0.72, dragging: true, expanded: true });
    expect(
      youtubeVolumeEditorReducer(changed, { type: "close" }),
    ).toBe(changed);
    expect(
      youtubeVolumeEditorReducer(changed, { type: "drag-release" }),
    ).toEqual({ draft: 0.72, dragging: false, expanded: true });
  });

  test("opens without changing volume and closes only outside an idle editor", () => {
    const initial = { draft: 0.36, dragging: false, expanded: false };
    const opened = youtubeVolumeEditorReducer(initial, { type: "open" });

    expect(opened.draft).toBe(initial.draft);
    expect(opened.expanded).toBe(true);
    expect(
      youtubeVolumeEditorReducer(opened, { type: "close" }),
    ).toEqual({ draft: 0.36, dragging: false, expanded: false });
  });

  test("changes pointer volume only from movement after the press", () => {
    expect(youtubeVolumeFromPointerDrag(0.4, 100, 100, 100)).toBe(0.4);
    expect(youtubeVolumeFromPointerDrag(0.4, 100, 120, 100)).toBe(0.6);
    expect(youtubeVolumeFromPointerDrag(0.4, 100, 80, 100)).toBe(0.2);
    expect(youtubeVolumeFromPointerDrag(0.9, 100, 140, 100)).toBe(1);
    expect(youtubeVolumeFromPointerDrag(0.1, 100, 60, 100)).toBe(0);
    expect(youtubeVolumeFromPointerDrag(0.36, 100, 140, 0)).toBe(0.36);
  });

  test("vertically centers the stable timeline group", async () => {
    const css = await Bun.file(
      new URL("./youtube-workspace.css", import.meta.url),
    ).text();
    const timelineRule =
      css.match(/\.youtube-workspace-transport-timeline \{([^}]*)\}/)?.[1] ?? "";

    expect(timelineRule).toContain("justify-content: center;");
    expect(timelineRule).not.toContain("justify-content: flex-end;");
  });

  test("keeps active footer controls visually separated at every breakpoint", async () => {
    const css = await Bun.file(
      new URL("./youtube-workspace.css", import.meta.url),
    ).text();
    const actionGroupRule =
      css.match(
        /\.youtube-workspace-transport-right-actions,\s*\.youtube-workspace-volume-editor\s*\{([^}]*)\}/,
      )?.[1] ?? "";
    const rightRule =
      css.match(/\.youtube-workspace-transport-right\s*\{([^}]*)\}/)?.[1] ?? "";

    expect(actionGroupRule).toContain("gap: 4px;");
    expect(rightRule).toContain("min-width: 288px;");
    expect(css).toMatch(
      /@container youtube-workspace \(max-width: 820px\)[\s\S]*?\.youtube-workspace-transport-right\s*\{[^}]*min-width: 252px;/,
    );
    expect(css).toMatch(
      /@container youtube-workspace \(max-width: 560px\)[\s\S]*?\.youtube-workspace-transport-right\s*\{[^}]*min-width: 240px;/,
    );
  });

  test("uses the Music transport material as a same-width floating card", async () => {
    const css = await Bun.file(
      new URL("./youtube-workspace.css", import.meta.url),
    ).text();
    const markup = renderToStaticMarkup(
      <YouTubeWorkspaceTransportBar
        playback={playback}
        labels={labels}
        {...callbacks}
      />,
    );
    const transportRule =
      css.match(/\.youtube-workspace-transport \{([^}]*)\}/)?.[1] ?? "";

    expect(markup).toContain(
      'class="app-glass-surface app-glass-group youtube-workspace-transport"',
    );
    expect(markup).toContain("<footer");
    expect(markup).toContain('data-material="regular"');
    expect(markup).toContain('data-elevation="floating"');
    expect(markup).toContain('data-shape="card"');
    expect(transportRule).toContain("position: relative;");
    expect(transportRule).toContain(
      "margin: 0 var(--youtube-watch-horizontal-gap)",
    );
    expect(transportRule).not.toContain("border-radius: 0");
    expect(transportRule).not.toContain("background:");
  });
});
