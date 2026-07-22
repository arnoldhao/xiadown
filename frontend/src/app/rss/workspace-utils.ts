import type { InfiniteData } from "@tanstack/react-query";

import type {
  RSSEntry,
  RSSEntryPage,
  RSSEntryState,
  RSSSubscription,
} from "./types";
import {
  controlledRSSEntryImageResource,
  controlledRSSResourceOrigin,
  controlledRSSResourceURL,
} from "./remote-resource";
import rssDocumentStyles from "../../shared/styles/dream/rss-documents.css?raw";

export type RSSCollectionRoute = "all" | "article" | "social" | "image" | "video";
export type RSSSubscriptionSort = "title" | "updated" | "unread";

export interface RSSIdentityScopedBoolean {
  identity: string;
  enabled: boolean;
}

export const RSS_READER_PROGRESS_CHANNEL = "xiadown-rss-reader-v1";
export const RSS_FOCUSED_IMAGE_LIMIT = 12;
export const RSS_READER_MAX_DOCUMENT_HEIGHT = 2_000_000;
export const RSS_READER_MAX_WHEEL_DELTA = 10_000;
export const RSS_READER_MAX_OUTLINE_ITEMS = 128;
export const RSS_READER_MAX_LAYOUT_SHIFTS = 128;
export const RSS_READER_MAX_VIDEO_EMBEDS = 16;
export const RSS_READER_OUTLINE_ID_PREFIX = "xiadown-rss-reader-heading-";
export const RSS_READER_OUTLINE_MIN_MARKER_WIDTH = 0.32;
export const RSS_READER_PREFERENCES_STORAGE_KEY = "xiadown:rss:reader-preferences:v1";

export type RSSReaderOpenMode = "reader" | "original";
export type RSSReaderFontSize = "small" | "medium" | "large";
export type RSSReaderDensity = "compact" | "comfortable" | "relaxed";

export interface RSSReaderPreferences {
  autoMarkRead: boolean;
  openMode: RSSReaderOpenMode;
  fontSize: RSSReaderFontSize;
  density: RSSReaderDensity;
}

export const DEFAULT_RSS_READER_PREFERENCES: Readonly<RSSReaderPreferences> = {
  autoMarkRead: true,
  openMode: "reader",
  fontSize: "medium",
  density: "comfortable",
};

export type RSSWorkspaceShortcut =
  | "previous-entry"
  | "next-entry"
  | "open-entry"
  | "close-entry"
  | "toggle-read"
  | "toggle-starred"
  | "toggle-unread-filter"
  | "refresh";

export interface RSSAudioPresentation {
  url: string;
  mimeType: string;
  durationMs: number;
}

export interface RSSReaderProgressMessage {
  channel: typeof RSS_READER_PROGRESS_CHANNEL;
  type: "progress";
  entryId: string;
  fraction: number;
  anchor: string;
}

export interface RSSReaderLayoutMessage {
  channel: typeof RSS_READER_PROGRESS_CHANNEL;
  type: "layout";
  entryId: string;
  documentId?: string;
  sequence?: number;
  height: number;
  shifts?: RSSReaderLayoutShift[];
  embeds?: RSSReaderVideoEmbedLayout[];
}

export interface RSSReaderLayoutShift {
  /** Document-space start and the old trailing edge before this reflow. */
  top: number;
  bottom: number;
  /** Signed block-size delta applied to content after bottom. */
  delta: number;
}

export type RSSReaderVideoEmbedProvider = "youtube" | "vimeo" | "bilibili";

export interface RSSReaderVideoEmbedIdentity {
  provider: RSSReaderVideoEmbedProvider;
  videoId: string;
}

export interface RSSReaderVideoEmbedActionLayout {
  top: number;
  left: number;
  width: number;
  height: number;
}

export interface RSSReaderVideoEmbedLayout extends RSSReaderVideoEmbedIdentity {
  title?: string;
  top: number;
  left: number;
  width: number;
  height: number;
  action: RSSReaderVideoEmbedActionLayout;
}

export interface RSSReaderWheelMessage {
  channel: typeof RSS_READER_PROGRESS_CHANNEL;
  type: "wheel";
  entryId: string;
  deltaY: number;
  deltaMode: 0 | 1 | 2;
}

export interface RSSReaderSelectionMessage {
  channel: typeof RSS_READER_PROGRESS_CHANNEL;
  type: "selection";
  entryId: string;
  active: boolean;
  clientY: number;
  screenY: number;
}

export interface RSSReaderLinkMessage {
  channel: typeof RSS_READER_PROGRESS_CHANNEL;
  type: "link";
  entryId: string;
  url: string;
}

export interface RSSReaderImageContextMessage {
  channel: typeof RSS_READER_PROGRESS_CHANNEL;
  type: "image-context";
  entryId: string;
  documentId: string;
  slot: string;
  alt: string;
  clientX: number;
  clientY: number;
}

export interface RSSReaderOutlineItem {
  id: string;
  title: string;
  depth: 1 | 2 | 3 | 4 | 5 | 6;
  top: number;
  /** Visible text assigned to this heading, up to the next outline heading. */
  textLength?: number;
}

export interface RSSReaderOutlineMessage {
  channel: typeof RSS_READER_PROGRESS_CHANNEL;
  type: "outline";
  entryId: string;
  items: RSSReaderOutlineItem[];
}

export interface RSSReaderOutlineMarkerMetric {
  index: number;
  /** Text length when available; otherwise the section's document-height span. */
  contentLength: number;
  /** 0..1 inline-size multiplier. The longest section is always 1. */
  widthFraction: number;
}

export interface RSSReaderOutlineMarker {
  active: boolean;
  contentLength: number;
  /** Exclusive original-outline boundary represented by this marker. */
  endIndex: number;
  fillFraction: number;
  item: RSSReaderOutlineItem;
  startIndex: number;
  widthFraction: number;
}

export interface RSSReaderDocumentSnapshot {
  key: string;
  documentId: string;
  document: string;
}

export interface RSSArticlePrintMetadata {
  source?: string;
  author?: string;
  published?: string;
}

export function normalizeRSSReaderPreferences(
  value: unknown,
): RSSReaderPreferences {
  if (!value || typeof value !== "object") {
    return { ...DEFAULT_RSS_READER_PREFERENCES };
  }
  const candidate = value as Partial<RSSReaderPreferences>;
  return {
    autoMarkRead: typeof candidate.autoMarkRead === "boolean"
      ? candidate.autoMarkRead
      : DEFAULT_RSS_READER_PREFERENCES.autoMarkRead,
    openMode: candidate.openMode === "original" ? "original" : "reader",
    fontSize:
      candidate.fontSize === "small" || candidate.fontSize === "large"
        ? candidate.fontSize
        : "medium",
    density:
      candidate.density === "compact" || candidate.density === "relaxed"
        ? candidate.density
        : "comfortable",
  };
}

export function readRSSReaderPreferences(): RSSReaderPreferences {
  if (typeof window === "undefined") {
    return { ...DEFAULT_RSS_READER_PREFERENCES };
  }
  try {
    return normalizeRSSReaderPreferences(JSON.parse(
      window.localStorage.getItem(RSS_READER_PREFERENCES_STORAGE_KEY) || "null",
    ));
  } catch {
    return { ...DEFAULT_RSS_READER_PREFERENCES };
  }
}

export function writeRSSReaderPreferences(preferences: RSSReaderPreferences) {
  if (typeof window === "undefined") return;
  try {
    window.localStorage.setItem(
      RSS_READER_PREFERENCES_STORAGE_KEY,
      JSON.stringify(normalizeRSSReaderPreferences(preferences)),
    );
  } catch {
    // The active session still keeps the preference when storage is unavailable.
  }
}

export function resolveRSSWorkspaceShortcut({
  key,
  altKey = false,
  ctrlKey = false,
  metaKey = false,
  shiftKey = false,
  repeat = false,
  isComposing = false,
}: Pick<KeyboardEvent, "key"> & Partial<Pick<KeyboardEvent,
  "altKey" | "ctrlKey" | "metaKey" | "shiftKey" | "repeat" | "isComposing"
>>): RSSWorkspaceShortcut | null {
  if (altKey || ctrlKey || metaKey || shiftKey || repeat || isComposing) return null;
  switch (key.toLocaleLowerCase()) {
    case "k": return "previous-entry";
    case "j": return "next-entry";
    case "enter": return "open-entry";
    case "escape": return "close-entry";
    case "m": return "toggle-read";
    case "s": return "toggle-starred";
    case "u": return "toggle-unread-filter";
    case "r": return "refresh";
    default: return null;
  }
}

export function resolveRSSAudioPresentation(
  entry: Pick<RSSEntry, "media" | "mediaUrl" | "mediaType">,
): RSSAudioPresentation | null {
  const media = entry.media.find((item) => (
    item.kind.toLocaleLowerCase() === "audio" ||
    item.mimeType?.toLocaleLowerCase().startsWith("audio/")
  ));
  const fallbackIsAudio = entry.mediaType?.toLocaleLowerCase().startsWith("audio/");
  const rawURL = media?.url || (fallbackIsAudio ? entry.mediaUrl : "") || "";
  const url = controlledRSSResourceURL(rawURL);
  if (!url) return null;
  return {
    url,
    mimeType: media?.mimeType || (fallbackIsAudio ? entry.mediaType : "") || "audio/mpeg",
    durationMs: Math.max(0, Math.trunc(media?.durationMs || 0)),
  };
}

/**
 * Speech follows the article body instead of serialised feed metadata. Some
 * feeds repeat an entity-escaped copy of the body in summary; treating that as
 * a separate prefix makes screen-reader speech announce markup and can consume
 * the utterance limit before the real article is reached.
 */
export function rssReaderSpeechText(
  entry: Pick<RSSEntry, "contentHtml" | "summary" | "title">,
  maximumLength = 50_000,
) {
  const boundedMaximum = Number.isFinite(maximumLength)
    ? Math.max(0, Math.trunc(maximumLength))
    : 50_000;
  if (boundedMaximum === 0) return "";
  const body = readableRSSHTMLText(entry.contentHtml);
  const summary = body ? "" : readableRSSHTMLText(entry.summary);
  const title = body || summary ? "" : readableRSSHTMLText(entry.title);
  return (body || summary || title).slice(0, boundedMaximum);
}

function readableRSSHTMLText(markup?: string) {
  const source = markup?.trim() || "";
  if (!source) return "";
  if (typeof DOMParser === "undefined") {
    const text = source
      .replace(/<(?:script|style|noscript|template|object|embed)\b[^>]*>[\s\S]*?<\/\s*(?:script|style|noscript|template|object)>/gi, " ")
      .replace(/<[^>]+>/g, " ")
      .replace(/\s+/g, " ")
      .trim();
    return decodeRSSSpeechEntities(text)
      .replace(/\s+/g, " ")
      .trim();
  }
  const document = new DOMParser().parseFromString(source, "text/html");
  document.querySelectorAll(
    "script,style,noscript,template,object,embed,svg,[hidden],[aria-hidden='true']",
  ).forEach((node) => node.remove());
  document.querySelectorAll(
    "address,article,aside,blockquote,br,caption,dd,div,dl,dt,figcaption,figure,footer,h1,h2,h3,h4,h5,h6,header,hr,li,main,nav,ol,p,pre,section,table,tbody,td,tfoot,th,thead,tr,ul",
  ).forEach((node) => node.append(" "));
  return document.body.textContent?.replace(/\s+/g, " ").trim() || "";
}

function decodeRSSSpeechEntities(value: string) {
  return value.replace(
    /&(?:#(\d{1,7})|#x([0-9a-f]{1,6})|amp|lt|gt|quot|apos|nbsp);/gi,
    (entity, decimal: string | undefined, hexadecimal: string | undefined) => {
      if (decimal || hexadecimal) {
        const codePoint = Number.parseInt(decimal || hexadecimal || "", hexadecimal ? 16 : 10);
        if (
          !Number.isFinite(codePoint) ||
          codePoint <= 0 ||
          codePoint > 0x10ffff ||
          (codePoint >= 0xd800 && codePoint <= 0xdfff)
        ) {
          return " ";
        }
        return String.fromCodePoint(codePoint);
      }
      switch (entity.toLocaleLowerCase()) {
        case "&amp;": return "&";
        case "&lt;": return "<";
        case "&gt;": return ">";
        case "&quot;": return '"';
        case "&apos;": return "'";
        case "&nbsp;": return " ";
        default: return entity;
      }
    },
  );
}

/**
 * Keeps transient reader/filter controls attached to the route or entry that
 * created them. A new identity therefore reads as disabled during its first
 * render instead of inheriting the previous screen until an effect runs.
 */
export function readRSSIdentityScopedBoolean(
  state: RSSIdentityScopedBoolean,
  identity: string,
) {
  return Boolean(identity && state.identity === identity && state.enabled);
}

export function setRSSIdentityScopedBoolean(
  identity: string,
  enabled: boolean,
): RSSIdentityScopedBoolean {
  return { identity, enabled };
}

export function toggleRSSIdentityScopedBoolean(
  state: RSSIdentityScopedBoolean,
  identity: string,
): RSSIdentityScopedBoolean {
  return setRSSIdentityScopedBoolean(
    identity,
    state.identity === identity ? !state.enabled : true,
  );
}

export function mergeRSSEntryPages(pages: readonly RSSEntryPage[]) {
  const entries = new Map<string, RSSEntry>();
  for (const page of pages) {
    for (const entry of page.items) entries.set(entry.id, entry);
  }
  return [...entries.values()];
}

/**
 * Dedicated video playback is an explicit collection preference. An auto
 * subscription may contain an article with an embedded player; a classifier
 * must not turn that article into a video-only screen merely because video
 * metadata was extracted from its body.
 */
export function resolveRSSCollectionPresentation(
  collectionRoute: RSSCollectionRoute,
  subscription?: RSSSubscription,
): RSSCollectionRoute {
  if (!subscription) return collectionRoute;
  switch (subscription.viewType) {
    case "article":
    case "social":
    case "image":
    case "video":
      return subscription.viewType;
  }
  switch (subscription.resolvedViewType) {
    case "article":
    case "social":
    case "image":
      return subscription.resolvedViewType;
    case "video":
      return "article";
    default:
      return "all";
  }
}

export function boundedRSSEntryImages(
  entry: RSSEntry,
  limit = RSS_FOCUSED_IMAGE_LIMIT,
) {
  const boundedLimit = Math.max(0, Math.min(RSS_FOCUSED_IMAGE_LIMIT, Math.floor(limit)));
  return [...new Set([
    // Full content-image slots accept the source's original dimensions. Try
    // them before thumbnail slots, whose stricter pixel budget can reject a
    // perfectly valid portrait even when the same image is available below.
    ...entry.imageUrls,
    ...entry.media
      .filter((item) => item.kind === "image")
      .map((item) => item.url),
    entry.thumbnailUrl,
    ...entry.media.map((item) => item.thumbnail),
  ].filter((item): item is string => Boolean(item)))]
    .slice(0, boundedLimit);
}

export function rssEntryImageCandidates(entry: RSSEntry) {
  return [...new Set([
    entry.thumbnailUrl,
    ...entry.media.map((item) => item.thumbnail),
    ...entry.imageUrls,
    ...entry.media
      .filter((item) => item.kind === "image")
      .map((item) => item.url),
  ].filter((item): item is string => Boolean(item)))]
    .slice(0, RSS_FOCUSED_IMAGE_LIMIT);
}

/**
 * Native players can briefly publish zero before an asynchronous resume seek
 * lands. Keep the persisted position until the player reaches that vicinity,
 * while allowing a failed seek to recover after a short grace period.
 */
export function shouldAcceptRSSResumedPlaybackProgress(
  currentTime: number,
  resumeAt: number,
  expiresAt: number,
  now = Date.now(),
) {
  if (now >= expiresAt) return true;
  return Math.max(0, Number(currentTime) || 0) + 2 >= Math.max(0, resumeAt);
}

export function filterAndSortRSSSubscriptions(
  subscriptions: readonly RSSSubscription[],
  query: string,
  sort: RSSSubscriptionSort,
  language?: string,
) {
  const normalizedQuery = query.trim().toLocaleLowerCase(language);
  const filtered = subscriptions.filter((subscription) => {
    if (!normalizedQuery) return true;
    return [
      subscription.title,
      subscription.feedUrl,
      subscription.siteUrl,
      subscription.description,
    ].some((value) => value?.toLocaleLowerCase(language).includes(normalizedQuery));
  });
  return [...filtered].sort((left, right) => {
    if (sort === "unread") {
      return right.unreadCount - left.unreadCount || compareSubscriptionTitles(left, right, language);
    }
    if (sort === "updated") {
      return subscriptionTimestamp(right) - subscriptionTimestamp(left) || compareSubscriptionTitles(left, right, language);
    }
    return compareSubscriptionTitles(left, right, language);
  });
}

export function resolveRSSReaderVideoEmbed(
  entry: RSSEntry,
): RSSReaderVideoEmbedIdentity | null {
  const platformIdentity = normalizedRSSReaderVideoEmbedIdentity(
    entry.platform,
    entry.platformVideoId,
  );
  if (platformIdentity) return platformIdentity;
  for (const candidate of [
    entry.playbackUrl,
    entry.url,
    ...entry.media
      .filter((item) => item.kind === "video" && item.mimeType === "text/html")
      .map((item) => item.url),
  ]) {
    const identity = rssReaderVideoEmbedIdentityFromURL(candidate);
    if (identity) return identity;
  }
  return null;
}

export function rssReaderVideoEmbedURL(
  value: RSSReaderVideoEmbedIdentity,
) {
  const identity = normalizedRSSReaderVideoEmbedIdentity(
    value.provider,
    value.videoId,
  );
  if (!identity) return "";
  switch (identity.provider) {
    case "youtube":
      return `https://www.youtube-nocookie.com/embed/${encodeURIComponent(identity.videoId)}`;
    case "vimeo":
      return `https://player.vimeo.com/video/${encodeURIComponent(identity.videoId)}`;
    case "bilibili":
      return identity.videoId.startsWith("BV")
        ? `https://player.bilibili.com/player.html?bvid=${encodeURIComponent(identity.videoId)}`
        : `https://player.bilibili.com/player.html?aid=${encodeURIComponent(identity.videoId.slice(2))}`;
  }
  return "";
}

export function rssReaderVideoDownloadURL(
  value: RSSReaderVideoEmbedIdentity,
) {
  const identity = normalizedRSSReaderVideoEmbedIdentity(
    value.provider,
    value.videoId,
  );
  if (!identity) return "";
  switch (identity.provider) {
    case "youtube":
      return `https://www.youtube.com/watch?v=${encodeURIComponent(identity.videoId)}`;
    case "vimeo":
      return `https://vimeo.com/${encodeURIComponent(identity.videoId)}`;
    case "bilibili":
      return `https://www.bilibili.com/video/${encodeURIComponent(identity.videoId)}/`;
  }
  return "";
}

function rssReaderContentWithVideoFallback(entry: RSSEntry) {
  const content = entry.contentHtml || `<p>${escapeHTML(entry.summary || "")}</p>`;
  if (
    /data-xiadown-rss-video-provider\s*=/i.test(content) ||
    /<video(?:\s|>)/i.test(content)
  ) {
    return content;
  }
  const embed = resolveRSSReaderVideoEmbed(entry);
  if (embed) {
    // Older rows were stored after iframe removal, so their exact source
    // position cannot be recovered. Keep those rows useful by reserving one
    // article-level player before the preserved body; newly fetched rows carry
    // an inline marker at the original position and never take this fallback.
    return `<figure data-xiadown-rss-video-provider="${embed.provider}" data-xiadown-rss-video-id="${embed.videoId}"></figure>${content}`;
  }
  const direct = rssReaderDirectVideoFallback(entry);
  if (!direct) return content;
  const poster = rssEntryImageCandidates(entry)
    .map(controlledRSSResourceURL)
    .find(Boolean) || "";
  return `<figure><video controls playsinline preload="metadata" data-xiadown-rss-video-fallback="true" src="${escapeHTMLAttribute(direct)}"${poster ? ` poster="${escapeHTMLAttribute(poster)}"` : ""}></video></figure>${content}`;
}

function rssReaderDirectVideoFallback(entry: RSSEntry) {
  for (const media of entry.media) {
    if (media.kind !== "video" || media.mimeType === "text/html") continue;
    const controlled = controlledRSSResourceURL(media.url);
    if (controlled) return controlled;
  }
  if (!entry.mediaType?.toLowerCase().startsWith("video/")) return "";
  return controlledRSSResourceURL(entry.mediaUrl);
}

function normalizedRSSReaderVideoEmbedIdentity(
  rawProvider: string | undefined,
  rawVideoID: string | undefined,
): RSSReaderVideoEmbedIdentity | null {
  const provider = rawProvider?.trim().toLowerCase() || "";
  const videoId = rawVideoID?.trim() || "";
  if (provider === "youtube" && /^[A-Za-z0-9_-]{11}$/.test(videoId)) {
    return { provider, videoId };
  }
  if (provider === "vimeo" && /^[1-9]\d{0,19}$/.test(videoId)) {
    return { provider, videoId };
  }
  if (provider === "bilibili") {
    if (/^BV[A-Za-z0-9]{10}$/i.test(videoId)) {
      return { provider, videoId: `BV${videoId.slice(2)}` };
    }
    const av = /^av([1-9]\d{0,19})$/i.exec(videoId);
    if (av) return { provider, videoId: `av${av[1]}` };
  }
  return null;
}

function rssReaderVideoEmbedIdentityFromURL(
  rawURL: string | undefined,
): RSSReaderVideoEmbedIdentity | null {
  if (!rawURL) return null;
  try {
    const parsed = new URL(rawURL);
    if (
      parsed.protocol !== "https:" ||
      parsed.username ||
      parsed.password ||
      (parsed.port && parsed.port !== "443") ||
      /%[0-9a-f]{2}/i.test(parsed.pathname)
    ) {
      return null;
    }
    const host = parsed.hostname.toLowerCase().replace(/\.$/, "");
    const segments = parsed.pathname.split("/").filter(Boolean);
    if (
      ["youtube.com", "www.youtube.com", "youtube-nocookie.com", "www.youtube-nocookie.com"].includes(host)
    ) {
      const videoId =
        segments.length === 2 && (segments[0] === "embed" || segments[0] === "shorts")
          ? segments[1]
          : segments.length === 1 && segments[0] === "watch"
          ? parsed.searchParams.get("v") || undefined
          : undefined;
      return normalizedRSSReaderVideoEmbedIdentity("youtube", videoId);
    }
    if (host === "youtu.be" && segments.length === 1) {
      return normalizedRSSReaderVideoEmbedIdentity("youtube", segments[0]);
    }
    if (host === "player.vimeo.com" && segments.length === 2 && segments[0] === "video") {
      return normalizedRSSReaderVideoEmbedIdentity("vimeo", segments[1]);
    }
    if (host === "vimeo.com" && segments.length === 1) {
      return normalizedRSSReaderVideoEmbedIdentity("vimeo", segments[0]);
    }
    if (host === "player.bilibili.com" && segments.join("/") === "player.html") {
      const bvid = parsed.searchParams.get("bvid") || undefined;
      if (bvid) return normalizedRSSReaderVideoEmbedIdentity("bilibili", bvid);
      const aid = parsed.searchParams.get("aid") || undefined;
      return normalizedRSSReaderVideoEmbedIdentity(
        "bilibili",
        aid ? `av${aid}` : undefined,
      );
    }
    if (
      (host === "bilibili.com" || host === "www.bilibili.com") &&
      segments.length === 2 && segments[0] === "video"
    ) {
      return normalizedRSSReaderVideoEmbedIdentity("bilibili", segments[1]);
    }
  } catch {
    return null;
  }
  return null;
}

export function buildRSSReaderDocument(
  entry: RSSEntry,
  theme: "light" | "dark",
  sourceURL?: string,
  preferences: RSSReaderPreferences = { ...DEFAULT_RSS_READER_PREFERENCES },
) {
  return buildRSSReaderDocumentSource(
    entry,
    theme,
    sourceURL,
    preferences,
    newReaderNonce(),
  );
}

function buildRSSReaderDocumentSource(
  entry: RSSEntry,
  theme: "light" | "dark",
  sourceURL: string | undefined,
  preferences: RSSReaderPreferences,
  documentId: string,
) {
  const normalizedPreferences = normalizeRSSReaderPreferences(preferences);
  const content = rssReaderContentWithVideoFallback(entry);
  const baseURL = safeReaderBaseURL(entry.url || sourceURL);
  const scriptNonce = newReaderNonce();
  const resourceOrigin = controlledRSSResourceOrigin([
    entry.thumbnailUrl,
    ...entry.imageUrls,
    ...entry.media.flatMap((item) => [item.url, item.thumbnail]),
  ]);
  const imageSources = resourceOrigin ? `${resourceOrigin} data:` : "data:";
  const fontSize = normalizedPreferences.fontSize === "small"
    ? 15
    : normalizedPreferences.fontSize === "large"
      ? 18
      : 16;
  const lineHeight = normalizedPreferences.density === "compact"
    ? 1.58
    : normalizedPreferences.density === "relaxed"
      ? 1.9
      : 1.75;
  const paragraphSpacing = normalizedPreferences.density === "compact"
    ? 1
    : normalizedPreferences.density === "relaxed"
      ? 1.5
      : 1.25;
  const readerStyleProperties = [
    `--app-rss-reader-font-size:${fontSize}px`,
    `--app-rss-reader-line-height:${lineHeight}`,
    `--app-rss-reader-paragraph-spacing:${paragraphSpacing}em`,
  ].join(";");
  return `<!doctype html><html class="app-rss-reader-document" data-theme="${theme}" style="${readerStyleProperties}"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width"><meta name="referrer" content="no-referrer"><meta http-equiv="Content-Security-Policy" content="default-src 'none'; img-src ${imageSources}; media-src ${imageSources}; frame-src 'none'; style-src 'unsafe-inline'; font-src 'none'; script-src 'nonce-${scriptNonce}'; object-src 'none'; form-action 'none'"><base${baseURL ? ` href="${escapeHTMLAttribute(baseURL)}"` : ""} target="_blank"><style>${rssDocumentStyles}</style></head><body>${content}<script nonce="${scriptNonce}">${readerLayoutScript(entry.id, documentId, rssReaderImageDimensionHints(entry))}</script></body></html>`;
}

/**
 * Builds a dedicated article-only print surface. The application shell never
 * enters this document, and the same projected-resource CSP used by the reader
 * prevents a feed from loading arbitrary third-party media while printing.
 */
export function buildRSSArticlePrintDocument(
  entry: RSSEntry,
  theme: "light" | "dark",
  sourceURL?: string,
  metadata: RSSArticlePrintMetadata = {},
) {
  const content = entry.contentHtml || `<p>${escapeHTML(entry.summary || "")}</p>`;
  const baseURL = safeReaderBaseURL(entry.url || sourceURL);
  const resourceOrigin = controlledRSSResourceOrigin([
    entry.thumbnailUrl,
    ...entry.imageUrls,
    ...entry.media.flatMap((item) => [item.url, item.thumbnail]),
  ]);
  const imageSources = resourceOrigin ? `${resourceOrigin} data:` : "data:";
  const byline = [metadata.source, metadata.author, metadata.published]
    .filter((value): value is string => Boolean(value))
    .map(escapeHTML)
    .join(" · ");
  return `<!doctype html><html class="app-rss-print-document" data-theme="${theme}"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width"><meta name="referrer" content="no-referrer"><meta http-equiv="Content-Security-Policy" content="default-src 'none'; img-src ${imageSources}; media-src ${imageSources}; frame-src 'none'; style-src 'unsafe-inline'; font-src 'none'; script-src 'none'; object-src 'none'; form-action 'none'"><base${baseURL ? ` href="${escapeHTMLAttribute(baseURL)}"` : ""} target="_blank"><title>${escapeHTML(entry.title)}</title><style>${rssDocumentStyles}</style></head><body><article><header><h1>${escapeHTML(entry.title)}</h1>${byline ? `<div class="rss-print-byline">${byline}</div>` : ""}</header><main>${content}</main></article></body></html>`;
}

/**
 * Keeps the random CSP nonce and srcDoc stable while only synchronized state is
 * changing. Replacing srcDoc navigates the iframe, so progress writes must not
 * cause the reader to reload itself and reset its scroll position.
 */
export function resolveRSSReaderDocumentSnapshot(
  current: RSSReaderDocumentSnapshot | null,
  entry: RSSEntry,
  theme: "light" | "dark",
  sourceURL?: string,
  preferences: RSSReaderPreferences = { ...DEFAULT_RSS_READER_PREFERENCES },
): RSSReaderDocumentSnapshot {
  const normalizedPreferences = normalizeRSSReaderPreferences(preferences);
  const key = JSON.stringify([
    entry.id,
    Math.max(1, Math.trunc(entry.revision)),
    theme,
    entry.url || "",
    sourceURL || "",
    normalizedPreferences,
  ]);
  if (current?.key === key) return current;
  const documentId = newReaderNonce();
  return {
    key,
    documentId,
    document: buildRSSReaderDocumentSource(
      entry,
      theme,
      sourceURL,
      normalizedPreferences,
      documentId,
    ),
  };
}

export function readRSSReaderProgressMessage(
  value: unknown,
  entryId: string,
): RSSReaderProgressMessage | null {
  if (!value || typeof value !== "object") return null;
  const candidate = value as Partial<RSSReaderProgressMessage>;
  if (
    candidate.channel !== RSS_READER_PROGRESS_CHANNEL ||
    candidate.type !== "progress" ||
    candidate.entryId !== entryId ||
    typeof candidate.fraction !== "number" ||
    !Number.isFinite(candidate.fraction) ||
    candidate.fraction < 0 ||
    candidate.fraction > 1 ||
    typeof candidate.anchor !== "string" ||
    candidate.anchor.length > 256
  ) {
    return null;
  }
  return candidate as RSSReaderProgressMessage;
}

export function readRSSReaderLayoutMessage(
  value: unknown,
  entryId: string,
  expectedDocumentId?: string,
): RSSReaderLayoutMessage | null {
  if (!value || typeof value !== "object") return null;
  const candidate = value as Partial<RSSReaderLayoutMessage>;
  if (
    candidate.channel !== RSS_READER_PROGRESS_CHANNEL ||
    candidate.type !== "layout" ||
    candidate.entryId !== entryId ||
    (expectedDocumentId !== undefined &&
      candidate.documentId !== expectedDocumentId) ||
    (candidate.documentId !== undefined && (
      typeof candidate.documentId !== "string" ||
      candidate.documentId.length < 1 ||
      candidate.documentId.length > 256
    )) ||
    (candidate.sequence !== undefined && (
      !Number.isSafeInteger(candidate.sequence) ||
      candidate.sequence < 1
    )) ||
    typeof candidate.height !== "number" ||
    !Number.isFinite(candidate.height) ||
    candidate.height < 1 ||
    candidate.height > RSS_READER_MAX_DOCUMENT_HEIGHT
  ) {
    return null;
  }
  const rawShifts = (candidate as { shifts?: unknown }).shifts;
  if (
    rawShifts !== undefined &&
    (!Array.isArray(rawShifts) ||
      rawShifts.length > RSS_READER_MAX_LAYOUT_SHIFTS)
  ) {
    return null;
  }
  const shifts: RSSReaderLayoutShift[] = [];
  const rawShiftItems = Array.isArray(rawShifts) ? rawShifts : [];
  for (const rawShift of rawShiftItems) {
    if (!rawShift || typeof rawShift !== "object") return null;
    const shift = rawShift as Partial<RSSReaderLayoutShift>;
    if (
      typeof shift.top !== "number" ||
      !Number.isFinite(shift.top) ||
      shift.top < 0 ||
      shift.top > RSS_READER_MAX_DOCUMENT_HEIGHT ||
      typeof shift.bottom !== "number" ||
      !Number.isFinite(shift.bottom) ||
      shift.bottom < shift.top ||
      shift.bottom > RSS_READER_MAX_DOCUMENT_HEIGHT ||
      typeof shift.delta !== "number" ||
      !Number.isFinite(shift.delta) ||
      Math.abs(shift.delta) > RSS_READER_MAX_DOCUMENT_HEIGHT
    ) {
      return null;
    }
    shifts.push({
      top: shift.top,
      bottom: shift.bottom,
      delta: shift.delta,
    });
  }
  shifts.sort((left, right) => left.top - right.top || left.bottom - right.bottom);
  const rawEmbeds = (candidate as { embeds?: unknown }).embeds;
  if (
    rawEmbeds !== undefined &&
    (!Array.isArray(rawEmbeds) ||
      rawEmbeds.length > RSS_READER_MAX_VIDEO_EMBEDS)
  ) {
    return null;
  }
  const embeds: RSSReaderVideoEmbedLayout[] = [];
  const rawEmbedItems = Array.isArray(rawEmbeds) ? rawEmbeds : [];
  for (const rawEmbed of rawEmbedItems) {
    if (!rawEmbed || typeof rawEmbed !== "object") return null;
    const embed = rawEmbed as Partial<RSSReaderVideoEmbedLayout>;
    const rawAction = (rawEmbed as { action?: unknown }).action;
    if (!rawAction || typeof rawAction !== "object" || Array.isArray(rawAction)) {
      return null;
    }
    const action = rawAction as Partial<RSSReaderVideoEmbedActionLayout>;
    const identity = normalizedRSSReaderVideoEmbedIdentity(
      typeof embed.provider === "string" ? embed.provider : undefined,
      typeof embed.videoId === "string" ? embed.videoId : undefined,
    );
    if (
      !identity ||
      (embed.title !== undefined && (
        typeof embed.title !== "string" || embed.title.length > 512
      )) ||
      typeof embed.top !== "number" ||
      !Number.isFinite(embed.top) ||
      embed.top < 0 ||
      embed.top > RSS_READER_MAX_DOCUMENT_HEIGHT ||
      typeof embed.left !== "number" ||
      !Number.isFinite(embed.left) ||
      embed.left < 0 ||
      embed.left > RSS_READER_MAX_DOCUMENT_HEIGHT ||
      typeof embed.width !== "number" ||
      !Number.isFinite(embed.width) ||
      embed.width < 1 ||
      embed.width > RSS_READER_MAX_DOCUMENT_HEIGHT ||
      typeof embed.height !== "number" ||
      !Number.isFinite(embed.height) ||
      embed.height < 1 ||
      embed.height > RSS_READER_MAX_DOCUMENT_HEIGHT ||
      embed.left + embed.width > RSS_READER_MAX_DOCUMENT_HEIGHT ||
      embed.top + embed.height > RSS_READER_MAX_DOCUMENT_HEIGHT ||
      embed.top + embed.height > candidate.height ||
      typeof action.top !== "number" ||
      !Number.isFinite(action.top) ||
      action.top < 0 ||
      action.top > RSS_READER_MAX_DOCUMENT_HEIGHT ||
      typeof action.left !== "number" ||
      !Number.isFinite(action.left) ||
      action.left < 0 ||
      action.left > RSS_READER_MAX_DOCUMENT_HEIGHT ||
      typeof action.width !== "number" ||
      !Number.isFinite(action.width) ||
      action.width < 1 ||
      action.width > RSS_READER_MAX_DOCUMENT_HEIGHT ||
      typeof action.height !== "number" ||
      !Number.isFinite(action.height) ||
      action.height < 1 ||
      action.height > 256 ||
      action.left !== embed.left ||
      action.width !== embed.width ||
      action.top < embed.top + embed.height ||
      action.top - (embed.top + embed.height) > 256 ||
      action.left + action.width > RSS_READER_MAX_DOCUMENT_HEIGHT ||
      action.top + action.height > RSS_READER_MAX_DOCUMENT_HEIGHT ||
      action.top + action.height > candidate.height
    ) {
      return null;
    }
    embeds.push({
      ...identity,
      ...(embed.title?.trim() ? { title: embed.title.trim() } : {}),
      top: embed.top,
      left: embed.left,
      width: embed.width,
      height: embed.height,
      action: {
        top: action.top,
        left: action.left,
        width: action.width,
        height: action.height,
      },
    });
  }
  embeds.sort((left, right) => left.top - right.top || left.left - right.left);
  return {
    channel: RSS_READER_PROGRESS_CHANNEL,
    type: "layout",
    entryId,
    ...(candidate.documentId === undefined
      ? {}
      : { documentId: candidate.documentId }),
    ...(candidate.sequence === undefined
      ? {}
      : { sequence: candidate.sequence }),
    height: Math.ceil(candidate.height),
    ...(rawShifts === undefined ? {} : { shifts }),
    ...(rawEmbeds === undefined ? {} : { embeds }),
  };
}

export function readRSSReaderWheelMessage(
  value: unknown,
  entryId: string,
): RSSReaderWheelMessage | null {
  if (!value || typeof value !== "object") return null;
  const candidate = value as Partial<RSSReaderWheelMessage>;
  if (
    candidate.channel !== RSS_READER_PROGRESS_CHANNEL ||
    candidate.type !== "wheel" ||
    candidate.entryId !== entryId ||
    typeof candidate.deltaY !== "number" ||
    !Number.isFinite(candidate.deltaY) ||
    Math.abs(candidate.deltaY) > RSS_READER_MAX_WHEEL_DELTA ||
    (candidate.deltaMode !== 0 && candidate.deltaMode !== 1 && candidate.deltaMode !== 2)
  ) {
    return null;
  }
  return candidate as RSSReaderWheelMessage;
}

export function readRSSReaderSelectionMessage(
  value: unknown,
  entryId: string,
): RSSReaderSelectionMessage | null {
  if (!value || typeof value !== "object") return null;
  const candidate = value as Partial<RSSReaderSelectionMessage>;
  if (
    candidate.channel !== RSS_READER_PROGRESS_CHANNEL ||
    candidate.type !== "selection" ||
    candidate.entryId !== entryId ||
    typeof candidate.active !== "boolean" ||
    typeof candidate.clientY !== "number" ||
    !Number.isFinite(candidate.clientY) ||
    Math.abs(candidate.clientY) > RSS_READER_MAX_DOCUMENT_HEIGHT ||
    typeof candidate.screenY !== "number" ||
    !Number.isFinite(candidate.screenY) ||
    Math.abs(candidate.screenY) > RSS_READER_MAX_DOCUMENT_HEIGHT
  ) {
    return null;
  }
  return candidate as RSSReaderSelectionMessage;
}

export function readRSSReaderLinkMessage(
  value: unknown,
  entryId: string,
): RSSReaderLinkMessage | null {
  if (!value || typeof value !== "object") return null;
  const candidate = value as Partial<RSSReaderLinkMessage>;
  if (
    candidate.channel !== RSS_READER_PROGRESS_CHANNEL ||
    candidate.type !== "link" ||
    candidate.entryId !== entryId ||
    typeof candidate.url !== "string" ||
    candidate.url.length < 1 ||
    candidate.url.length > 4_096
  ) {
    return null;
  }
  try {
    const parsed = new URL(candidate.url);
    if (
      (parsed.protocol !== "https:" && parsed.protocol !== "http:") ||
      parsed.username !== "" ||
      parsed.password !== ""
    ) {
      return null;
    }
    return {
      channel: RSS_READER_PROGRESS_CHANNEL,
      type: "link",
      entryId,
      url: parsed.href,
    };
  } catch {
    return null;
  }
}

export function readRSSReaderImageContextMessage(
  value: unknown,
  entryId: string,
  expectedDocumentId: string,
): RSSReaderImageContextMessage | null {
  if (!value || typeof value !== "object") return null;
  const candidate = value as {
    channel?: unknown;
    type?: unknown;
    entryId?: unknown;
    documentId?: unknown;
    src?: unknown;
    alt?: unknown;
    clientX?: unknown;
    clientY?: unknown;
  };
  if (
    candidate.channel !== RSS_READER_PROGRESS_CHANNEL ||
    candidate.type !== "image-context" ||
    candidate.entryId !== entryId ||
    candidate.documentId !== expectedDocumentId ||
    typeof candidate.src !== "string" ||
    candidate.src.length < 1 ||
    candidate.src.length > 4_096 ||
    typeof candidate.alt !== "string" ||
    candidate.alt.length > 512 ||
    typeof candidate.clientX !== "number" ||
    !Number.isFinite(candidate.clientX) ||
    candidate.clientX < 0 ||
    candidate.clientX > RSS_READER_MAX_DOCUMENT_HEIGHT ||
    typeof candidate.clientY !== "number" ||
    !Number.isFinite(candidate.clientY) ||
    candidate.clientY < 0 ||
    candidate.clientY > RSS_READER_MAX_DOCUMENT_HEIGHT
  ) {
    return null;
  }
  const resource = controlledRSSEntryImageResource(candidate.src, entryId);
  if (!resource) return null;
  return {
    channel: RSS_READER_PROGRESS_CHANNEL,
    type: "image-context",
    entryId,
    documentId: expectedDocumentId,
    slot: resource.slot,
    alt: candidate.alt.trim(),
    clientX: candidate.clientX,
    clientY: candidate.clientY,
  };
}

export function readRSSReaderOutlineMessage(
  value: unknown,
  entryId: string,
): RSSReaderOutlineMessage | null {
  if (!value || typeof value !== "object") return null;
  const candidate = value as Partial<RSSReaderOutlineMessage>;
  if (
    candidate.channel !== RSS_READER_PROGRESS_CHANNEL ||
    candidate.type !== "outline" ||
    candidate.entryId !== entryId ||
    !Array.isArray(candidate.items) ||
    candidate.items.length > RSS_READER_MAX_OUTLINE_ITEMS
  ) {
    return null;
  }
  const items: RSSReaderOutlineItem[] = [];
  const seenIDs = new Set<string>();
  for (const valueItem of candidate.items) {
    if (!valueItem || typeof valueItem !== "object") return null;
    const item = valueItem as Partial<RSSReaderOutlineItem>;
    if (
      typeof item.id !== "string" ||
      item.id.length < 1 ||
      item.id.length > 256 ||
      typeof item.title !== "string" ||
      item.title.length > 240 ||
      !Number.isInteger(item.depth) ||
      (item.depth ?? 0) < 1 ||
      (item.depth ?? 0) > 6 ||
      typeof item.top !== "number" ||
      !Number.isFinite(item.top) ||
      (item.textLength !== undefined && (
        !Number.isInteger(item.textLength) ||
        item.textLength < 0 ||
        item.textLength > RSS_READER_MAX_DOCUMENT_HEIGHT
      ))
    ) {
      return null;
    }
    if (seenIDs.has(item.id)) continue;
    seenIDs.add(item.id);
    items.push({
      id: item.id,
      title: item.title,
      depth: item.depth as RSSReaderOutlineItem["depth"],
      top: Math.max(
        0,
        Math.min(RSS_READER_MAX_DOCUMENT_HEIGHT, Math.round(item.top)),
      ),
      ...(item.textLength === undefined ? {} : { textLength: item.textLength }),
    });
  }
  items.sort((left, right) => left.top - right.top);
  return {
    channel: RSS_READER_PROGRESS_CHANNEL,
    type: "outline",
    entryId,
    items,
  };
}

export function resolveRSSReaderOutlineProgress(
  outline: readonly Pick<RSSReaderOutlineItem, "top">[],
  activationTop: number,
  contentEnd: number,
  options: { atDocumentEnd?: boolean } = {},
) {
  if (outline.length === 0) {
    return { activeIndex: -1, sectionFraction: 0 };
  }
  // The activation line intentionally sits near the upper third of the
  // viewport. At the physical scroll boundary it therefore cannot reach the
  // document end on its own, so explicitly complete the final section.
  if (options.atDocumentEnd) {
    return { activeIndex: outline.length - 1, sectionFraction: 1 };
  }
  const boundedActivation = Math.max(0, Number(activationTop) || 0);
  let activeIndex = 0;
  for (let index = 1; index < outline.length; index += 1) {
    if (outline[index].top > boundedActivation) break;
    activeIndex = index;
  }
  const start = Math.max(0, outline[activeIndex].top);
  const end = Math.max(
    start + 1,
    activeIndex + 1 < outline.length
      ? outline[activeIndex + 1].top
      : contentEnd,
  );
  return {
    activeIndex,
    sectionFraction: Math.max(0, Math.min(1, (boundedActivation - start) / (end - start))),
  };
}

/**
 * Scales outline markers by section content while preserving a usable visual
 * target for very short sections. Callers can keep the button hit-area full
 * width and right-align only its visual track using `widthFraction`.
 */
export function resolveRSSReaderOutlineMarkerMetrics(
  outline: readonly Pick<RSSReaderOutlineItem, "top" | "textLength">[],
  contentEnd: number,
  minimumWidth = RSS_READER_OUTLINE_MIN_MARKER_WIDTH,
): RSSReaderOutlineMarkerMetric[] {
  if (outline.length === 0) return [];

  const boundedContentEnd = Math.max(0, Number(contentEnd) || 0);
  const textLengths = outline.map((item) => item.textLength);
  const canUseTextLengths = textLengths.every((length) => (
    typeof length === "number" && Number.isFinite(length) && length >= 0
  )) && textLengths.some((length) => (length ?? 0) > 0);
  const contentLengths = canUseTextLengths
    ? textLengths.map((length) => Math.max(0, length ?? 0))
    : outline.map((item, index) => {
        const start = Math.max(0, Number(item.top) || 0);
        const end = index + 1 < outline.length
          ? Math.max(start, Number(outline[index + 1].top) || 0)
          : Math.max(start, boundedContentEnd);
        return Math.max(1, end - start);
      });
  const longest = Math.max(1, ...contentLengths);
  const boundedMinimum = Number.isFinite(minimumWidth)
    ? Math.max(0, Math.min(1, minimumWidth))
    : RSS_READER_OUTLINE_MIN_MARKER_WIDTH;

  return contentLengths.map((contentLength, index) => ({
    index,
    contentLength,
    widthFraction: Math.max(boundedMinimum, Math.min(1, contentLength / longest)),
  }));
}

/**
 * Keeps the compact rail stable for long documents by grouping adjacent
 * chapters into fixed ranges. Progress within a range remains continuous, so
 * an active chapter is never lost merely because the rail is capped.
 */
export function resolveRSSReaderOutlineMarkers(
  outline: readonly RSSReaderOutlineItem[],
  contentEnd: number,
  progress: { activeIndex: number; sectionFraction: number },
  maximumMarkers = 12,
): RSSReaderOutlineMarker[] {
  if (outline.length === 0) return [];

  const markerLimit = Number.isFinite(maximumMarkers)
    ? Math.max(1, Math.trunc(maximumMarkers))
    : 12;
  const markerCount = Math.min(markerLimit, outline.length);
  const metrics = resolveRSSReaderOutlineMarkerMetrics(outline, contentEnd);
  const lengths = outline.map((_, index) => Math.max(
    1,
    metrics[index]?.contentLength ?? 1,
  ));
  const activeIndex = Number.isInteger(progress.activeIndex)
    ? Math.max(-1, Math.min(outline.length - 1, progress.activeIndex))
    : -1;
  const sectionFraction = Math.max(
    0,
    Math.min(1, Number(progress.sectionFraction) || 0),
  );

  const grouped = Array.from({ length: markerCount }, (_, slot) => {
    const startIndex = Math.floor((slot * outline.length) / markerCount);
    const endIndex = Math.floor(((slot + 1) * outline.length) / markerCount);
    const contentLength = lengths
      .slice(startIndex, endIndex)
      .reduce((total, length) => total + length, 0);
    const active = activeIndex >= startIndex && activeIndex < endIndex;
    let fillFraction = 0;
    if (activeIndex >= endIndex) {
      fillFraction = 1;
    } else if (active) {
      const completedLength = lengths
        .slice(startIndex, activeIndex)
        .reduce((total, length) => total + length, 0);
      fillFraction = (
        completedLength + lengths[activeIndex] * sectionFraction
      ) / contentLength;
    }
    return {
      active,
      contentLength,
      endIndex,
      fillFraction: Math.max(0, Math.min(1, fillFraction)),
      item: outline[startIndex],
      startIndex,
    };
  });
  const longestGroup = Math.max(
    1,
    ...grouped.map((marker) => marker.contentLength),
  );

  return grouped.map((marker) => ({
    ...marker,
    widthFraction: Math.max(
      RSS_READER_OUTLINE_MIN_MARKER_WIDTH,
      Math.min(1, marker.contentLength / longestGroup),
    ),
  }));
}

export function rssReaderWheelPixels(
  message: Pick<RSSReaderWheelMessage, "deltaY" | "deltaMode">,
  clientHeight: number,
) {
  const viewport = Number.isFinite(clientHeight) && clientHeight > 0
    ? clientHeight
    : 800;
  const multiplier = message.deltaMode === 1
    ? 16
    : message.deltaMode === 2
      ? viewport
      : 1;
  const maximum = Math.max(128, viewport * 4);
  return Math.max(-maximum, Math.min(maximum, message.deltaY * multiplier));
}

export function rssReaderScrollFraction(
  scrollTop: number,
  scrollHeight: number,
  clientHeight: number,
) {
  const maximum = Math.max(0, scrollHeight - clientHeight);
  if (maximum <= 0) return 1;
  const boundedScrollTop = Math.max(0, Number(scrollTop) || 0);
  // DOM scroll geometry mixes integer scrollHeight/clientHeight with a
  // potentially fractional scrollTop. Snap the final device-pixel sliver so
  // a physically bottomed-out reader consistently reports completion.
  if (maximum - boundedScrollTop <= 1) return 1;
  return Math.max(0, Math.min(1, boundedScrollTop / maximum));
}

export function updateEntryStateInCache(
  current: RSSEntryPage | InfiniteData<RSSEntryPage> | undefined,
  state: RSSEntryState,
) {
  if (!current) return current;
  if ("pages" in current) {
    return {
      ...current,
      pages: current.pages.map((page) => updateEntryPageState(page, state)),
    };
  }
  return updateEntryPageState(current, state);
}

/**
 * Applies a full state snapshot without allowing a late response for one field
 * to roll back a newer read/star/progress value from another mutation.
 */
export function applyRSSStateToEntry(
  entry: RSSEntry,
  state: RSSEntryState,
): RSSEntry {
  if (entry.id !== state.entryId) return entry;

  const currentRevisions = normalizedFieldRevisions(entry.fieldRevisions);
  const incomingRevisions = normalizedFieldRevisions(state.fieldRevisions);
  const next: RSSEntry = { ...entry };

  if (incomingRevisions.read >= currentRevisions.read) {
    next.readAt = state.readAt;
  }
  if (incomingRevisions.starred >= currentRevisions.starred) {
    next.starredAt = state.starredAt;
  }
  if (incomingRevisions.articleProgress >= currentRevisions.articleProgress) {
    next.articleProgress = state.articleProgress;
  }
  if (
    incomingRevisions.videoProgressSeconds >=
    currentRevisions.videoProgressSeconds
  ) {
    next.videoProgressSeconds = state.videoProgressSeconds;
    next.videoDurationSeconds = state.videoDurationSeconds;
    next.videoCompleted = state.videoCompleted;
  }

  next.fieldRevisions = {
    read: Math.max(currentRevisions.read, incomingRevisions.read),
    starred: Math.max(currentRevisions.starred, incomingRevisions.starred),
    articleProgress: Math.max(
      currentRevisions.articleProgress,
      incomingRevisions.articleProgress,
    ),
    videoProgressSeconds: Math.max(
      currentRevisions.videoProgressSeconds,
      incomingRevisions.videoProgressSeconds,
    ),
  };
  const currentStateRevision = nonNegativeInteger(entry.stateRevision);
  const incomingStateRevision = nonNegativeInteger(state.revision);
  next.stateRevision = Math.max(currentStateRevision, incomingStateRevision);
  if (incomingStateRevision >= currentStateRevision) {
    next.readStateUpdatedAt = state.updatedAt;
  }
  return next;
}

function updateEntryPageState(current: RSSEntryPage, state: RSSEntryState) {
  return {
    ...current,
    items: current.items.map((entry) =>
      applyRSSStateToEntry(entry, state),
    ),
  };
}

function normalizedFieldRevisions(
  value: RSSEntry["fieldRevisions"] | RSSEntryState["fieldRevisions"],
) {
  return {
    read: nonNegativeInteger(value?.read),
    starred: nonNegativeInteger(value?.starred),
    articleProgress: nonNegativeInteger(value?.articleProgress),
    videoProgressSeconds: nonNegativeInteger(value?.videoProgressSeconds),
  };
}

function nonNegativeInteger(value: number | undefined) {
  return Number.isFinite(value)
    ? Math.max(0, Math.trunc(value ?? 0))
    : 0;
}

function compareSubscriptionTitles(left: RSSSubscription, right: RSSSubscription, language?: string) {
  return left.title.localeCompare(right.title, language, { sensitivity: "base" });
}

function subscriptionTimestamp(subscription: RSSSubscription) {
  const value = new Date(subscription.lastSuccessAt || subscription.updatedAt).getTime();
  return Number.isFinite(value) ? value : 0;
}

function safeReaderBaseURL(value?: string) {
  if (!value) return "";
  try {
    const parsed = new URL(value);
    return parsed.protocol === "https:" || parsed.protocol === "http:" ? parsed.href : "";
  } catch {
    return "";
  }
}

function newMutationID() {
  return typeof crypto !== "undefined" && "randomUUID" in crypto
    ? crypto.randomUUID()
    : `${Date.now()}-${Math.random().toString(16).slice(2)}`;
}

function newReaderNonce() {
  return newMutationID().replace(/[^A-Za-z0-9_-]/g, "");
}

function rssReaderImageDimensionHints(entry: RSSEntry) {
  const hints: Array<[string, number, number]> = [];
  const seen = new Set<string>();
  for (const media of entry.media) {
    const url = media.kind === "image" ? media.url.trim() : "";
    const width = Math.trunc(media.width ?? 0);
    const height = Math.trunc(media.height ?? 0);
    if (
      !url ||
      seen.has(url) ||
      width < 1 ||
      height < 1 ||
      width > RSS_READER_MAX_DOCUMENT_HEIGHT ||
      height > RSS_READER_MAX_DOCUMENT_HEIGHT
    ) {
      continue;
    }
    seen.add(url);
    hints.push([url, width, height]);
  }
  return hints;
}

function readerLayoutScript(
  entryId: string,
  documentId: string,
  imageDimensionHints: readonly (readonly [string, number, number])[],
) {
  const serializedEntryID = safeScriptJSON(entryId);
  return `(()=>{
const channel=${safeScriptJSON(RSS_READER_PROGRESS_CHANNEL)};
const entryId=${serializedEntryID};
const documentId=${safeScriptJSON(documentId)};
const maximum=${RSS_READER_MAX_DOCUMENT_HEIGHT};
const maximumWheel=${RSS_READER_MAX_WHEEL_DELTA};
const maximumOutline=${RSS_READER_MAX_OUTLINE_ITEMS};
const maximumShifts=${RSS_READER_MAX_LAYOUT_SHIFTS};
const maximumVideoEmbeds=${RSS_READER_MAX_VIDEO_EMBEDS};
const headingPrefix=${safeScriptJSON(RSS_READER_OUTLINE_ID_PREFIX)};
const hintedImageDimensions=new Map(${safeScriptJSON(imageDimensionHints)}.map(([url,width,height])=>[url,{width,height}]));
const images=Array.from(document.querySelectorAll("img"));
for(const image of images){
  const authoredWidth=Number(image.getAttribute("width"));
  const authoredHeight=Number(image.getAttribute("height"));
  const hasAuthoredDimensions=Number.isFinite(authoredWidth)&&authoredWidth>0&&authoredWidth<=maximum&&Number.isFinite(authoredHeight)&&authoredHeight>0&&authoredHeight<=maximum;
  const hint=hintedImageDimensions.get(image.currentSrc||image.src);
  if(!hasAuthoredDimensions&&hint){
    image.setAttribute("width",String(hint.width));
    image.setAttribute("height",String(hint.height));
  }
  image.dataset.rssReaderImageState=image.complete
    ? image.naturalWidth>0?"loaded":"failed"
    : "loading";
}
const videoEmbedNodes=Array.from(document.querySelectorAll("figure[data-xiadown-rss-video-provider][data-xiadown-rss-video-id]"));
let acceptedVideoEmbedCount=0;
const videoEmbeds=videoEmbedNodes
  .map(node=>{
    const provider=(node.getAttribute("data-xiadown-rss-video-provider")||"").trim().toLowerCase();
    let videoId=(node.getAttribute("data-xiadown-rss-video-id")||"").trim();
    const valid=provider==="youtube"
      ? /^[A-Za-z0-9_-]{11}$/.test(videoId)
      : provider==="vimeo"
        ? /^[1-9][0-9]{0,19}$/.test(videoId)
        : provider==="bilibili"&&(/^(?:BV[A-Za-z0-9]{10}|av[1-9][0-9]{0,19})$/i.test(videoId));
    if(!valid){node.remove();return null}
    const markerStyle=getComputedStyle(node);
    if(node.hidden||node.closest("[hidden]")||node.getClientRects().length===0||markerStyle.display==="none"||markerStyle.visibility==="hidden"||markerStyle.visibility==="collapse"||markerStyle.contentVisibility==="hidden")return null;
    if(acceptedVideoEmbedCount>=maximumVideoEmbeds){node.remove();return null}
    acceptedVideoEmbedCount+=1;
    if(provider==="bilibili")videoId=/^bv/i.test(videoId)?"BV"+videoId.slice(2):"av"+videoId.slice(2);
    const width=Number(node.getAttribute("data-xiadown-rss-video-width"));
    const height=Number(node.getAttribute("data-xiadown-rss-video-height"));
    if(Number.isFinite(width)&&width>0&&width<=maximum&&Number.isFinite(height)&&height>0&&height<=maximum){
      node.style.aspectRatio=String(width)+" / "+String(height);
    }
    node.setAttribute("aria-hidden","true");
    const action=document.createElement("div");
    action.className="rss-reader-video-embed-action-slot";
    action.setAttribute("data-xiadown-rss-video-action-slot","true");
    action.setAttribute("aria-hidden","true");
    node.insertAdjacentElement("afterend",action);
    const title=(node.getAttribute("title")||"").trim().slice(0,512);
    return {node,action,provider,videoId,title};
  })
  .filter(Boolean);
const outlineNodes=Array.from(document.querySelectorAll("h1,h2,h3,h4,h5,h6")).slice(0,maximumOutline);
const reservedIds=new Set(Array.from(document.querySelectorAll("[id]")).filter(node=>!outlineNodes.includes(node)).map(node=>node.id).filter(Boolean));
let headingNumber=0;
for(const node of outlineNodes){
  let id="";
  do{id=headingPrefix+(++headingNumber)}while(reservedIds.has(id));
  node.id=id;
  reservedIds.add(id);
}
const outlineTextLengths=Array(outlineNodes.length).fill(0);
if(typeof document.createTreeWalker==="function"){
  const outlineIndexes=new Map(outlineNodes.map((node,index)=>[node,index]));
  const walker=document.createTreeWalker(document.body,5);
  let activeOutline=-1;
  for(let node=walker.nextNode();node;node=walker.nextNode()){
    if(node.nodeType===1){
      const index=outlineIndexes.get(node);
      if(index!==undefined)activeOutline=index;
      continue;
    }
    if(activeOutline<0||node.parentElement?.closest("script,style,noscript,template,[hidden],[aria-hidden='true']"))continue;
    const length=(node.textContent||"").replace(/\\s+/g," ").trim().length;
    outlineTextLengths[activeOutline]=Math.min(maximum,outlineTextLengths[activeOutline]+length);
  }
}
let frame=0;
let lastHeight=0;
let lastOutline="";
let lastVideoEmbeds="";
let layoutSequence=0;
let pendingShifts=[];
let shiftOverflow=false;
let selecting=false;
let selectionReported=false;
let selectionStartX=0;
let selectionStartY=0;
const collectOutline=()=>outlineNodes.map((node,index)=>({
  id:node.id,
  title:(node.textContent||"").trim().slice(0,240),
  depth:Number(node.tagName.slice(1)),
  top:Math.max(0,Math.min(maximum,Math.round(node.getBoundingClientRect().top))),
  textLength:outlineTextLengths[index],
}));
const collectVideoEmbeds=()=>videoEmbeds.flatMap(item=>{
  const bounds=item.node.getBoundingClientRect();
  const actionBounds=item.action.getBoundingClientRect();
  if(bounds.width<1||bounds.height<1||actionBounds.width<1||actionBounds.height<1)return [];
  const top=Math.max(0,Math.min(maximum-1,Math.round(bounds.top)));
  const left=Math.max(0,Math.min(maximum-1,Math.round(bounds.left)));
  const width=Math.max(1,Math.min(maximum-left,Math.round(bounds.width)));
  const height=Math.max(1,Math.min(maximum-top,Math.round(bounds.height)));
  const actionTop=Math.max(0,Math.min(maximum-1,Math.round(actionBounds.top)));
  const actionLeft=Math.max(0,Math.min(maximum-1,Math.round(actionBounds.left)));
  return [{
    provider:item.provider,
    videoId:item.videoId,
    ...(item.title?{title:item.title}:{}),
    top,
    left,
    width,
    height,
    action:{
      top:actionTop,
      left:actionLeft,
      width:Math.max(1,Math.min(maximum-actionLeft,Math.round(actionBounds.width))),
      height:Math.max(1,Math.min(256,maximum-actionTop,Math.round(actionBounds.height))),
    },
  }];
});
const measureHeight=()=>Math.max(1,Math.min(maximum,Math.ceil(Math.max(document.body.scrollHeight,document.documentElement.scrollHeight))));
const report=()=>{
  frame=0;
  const height=measureHeight();
  const embeds=collectVideoEmbeds();
  const embedSignature=JSON.stringify(embeds);
  const hasPendingLayout=pendingShifts.length>0||shiftOverflow;
  const shifts=shiftOverflow?[]:pendingShifts.sort((left,right)=>left.top-right.top||left.bottom-right.bottom);
  pendingShifts=[];
  shiftOverflow=false;
  if(height!==lastHeight||hasPendingLayout||embedSignature!==lastVideoEmbeds){
    lastHeight=height;
    lastVideoEmbeds=embedSignature;
    parent.postMessage({channel,type:"layout",entryId,documentId,sequence:++layoutSequence,height,shifts,embeds},"*");
  }
  const items=collectOutline();
  const signature=JSON.stringify(items);
  if(signature!==lastOutline){
    lastOutline=signature;
    parent.postMessage({channel,type:"outline",entryId,items},"*");
  }
};
const schedule=()=>{if(!frame)frame=requestAnimationFrame(report)};
const reportSelection=(event,active)=>{
  const rawClientY=Number(event&&event.clientY);
  const rawScreenY=Number(event&&event.screenY);
  const clientY=Number.isFinite(rawClientY)?Math.max(-maximum,Math.min(maximum,rawClientY)):0;
  const screenY=Number.isFinite(rawScreenY)?Math.max(-maximum,Math.min(maximum,rawScreenY)):0;
  parent.postMessage({channel,type:"selection",entryId,active,clientY,screenY},"*");
};
const stopSelection=event=>{
  if(!selecting&&!selectionReported)return;
  selecting=false;
  if(selectionReported){
    selectionReported=false;
    reportSelection(event,false);
  }
};
new ResizeObserver(schedule).observe(document.documentElement);
const imageHeights=new WeakMap(images.map(image=>[image,image.getBoundingClientRect().height]));
const imageResizeObserver=new ResizeObserver(entries=>{
  for(const entry of entries){
    const image=entry.target;
    const previousHeight=imageHeights.get(image);
    const bounds=image.getBoundingClientRect();
    const nextHeight=bounds.height;
    imageHeights.set(image,nextHeight);
    if(previousHeight===undefined||Math.abs(nextHeight-previousHeight)<0.5)continue;
    const top=Math.max(0,Math.min(maximum,bounds.top));
    const bottom=Math.max(top,Math.min(maximum,top+previousHeight));
    const delta=Math.max(-maximum,Math.min(maximum,nextHeight-previousHeight));
    if(!delta)continue;
    if(pendingShifts.length<maximumShifts&&!shiftOverflow){
      pendingShifts.push({top,bottom,delta});
    }else{
      pendingShifts=[];
      shiftOverflow=true;
    }
  }
  schedule();
});
for(const image of images)imageResizeObserver.observe(image);
for(const media of document.querySelectorAll("img,video")){
  media.addEventListener("load",event=>{
    if(event.currentTarget instanceof HTMLImageElement)event.currentTarget.dataset.rssReaderImageState="loaded";
    schedule();
  },{once:true});
  media.addEventListener("error",event=>{
    if(event.currentTarget instanceof HTMLImageElement)event.currentTarget.dataset.rssReaderImageState="failed";
    schedule();
  },{once:true});
}
addEventListener("wheel",event=>{
  const deltaY=Math.max(-maximumWheel,Math.min(maximumWheel,Number.isFinite(event.deltaY)?event.deltaY:0));
  const deltaMode=event.deltaMode===1||event.deltaMode===2?event.deltaMode:0;
  if(deltaY)parent.postMessage({channel,type:"wheel",entryId,deltaY,deltaMode},"*");
},{passive:true});
addEventListener("contextmenu",event=>{
  if(!event.isTrusted)return;
  const image=event.target instanceof Element?event.target.closest("img"):null;
  if(!(image instanceof HTMLImageElement))return;
  const src=image.currentSrc||image.src;
  if(!src)return;
  event.preventDefault();
  parent.postMessage({
    channel,
    type:"image-context",
    entryId,
    documentId,
    src,
    alt:(image.alt||image.title||"").trim().slice(0,512),
    clientX:Math.max(0,Math.min(maximum,event.clientX)),
    clientY:Math.max(0,Math.min(maximum,event.clientY)),
  },"*");
},true);
addEventListener("click",event=>{
  const target=event.target instanceof Element?event.target.closest("a[href]"):null;
  if(!target)return;
  event.preventDefault();
  if(!event.isTrusted||event.button!==0||event.metaKey||event.ctrlKey||event.shiftKey||event.altKey)return;
  try{
    const url=new URL(target.href);
    if((url.protocol==="https:"||url.protocol==="http:")&&!url.username&&!url.password){
      parent.postMessage({channel,type:"link",entryId,url:url.href},"*");
    }
  }catch{}
},true);
addEventListener("pointerdown",event=>{
  if(event.button!==0||event.isPrimary===false)return;
  selecting=true;
  selectionReported=false;
  selectionStartX=event.clientX;
  selectionStartY=event.clientY;
},{passive:true});
addEventListener("pointermove",event=>{
  if(!selecting)return;
  if((event.buttons&1)!==1){stopSelection(event);return}
  if(Math.hypot(event.clientX-selectionStartX,event.clientY-selectionStartY)<3)return;
  const selection=document.getSelection();
  if(!selection||selection.isCollapsed){
    if(selectionReported){selectionReported=false;reportSelection(event,false)}
    return;
  }
  selectionReported=true;
  reportSelection(event,true);
},{passive:true});
addEventListener("pointerup",stopSelection,{passive:true});
addEventListener("pointercancel",stopSelection,{passive:true});
addEventListener("dragstart",stopSelection,{passive:true});
addEventListener("blur",stopSelection,{passive:true});
addEventListener("load",schedule,{once:true});
schedule();
})();`;
}

function safeScriptJSON(value: unknown) {
  return JSON.stringify(value).replace(/</g, "\\u003c");
}

function escapeHTML(value: string) {
  return value.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;").replace(/"/g, "&quot;");
}

function escapeHTMLAttribute(value: string) {
  return escapeHTML(value).replace(/'/g, "&#39;");
}
