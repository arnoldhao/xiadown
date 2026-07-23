import type { RSSEntry } from "./types";
import type { RSSCollectionRoute } from "./workspace-utils";
import { controlledRSSResourceURL } from "./remote-resource";
import {
  resolveRSSBilibiliPlaybackIdentity,
  type RSSBilibiliPlaybackAdapterKey,
} from "./bilibili-playback-adapters";

export {
  normalizedBilibiliBangumiID,
  normalizedBilibiliVideoID,
  resolveRSSBilibiliPlaybackIdentity,
} from "./bilibili-playback-adapters";

export type RSSVideoPlaybackMode =
  | "youtube-native"
  | "bilibili-native"
  | "site"
  | "embed"
  | "direct"
  | "unavailable";

export interface RSSVideoExperience {
  mode: RSSVideoPlaybackMode;
  siteKey?: string;
  bilibiliAdapter?: RSSBilibiliPlaybackAdapterKey;
  playbackUrl?: string;
  targetUrl?: string;
  appSessionPreferred: boolean;
}

export interface RSSVideoBatchDownloadTarget {
  entryId: string;
  url: string;
  source: "xiadown.rss";
  caller: string;
}

interface RSSVideoPlatformPolicy {
  siteKey: string;
  domains: readonly string[];
  embedDomains?: readonly string[];
}

/**
 * A single policy registry drives playback and App Session selection. Keeping
 * embed hosts separate from page hosts prevents feed-authored playbackUrl
 * values from turning the focused reader into an arbitrary webview.
 */
const RSS_VIDEO_PLATFORM_POLICIES: readonly RSSVideoPlatformPolicy[] = [
  {
    siteKey: "bilibili",
    domains: ["bilibili.com", "b23.tv"],
  },
  {
    siteKey: "youtube",
    domains: ["youtube.com", "youtube-nocookie.com", "youtu.be"],
    embedDomains: ["youtube-nocookie.com", "youtube.com"],
  },
  {
    siteKey: "vimeo",
    domains: ["vimeo.com", "player.vimeo.com"],
    embedDomains: ["player.vimeo.com"],
  },
  {
    siteKey: "tiktok",
    domains: ["tiktok.com", "tiktokv.com", "vm.tiktok.com"],
  },
  {
    siteKey: "douyin",
    domains: ["douyin.com", "iesdouyin.com"],
  },
  {
    siteKey: "xiaohongshu",
    domains: [
      "xiaohongshu.com",
      "xhs.cn",
      "xhslink.com",
      "xhslink.cn",
      "xhsurl.com",
      "rl.ink",
    ],
  },
  { siteKey: "china_private", domains: ["rednote.com"] },
  { siteKey: "instagram", domains: ["instagram.com"] },
  { siteKey: "x", domains: ["x.com", "twitter.com"] },
  {
    siteKey: "facebook",
    domains: ["facebook.com", "fb.watch"],
  },
  {
    siteKey: "twitch",
    domains: ["twitch.tv", "clips.twitch.tv"],
  },
  {
    siteKey: "niconico",
    domains: ["nicovideo.jp", "nico.ms", "nicovideo.cdn.nimg.jp"],
  },
] as const;

export function resolveRSSVideoExperience(entry: RSSEntry): RSSVideoExperience {
  const targetUrl = canonicalRSSVideoTarget(entry);
  const siteKey = [
    entry.url,
    entry.playbackUrl,
    targetUrl,
    entry.downloadTarget,
    entry.mediaUrl,
  ]
    .map((candidate) => siteKeyForRSSVideo(candidate || "", entry.platform))
    .find(Boolean) || "";
  const youtubeVideoID = normalizedYouTubeVideoID(entry);
  if (
    (entry.platform === "youtube" || siteKey === "youtube") &&
    youtubeVideoID
  ) {
    return {
      mode: "youtube-native",
      siteKey: "youtube",
      targetUrl,
      appSessionPreferred: false,
    };
  }
  const bilibiliIdentity = resolveRSSBilibiliPlaybackIdentity(entry);
  if ((entry.platform === "bilibili" || siteKey === "bilibili") && bilibiliIdentity) {
    return {
      mode: "bilibili-native",
      siteKey: "bilibili",
      bilibiliAdapter: bilibiliIdentity.adapter,
      // The native session navigates the ordinary Bilibili video page so it
      // can reuse App Session cookies. Both ordinary-video and Bangumi
      // adapters use a full first-party page, never an iframe destination.
      playbackUrl: bilibiliIdentity.playbackUrl,
      targetUrl,
      appSessionPreferred: true,
    };
  }
  const policy = policyForSiteKey(siteKey);
  const direct = entry.media.find(
    (item) =>
      item.kind === "video" &&
      item.mimeType !== "text/html" &&
      Boolean(controlledRSSResourceURL(item.url)),
  )?.url;
  if (direct) {
    return {
      mode: "direct",
      playbackUrl: direct,
      targetUrl,
      siteKey,
      appSessionPreferred: false,
    };
  }
  if (entry.playbackUrl && isAllowedEmbedURL(entry.playbackUrl, policy)) {
    return {
      mode: "embed",
      playbackUrl: entry.playbackUrl,
      targetUrl,
      siteKey,
      appSessionPreferred: false,
    };
  }
  if (entry.playbackUrl && isSafeDirectVideoURL(entry.playbackUrl)) {
    return {
      mode: "direct",
      playbackUrl: entry.playbackUrl,
      targetUrl,
      siteKey,
      appSessionPreferred: false,
    };
  }
  const sitePlaybackUrl = safeRSSVideoSitePageURL(entry, siteKey);
  if (sitePlaybackUrl) {
    const playbackSiteKey = siteKeyForRSSVideo(sitePlaybackUrl);
    return {
      mode: "site",
      playbackUrl: sitePlaybackUrl,
      targetUrl,
      siteKey: playbackSiteKey,
      // The native handler independently derives the actual site policy from
      // the URL before it considers credentials. This flag is descriptive;
      // it is never an instruction to send cookies.
      appSessionPreferred: Boolean(
        playbackSiteKey && playbackSiteKey !== "china_private",
      ),
    };
  }
  return { mode: "unavailable", targetUrl, siteKey, appSessionPreferred: false };
}

/**
 * Content kind can outlive a classifier upgrade in the local archive. Keep
 * the presentation decision evidence-based so a stale `video` row can never
 * create a native site player for an ordinary article page.
 */
export function shouldUseRSSVideoPresentation(entry: RSSEntry) {
  return entry.kind === "video" && resolveRSSVideoExperience(entry).mode !== "unavailable";
}

/**
 * Playable media is necessary but not sufficient for the dedicated player.
 * Article bodies commonly embed YouTube or Bilibili; only an explicitly
 * video-oriented collection is allowed to replace the article reader.
 */
export function shouldUseRSSVideoLayoutPresentation(
  presentation: RSSCollectionRoute,
  entry: RSSEntry,
) {
  return presentation === "video" && shouldUseRSSVideoPresentation(entry);
}

/**
 * A video grid has one visual grammar for every card. Mixed or legacy pages
 * therefore use the article list, while explicit video entries still open in
 * their native video detail when selected.
 */
export function shouldUseRSSVideoCollectionPresentation(
  entries: readonly RSSEntry[],
) {
  return entries.length === 0 || entries.every(shouldUseRSSVideoPresentation);
}

/**
 * App Session credentials are intentionally not represented as a playback
 * destination. Native players consume them behind the Wails boundary before
 * navigation; the reader always stays on the video page.
 */
export function canonicalRSSVideoTarget(entry: RSSEntry) {
  const originalTarget = (entry.downloadTarget || entry.mediaUrl || entry.url || "").trim();
  const siteKey = [entry.url, entry.downloadTarget, entry.mediaUrl]
    .map((candidate) => siteKeyForRSSVideo(candidate || "", entry.platform))
    .find(Boolean) || "";
  const youtubeVideoID = siteKey === "youtube" ? normalizedYouTubeVideoID(entry) : "";
  if (youtubeVideoID) {
    return `https://www.youtube.com/watch?v=${youtubeVideoID}`;
  }
  const bilibiliIdentity = siteKey === "bilibili"
    ? resolveRSSBilibiliPlaybackIdentity(entry)
    : null;
  if (bilibiliIdentity) {
    return bilibiliIdentity.playbackUrl;
  }
  return originalTarget;
}

export function normalizedYouTubeVideoID(entry: Pick<RSSEntry, "platformVideoId" | "url" | "playbackUrl">) {
  const candidates = [entry.platformVideoId || ""];
  for (const rawURL of [entry.url, entry.playbackUrl]) {
    if (!rawURL) continue;
    try {
      const parsed = new URL(rawURL);
      candidates.push(
        parsed.hostname.endsWith("youtu.be")
          ? parsed.pathname.split("/").filter(Boolean)[0] || ""
          : parsed.searchParams.get("v") || parsed.pathname.match(/\/(?:embed|shorts)\/([^/?#]+)/)?.[1] || "",
      );
    } catch {
      // Ignore invalid feed URLs.
    }
  }
  return candidates.map((candidate) => candidate.trim()).find(
    (candidate) => /^[A-Za-z0-9_-]{11}$/.test(candidate),
  ) || "";
}

export function siteKeyForRSSVideo(rawURL: string, platform?: string) {
  const normalizedPlatform = platform?.trim().toLowerCase() || "";
  // `china_private` was the legacy umbrella value for Douyin and
  // Xiaohongshu. Prefer the URL registry for that one value so archived rows
  // migrate to their independent App Sessions without a data rewrite.
  if (
    normalizedPlatform !== "china_private" &&
    policyForSiteKey(normalizedPlatform)
  ) {
    return normalizedPlatform;
  }
  try {
    const host = new URL(rawURL).hostname.toLowerCase();
    const detected = RSS_VIDEO_PLATFORM_POLICIES.find((policy) =>
      policy.domains.some((domain) => hostMatches(host, domain)),
    )?.siteKey || "";
    return detected || (policyForSiteKey(normalizedPlatform)?.siteKey ?? "");
  } catch {
    return policyForSiteKey(normalizedPlatform)?.siteKey ?? "";
  }
}

function policyForSiteKey(siteKey?: string) {
  return RSS_VIDEO_PLATFORM_POLICIES.find(
    (policy) => policy.siteKey === siteKey,
  );
}

function isAllowedEmbedURL(
  rawURL: string,
  policy: RSSVideoPlatformPolicy | undefined,
) {
  if (!policy?.embedDomains?.length) return false;
  try {
    const parsed = new URL(rawURL);
    if (
      parsed.protocol !== "https:" ||
      parsed.username ||
      parsed.password ||
      (parsed.port && parsed.port !== "443") ||
      !policy.embedDomains.some(
        (domain) => hostMatches(parsed.hostname.toLowerCase(), domain),
      )
    ) {
      return false;
    }
    const pathname = parsed.pathname.replace(/\/+$/, "");
    switch (policy.siteKey) {
      case "youtube":
        return /^\/embed\/[A-Za-z0-9_-]{11}$/.test(pathname);
      case "vimeo":
        return /^\/video\/[1-9]\d*$/.test(pathname);
      default:
        return false;
    }
  } catch {
    return false;
  }
}

function isSafeDirectVideoURL(rawURL: string) {
  if (!controlledRSSResourceURL(rawURL)) return false;
  try {
    const pathname = new URL(rawURL).pathname.toLowerCase();
    return [".mp4", ".m4v", ".webm", ".mov", ".m3u8", ".mpd"].some(
      (extension) => pathname.endsWith(extension),
    );
  } catch {
    return false;
  }
}

/**
 * Last-resort playback keeps a recognized video page interactive. A host
 * allowlist is not video evidence by itself: ordinary articles, profiles, and
 * discovery pages on the same domains stay in the article reader.
 */
function safeRSSVideoSitePageURL(entry: RSSEntry, siteKey: string) {
  const candidates = [
    entry.url,
    entry.playbackUrl,
    entry.downloadTarget,
    entry.mediaUrl,
  ];
  if (!siteKey) return "";
  return candidates.find(
    (candidate) =>
      Boolean(candidate) &&
      siteKeyForRSSVideo(candidate || "") === siteKey &&
      isSafePublicSitePageURL(candidate || "") &&
      isExplicitRSSVideoSitePageURL(candidate || "", siteKey),
  )?.trim() || "";
}

function isExplicitRSSVideoSitePageURL(rawURL: string, siteKey: string) {
  try {
    const parsed = new URL(rawURL);
    const host = parsed.hostname.toLowerCase();
    const segments = parsed.pathname.split("/").filter(Boolean);
    const positiveID = (value = "") => /^[1-9]\d*$/.test(value);
    const stableToken = (value = "") => /^[A-Za-z0-9_-]{3,128}$/.test(value);
    switch (siteKey) {
      case "bilibili":
        return (
          hostMatches(host, "bilibili.com") &&
          segments.length === 3 &&
          segments[0] === "bangumi" &&
          segments[1] === "play" &&
          /^(?:ep|ss)[1-9]\d*$/i.test(segments[2] || "")
        );
      case "youtube":
        return Boolean(normalizedYouTubeVideoID({
          platformVideoId: "",
          url: rawURL,
        }));
      case "vimeo":
        return (
          hostMatches(host, "vimeo.com") &&
          positiveID(segments[segments.length - 1])
        );
      case "tiktok":
        return (
          hostMatches(host, "tiktok.com") &&
          !hostMatches(host, "vm.tiktok.com") &&
          segments.length === 3 &&
          /^@[A-Za-z0-9._]{1,64}$/.test(segments[0] || "") &&
          segments[1] === "video" &&
          positiveID(segments[2])
        );
      case "douyin":
        return (
          hostMatches(host, "douyin.com") &&
          segments.length === 2 &&
          segments[0] === "video" &&
          positiveID(segments[1])
        );
      case "xiaohongshu":
        return (
          hostMatches(host, "xiaohongshu.com") &&
          ((segments.length === 2 && segments[0] === "explore") ||
            (segments.length === 3 &&
              segments[0] === "discovery" &&
              segments[1] === "item")) &&
          /^[a-f0-9]{24}$/i.test(segments[segments.length - 1] || "")
        );
      case "instagram":
        return (
          hostMatches(host, "instagram.com") &&
          segments.length === 2 &&
          segments[0] === "reel" &&
          stableToken(segments[1])
        );
      case "facebook":
        if (host === "fb.watch") {
          return segments.length === 1 && stableToken(segments[0]);
        }
        if (!hostMatches(host, "facebook.com")) return false;
        if (
          segments.length === 2 &&
          segments[0] === "reel" &&
          positiveID(segments[1])
        ) {
          return true;
        }
        return (
          segments.length === 1 &&
          segments[0] === "watch" &&
          parsed.searchParams.getAll("v").length === 1 &&
          positiveID(parsed.searchParams.get("v") || "")
        );
      case "twitch":
        if (host === "clips.twitch.tv") {
          return segments.length === 1 && stableToken(segments[0]);
        }
        return (
          hostMatches(host, "twitch.tv") &&
          segments.length === 2 &&
          segments[0] === "videos" &&
          positiveID(segments[1])
        );
      case "niconico":
        return (
          ((hostMatches(host, "nicovideo.jp") &&
            segments.length === 2 &&
            segments[0] === "watch") ||
            (host === "nico.ms" && segments.length === 1)) &&
          /^(?:sm|nm|so)[1-9]\d*$/i.test(segments[segments.length - 1] || "")
        );
      default:
        return false;
    }
  } catch {
    return false;
  }
}

export function isSafePublicSitePageURL(rawURL: string) {
  try {
    const parsed = new URL(rawURL);
    const host = parsed.hostname.toLowerCase().replace(/^\[|\]$/g, "");
    if (
      parsed.protocol !== "https:" ||
      parsed.username ||
      parsed.password ||
      (parsed.port && parsed.port !== "443") ||
      !host ||
      host === "localhost" ||
      host.endsWith(".localhost") ||
      isIPAddress(host)
    ) {
      return false;
    }
    return host.includes(".");
  } catch {
    return false;
  }
}

function isIPAddress(host: string) {
  if (host.includes(":")) return true;
  const octets = host.split(".");
  return octets.length === 4 && octets.every((octet) => {
    if (!/^\d{1,3}$/.test(octet)) return false;
    const value = Number(octet);
    return value >= 0 && value <= 255;
  });
}

function hostMatches(host: string, domain: string) {
  return host === domain || host.endsWith(`.${domain}`);
}

export function buildRSSVideoBatchDownloadTargets(
  entries: readonly RSSEntry[],
): RSSVideoBatchDownloadTarget[] {
  const seen = new Set<string>();
  return entries
    .filter(shouldUseRSSVideoPresentation)
    .map((entry) => ({
      entryId: entry.id,
      url: canonicalRSSVideoTarget(entry),
      source: "xiadown.rss" as const,
      caller: `rss-entry:${entry.id}`,
    }))
    .filter((target) => {
      if (!target.url || seen.has(target.url)) return false;
      seen.add(target.url);
      return true;
    });
}

/** Kept for ordinary multiline batch-dialog callers. */
export function buildRSSVideoBatchDownloadTarget(entries: readonly RSSEntry[]) {
  return buildRSSVideoBatchDownloadTargets(entries)
    .map((target) => target.url)
    .join("\n");
}
