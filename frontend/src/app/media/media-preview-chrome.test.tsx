import { describe, expect, mock, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";

import type { VidstackPreviewLabels } from "./VidstackPreview";
import { VidstackPreview } from "./VidstackPreview";

mock.module("@wailsio/runtime", () => ({
  Events: {
    On: () => () => undefined,
    Types: {
      Common: {
        WindowFullscreen: "window-fullscreen",
        WindowUnFullscreen: "window-unfullscreen",
      },
    },
  },
  Window: {
    Fullscreen: () => Promise.resolve(),
    UnFullscreen: () => Promise.resolve(),
  },
}));

const { MediaPreviewSurface } = await import("./MediaPreviewSurface");

const labels: VidstackPreviewLabels = {
  noPreview: "No preview",
  previewEnterFullscreen: "Enter screen fullscreen",
  previewExitFullscreen: "Exit screen fullscreen",
  previewMute: "Mute preview",
  previewPause: "Pause preview",
  previewPlay: "Play preview",
  previewPlaybackStalled: "Playback stalled",
  previewSeek: "Seek preview",
  previewUnmute: "Unmute preview",
  previewVolume: "Preview volume",
  previewWindowFullscreen: "Enter window fullscreen",
  previewWindowRestore: "Restore window",
};

function renderVideo(chrome?: "full" | "transport") {
  return renderToStaticMarkup(
    <VidstackPreview
      labels={labels}
      mediaUrl="https://example.test/preview.mp4"
      title="Preview video"
      chrome={chrome}
    />,
  );
}

describe("media preview chrome", () => {
  test("keeps the existing full chrome as the default", () => {
    const markup = renderVideo();

    expect(markup).toContain('data-preview-chrome="full"');
    expect(markup).toContain(`aria-label="${labels.previewPlay}"`);
    expect(markup).toContain(`aria-label="${labels.previewSeek}"`);
    expect(markup).toContain(`aria-label="${labels.previewMute}"`);
    expect(markup).toContain(
      `aria-label="${labels.previewWindowFullscreen}"`,
    );
    expect(markup).toContain(
      `aria-label="${labels.previewEnterFullscreen}"`,
    );
  });

  test("transport chrome exposes only playback, timeline, and volume controls", () => {
    const markup = renderVideo("transport");

    expect(markup).toContain('data-preview-chrome="transport"');
    expect(markup).toContain(`aria-label="${labels.previewPlay}"`);
    expect(markup).toContain(`aria-label="${labels.previewSeek}"`);
    expect(markup).toContain(`aria-label="${labels.previewMute}"`);
    expect(markup).toContain(`aria-label="${labels.previewVolume}"`);
    expect(markup).not.toContain(labels.previewWindowFullscreen);
    expect(markup).not.toContain(labels.previewWindowRestore);
    expect(markup).not.toContain(labels.previewEnterFullscreen);
    expect(markup).not.toContain(labels.previewExitFullscreen);
    expect(markup).not.toContain("lucide-maximize");
    expect(markup).not.toContain("lucide-minimize");
  });

  test("MediaPreviewSurface forwards the opt-in chrome contract", async () => {
    const [surfaceSource, previewSource] = await Promise.all([
      Bun.file(new URL("./MediaPreviewSurface.tsx", import.meta.url)).text(),
      Bun.file(new URL("./VidstackPreview.tsx", import.meta.url)).text(),
    ]);

    expect(surfaceSource).toContain('chrome?: VidstackPreviewProps["chrome"]');
    expect(surfaceSource).toContain("chrome={props.chrome}");
    expect(surfaceSource).toContain('import("@/app/media/VidstackPreview")');
    expect(previewSource).toContain('props.chrome !== "transport"');
  });

  test("keeps the player recipe global while loading vendor CSS with the lazy player", async () => {
    const [previewSource, dreamStyles, entrypoint] = await Promise.all([
      Bun.file(new URL("./VidstackPreview.tsx", import.meta.url)).text(),
      Bun.file(
        new URL("../../shared/styles/dream/completed.css", import.meta.url),
      ).text(),
      Bun.file(new URL("../../index.css", import.meta.url)).text(),
    ]);

    expect(previewSource).not.toContain('style={{ aspectRatio: "auto" }}');
    expect(dreamStyles).toMatch(
      /\.app-completed-preview-player\s*\{[^}]*aspect-ratio:\s*auto;/s,
    );
    expect(previewSource).toContain(
      'import "@vidstack/react/player/styles/base.css";',
    );
    expect(previewSource).not.toContain("styles/default/theme.css");
    expect(previewSource).not.toContain("tw-shimmer");
    expect(entrypoint).not.toContain("@vidstack/react/player/styles/base.css");
    expect(entrypoint).toContain('@import "./shared/styles/dream.css";');
  });
});
