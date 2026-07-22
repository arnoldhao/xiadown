import { describe, expect, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";

import {
  RSSWorkspaceSidebar,
} from "./RSSWorkspaceSidebar";
import {
  createRSSSubscriptionRouteId,
  isRSSSubscriptionContextMenuKey,
  parseRSSSubscriptionRouteId,
} from "../rss/rss-routes";

const catalog = {
  sidebarAriaLabel: "rss",
  unreadLabel: "Unread",
  sections: {
    collections: { label: "Lists" },
    discovery: { label: "Discovery" },
    subscriptions: { label: "Subscriptions" },
  },
  routes: {
    search: { label: "Search" },
    all: { label: "All" },
    articles: { label: "Articles" },
    social: { label: "Social" },
    images: { label: "Images" },
    videos: { label: "Videos" },
    starred: { label: "Favorites" },
    manageSubscriptions: { label: "Manager" },
    discoveryBrowse: { label: "Browse" },
  },
} as const;

describe("RSS workspace sidebar", () => {
  test("renders Discovery routes and subscriptions without an empty control panel", () => {
    const markup = renderToStaticMarkup(
      <RSSWorkspaceSidebar
        activeRouteId="all"
        catalog={catalog}
        collectionUnreadCounts={{
          all: 42,
          articles: 12,
          social: 0,
          images: Number.NaN,
          videos: 1_204,
          starred: 3,
        }}
        onNavigate={() => undefined}
        subscriptions={[
          {
            id: "feed/one",
            workspaceId: "rss-default",
            feedUrl: "https://example.com/feed.xml",
            title: "Example feed",
            viewType: "auto",
            enabled: true,
            unreadCount: 12,
            createdAt: "2026-07-13T00:00:00Z",
            updatedAt: "2026-07-13T00:00:00Z",
            revision: 1,
          },
        ]}
      />,
    );

    for (const label of ["Search", "All", "Articles", "Social", "Images", "Videos", "Favorites", "Manager", "Browse", "Example feed"]) {
      expect(markup).toContain(label);
    }
    expect(markup).toContain('data-section="discovery"');
    expect(markup).toContain('data-section="subscriptions"');
    expect(markup).not.toContain('data-section="collections"');
    expect(markup).not.toContain("Reading");
    expect(markup).not.toContain("Customize sidebar");
    expect(markup).not.toContain("Search feeds");
    expect(markup).not.toContain('data-route-id="add-subscription"');
    expect(markup).not.toContain('data-route-id="discovery-search"');
    expect(markup).not.toContain("app-workspace-wide-sidebar__control-panel");
    expect(markup).toContain("app-rss-workspace-sidebar__favicon");
    expect(markup).toContain('aria-label="Example feed, 12 Unread"');
    expect(markup).toContain('aria-label="All, 42 Unread"');
    expect(markup).toContain('aria-label="Articles, 12 Unread"');
    expect(markup).toContain('aria-label="Videos, 1204 Unread"');
    expect(markup).toContain('aria-label="Favorites, 3 Unread"');
    expect(markup).toContain(">999+<");
    expect(markup).toContain(">12<");
    expect(markup.indexOf("Videos")).toBeLessThan(markup.indexOf("Favorites"));
    expect(markup.indexOf("Favorites")).toBeLessThan(markup.indexOf("Manager"));
    expect(markup.indexOf("Browse")).toBeLessThan(markup.indexOf("Example feed"));
  });

  test("keeps collection count props optional and hides missing or zero totals", () => {
    const markup = renderToStaticMarkup(
      <RSSWorkspaceSidebar
        activeRouteId="articles"
        catalog={catalog}
        onNavigate={() => undefined}
        subscriptions={[]}
      />,
    );

    expect(markup).toContain('aria-label="Articles"');
    expect(markup).not.toContain("Unread");
    expect(markup).not.toContain("app-workspace-nav-button__badge");
  });

  test("renders organization routes and keeps orphaned category feeds reachable", () => {
    const baseSubscription = {
      workspaceId: "rss-default",
      feedUrl: "https://example.com/feed.xml",
      viewType: "article" as const,
      enabled: true,
      unreadCount: 1,
      createdAt: "2026-07-17T00:00:00Z",
      updatedAt: "2026-07-17T00:00:00Z",
      revision: 1,
    };
    const markup = renderToStaticMarkup(
      <RSSWorkspaceSidebar
        activeRouteId="all"
        catalog={catalog}
        categories={[{
          id: "reading",
          workspaceId: "rss-default",
          title: "Reading folder",
          sortOrder: 0,
          subscriptionCount: 1,
          unreadCount: 1,
          createdAt: "2026-07-17T00:00:00Z",
          updatedAt: "2026-07-17T00:00:00Z",
          revision: 1,
        }]}
        collections={[{
          id: "daily",
          workspaceId: "rss-default",
          title: "Daily list",
          kind: "subscriptions",
          viewType: "auto",
          sortOrder: 0,
          itemCount: 1,
          unreadCount: 1,
          createdAt: "2026-07-17T00:00:00Z",
          updatedAt: "2026-07-17T00:00:00Z",
          revision: 1,
        }]}
        onNavigate={() => undefined}
        subscriptions={[
          { ...baseSubscription, id: "known", title: "Known feed", categoryId: "reading" },
          { ...baseSubscription, id: "orphan", title: "Orphan feed", categoryId: "deleted" },
        ]}
      />,
    );

    for (const label of ["Reading folder", "Known feed", "Orphan feed", "Daily list"]) {
      expect(markup).toContain(label);
    }
    expect(markup).toContain('data-route-id="category:reading"');
    expect(markup).toContain('data-route-id="collection:daily"');
  });

  test("round-trips subscription IDs through route-safe encoding", () => {
    const subscriptionID = "feed/\u4e2d\u6587?id=1";
    const route = createRSSSubscriptionRouteId(subscriptionID);
    expect(route).toBe("subscription:feed%2F%E4%B8%AD%E6%96%87%3Fid%3D1");
    expect(parseRSSSubscriptionRouteId(route)).toBe(subscriptionID);
    expect(parseRSSSubscriptionRouteId("subscription:%E0%A4%A")).toBeNull();
  });

  test("supports the native context-menu key and Shift+F10", () => {
    expect(isRSSSubscriptionContextMenuKey("ContextMenu")).toBeTrue();
    expect(isRSSSubscriptionContextMenuKey("F10", true)).toBeTrue();
    expect(isRSSSubscriptionContextMenuKey("F10", false)).toBeFalse();
    expect(isRSSSubscriptionContextMenuKey("Enter", true)).toBeFalse();
  });

  test("keeps paused subscriptions navigable so saved entries remain readable", () => {
    const markup = renderToStaticMarkup(
      <RSSWorkspaceSidebar
        activeRouteId="all"
        catalog={catalog}
        onNavigate={() => undefined}
        subscriptions={[
          {
            id: "paused-feed",
            workspaceId: "rss-default",
            feedUrl: "https://example.com/paused.xml",
            title: "Paused feed",
            viewType: "article",
            enabled: false,
            unreadCount: 3,
            createdAt: "2026-07-13T00:00:00Z",
            updatedAt: "2026-07-13T00:00:00Z",
            revision: 1,
          },
        ]}
      />,
    );
    expect(markup).toContain("Paused feed");
    expect(markup).not.toContain("disabled=\"\"");
    expect(markup).not.toContain('aria-disabled="true"');
  });

  test("renders only controlled tokenized loopback subscription icons", () => {
    const token = "a".repeat(64);
    const controlledIcon = `http://127.0.0.1:43127/_xiadown/${token}/api/rss/subscriptions/safe-feed/icon`;
    const maliciousIcon = "https://attacker.example/tracking.png";
    const markup = renderToStaticMarkup(
      <RSSWorkspaceSidebar
        activeRouteId="all"
        catalog={catalog}
        onNavigate={() => undefined}
        subscriptions={[
          {
            id: "unsafe-feed",
            workspaceId: "rss-default",
            feedUrl: "https://example.com/unsafe.xml",
            title: "Unsafe feed",
            iconUrl: maliciousIcon,
            viewType: "auto",
            enabled: true,
            unreadCount: 0,
            createdAt: "2026-07-13T00:00:00Z",
            updatedAt: "2026-07-13T00:00:00Z",
            revision: 1,
          },
          {
            id: "safe-feed",
            workspaceId: "rss-default",
            feedUrl: "https://example.com/safe.xml",
            title: "Safe feed",
            iconUrl: controlledIcon,
            viewType: "auto",
            enabled: true,
            unreadCount: 0,
            createdAt: "2026-07-13T00:00:00Z",
            updatedAt: "2026-07-13T00:00:00Z",
            revision: 1,
          },
        ]}
      />,
    );

    expect(markup).not.toContain(maliciousIcon);
    expect(markup).toContain(`src="${controlledIcon}"`);
  });

  test("keeps RSS unread badges visually secondary and removes the legacy spacer", async () => {
    const [navigationCSS, rssCSS] = await Promise.all([
      Bun.file(
        new URL("../../shared/styles/dream/workspace.css", import.meta.url),
      ).text(),
      Bun.file(new URL("../rss/rss-workspace.css", import.meta.url)).text(),
    ]);
    expect(navigationCSS).toContain(".app-rss-workspace-sidebar .app-workspace-nav-button__badge");
    expect(navigationCSS).toContain("font-size: 9px");
    expect(navigationCSS).toContain("font-variant-numeric: tabular-nums");
    expect(rssCSS).not.toContain("app-rss-workspace-sidebar__control-placeholder");
  });

  test("wires every subscription context action through the app controller", async () => {
    const [sidebarSource, mainSource] = await Promise.all([
      Bun.file(new URL("./RSSWorkspaceSidebar.tsx", import.meta.url)).text(),
      Bun.file(new URL("../main/MainApp.tsx", import.meta.url)).text(),
    ]);
    expect(sidebarSource).toContain("onMarkSubscriptionRead?.(subscription)");
    expect(sidebarSource).toContain(
      "onEditSubscription?.(subscription, returnFocusRef.current)",
    );
    expect(sidebarSource).toContain(
      "onUnsubscribe?.(subscription, returnFocusRef.current)",
    );
    expect(sidebarSource).toContain("target?.focus()");
    expect(sidebarSource).toContain("button[data-route-id=\"all\"]");
    expect(mainSource).toContain(
      "setRSSEditingSubscription({ subscription, returnFocusTarget })",
    );
    expect(mainSource).toContain(
      "rssUnsubscribeReturnFocusTargetRef.current = returnFocusTarget",
    );
    expect(mainSource).toContain("rssMarkAllRead");
    expect(mainSource).toContain("rssDeleteSubscription");
    expect(mainSource).toContain(
      "returnFocusTarget={rssEditingSubscription.returnFocusTarget}",
    );
  });
});
