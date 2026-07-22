import { describe, expect, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";

import type { ListenNowPlayingStatus } from "./Listen";
import {
  MusicWorkspaceTransportBar,
  resolveWorkspaceTransportArtistParts,
  resolveWorkspaceTransportMenuArtists,
  resolveWorkspaceTransportStatus,
  transportVolumeEditorReducer,
  type WorkspaceTransportLabels,
} from "./WorkspaceTransportBar";

const labels: WorkspaceTransportLabels = {
  idleStatus: "Not Playing",
  idleSubtitle: "Nothing is playing.",
  shuffle: "Shuffle",
  previous: "Previous",
  play: "Play",
  pause: "Pause",
  next: "Next",
  repeatOne: "Repeat one",
  live: "Live",
  lyrics: "Lyrics",
  upNext: "Up next",
  volume: "Volume",
  fullscreen: "Fullscreen",
  more: "More",
  favorite: "Favorite",
  download: "Download",
  openURL: "Open URL",
};

const status: ListenNowPlayingStatus = {
  state: "playing",
  title: "Long track title",
  subtitle: "Artist",
  artworkURL: "https://example.com/cover.jpg",
  mode: "muse",
  playbackSource: "youtube_music",
  playbackSourceLabel: "YouTube Music",
  canControl: true,
  favoriteActive: false,
  progress: { currentTime: 35, duration: 100, bufferedTime: 60 },
};

const callbacks = {
  onCommand: () => undefined,
  onOpenArtist: () => undefined,
  onOpenPlayer: () => undefined,
  onOpenLyrics: () => undefined,
  onOpenQueue: () => undefined,
  onFullscreen: () => undefined,
};

describe("MusicWorkspaceTransportBar", () => {
  test("keeps the floating transport mounted before playback starts", () => {
    const markup = renderToStaticMarkup(
      <MusicWorkspaceTransportBar
        {...callbacks}
        status={null}
        labels={labels}
      />,
    );
    const idleTimeline = markup.match(
      /<div class="app-workspace-transport__timeline"[^>]*>/,
    )?.[0];

    expect(markup).toContain("app-workspace-transport");
    expect(markup).toContain('data-state="idle"');
    expect(markup).toContain(">Not Playing<");
    expect(markup).toContain(">Nothing is playing.<");
    expect(idleTimeline).toContain('aria-hidden="true"');
    expect(idleTimeline).not.toContain('role="progressbar"');
    expect(idleTimeline).not.toContain("aria-valuemin");
    expect(idleTimeline).not.toContain("aria-valuemax");
    expect(idleTimeline).not.toContain("aria-valuenow");
    expect(markup).toContain(">0:00<");
    expect(markup).toContain('style="width:0%"');
    expect(markup).toMatch(
      /<button[^>]*aria-label="Play"[^>]*disabled=""/,
    );
  });

  test("projects a previous Live track into a clean idle transport", () => {
    const previousStatus: ListenNowPlayingStatus = {
      ...status,
      state: "idle",
      live: true,
      mode: "hush",
      title: "Previous live track",
      subtitle: "Previous artist",
      artists: [{ name: "Previous artist", browseId: "UCprevious" }],
      playbackSourceLabel: "Previous source",
      sourceURL: "https://example.com/previous",
      favoriteActive: true,
      canFavorite: true,
      canPrevious: true,
      canNext: true,
      muted: true,
      volume: 0.75,
    };
    const projected = resolveWorkspaceTransportStatus(previousStatus, labels);
    const markup = renderToStaticMarkup(
      <MusicWorkspaceTransportBar
        {...callbacks}
        status={previousStatus}
        labels={labels}
        muted={previousStatus.muted}
        volume={previousStatus.volume}
        onDownload={() => undefined}
        onFavorite={() => undefined}
        onOpenURL={() => undefined}
        onToggleMute={() => undefined}
        onVolumeChange={() => undefined}
      />,
    );

    expect(projected).toEqual({
      state: "idle",
      live: false,
      mediaId: "",
      title: "Not Playing",
      subtitle: "Nothing is playing.",
      artists: [],
      artworkURL: "",
      artworkCandidates: [],
      playbackSource: "unknown",
      mode: "muse",
      canControl: false,
      progress: { currentTime: 0, duration: 0, bufferedTime: 0 },
    });
    expect(markup).toContain('data-state="idle"');
    expect(markup).toContain('data-live="false"');
    expect(markup).toContain(">Not Playing<");
    expect(markup).toContain(">Nothing is playing.<");
    expect(markup).not.toContain("Previous live track");
    expect(markup).not.toContain("Previous artist");
    expect(markup).not.toContain("Previous source");
    expect(markup).not.toContain("app-workspace-transport__track-artist-open");
    expect(markup).not.toContain("app-workspace-transport__favorite");
    expect(markup).not.toContain("Live · -0:00");
    expect(markup).toMatch(
      /<button[^>]*aria-label="Volume"[^>]*disabled=""/,
    );
  });

  test("renders the shared floating capsule surface contract", () => {
    const markup = renderToStaticMarkup(
      <MusicWorkspaceTransportBar
        {...callbacks}
        status={status}
        labels={labels}
      />,
    );

    expect(markup).toContain(
      'class="app-glass-surface app-glass-group app-workspace-transport"',
    );
    expect(markup).toContain('data-material="regular"');
    expect(markup).toContain('data-elevation="floating"');
    expect(markup).toContain('data-shape="capsule"');
  });

  test("keeps loading playback actionable as pause and shows a spinner", () => {
    const markup = renderToStaticMarkup(
      <MusicWorkspaceTransportBar
        {...callbacks}
        status={{ ...status, state: "loading" }}
        labels={labels}
      />,
    );
    const primaryButton = markup.match(
      /<button[^>]*data-transport-emphasis="primary"[^>]*>.*?<\/button>/,
    )?.[0];

    expect(markup).toContain('data-state="loading"');
    expect(primaryButton).toContain('aria-label="Pause"');
    expect(primaryButton).toContain('title="Pause"');
    expect(primaryButton).toContain("lucide-loader-circle");
    expect(primaryButton).toContain("listen-loading-spinner");
    expect(primaryButton).not.toContain('disabled=""');
    expect(primaryButton).not.toContain("lucide-play");
    expect(primaryButton).not.toContain("lucide-pause");
  });

  test("keeps title passive, artist actionable, and favorite as a sibling", () => {
    const markup = renderToStaticMarkup(
      <MusicWorkspaceTransportBar
        {...callbacks}
        status={status}
        labels={labels}
        onFavorite={() => undefined}
      />,
    );

    expect(markup).toMatch(
      /<button[^>]*app-workspace-transport__favorite[^>]*data-active="false"[^>]*aria-label="Favorite"[^>]*aria-pressed="false"/,
    );
    expect(markup).toContain('class="app-workspace-transport__track-title"');
    expect(markup).not.toContain("app-workspace-transport__track-title-open");
    expect(markup).toMatch(
      /<button[^>]*app-workspace-transport__track-artist-open[^>]*aria-label="Artist"/,
    );
    expect(markup).not.toMatch(
      /<button[^>]*app-workspace-transport__track-title[^>]*>/,
    );
  });

  test("renders structured artists as independent navigation targets", () => {
    const multiArtistStatus: ListenNowPlayingStatus = {
      ...status,
      subtitle: "AGA, Gin Lee",
      artists: [
        { name: "AGA", browseId: "UCaga" },
        { name: "Gin Lee", browseId: "UCgin" },
      ],
    };
    const parts = resolveWorkspaceTransportArtistParts(multiArtistStatus);
    const markup = renderToStaticMarkup(
      <MusicWorkspaceTransportBar
        {...callbacks}
        status={multiArtistStatus}
        labels={labels}
      />,
    );

    expect(parts).toEqual([
      { kind: "artist", artist: { name: "AGA", browseId: "UCaga" } },
      { kind: "separator", text: ", " },
      { kind: "artist", artist: { name: "Gin Lee", browseId: "UCgin" } },
    ]);
    expect(markup).toMatch(
      /<button[^>]*app-workspace-transport__track-artist-open[^>]*aria-label="AGA"/,
    );
    expect(markup).toMatch(
      /<button[^>]*app-workspace-transport__track-artist-open[^>]*aria-label="Gin Lee"/,
    );
    expect(markup).not.toMatch(
      /<button[^>]*app-workspace-transport__track-artist-open[^>]*aria-label="AGA, Gin Lee"/,
    );
  });

  test("limits the More menu artist shortcuts to three structured artists", () => {
    const parts = resolveWorkspaceTransportArtistParts({
      subtitle: "One, Two, Three, Four",
      artists: [
        { name: "One", browseId: "UC1" },
        { name: "Two", browseId: "UC2" },
        { name: "Three", browseId: "UC3" },
        { name: "Four", browseId: "UC4" },
      ],
    });

    expect(resolveWorkspaceTransportMenuArtists(parts)).toEqual([
      { name: "One", browseId: "UC1" },
      { name: "Two", browseId: "UC2" },
      { name: "Three", browseId: "UC3" },
    ]);
  });

  test("splits an unstructured multi-artist subtitle without inventing browse ids", () => {
    expect(
      resolveWorkspaceTransportArtistParts({
        subtitle: "Artist A feat. Artist B",
      }),
    ).toEqual([
      { kind: "artist", artist: { name: "Artist A" } },
      { kind: "separator", text: " feat. " },
      { kind: "artist", artist: { name: "Artist B" } },
    ]);
  });

  test("uses the standard full timeline with a zero remaining time for Live", () => {
    const markup = renderToStaticMarkup(
      <MusicWorkspaceTransportBar
        {...callbacks}
        status={{
          ...status,
          live: true,
          mode: "muse",
          progress: { currentTime: 180, duration: 240, bufferedTime: 240 },
        }}
        labels={labels}
      />,
    );

    expect(markup).toContain('role="progressbar"');
    expect(markup).toContain('data-live="true"');
    expect(markup).toContain(
      `aria-label="Long track title · ${labels.live}"`,
    );
    expect(markup).toContain('aria-valuemax="100"');
    expect(markup).toContain('aria-valuenow="100"');
    expect(markup).toContain(`aria-valuetext="${labels.live} · -0:00"`);
    expect(markup).toContain('style="width:100%"');
    expect(markup).toContain(">-0:00</span>");
    expect(markup).not.toContain("app-workspace-transport__timeline--live");
    expect(markup).not.toContain("app-workspace-transport__timeline-input");
    expect(markup).not.toContain('tabindex="0"');
  });

  test("uses a native seek slider for finite controllable playback", async () => {
    const markup = renderToStaticMarkup(
      <MusicWorkspaceTransportBar
        {...callbacks}
        status={status}
        labels={labels}
      />,
    );
    const source = await Bun.file(
      new URL("./WorkspaceTransportBar.tsx", import.meta.url),
    ).text();

    expect(markup).toMatch(
      /class="app-workspace-transport__timeline-input"[^>]*type="range"[^>]*min="0"[^>]*max="100"[^>]*step="1"[^>]*value="35"/,
    );
    expect(markup).not.toContain('role="progressbar"');
    expect(source).toContain('props.onCommand("seek", Number(event.currentTarget.value))');
  });

  test("exposes inline mute and volume controls only when fully configured", () => {
    const configured = renderToStaticMarkup(
      <MusicWorkspaceTransportBar
        {...callbacks}
        status={status}
        labels={labels}
        volume={0.35}
        muted={false}
        onToggleMute={() => undefined}
        onVolumeChange={() => undefined}
      />,
    );
    const fallback = renderToStaticMarkup(
      <MusicWorkspaceTransportBar
        {...callbacks}
        status={status}
        labels={labels}
      />,
    );

    expect(configured).toContain('class="app-workspace-transport__volume-editor"');
    expect(configured).toContain('type="range"');
    expect(configured).toContain('value="35"');
    expect(configured).toContain('aria-valuetext="35%"');
    expect(fallback).not.toContain(
      'class="app-workspace-transport__volume-editor"',
    );
    expect(fallback).toContain(
      'class="app-workspace-transport__timeline-input"',
    );
  });

  test("uses Apple-style lyrics and queue symbols", () => {
    const markup = renderToStaticMarkup(
      <MusicWorkspaceTransportBar
        {...callbacks}
        status={status}
        labels={labels}
      />,
    );

    expect(markup).toContain("lucide-message-square-quote");
    expect(markup).toContain("lucide-list-end");
    expect(markup).not.toContain("lucide-text-quote");
    expect(markup).not.toContain("lucide-list-music");
  });

  test("places a direct download action immediately before lyrics", () => {
    const markup = renderToStaticMarkup(
      <MusicWorkspaceTransportBar
        {...callbacks}
        status={status}
        labels={labels}
        onDownload={() => undefined}
      />,
    );
    const downloadButton = markup.indexOf('aria-label="Download"');
    const lyricsButton = markup.indexOf('aria-label="Lyrics"');

    expect(downloadButton).toBeGreaterThan(-1);
    expect(lyricsButton).toBeGreaterThan(downloadButton);
    expect(markup.slice(downloadButton, lyricsButton)).toContain(
      "lucide-download",
    );
  });

  test("holds external volume updates while dragging and accepts them after release", () => {
    const initial = { draft: 0.35, dragging: false };
    const dragging = transportVolumeEditorReducer(initial, {
      type: "drag-start",
    });
    const ignoredSync = transportVolumeEditorReducer(dragging, {
      type: "sync",
      volume: 0.8,
    });
    const input = transportVolumeEditorReducer(ignoredSync, {
      type: "input",
      volume: 0.52,
    });
    const released = transportVolumeEditorReducer(input, { type: "drag-end" });
    const synced = transportVolumeEditorReducer(released, {
      type: "sync",
      volume: 0.6,
    });

    expect(dragging).toEqual({ draft: 0.35, dragging: true });
    expect(ignoredSync).toBe(dragging);
    expect(input).toEqual({ draft: 0.52, dragging: true });
    expect(released).toEqual({ draft: 0.52, dragging: false });
    expect(synced).toEqual({ draft: 0.6, dragging: false });
  });

  test("wires real-time volume input and explicit-only dismissal", async () => {
    const source = await Bun.file(
      new URL("./WorkspaceTransportBar.tsx", import.meta.url),
    ).text();

    expect(source).toContain("onInput={(event) => {");
    expect(source).toContain('document.addEventListener("pointerdown"');
    expect(source).toContain('event.key === "Escape"');
    expect(source).not.toContain("onBlur={(event)");
    expect(source).toContain('data-volume-dragging={volumeEditor.dragging ? "true" : "false"}');
    expect(source).toContain("onOpenArtist(part.artist)");
    expect(source).toContain("onOpenArtist(artist)");
    expect(source).toContain('className="app-workspace-transport__more-menu"');
    expect(source).toContain("<Download");
    expect(source).toContain("<ExternalLink");
    expect(source).toContain("<UserRound");
  });
});
