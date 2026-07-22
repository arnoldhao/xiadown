import type { TFunction } from "@/shared/i18n";
import type {
  RSSDiscoveryRoute,
  RSSEntry,
  RSSListDiscoveryRequest,
  RSSSubscription,
} from "./types";

const CATEGORY_I18N_KEYS: Record<string, string> = {
  all: "xiadown.rss.discoveryCategoryAll",
  "social-media": "xiadown.rss.discoveryCategorySocialMedia",
  "new-media": "xiadown.rss.discoveryCategoryNewMedia",
  "traditional-media": "xiadown.rss.discoveryCategoryTraditionalMedia",
  bbs: "xiadown.rss.discoveryCategoryBbs",
  blog: "xiadown.rss.discoveryCategoryBlog",
  programming: "xiadown.rss.discoveryCategoryProgramming",
  design: "xiadown.rss.discoveryCategoryDesign",
  live: "xiadown.rss.discoveryCategoryLive",
  multimedia: "xiadown.rss.discoveryCategoryMultimedia",
  picture: "xiadown.rss.discoveryCategoryPicture",
  anime: "xiadown.rss.discoveryCategoryAnime",
  "program-update": "xiadown.rss.discoveryCategoryProgramUpdate",
  university: "xiadown.rss.discoveryCategoryUniversity",
  forecast: "xiadown.rss.discoveryCategoryForecast",
  travel: "xiadown.rss.discoveryCategoryTravel",
  shopping: "xiadown.rss.discoveryCategoryShopping",
  game: "xiadown.rss.discoveryCategoryGame",
  reading: "xiadown.rss.discoveryCategoryReading",
  government: "xiadown.rss.discoveryCategoryGovernment",
  study: "xiadown.rss.discoveryCategoryStudy",
  journal: "xiadown.rss.discoveryCategoryJournal",
  finance: "xiadown.rss.discoveryCategoryFinance",
  sport: "xiadown.rss.discoveryCategorySport",
  other: "xiadown.rss.discoveryCategoryOther",
};

export const RSS_DISCOVERY_CATEGORY_EMOJI: Record<string, string> = {
  all: "✨",
  "social-media": "💬",
  "new-media": "⚡️",
  "traditional-media": "📰",
  bbs: "🗯️",
  blog: "✍️",
  programming: "💻",
  design: "🎨",
  live: "📡",
  multimedia: "🎬",
  picture: "📷",
  anime: "🌸",
  "program-update": "📦",
  university: "🏫",
  forecast: "🌤️",
  travel: "🧳",
  shopping: "🛍️",
  game: "🎮",
  reading: "📚",
  government: "🏛️",
  study: "🎓",
  journal: "🔬",
  finance: "📈",
  sport: "🏅",
  other: "🧭",
};

export function normalizeDiscoveryRequest(
  request: RSSListDiscoveryRequest,
): RSSListDiscoveryRequest {
  const query = request.query?.trim() || undefined;
  const categoryId = request.categoryId?.trim() || undefined;
  const language = request.language?.trim() || undefined;
  const sort = request.sort === "title" ? "title" : "popular";
  const offset = normalizeNonNegativeInteger(request.offset);
  const limit = normalizeNonNegativeInteger(request.limit);
  return {
    ...(query ? { query } : {}),
    ...(categoryId ? { categoryId } : {}),
    ...(language ? { language } : {}),
    sort,
    ...(offset !== undefined ? { offset } : {}),
    ...(limit !== undefined ? { limit } : {}),
    ...(request.forceRefresh ? { forceRefresh: true } : {}),
  };
}

export function isRSSDiscoveryAddress(value: string) {
  const normalized = value.trim().toLowerCase();
  return /^(https?:\/\/|rsshub:\/\/|feed:\/\/)/.test(normalized) || normalized.startsWith("/");
}

export function canonicalizeRSSHubInput(value: string) {
  const normalized = value.trim();
  if (normalized.startsWith("/")) return `rsshub://${normalized.replace(/^\/+/, "")}`;
  if (normalized.toLowerCase().startsWith("feed://")) return `https://${normalized.slice("feed://".length)}`;
  return normalized;
}

export function rssDiscoveryRouteSubscribed(
  route: RSSDiscoveryRoute,
  subscriptions: readonly RSSSubscription[],
) {
  if (route.needsParameters) {
    return subscriptions.some((subscription) =>
      rssDiscoveryRouteMatchesFeedURL(route, subscription.feedUrl),
    );
  }
  const routeURL = normalizeFeedAddress(route.url);
  return subscriptions.some((subscription) => normalizeFeedAddress(subscription.feedUrl) === routeURL);
}

export function rssDiscoveryRouteMatchesFeedURL(
  route: RSSDiscoveryRoute,
  feedURL: string,
) {
  const normalizedURL = normalizeFeedAddress(feedURL);
  if (!normalizedURL.startsWith("rsshub://")) return false;
  const concretePath = normalizedURL.slice("rsshub://".length);
  if (!concretePath || /(^|\/):|[{}*]/.test(concretePath)) return false;

  const patternParts: string[] = [];
  for (const segment of splitRSSDiscoveryRoutePath(route.routePath)) {
    const match = segment.match(/^:([A-Za-z0-9_-]+)(?:\{.*\})?(\?)?$/);
    if (!match) {
      if (!segment || /[:*{}?]/.test(segment)) return false;
      patternParts.push(`/${escapeRegExp(segment.toLowerCase())}`);
      continue;
    }
    const parameter = route.parameters.find((item) => item.name === match[1]);
    if (!parameter) return false;
    const optional = parameter.optional || match[2] === "?";
    if (parameter.catchAll) {
      patternParts.push(optional ? "(?:/[^/]+)*" : "(?:/[^/]+)+");
    } else {
      patternParts.push(optional ? "(?:/[^/]+)?" : "/[^/]+");
    }
  }
  if (patternParts.length === 0) return false;
  return new RegExp(`^${patternParts.join("").replace(/^\//, "")}$`, "i").test(concretePath);
}

export function rssFeedAddressSubscribed(
  url: string,
  subscriptions: readonly RSSSubscription[],
) {
  const normalizedURL = normalizeFeedAddress(url);
  return subscriptions.some((subscription) => normalizeFeedAddress(subscription.feedUrl) === normalizedURL);
}

export function mergeRSSDiscoveryPages(routes: readonly RSSDiscoveryRoute[]) {
  const byID = new Map<string, RSSDiscoveryRoute>();
  routes.forEach((route) => byID.set(route.id, route));
  return [...byID.values()];
}

/**
 * Keeps the backend's relevance ordering while removing duplicate local hits.
 * Some feeds republish an item with a new entry id but the same canonical URL,
 * so identity alone is not sufficient for a unified search surface.
 */
export function mergeRSSLocalSearchEntries(entries: readonly RSSEntry[]) {
  const seenIDs = new Set<string>();
  const seenURLs = new Set<string>();
  const results: RSSEntry[] = [];

  for (const entry of entries) {
    const id = entry.id.trim();
    const url = normalizeEntryAddress(entry.url);
    if ((id && seenIDs.has(id)) || (url && seenURLs.has(url))) {
      continue;
    }
    if (id) seenIDs.add(id);
    if (url) seenURLs.add(url);
    results.push(entry);
  }
  return results;
}

/**
 * Flattens paginated local search results without moving data-only helpers
 * into the React page module. Keeping that module component-only lets Vite
 * preserve its Fast Refresh boundary during development.
 */
export function mergeRSSLocalEntryPages(
  pages: readonly { items: readonly RSSEntry[] }[],
) {
  return mergeRSSLocalSearchEntries(pages.flatMap((page) => page.items));
}

export type RSSDiscoveryRouteConfigurationErrorCode =
  | "required"
  | "optionalOrder"
  | "invalidOption"
  | "invalidValue"
  | "invalidTemplate";

export class RSSDiscoveryRouteConfigurationError extends Error {
  constructor(
    public readonly code: RSSDiscoveryRouteConfigurationErrorCode,
    public readonly parameterName = "",
  ) {
    super(`${code}:${parameterName}`);
  }
}

export function initialRSSDiscoveryParameterValues(route: RSSDiscoveryRoute) {
  return Object.fromEntries(
    route.parameters.map((parameter) => [
      parameter.name,
      parameter.defaultValue ?? "",
    ]),
  );
}

export function buildRSSDiscoveryRouteURL(
  route: RSSDiscoveryRoute,
  values: Readonly<Record<string, string>>,
) {
  if (!route.needsParameters && route.parameters.length === 0) {
    if (!route.url.toLowerCase().startsWith("rsshub://")) {
      throw new RSSDiscoveryRouteConfigurationError("invalidTemplate");
    }
    return route.url;
  }

  const parameters = new Map(route.parameters.map((parameter) => [parameter.name, parameter]));
  let skippedOptional = "";
  const outputSegments: string[] = [];
  for (const segment of splitRSSDiscoveryRoutePath(route.routePath)) {
    const match = segment.match(/^:([A-Za-z0-9_-]+)(?:\{.*\})?(\?)?$/);
    if (!match) {
      if (!segment || /[:*{}?]/.test(segment)) {
        throw new RSSDiscoveryRouteConfigurationError("invalidTemplate");
      }
      outputSegments.push(segment);
      continue;
    }

    const name = match[1];
    const parameter = parameters.get(name);
    if (!parameter) {
      throw new RSSDiscoveryRouteConfigurationError("invalidTemplate", name);
    }
    const value = (values[name] ?? "").trim();
    const optional = parameter.optional || match[2] === "?";
    if (!value) {
      if (!optional) {
        throw new RSSDiscoveryRouteConfigurationError("required", name);
      }
      skippedOptional ||= name;
      continue;
    }
    if (skippedOptional) {
      throw new RSSDiscoveryRouteConfigurationError("optionalOrder", skippedOptional);
    }
    if (parameter.options.length > 0 && !parameter.options.some((option) => option.value === value)) {
      throw new RSSDiscoveryRouteConfigurationError("invalidOption", name);
    }

    const valueSegments = parameter.catchAll ? value.split("/") : [value];
    if (valueSegments.some((part) => !part || part === "." || part === "..")) {
      throw new RSSDiscoveryRouteConfigurationError("invalidValue", name);
    }
    outputSegments.push(...valueSegments.map(encodeURIComponent));
  }
  const canonicalPath = outputSegments.join("/");
  if (!canonicalPath || canonicalPath.includes(":")) {
    throw new RSSDiscoveryRouteConfigurationError("invalidTemplate");
  }
  return `rsshub://${canonicalPath}`;
}

export function catalogLanguageForLocale(locale: string) {
  const normalized = locale.trim().replace(/_/g, "-").toLowerCase();
  if (normalized.startsWith("ja")) return "ja";
  if (normalized.startsWith("ko")) return "ko";
  if (normalized.startsWith("es")) return "es";
  if (normalized === "zh-tw" || normalized === "zh-hk") return "zh-TW";
  if (normalized.startsWith("zh")) return "zh-CN";
  if (normalized.startsWith("en")) return "en";
  return "";
}

export function formatRSSDiscoveryNumber(value: number, locale: string) {
  return new Intl.NumberFormat(locale).format(value);
}

export function formatRSSDiscoveryDate(value: string, locale: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "";
  return new Intl.DateTimeFormat(locale, {
    dateStyle: "medium",
  }).format(date);
}

export function rssDiscoveryCategoryLabel(categoryId: string, t: TFunction) {
  const key = CATEGORY_I18N_KEYS[categoryId];
  if (key) return t(key);
  return categoryId
    .split(/[-_]/)
    .filter(Boolean)
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(" ");
}

export function rssDiscoveryRouteInitials(route: RSSDiscoveryRoute) {
  const label = route.sourceName.trim() || route.sourceId.trim();
  if (!label) return "";

  const words = label.match(/[\p{L}\p{N}]+/gu) ?? [];
  if (words.length > 1) {
    return words.slice(0, 2).map((word) => Array.from(word)[0]).join("").toUpperCase();
  }

  const glyphs = Array.from(words[0] ?? "");
  const uppercaseGlyphs = glyphs.filter((glyph, index) => (
    index === 0 || (glyph.toUpperCase() === glyph && glyph.toLowerCase() !== glyph)
  ));
  return (uppercaseGlyphs.length > 1 ? uppercaseGlyphs : glyphs).slice(0, 2).join("").toUpperCase();
}

// Discovery search also surfaces subscriptions the user already owns. Match
// any token for broad recall, then reward complete-token and phrase matches so
// the most useful result remains first.
export function searchRSSSubscriptions(
  subscriptions: readonly RSSSubscription[],
  query: string,
) {
  const phrase = normalizeDiscoveryText(query);
  const tokens = discoveryTextTokens(phrase);
  if (!phrase || tokens.length === 0) return [];

  return subscriptions
    .map((subscription) => {
      const fields = [
        [subscription.title, 10],
        [subscription.feedUrl, 6],
        [subscription.siteUrl ?? "", 5],
        [subscription.description ?? "", 2],
      ] as const;
      let matchedTokens = 0;
      let score = 0;
      for (const token of tokens) {
        let tokenScore = 0;
        for (const [rawField, weight] of fields) {
          const field = normalizeDiscoveryText(rawField);
          if (!field.includes(token)) continue;
          tokenScore = Math.max(tokenScore, weight + (discoveryTextTokens(field).includes(token) ? weight : 0));
        }
        if (tokenScore > 0) {
          matchedTokens += 1;
          score += tokenScore;
        }
      }
      for (const [rawField, weight] of fields) {
        if (normalizeDiscoveryText(rawField).includes(phrase)) {
          score += weight * 8;
        }
      }
      if (matchedTokens === tokens.length) score += 40;
      return { subscription, matchedTokens, score };
    })
    .filter((result) => result.matchedTokens > 0)
    .sort((left, right) => (
      right.score - left.score ||
      right.matchedTokens - left.matchedTokens ||
      right.subscription.unreadCount - left.subscription.unreadCount ||
      left.subscription.title.localeCompare(right.subscription.title)
    ))
    .map((result) => result.subscription);
}

function normalizeDiscoveryText(value: string) {
  return value.normalize("NFKC").trim().toLocaleLowerCase();
}

function discoveryTextTokens(value: string) {
  return [...new Set(
    normalizeDiscoveryText(value)
      .split(/[^\p{L}\p{N}]+/u)
      .filter(Boolean)
      .slice(0, 12),
  )];
}

function normalizeFeedAddress(value: string) {
  return value.trim().replace(/\/+$/, "").toLowerCase();
}

function normalizeEntryAddress(value: string | undefined) {
  const normalized = value?.trim();
  if (!normalized) return "";
  try {
    const url = new URL(normalized);
    url.hash = "";
    return url.toString().replace(/\/$/, "").toLowerCase();
  } catch {
    return normalized.replace(/\/+$/, "").toLowerCase();
  }
}

function splitRSSDiscoveryRoutePath(value: string) {
  const path = value.trim().replace(/^\/+|\/+$/g, "");
  const parts: string[] = [];
  let start = 0;
  let braceDepth = 0;
  for (let index = 0; index < path.length; index += 1) {
    const character = path[index];
    if (character === "{") braceDepth += 1;
    if (character === "}" && braceDepth > 0) braceDepth -= 1;
    if (character === "/" && braceDepth === 0) {
      if (index > start) parts.push(path.slice(start, index));
      start = index + 1;
    }
  }
  if (start < path.length) parts.push(path.slice(start));
  return parts;
}

function escapeRegExp(value: string) {
  return value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

function normalizeNonNegativeInteger(value: number | undefined) {
  if (value === undefined || !Number.isFinite(value)) return undefined;
  return Math.max(0, Math.trunc(value));
}
