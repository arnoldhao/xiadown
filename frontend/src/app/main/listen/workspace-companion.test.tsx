import { describe, expect, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";

import { getXiaText } from "@/features/xiadown/shared";
import type { ListenOnlineItem } from "@/app/main/listen/types";

import {
  ListenWorkspaceLocalQueueCompanion,
  ListenWorkspaceLyricsCompanion,
  ListenWorkspaceOnlineQueueCompanion,
  ListenWorkspaceQueueModeSwitch,
} from "./workspace-companion";

async function readWorkspaceCss() {
  const [layout, appearance] = await Promise.all([
    Bun.file(new URL("../../workspace/workspace.css", import.meta.url)).text(),
    Bun.file(
      new URL("../../../shared/styles/dream/workspace.css", import.meta.url),
    ).text(),
  ]);
  return `${layout}\n${appearance}`;
}

function onlineItem(
  id: string,
  title: string,
  channel: string,
): ListenOnlineItem {
  return {
    id,
    group: "playlist",
    videoId: `${id}-video`,
    title,
    channel,
    description: "",
    durationLabel: "3:21",
  };
}

describe("music workspace companion surfaces", () => {
  test("dedicated Lyrics keeps only its three-control center slot", async () => {
    const text = getXiaText("en");
    const appearance = await Bun.file(
      new URL("../../../shared/styles/dream/listen.css", import.meta.url),
    ).text();
    const markup = renderToStaticMarkup(
      <ListenWorkspaceLyricsCompanion
        artworkCandidates={["https://example.test/art.jpg"]}
        title={text.listen.nowPlaying}
        text={text}
        source="youtube_music"
        lyricsControls={<div data-testid="lyrics-controls" />}
        onOpenSource={() => undefined}
        onRequestFullscreen={() => undefined}
      >
        <div data-testid="lyrics-lines">{text.listen.lyrics}</div>
      </ListenWorkspaceLyricsCompanion>,
    );

    expect(markup).toContain('data-listen-companion-mode="lyrics"');
    expect(markup).toContain('data-testid="lyrics-lines"');
    expect(markup).toContain(text.listen.lyrics);
    expect(markup).not.toContain(text.listen.nowPlaying);
    expect(markup).toContain("listen-workspace-lyrics-content");
    expect(markup).not.toContain('data-listen-companion-title-safe-area="true"');
    expect(markup).toContain("flex-1");
    expect(markup).toContain("min-h-0");
    expect(markup).not.toContain("bg-transparent");
    expect(appearance).toMatch(
      /\.listen-workspace-companion-player,\s*\.listen-workspace-queue-surface\s*\{[^}]*background:\s*transparent;/s,
    );
    expect(markup).toContain("listen-player-footer__bar--companion");
    expect(markup).toContain("listen-player-footer__center-context");
    expect(markup).toContain('data-footer-region="dynamic"');
    expect(markup.match(/data-testid="lyrics-controls"/g)).toHaveLength(1);
    expect(markup).not.toContain(`aria-label="${text.workspace.youtubeMusic}"`);
    expect(markup).not.toContain(
      `aria-label="${text.completed.previewEnterFullscreen}"`,
    );
    expect(markup).not.toContain(`aria-label="${text.listen.lyrics}"`);
    expect(markup).not.toContain(`aria-label="${text.listen.upNext}"`);
    expect(markup).not.toContain("lucide-fullscreen");
    expect(markup).not.toContain("https://example.test/art.jpg");
  });

  test("queue presentation projects live queue data and real edit controls", () => {
    const text = getXiaText("en");
    const markup = renderToStaticMarkup(
      <ListenWorkspaceOnlineQueueCompanion
        queueTitle={text.listen.nowPlaying}
        queueItems={[
          onlineItem("one", "First track", "Artist One"),
          onlineItem("two", "Second track", "Artist Two"),
        ]}
        selectedQueueId="one"
        httpBaseURL="http://127.0.0.1:34115"
        playMode="shuffle"
        text={text}
        queueCanUndo
        queueCanRedo={false}
        onPlayModeChange={() => undefined}
        onClearQueue={() => undefined}
        onRemoveQueueItem={() => undefined}
        onMoveQueueItem={() => undefined}
        onUndoQueueEdit={() => undefined}
        onRedoQueueEdit={() => undefined}
        onSelectQueueTrack={() => undefined}
      />,
    );

    expect(markup).toContain('data-listen-companion-mode="queue"');
    expect(markup).toContain("First track");
    expect(markup).toContain("Second track");
    expect(markup).toContain('data-selected="true"');
    expect(markup).toContain('data-active="true"');
    expect(markup).toContain('role="radiogroup"');
    expect(markup.match(/role="radio"/g)).toHaveLength(3);
    expect(markup).toContain(text.actions.clear);
    expect(markup).toContain("listen-workspace-queue-content");
    expect(markup).toContain('data-companion-scroll-owner="queue"');
    expect(markup).toContain("listen-workspace-queue-footer");
    expect(markup).toContain("justify-between");
    expect(markup).toContain("listen-workspace-queue-footer__actions");
    expect(markup).not.toContain("border-t");
    expect(markup).not.toContain("border-sidebar-foreground/[0.08]");
    expect(markup).not.toContain("listen-workspace-queue-toolbar");
    expect(markup.indexOf("listen-workspace-queue-content")).toBeLessThan(
      markup.indexOf("listen-workspace-queue-footer"),
    );
    expect(markup).not.toContain(text.listen.nowPlaying);
    expect(markup).not.toContain("listen-single-lyrics-panel");
  });

  test("keeps embedded Queue content while its host owns the footer", () => {
    const text = getXiaText("en");
    const markup = renderToStaticMarkup(
      <ListenWorkspaceOnlineQueueCompanion
        queueTitle={text.listen.upNext}
        queueItems={[onlineItem("embedded", "Embedded track", "Artist")]}
        selectedQueueId="embedded"
        httpBaseURL="http://127.0.0.1:34115"
        playMode="repeat"
        text={text}
        showFooter={false}
        onPlayModeChange={() => undefined}
        onClearQueue={() => undefined}
        onUndoQueueEdit={() => undefined}
        onRedoQueueEdit={() => undefined}
        onSelectQueueTrack={() => undefined}
      />,
    );

    expect(markup).toContain("Embedded track");
    expect(markup).toContain("listen-workspace-queue-content");
    expect(markup).toContain('data-companion-scroll-owner="queue"');
    expect(markup).not.toContain("listen-workspace-queue-footer");
    expect(markup).not.toContain('role="radiogroup"');
    expect(markup).not.toContain(text.actions.clear);
    expect(markup).not.toContain(text.listen.undoQueue);
    expect(markup).not.toContain(text.listen.redoQueue);
  });

  test("keeps song artwork rectangular and restores the queue thumbnail size", async () => {
    const text = getXiaText("en");
    const [markup, appearance] = await Promise.all([
      Promise.resolve(
        renderToStaticMarkup(
          <ListenWorkspaceOnlineQueueCompanion
            queueTitle={text.listen.upNext}
            queueItems={[onlineItem("shape", "Square song", "Artist")]}
            selectedQueueId="shape"
            httpBaseURL="http://127.0.0.1:34115"
            playMode="order"
            text={text}
            onPlayModeChange={() => undefined}
            onSelectQueueTrack={() => undefined}
          />,
        ),
      ),
      Bun.file(
        new URL("../../../shared/styles/dream/listen.css", import.meta.url),
      ).text(),
    ]);
    const artworkRule = appearance.match(
      /:root \.app-dream-button\.listen-workspace-queue-row__artwork\[data-app-button\]\[data-size\]\s*\{([^}]*)\}/s,
    )?.[1];

    expect(markup).toMatch(
      /<button[^>]*listen-workspace-queue-row__artwork[^>]*data-shape="square"/,
    );
    expect(artworkRule).toContain("--app-button-inline-size: 2.65rem");
    expect(artworkRule).toContain("--app-button-block-size: 2.65rem");
    expect(artworkRule).toContain("--app-button-padding: 0");
    expect(artworkRule).toContain("--app-button-border: 0");
    expect(artworkRule).toContain(
      "border-radius: var(--app-radius-control-inner)",
    );
    expect(artworkRule).not.toContain("--app-radius-capsule");
  });

  test("renders an accessible three-mode switch with Lyrics-sized visuals", async () => {
    const text = getXiaText("en");
    const markup = renderToStaticMarkup(
      <ListenWorkspaceQueueModeSwitch
        playMode="shuffle"
        text={text}
        onChange={() => undefined}
      />,
    );
    const [layoutCss, appearanceCss] = await Promise.all([
      Bun.file(new URL("./listen.css", import.meta.url)).text(),
      Bun.file(
        new URL("../../../shared/styles/dream/listen.css", import.meta.url),
      ).text(),
    ]);
    const radios = markup.match(/<button[^>]*role="radio"[^>]*>/g) ?? [];
    const labels = radios.map(
      (radio) => radio.match(/aria-label="([^"]+)"/)?.[1],
    );
    const activeRadio = radios.find((radio) =>
      radio.includes('aria-checked="true"'),
    );
    const modeButtonRule = appearanceCss.match(
      /\.listen-companion-mode-controls__button\s*\{([^}]*)\}/s,
    )?.[1];
    const modeActiveRule = appearanceCss.match(
      /\.listen-companion-mode-controls__button\[data-active="true"\]\s*\{([^}]*)\}/s,
    )?.[1]?.trim();
    const lyricsActiveRule = appearanceCss.match(
      /\.listen-lyrics-controls__mode-button\[data-active="true"\]\s*\{([^}]*)\}/s,
    )?.[1]?.trim();

    expect(markup).toContain('role="radiogroup"');
    expect(radios).toHaveLength(3);
    expect(labels).toEqual([
      text.listen.playModeOrder,
      text.listen.playModeShuffle,
      text.listen.playModeRepeat,
    ]);
    expect(activeRadio).toContain(`aria-label="${text.listen.playModeShuffle}"`);
    expect(activeRadio).toContain('data-active="true"');
    expect(activeRadio).toContain('tabindex="0"');
    expect(markup.match(/tabindex="-1"/g)).toHaveLength(2);
    expect(markup).toContain("lucide-list-music");
    expect(markup).toContain("lucide-shuffle");
    expect(markup).toContain("lucide-repeat-1");
    expect(modeButtonRule).toContain("--app-button-inline-size: 2.5rem");
    expect(modeButtonRule).toContain("--app-button-block-size: 2.5rem");
    expect(modeButtonRule).toContain("--app-button-icon-size: 1rem");
    expect(layoutCss).not.toMatch(
      /\.listen-companion-mode-controls__button\s*\{[^}]*\b(?:width|height):[^;]*!important/s,
    );
    expect(modeActiveRule).toBe(lyricsActiveRule);
  });

  test("local queue exposes the same clear, item edit, and history controls", () => {
    const text = getXiaText("en");
    const markup = renderToStaticMarkup(
      <ListenWorkspaceLocalQueueCompanion
        queueTitle={text.listen.upNext}
        queueItems={[
          {
            id: "local-one",
            title: "Local track",
            author: "Local artist",
            album: "",
            albumArtist: "",
            genre: "",
            trackNumber: 0,
            discNumber: 0,
            year: 0,
            lyricsTitle: "Local track",
            lyricsArtist: "Local artist",
            path: "/tmp/local.mp3",
            previewURL: "",
            coverURL: "",
            durationLabel: "2:03",
            durationSeconds: 123,
            format: "mp3",
            audioCodec: "mp3",
            sizeBytes: 1,
            metadataWritable: true,
            modTimeUnix: 1,
            createdAtUnix: 1,
            playbackSupported: true,
            playbackUnsupportedReason: "",
            probeError: "",
          },
          {
            id: "local-two",
            title: "Second local track",
            author: "Local artist",
            album: "",
            albumArtist: "",
            genre: "",
            trackNumber: 0,
            discNumber: 0,
            year: 0,
            lyricsTitle: "Second local track",
            lyricsArtist: "Local artist",
            path: "/tmp/local-two.mp3",
            previewURL: "",
            coverURL: "",
            durationLabel: "3:04",
            durationSeconds: 184,
            format: "mp3",
            audioCodec: "mp3",
            sizeBytes: 1,
            metadataWritable: true,
            modTimeUnix: 1,
            createdAtUnix: 1,
            playbackSupported: true,
            playbackUnsupportedReason: "",
            probeError: "",
          },
        ]}
        selectedQueueId="local-one"
        playMode="order"
        text={text}
        queueCanUndo
        queueCanRedo={false}
        onPlayModeChange={() => undefined}
        onClearQueue={() => undefined}
        onRemoveQueueItem={() => undefined}
        onMoveQueueItem={() => undefined}
        onUndoQueueEdit={() => undefined}
        onRedoQueueEdit={() => undefined}
        onSelectQueueTrack={() => undefined}
      />,
    );

    expect(markup).not.toContain(text.listen.upNext);
    expect(markup).toContain("Local track");
    expect(markup).toContain("Second local track");
    expect(markup).toContain(text.actions.clear);
    expect(markup).toContain(text.listen.undoQueue);
    expect(markup).toContain(text.listen.redoQueue);
    expect(markup).toContain(`${text.listen.more}: Local track`);
  });

  test("music shell toggles destinations and overlays one integrated title row", async () => {
    const [mainSource, workspaceCss] = await Promise.all([
      Bun.file(new URL("../MainApp.tsx", import.meta.url)).text(),
      readWorkspaceCss(),
    ]);

    expect(mainSource).toContain("const toggleMusicCompanion");
    expect(mainSource).toContain("const requestMusicCompanionMode");
    expect(mainSource).toContain("toggleCompanion({");
    expect(mainSource).toContain(
      'onOpenLyrics={() => requestMusicCompanionMode("lyrics")}',
    );
    expect(mainSource).toContain(
      'onOpenQueue={() => requestMusicCompanionMode("queue")}',
    );
    expect(mainSource).toContain(
      "onOpenPlaybackSource={openMusicPlaybackSource}",
    );
    expect(mainSource).toContain(
      "onRequestPlayerFullscreen={requestMusicPlayerFullscreen}",
    );
    expect(mainSource).toContain(
      'companion.destination?.id === "lyrics"\n            ? "off"',
    );
    expect(mainSource).toContain('case "youtube_music":');
    expect(mainSource).toContain('case "radio":');
    expect(mainSource).toContain('case "local":');
    expect(mainSource).toContain("const openCurrentMusicArtist");
    expect(mainSource).toContain('sendListenCommand("open-artist")');
    expect(mainSource).toContain(
      'sendListenCommand("open-artist", undefined, artist)',
    );
    expect(mainSource).toContain("onOpenArtist={");
    expect(workspaceCss).toContain('[data-destination="player"]');
    expect(workspaceCss).toContain('[data-destination="lyrics"]');
    expect(workspaceCss).toContain('[data-destination="queue"]');
    expect(workspaceCss).toContain("border-bottom: 0");
    expect(workspaceCss).not.toMatch(
      /\.app-workspace-companion__content\s*\{\s*padding-top:\s*48px/,
    );
  });

  test("the portaled player surface preserves the companion height chain", async () => {
    const pageViewSource = await Bun.file(
      new URL("./PageView.tsx", import.meta.url),
    ).text();

    expect(pageViewSource).toContain(
      '"listen-content-surface app-workspace-primary-subpane relative flex h-full min-h-0 w-full',
    );
    expect(pageViewSource).toContain("listOpen &&");
    expect(pageViewSource).toContain("!props.playerPortalTarget &&");
    expect(pageViewSource).toContain(
      '"app-workspace-primary-subpane--leading"',
    );
  });
});
