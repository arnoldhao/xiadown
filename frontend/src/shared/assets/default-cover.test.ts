import { describe, expect, test } from "bun:test";

import {
  COMPLETED_DEFAULT_COVER_IMAGE_URLS,
  DEFAULT_COVER_IMAGE_URL,
  isListenDefaultCoverImageURL,
  LISTEN_DEFAULT_COVER_IMAGE_URL,
} from "./default-cover";

describe("completed default covers", () => {
  test("uses unique semantic tokens instead of loading bitmap placeholders", () => {
    const tokens = Object.values(COMPLETED_DEFAULT_COVER_IMAGE_URLS);

    expect(new Set(tokens).size).toBe(tokens.length);
    for (const token of tokens) {
      expect(token).toStartWith("xiadown-library-default:");
      expect(token).not.toMatch(/\.(?:avif|jpe?g|png|webp)$/i);
    }
  });

  test("uses bundled music artwork instead of the shipped app icon", async () => {
    expect(LISTEN_DEFAULT_COVER_IMAGE_URL).toBe("/listen_default_cover.svg");
    expect(LISTEN_DEFAULT_COVER_IMAGE_URL).not.toBe(DEFAULT_COVER_IMAGE_URL);
    expect(LISTEN_DEFAULT_COVER_IMAGE_URL).not.toContain("appicon");
    expect(isListenDefaultCoverImageURL(LISTEN_DEFAULT_COVER_IMAGE_URL)).toBe(
      true,
    );

    const artwork = Bun.file(
      new URL(`../../../public${LISTEN_DEFAULT_COVER_IMAGE_URL}`, import.meta.url),
    );
    expect(await artwork.exists()).toBe(true);
    expect(await artwork.text()).toContain('data-listen-default-cover="music"');
  });
});
