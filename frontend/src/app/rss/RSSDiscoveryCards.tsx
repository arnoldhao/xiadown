import { Check, Flame, Rss, Settings2 } from "lucide-react";

import type { TFunction } from "@/shared/i18n";
import {
  RSS_DISCOVERY_CATEGORY_EMOJI,
  formatRSSDiscoveryNumber,
  rssDiscoveryCategoryLabel,
  rssDiscoveryRouteInitials,
  rssDiscoveryRouteSubscribed,
} from "./discovery-utils";
import { controlledRSSResourceURL } from "./remote-resource";
import { RSSRemoteImage } from "./RSSRemoteImage";
import type {
  RSSDiscoveryCategory,
  RSSDiscoveryRoute,
  RSSSubscription,
} from "./types";

export function RSSDiscoveryCategoryGrid({
  categories,
  t,
  language,
  onSelect,
}: {
  categories: readonly RSSDiscoveryCategory[];
  t: TFunction;
  language: string;
  onSelect: (categoryId: string) => void;
}) {
  return (
    <div className="rss-discovery-category-grid">
      {categories.map((category) => (
        <button className="rss-discovery-category-card app-dream-card app-motion-surface" key={category.id} onClick={() => onSelect(category.id)} type="button">
          <span aria-hidden="true" className="rss-discovery-category-card__icon">
            <span className="rss-discovery-category-card__icon-fallback">
              {RSS_DISCOVERY_CATEGORY_EMOJI[category.id] || "🧭"}
            </span>
          </span>
          <span className="rss-discovery-category-card__copy">
            <strong>{rssDiscoveryCategoryLabel(category.id, t)}</strong>
            <small>{t("xiadown.rss.categoryDescription")}</small>
            <span>{formatRSSDiscoveryNumber(category.count, language)} {t("xiadown.rss.routes")}</span>
          </span>
          <span aria-hidden="true" className="rss-discovery-category-card__chevron">›</span>
        </button>
      ))}
    </div>
  );
}

export function RSSDiscoveryRouteGrid({
  routes,
  subscriptions,
  t,
  language,
  onPreview,
}: {
  routes: readonly RSSDiscoveryRoute[];
  subscriptions: readonly RSSSubscription[];
  t: TFunction;
  language: string;
  onPreview: (route: RSSDiscoveryRoute) => void;
}) {
  // A catalog source commonly owns dozens of routes, but every projected
  // route URL resolves to the same source favicon. Reuse one opaque controlled
  // candidate per source so WebKit can coalesce the image load instead of
  // briefly showing initials while it downloads the same favicon per card.
  const iconURLBySource = rssDiscoverySourceIconURLs(routes);
  return (
    <div className="rss-discovery-route-grid">
      {routes.map((route) => {
        const subscribed = rssDiscoveryRouteSubscribed(route, subscriptions);
        const requirements = [
          route.needsParameters ? t("xiadown.rss.requiresParameters") : "",
          route.requiresConfig ? t("xiadown.rss.requiresConfig") : "",
          route.requiresPuppeteer ? t("xiadown.rss.requiresPuppeteer") : "",
        ].filter(Boolean).join(" · ");
        return (
          <button
            className="rss-discovery-route-card app-dream-card app-motion-surface"
            data-subscribed={subscribed || undefined}
            key={route.id}
            onClick={(event) => {
              // WKWebView does not always focus a button on pointer click. Keep
              // an explicit opener so the modal can restore focus reliably.
              event.currentTarget.focus();
              onPreview(route);
            }}
            type="button"
          >
            <RSSDiscoveryRouteIcon
              iconUrl={iconURLBySource.get(discoverySourceIconKey(route))}
              route={route}
            />
            <span className="rss-discovery-route-card__copy">
              <span className="rss-discovery-route-card__title">
                <strong>{route.title || route.sourceName}</strong>
                {subscribed ? <mark><Check />{t("xiadown.rss.subscribed")}</mark> : null}
              </span>
              <small>{route.description || route.url}</small>
              <span className="rss-discovery-route-card__footer">
                <span className="rss-discovery-route-card__meta">
                  {route.categories.slice(0, 2).map((category) => <em key={category}>{rssDiscoveryCategoryLabel(category, t)}</em>)}
                  {route.heat > 0 ? <em><Flame />{formatRSSDiscoveryNumber(route.heat, language)}</em> : null}
                </span>
                {requirements ? (
                  <span className="rss-discovery-route-card__requirements" title={requirements}>
                    <Settings2 />
                    <span>{requirements}</span>
                  </span>
                ) : null}
              </span>
            </span>
          </button>
        );
      })}
    </div>
  );
}

export function RSSDiscoveryRouteIcon({
  route,
  iconUrl,
}: {
  route: RSSDiscoveryRoute;
  iconUrl?: string;
}) {
  const initials = rssDiscoveryRouteInitials(route);
  const categoryEmoji = route.categories
    .map((category) => RSS_DISCOVERY_CATEGORY_EMOJI[category])
    .find(Boolean);
  const fallback = initials ? (
    <span className="rss-discovery-route-icon__initials">{initials}</span>
  ) : categoryEmoji ? (
    <span className="rss-discovery-route-icon__emoji">{categoryEmoji}</span>
  ) : (
    <Rss />
  );
  return (
    <span aria-hidden="true" className="rss-favicon rss-discovery-route-icon">
      <RSSRemoteImage alt="" fallback={fallback} sources={[iconUrl ?? route.iconUrl]} />
    </span>
  );
}

export function rssDiscoverySourceIconURLs(routes: readonly RSSDiscoveryRoute[]) {
  const result = new Map<string, string>();
  for (const route of routes) {
    const key = discoverySourceIconKey(route);
    if (result.has(key)) continue;
    const controlled = controlledRSSResourceURL(route.iconUrl);
    if (controlled) result.set(key, controlled);
  }
  return result;
}

function discoverySourceIconKey(route: RSSDiscoveryRoute) {
  const provider = route.provider.trim().toLowerCase();
  const sourceId = route.sourceId.trim().toLowerCase();
  return sourceId ? `${provider}:${sourceId}` : route.id;
}
