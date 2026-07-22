import { describe, expect, test } from "bun:test";

import {
  buildYouTubeImageCandidates,
  nextYouTubeImageCandidate,
} from "@/app/youtube/YouTubeImage";

describe("YouTubeImage", () => {
  test("normalizes protocol-relative sources and adds deterministic video fallbacks", () => {
    expect(
      buildYouTubeImageCandidates(
        " //i.ytimg.com/vi/AbCdEfGh123/maxresdefault.jpg ",
        "AbCdEfGh123",
      ),
    ).toEqual([
      "https://i.ytimg.com/vi/AbCdEfGh123/maxresdefault.jpg",
      "https://i.ytimg.com/vi_webp/AbCdEfGh123/hqdefault.webp",
      "https://i.ytimg.com/vi/AbCdEfGh123/hqdefault.jpg",
      "https://i.ytimg.com/vi/AbCdEfGh123/mqdefault.jpg",
      "https://i.ytimg.com/vi/AbCdEfGh123/default.jpg",
    ]);
  });

  test("does not invent video fallbacks for non-video identifiers", () => {
    expect(buildYouTubeImageCandidates("https://example.com/avatar", "playlist"))
      .toEqual(["https://example.com/avatar"]);
  });

  test("walks every candidate before revealing the final fallback", () => {
    expect(nextYouTubeImageCandidate(0, 3)).toBe(1);
    expect(nextYouTubeImageCandidate(1, 3)).toBe(2);
    expect(nextYouTubeImageCandidate(2, 3)).toBeNull();
  });
});
