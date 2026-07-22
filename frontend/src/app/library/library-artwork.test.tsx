import { describe, expect, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";

import { COMPLETED_DEFAULT_COVER_IMAGE_URLS } from "@/shared/assets/default-cover";

import {
  isLibraryDefaultArtworkURL,
  LibraryArtwork,
} from "./LibraryArtwork";
import {
  LIBRARY_PAPER_NOTCH_POSITIONS,
  PLACEHOLDER_PAPER_HORIZONTAL_NOTCH_OFFSETS,
  PLACEHOLDER_PAPER_TRANSFORM,
} from "./library-paper-geometry";

describe("LibraryArtwork", () => {
  test("replaces bitmap defaults with a theme-aware runtime icon", () => {
    const markup = renderToStaticMarkup(
      <LibraryArtwork
        src={COMPLETED_DEFAULT_COVER_IMAGE_URLS.video}
        category="video"
        alt="Video placeholder"
      />,
    );

    expect(markup).toContain("app-library-artwork--placeholder");
    expect(markup).toContain('data-artwork-kind="video"');
    expect(markup).toContain("app-library-artwork__placeholder-paper");
    expect(markup).toContain("app-library-artwork__placeholder-icon");
    expect(markup).toContain(`transform="${PLACEHOLDER_PAPER_TRANSFORM}"`);
    expect(PLACEHOLDER_PAPER_TRANSFORM).not.toContain("rotate(");
    expect(markup.match(/<circle /g)).toHaveLength(
      (LIBRARY_PAPER_NOTCH_POSITIONS.length * 2) +
        (PLACEHOLDER_PAPER_HORIZONTAL_NOTCH_OFFSETS.length * 2),
    );
    expect(markup.match(/cx="-56"/g)).toHaveLength(
      LIBRARY_PAPER_NOTCH_POSITIONS.length,
    );
    expect(markup.match(/cx="0"/g)).toHaveLength(
      LIBRARY_PAPER_NOTCH_POSITIONS.length,
    );
    expect(markup.match(/cy="0"/g)).toHaveLength(
      PLACEHOLDER_PAPER_HORIZONTAL_NOTCH_OFFSETS.length,
    );
    expect(markup.match(/cy="84.8"/g)).toHaveLength(
      PLACEHOLDER_PAPER_HORIZONTAL_NOTCH_OFFSETS.length,
    );
    expect(markup.match(/<mask /g)).toHaveLength(1);
    expect(markup.match(/<svg/g)).toHaveLength(2);
    expect(markup).not.toContain("<img");
  });

  test("keeps real artwork as a lazily decoded image", () => {
    const markup = renderToStaticMarkup(
      <LibraryArtwork
        src="http://127.0.0.1/artwork/example.webp"
        fallbackSrc={COMPLETED_DEFAULT_COVER_IMAGE_URLS.image}
        category="image"
        alt="Example"
      />,
    );

    expect(markup).toContain("<img");
    expect(markup).toContain('loading="lazy"');
    expect(markup).not.toContain("app-library-artwork--placeholder");
    expect(markup).not.toContain("app-library-artwork__placeholder-paper");
  });

  test("creates a unique paper-edge mask for each semantic placeholder", () => {
    const markup = renderToStaticMarkup(
      <>
        <LibraryArtwork category="audio" alt="" />
        <LibraryArtwork category="book" alt="" />
      </>,
    );
    const ids = [...markup.matchAll(/<mask id="([^"]+)"/g)].map((match) => match[1]!);
    const references = [...markup.matchAll(/mask="url\(#([^\)]+)\)"/g)].map(
      (match) => match[1]!,
    );

    expect(ids).toHaveLength(2);
    expect(new Set(ids).size).toBe(2);
    expect(references).toEqual(ids);
  });

  test("recognizes canonical semantic defaults with cache-busting query strings", () => {
    expect(
      isLibraryDefaultArtworkURL(
        `${COMPLETED_DEFAULT_COVER_IMAGE_URLS.audio}?revision=1`,
      ),
    ).toBe(true);
    expect(isLibraryDefaultArtworkURL("/artwork/audio.webp")).toBe(false);
  });
});
