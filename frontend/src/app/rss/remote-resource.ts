const RSS_LOOPBACK_TOKEN_PATH = /^\/_xiadown\/[0-9a-f]{64}\/api\/rss\//i;
const RSS_LOCAL_SUBSCRIPTION_RESOURCE = /^\/_xiadown\/[0-9a-f]{64}\/api\/rss\/subscriptions\/[^/]+\/icon$/i;
const RSS_LOCAL_DISCOVERY_RESOURCE = /^\/_xiadown\/[0-9a-f]{64}\/api\/rss\/discovery\/(?:categories|routes)\/[^/]+\/icon$/i;
const RSS_LOCAL_ENTRY_RESOURCE = /^\/_xiadown\/[0-9a-f]{64}\/api\/rss\/entries\/([^/]+)\/resources\/(thumbnail|image-(0|[1-9][0-9]*)|media-(0|[1-9][0-9]*)(?:-thumbnail)?)$/i;

export interface ControlledRSSEntryImageResource {
  entryId: string;
  slot: string;
}

/**
 * Feed-authored URLs are never valid render sources. Wails projects persisted
 * entity slots and discovery identifiers onto the tokenized loopback resource
 * handler; this is the final fail-closed boundary if an unprojected DTO slips
 * through.
 */
export function controlledRSSResourceURL(value: string | undefined) {
  const trimmed = value?.trim() ?? "";
  if (!trimmed) return "";
  try {
    const parsed = new URL(trimmed);
    if (
      parsed.protocol !== "http:" ||
      parsed.hostname !== "127.0.0.1" ||
      parsed.username ||
      parsed.password ||
      !validRSSResourceVersion(parsed.search) ||
      parsed.hash ||
      !RSS_LOOPBACK_TOKEN_PATH.test(parsed.pathname)
    ) {
      return "";
    }
    if (
      !RSS_LOCAL_SUBSCRIPTION_RESOURCE.test(parsed.pathname) &&
      !RSS_LOCAL_DISCOVERY_RESOURCE.test(parsed.pathname) &&
      !isControlledRSSEntryResource(parsed.pathname)
    ) {
      return "";
    }
    return parsed.toString();
  } catch {
    return "";
  }
}

function validRSSResourceVersion(search: string) {
  return search === "" || /^\?v=[1-9][0-9]{0,18}$/.test(search);
}

function isControlledRSSEntryResource(pathname: string) {
  const match = RSS_LOCAL_ENTRY_RESOURCE.exec(pathname);
  if (!match) return false;
  const rawIndex = match[3] ?? match[4];
  return rawIndex === undefined || Number(rawIndex) < 64;
}

/** Reduces a tokenized render URL back to its opaque persisted entity slot. */
export function controlledRSSEntryImageResource(
  value: string | undefined,
  expectedEntryId?: string,
): ControlledRSSEntryImageResource | null {
  const controlled = controlledRSSResourceURL(value);
  if (!controlled) return null;
  try {
    const match = RSS_LOCAL_ENTRY_RESOURCE.exec(new URL(controlled).pathname);
    if (!match || !isControlledRSSEntryResource(new URL(controlled).pathname)) {
      return null;
    }
    const entryId = decodeURIComponent(match[1]);
    if (
      !entryId ||
      entryId.length > 512 ||
      /[\\/\u0000-\u001f\u007f]/.test(entryId) ||
      (expectedEntryId !== undefined && entryId !== expectedEntryId)
    ) {
      return null;
    }
    return { entryId, slot: match[2].toLowerCase() };
  } catch {
    return null;
  }
}

export function controlledRSSResourceOrigin(values: readonly (string | undefined)[]) {
  for (const value of values) {
    const controlled = controlledRSSResourceURL(value);
    if (controlled) return new URL(controlled).origin;
  }
  return "";
}
