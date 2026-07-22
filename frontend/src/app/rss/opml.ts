import { DOMParser } from "@xmldom/xmldom";

import type { RSSSubscription, RSSViewType } from "./types";

export interface RSSOPMLSubscription {
  url: string;
  title?: string;
  viewType?: RSSViewType;
}

const RSS_VIEW_TYPES = new Set<RSSViewType>([
  "auto",
  "article",
  "social",
  "image",
  "video",
]);

export const RSS_OPML_MAX_SOURCE_BYTES = 2 * 1024 * 1024;
export const RSS_OPML_MAX_SUBSCRIPTIONS = 1_000;

export function exportRSSSubscriptionsToOPML(
  subscriptions: readonly RSSSubscription[],
  createdAt = new Date(),
) {
  const outlines = subscriptions
    .map((subscription) => {
      const title = escapeXML(subscription.title || subscription.feedUrl);
      return `    <outline type="rss" text="${title}" title="${title}" xmlUrl="${escapeXML(subscription.feedUrl)}" htmlUrl="${escapeXML(subscription.siteUrl || "")}" xiadownViewType="${subscription.viewType}"/>`;
    })
    .join("\n");
  return `<?xml version="1.0" encoding="UTF-8"?>\n<opml version="2.0">\n  <head>\n    <title>XiaDown RSS subscriptions</title>\n    <dateCreated>${createdAt.toUTCString()}</dateCreated>\n  </head>\n  <body>\n${outlines}\n  </body>\n</opml>\n`;
}

export function parseRSSSubscriptionsFromOPML(source: string) {
  if (
    source.length > RSS_OPML_MAX_SOURCE_BYTES ||
    new TextEncoder().encode(source).byteLength > RSS_OPML_MAX_SOURCE_BYTES
  ) {
    throw new Error("invalid_opml");
  }
  const parseProblems: string[] = [];
  const document = new DOMParser({
    errorHandler: {
      warning: (message) => parseProblems.push(message),
      error: (message) => parseProblems.push(message),
      fatalError: (message) => parseProblems.push(message),
    },
  }).parseFromString(source, "application/xml");
  const parserErrors = document.getElementsByTagName("parsererror");
  if (!document.documentElement || parserErrors.length > 0 || parseProblems.length > 0) {
    throw new Error("invalid_opml");
  }
  const seen = new Set<string>();
  const items: RSSOPMLSubscription[] = [];
  const outlines = document.getElementsByTagName("outline");
  for (let index = 0; index < outlines.length; index += 1) {
    const outline = outlines.item(index);
    const url = normalizeOPMLFeedURL(
      outline?.getAttribute("xmlUrl") || outline?.getAttribute("url") || "",
    );
    if (!url || seen.has(url)) continue;
    seen.add(url);
    const title = (
      outline?.getAttribute("title") || outline?.getAttribute("text") || ""
    ).trim();
    const rawViewType = outline?.getAttribute("xiadownViewType")?.trim() as
      | RSSViewType
      | undefined;
    if (items.length >= RSS_OPML_MAX_SUBSCRIPTIONS) {
      throw new Error("invalid_opml");
    }
    items.push({
      url,
      ...(title ? { title } : {}),
      ...(rawViewType && RSS_VIEW_TYPES.has(rawViewType)
        ? { viewType: rawViewType }
        : {}),
    });
  }
  return items;
}

function normalizeOPMLFeedURL(value: string) {
  const trimmed = value.trim();
  if (!trimmed) return "";
  if (/^rsshub:\/\/[A-Za-z0-9]/i.test(trimmed)) {
    return `rsshub://${trimmed.slice("rsshub://".length).replace(/^\/+|\/+$/g, "")}`;
  }
  if (/^feed:\/\//i.test(trimmed)) {
    return `https://${trimmed.slice("feed://".length)}`;
  }
  try {
    const parsed = new URL(trimmed);
    return parsed.protocol === "https:" || parsed.protocol === "http:"
      ? parsed.href
      : "";
  } catch {
    return "";
  }
}

function escapeXML(value: string) {
  return value
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;")
    .replace(/'/g, "&apos;");
}
