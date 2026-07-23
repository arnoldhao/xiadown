import { describe, expect, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";

import { LISTEN_DEFAULT_COVER_IMAGE_URL } from "./default-cover";
import {
  ListenCoverArtwork,
  ListenDefaultCoverArtwork,
  isListenCoverImageReady,
} from "./listen-cover-artwork";

describe("Listen player default artwork", () => {
  test("renders a theme-aware music icon without an app-icon image", () => {
    const markup = renderToStaticMarkup(
      <ListenDefaultCoverArtwork alt="Now playing" />,
    );

    expect(markup).toContain('data-listen-default-cover="music"');
    expect(markup).toContain("lucide-music-2");
    expect(markup).toContain('role="img"');
    expect(markup).not.toContain("appicon");
    expect(markup).not.toContain("<img");
  });

  test("resolves the bundled fallback to the live component in player surfaces", () => {
    const markup = renderToStaticMarkup(
      <ListenCoverArtwork
        alt="Now playing"
        candidates={[LISTEN_DEFAULT_COVER_IMAGE_URL]}
      />,
    );

    expect(markup).toContain('data-listen-default-cover="music"');
    expect(markup).not.toContain(`src="${LISTEN_DEFAULT_COVER_IMAGE_URL}"`);
    expect(markup).not.toContain("appicon");
  });

  test("shows the music cover while a new remote cover is still loading", () => {
    const markup = renderToStaticMarkup(
      <ListenCoverArtwork
        alt="New track"
        candidates={["https://images.example/new-track.jpg"]}
      />,
    );

    expect(markup).toContain('data-listen-default-cover="music"');
    expect(markup).toContain('src="https://images.example/new-track.jpg"');
    expect(markup).toContain('data-artwork-ready="false"');
    expect(markup).toContain("listen-cover-artwork__image");
    expect(markup).not.toContain("opacity-0");
    expect(markup).not.toContain("appicon");
  });

  test("recognizes an asynchronously assigned cached cover without a load event", async () => {
    expect(isListenCoverImageReady({ complete: true, naturalWidth: 640 })).toBe(
      true,
    );
    expect(isListenCoverImageReady({ complete: true, naturalWidth: 0 })).toBe(
      false,
    );
    expect(isListenCoverImageReady({ complete: false, naturalWidth: 640 })).toBe(
      false,
    );

    const source = await Bun.file(
      new URL("./listen-cover-artwork.tsx", import.meta.url),
    ).text();
    expect(source).toContain("ref={handleImageRef}");
    expect(source).toContain("if (isListenCoverImageReady(image))");
  });

  test("keeps direct-child artwork styling attached after the image wrapper", async () => {
    const [listenCSS, componentCSS, transportCSS] = await Promise.all([
      Bun.file(
        new URL("../../app/main/listen/listen.css", import.meta.url),
      ).text(),
      Bun.file(
        new URL("../styles/dream/components.css", import.meta.url),
      ).text(),
      Bun.file(
        new URL("../styles/dream/transport.css", import.meta.url),
      ).text(),
    ]);

    expect(listenCSS).toMatch(
      /\.listen-artwork-frame\s*>\s*\[data-listen-cover-artwork="true"\]/,
    );
    expect(componentCSS).toMatch(
      /\.listen-panel-artwork-main\s*>\s*\[data-listen-cover-artwork="true"\]/,
    );
    expect(componentCSS).toContain(
      '> :is(img, [data-listen-default-cover="music"])',
    );
    expect(transportCSS).toContain(
      '> [data-listen-cover-artwork="true"]',
    );
  });
});
