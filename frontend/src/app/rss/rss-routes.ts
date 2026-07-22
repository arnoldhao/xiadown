export const RSS_WORKSPACE_ROUTE_IDS = {
  search: "search",
  all: "all",
  articles: "articles",
  social: "social",
  images: "images",
  videos: "videos",
  starred: "starred",
  manageSubscriptions: "manage-subscriptions",
  discoverySearch: "discovery-search",
  discoveryBrowse: "discovery-browse",
  addSubscription: "add-subscription",
} as const;

export type RSSWorkspaceStaticRouteId =
  (typeof RSS_WORKSPACE_ROUTE_IDS)[keyof typeof RSS_WORKSPACE_ROUTE_IDS];

export const RSS_CATEGORY_ROUTE_PREFIX = "category:";
export const RSS_COLLECTION_ROUTE_PREFIX = "collection:";
export const RSS_SUBSCRIPTION_ROUTE_PREFIX = "subscription:";

export function isMalformedRSSDynamicRouteID(routeID: string) {
  for (const prefix of [
    RSS_CATEGORY_ROUTE_PREFIX,
    RSS_COLLECTION_ROUTE_PREFIX,
    RSS_SUBSCRIPTION_ROUTE_PREFIX,
  ]) {
    if (routeID.startsWith(prefix)) {
      return parseRSSDynamicRouteID(prefix, routeID) === null;
    }
  }
  return false;
}

export function createRSSCategoryRouteID(id: string) {
  return createRSSDynamicRouteID(RSS_CATEGORY_ROUTE_PREFIX, id);
}

export function parseRSSCategoryRouteID(routeID: string) {
  return parseRSSDynamicRouteID(RSS_CATEGORY_ROUTE_PREFIX, routeID);
}

export function createRSSCollectionRouteID(id: string) {
  return createRSSDynamicRouteID(RSS_COLLECTION_ROUTE_PREFIX, id);
}

export function parseRSSCollectionRouteID(routeID: string) {
  return parseRSSDynamicRouteID(RSS_COLLECTION_ROUTE_PREFIX, routeID);
}

export function createRSSSubscriptionRouteID(id: string) {
  return createRSSDynamicRouteID(RSS_SUBSCRIPTION_ROUTE_PREFIX, id);
}

export function parseRSSSubscriptionRouteID(routeID: string) {
  return parseRSSDynamicRouteID(RSS_SUBSCRIPTION_ROUTE_PREFIX, routeID);
}

export const createRSSSubscriptionRouteId = createRSSSubscriptionRouteID;
export const parseRSSSubscriptionRouteId = parseRSSSubscriptionRouteID;

export function isRSSSubscriptionContextMenuKey(
  key: string,
  shiftKey = false,
) {
  return key === "ContextMenu" || (key === "F10" && shiftKey);
}

function createRSSDynamicRouteID(prefix: string, rawID: string) {
  return `${prefix}${encodeURIComponent(rawID.trim())}`;
}

function parseRSSDynamicRouteID(prefix: string, routeID: string) {
  if (!routeID.startsWith(prefix)) return null;
  const value = routeID.slice(prefix.length);
  if (!value) return null;
  try {
    return decodeURIComponent(value).trim() || null;
  } catch {
    return null;
  }
}
