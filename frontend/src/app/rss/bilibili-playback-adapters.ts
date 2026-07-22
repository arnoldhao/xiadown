import type { RSSEntry } from "./types";

export type RSSBilibiliPlaybackAdapterKey = "video" | "bangumi";

export interface RSSBilibiliPlaybackIdentity {
  adapter: RSSBilibiliPlaybackAdapterKey;
  platformVideoId: string;
  playbackUrl: string;
}

export interface RSSBilibiliPlaybackAdapter {
  key: RSSBilibiliPlaybackAdapterKey;
  normalizePlatformVideoId(value: string): string;
  platformVideoIdFromURL(rawURL: string): string;
  canonicalPageURL(platformVideoId: string): string;
}

type RSSBilibiliIdentitySource = Pick<
  RSSEntry,
  "platformVideoId" | "url" | "playbackUrl"
>;

function normalizeOrdinaryVideoID(value: string) {
  const trimmed = value.trim();
  if (/^BV[0-9A-Za-z]{10}$/i.test(trimmed)) {
    // BVID suffixes are case-sensitive. Only the stable `BV` prefix is
    // canonicalized so metadata from a different identity is never accepted.
    return `BV${trimmed.slice(2)}`;
  }
  if (/^av\d+$/i.test(trimmed)) {
    const numericID = trimmed.slice(2).replace(/^0+/, "");
    if (numericID) return `av${numericID}`;
  }
  if (/^\d+$/.test(trimmed)) {
    const numericID = trimmed.replace(/^0+/, "");
    if (numericID) return `av${numericID}`;
  }
  return "";
}

function ordinaryVideoIDFromURL(rawURL: string) {
  try {
    const parsed = new URL(rawURL);
    const pathMatch = parsed.pathname.match(
      /\/(BV[0-9A-Za-z]{10}|av\d+)(?:\/|$)/i,
    );
    return normalizeOrdinaryVideoID(
      pathMatch?.[1] ||
        parsed.searchParams.get("bvid") ||
        parsed.searchParams.get("aid") ||
        "",
    );
  } catch {
    return "";
  }
}

function normalizeBangumiID(value: string) {
  const match = value.trim().match(/^(ep|ss)(\d+)$/i);
  if (!match) return "";
  const numericID = match[2].replace(/^0+/, "");
  return numericID ? `${match[1].toLowerCase()}${numericID}` : "";
}

function bangumiIDFromURL(rawURL: string) {
  try {
    const parsed = new URL(rawURL);
    return normalizeBangumiID(
      parsed.pathname.match(/\/bangumi\/play\/((?:ep|ss)\d+)(?:\/|$)/i)?.[1] ||
        "",
    );
  } catch {
    return "";
  }
}

/** Ordinary Bilibili `/video/BV…` and `/video/av…` playback adapter. */
export const RSS_BILIBILI_VIDEO_ADAPTER: RSSBilibiliPlaybackAdapter = {
  key: "video",
  normalizePlatformVideoId: normalizeOrdinaryVideoID,
  platformVideoIdFromURL: ordinaryVideoIDFromURL,
  canonicalPageURL: (platformVideoId) =>
    `https://www.bilibili.com/video/${encodeURIComponent(platformVideoId)}/`,
};

/** Bilibili PGC `/bangumi/play/ep…` and `/bangumi/play/ss…` adapter. */
export const RSS_BILIBILI_BANGUMI_ADAPTER: RSSBilibiliPlaybackAdapter = {
  key: "bangumi",
  normalizePlatformVideoId: normalizeBangumiID,
  platformVideoIdFromURL: bangumiIDFromURL,
  canonicalPageURL: (platformVideoId) =>
    `https://www.bilibili.com/bangumi/play/${encodeURIComponent(platformVideoId)}`,
};

export const RSS_BILIBILI_PLAYBACK_ADAPTERS = [
  RSS_BILIBILI_VIDEO_ADAPTER,
  RSS_BILIBILI_BANGUMI_ADAPTER,
] as const;

/**
 * Resolves the feed-authored identity before consulting page URLs. This keeps
 * adapter selection deterministic when feeds include several related links.
 */
export function resolveRSSBilibiliPlaybackIdentity(
  source: RSSBilibiliIdentitySource,
): RSSBilibiliPlaybackIdentity | null {
  const explicitID = source.platformVideoId?.trim() || "";
  if (explicitID) {
    for (const adapter of RSS_BILIBILI_PLAYBACK_ADAPTERS) {
      const platformVideoId = adapter.normalizePlatformVideoId(explicitID);
      if (platformVideoId) return identityForAdapter(adapter, platformVideoId);
    }
  }

  for (const rawURL of [source.url, source.playbackUrl]) {
    if (!rawURL) continue;
    for (const adapter of RSS_BILIBILI_PLAYBACK_ADAPTERS) {
      const platformVideoId = adapter.platformVideoIdFromURL(rawURL);
      if (platformVideoId) return identityForAdapter(adapter, platformVideoId);
    }
  }
  return null;
}

/** Kept as the ordinary-video identity helper used by existing callers. */
export function normalizedBilibiliVideoID(source: RSSBilibiliIdentitySource) {
  const explicitID = RSS_BILIBILI_VIDEO_ADAPTER.normalizePlatformVideoId(
    source.platformVideoId || "",
  );
  if (explicitID) return explicitID;
  for (const rawURL of [source.url, source.playbackUrl]) {
    if (!rawURL) continue;
    const platformVideoId =
      RSS_BILIBILI_VIDEO_ADAPTER.platformVideoIdFromURL(rawURL);
    if (platformVideoId) return platformVideoId;
  }
  return "";
}

export function normalizedBilibiliBangumiID(source: RSSBilibiliIdentitySource) {
  const explicitID = RSS_BILIBILI_BANGUMI_ADAPTER.normalizePlatformVideoId(
    source.platformVideoId || "",
  );
  if (explicitID) return explicitID;
  for (const rawURL of [source.url, source.playbackUrl]) {
    if (!rawURL) continue;
    const platformVideoId =
      RSS_BILIBILI_BANGUMI_ADAPTER.platformVideoIdFromURL(rawURL);
    if (platformVideoId) return platformVideoId;
  }
  return "";
}

export function canonicalRSSBilibiliIdentityKey(
  source: RSSBilibiliIdentitySource,
) {
  const identity = resolveRSSBilibiliPlaybackIdentity(source);
  return identity ? `${identity.adapter}:${identity.platformVideoId}` : "";
}

function identityForAdapter(
  adapter: RSSBilibiliPlaybackAdapter,
  platformVideoId: string,
): RSSBilibiliPlaybackIdentity {
  return {
    adapter: adapter.key,
    platformVideoId,
    playbackUrl: adapter.canonicalPageURL(platformVideoId),
  };
}
