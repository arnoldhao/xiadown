import type { RSSEntry, RSSSubscription } from "./types";
import { canonicalRSSBilibiliIdentityKey } from "./bilibili-playback-adapters";

export interface RSSBilibiliPageMetadata {
  sessionId: string;
  platformVideoId: string;
  publisher?: string;
  publishedAt?: string;
  viewCount?: number;
  likeCount?: number;
}

export interface RSSBilibiliDisplayMetadata {
  publisher: string;
  publishedAt: string;
  viewCount: number;
  likeCount: number;
}

/**
 * Prefers metadata observed from the active canonical Bilibili document while
 * retaining feed metadata as a stable, offline-safe fallback.
 */
export function resolveRSSBilibiliDisplayMetadata(
  entry: Pick<
    RSSEntry,
    "author" | "publishedAt" | "sourceUpdatedAt" | "createdAt"
  >,
  source: Pick<RSSSubscription, "title"> | undefined,
  live: RSSBilibiliPageMetadata | null | undefined,
  expectedPlatformVideoId: string,
): RSSBilibiliDisplayMetadata {
  const expectedVideoID = canonicalBilibiliVideoID(expectedPlatformVideoId);
  const currentLive =
    expectedVideoID && canonicalBilibiliVideoID(live?.platformVideoId) === expectedVideoID
      ? live
      : null;
  return {
    publisher:
      cleanText(currentLive?.publisher) ||
      cleanText(entry.author) ||
      cleanText(source?.title),
    publishedAt:
      cleanText(currentLive?.publishedAt) ||
      cleanText(entry.publishedAt) ||
      cleanText(entry.sourceUpdatedAt) ||
      cleanText(entry.createdAt),
    viewCount: positiveInteger(currentLive?.viewCount),
    likeCount: positiveInteger(currentLive?.likeCount),
  };
}

function canonicalBilibiliVideoID(value: string | null | undefined) {
  return canonicalRSSBilibiliIdentityKey({
    platformVideoId: value || "",
    url: "",
    playbackUrl: "",
  });
}

function cleanText(value: string | null | undefined) {
  return value?.trim() || "";
}

function positiveInteger(value: number | null | undefined) {
  const number = Number(value);
  return Number.isFinite(number) && number > 0 ? Math.floor(number) : 0;
}
