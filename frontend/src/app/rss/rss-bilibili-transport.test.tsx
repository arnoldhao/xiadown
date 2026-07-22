import { describe, expect, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";

import { YouTubeWorkspaceTransportBar } from "@/app/youtube/YouTubeWorkspaceTransportBar";

import type { RSSBilibiliPlayerStatus } from "./api";
import {
  createBilibiliTransportPlayback,
  RSS_BILIBILI_TRANSPORT_CONTROLS,
} from "./video-transport";

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

const unavailableStatus: RSSBilibiliPlayerStatus = {
  provider: "bilibili",
  sessionId: "",
  available: false,
  platformVideoId: "BV1xx411c7mD",
  state: "error",
  title: "Unavailable Bilibili video",
  currentTime: 0,
  duration: 0,
  bufferedTime: 0,
  volume: 1,
  muted: false,
  playbackRate: 1,
  fullscreen: false,
  danmakuEnabled: false,
  controls: {
    playPause: false,
    seek: false,
    volume: false,
    playbackRate: false,
    fullscreen: false,
    captions: false,
    quality: false,
    danmaku: false,
  },
  captionOptions: [],
  qualityOptions: [],
  playbackRateOptions: [],
  selections: { playbackRateId: "1" },
};

const noop = () => {};

describe("RSS Bilibili fallback transport", () => {
  test("keeps every Bilibili control visible while bridge-only actions are disabled", () => {
    const playback = createBilibiliTransportPlayback(
      null,
      unavailableStatus,
      { title: "Unavailable Bilibili video", videoDurationSeconds: 0 },
    );
    const markup = renderToStaticMarkup(
      <YouTubeWorkspaceTransportBar
        playback={playback}
        labels={labels}
        visibleControls={RSS_BILIBILI_TRANSPORT_CONTROLS}
        onPrevious={noop}
        onTogglePlayback={noop}
        onNext={noop}
        onDownload={noop}
        onFullscreen={noop}
        onToggleMute={noop}
        onToggleCaptions={noop}
        onSelectCaption={noop}
        onSelectAudioTrack={noop}
        onSelectQuality={noop}
        onToggleDanmaku={noop}
        onSelectPlaybackRate={noop}
        onVolumeChange={noop}
        onSeek={noop}
      />,
    );

    expect(markup).toMatch(/class="[^"]*youtube-workspace-transport[^"]*"/);
    for (const label of [
      "Play",
      "Download",
      "Playback speed: 1×",
      "Captions",
      "Quality",
      "Danmaku",
      "Volume",
      "Fullscreen",
    ]) {
      expect(markup).toContain(`aria-label="${label}"`);
    }
    for (const label of [
      "Play",
      "Playback speed: 1×",
      "Captions",
      "Quality",
      "Danmaku",
      "Volume",
      "Fullscreen",
    ]) {
      expect(markup).toMatch(new RegExp(`aria-label="${label}"[^>]*disabled`));
    }
    expect(markup).not.toMatch(/aria-label="Download"[^>]*disabled/);
    expect(markup).not.toContain('aria-label="Audio track"');
    expect(markup).not.toContain('aria-label="Up Next"');

    const labelsInOrder = [
      "Download",
      "Playback speed: 1×",
      "Captions",
      "Quality",
      "Danmaku",
      "Volume",
      "Fullscreen",
    ];
    const positions = labelsInOrder.map((label) =>
      markup.indexOf(`aria-label="${label}"`));
    expect(positions.every((position) => position >= 0)).toBeTrue();
    expect([...positions].sort((left, right) => left - right)).toEqual(positions);
  });

  test("shares the YouTube footer's right-edge geometry", async () => {
    const css = await Bun.file(
      new URL("../youtube/youtube-workspace.css", import.meta.url),
    ).text();
    const rssCSS = await Bun.file(
      new URL("./rss-workspace.css", import.meta.url),
    ).text();

    expect(css).toMatch(
      /\.youtube-workspace-transport-right \{[^}]*min-width: 288px;[^}]*justify-content: flex-end;/,
    );
    expect(css).toMatch(
      /\.youtube-workspace-transport-right-actions,[^}]*justify-content: flex-end;/,
    );
    expect(rssCSS).toMatch(
      /\.rss-video-watch-page \.youtube-workspace-transport-action-download \{[^}]*display: inline-grid !important;/,
    );
    expect(rssCSS).not.toMatch(
      /\.rss-video-watch-page \.youtube-workspace-transport-action-download \{[^}]*display: inline-flex !important;/,
    );
  });
});
