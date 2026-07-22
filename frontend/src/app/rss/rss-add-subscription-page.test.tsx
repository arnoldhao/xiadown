import { describe, expect, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";

import {
  canonicalizeRSSHubInput,
  catalogLanguageForLocale,
  buildRSSDiscoveryRouteURL,
  formatRSSDiscoveryDate,
  formatRSSDiscoveryNumber,
  initialRSSDiscoveryParameterValues,
  isRSSDiscoveryAddress,
  mergeRSSLocalEntryPages,
  mergeRSSLocalSearchEntries,
  mergeRSSDiscoveryPages,
  rssDiscoveryRouteInitials,
  rssDiscoveryRouteMatchesFeedURL,
  rssDiscoveryRouteSubscribed,
  searchRSSSubscriptions,
  normalizeDiscoveryRequest,
  RSSDiscoveryRouteConfigurationError,
} from "./discovery-utils";
import {
  RSSDiscoveryCategoryGrid,
  RSSDiscoveryRouteGrid,
  RSSDiscoveryRouteIcon,
  rssDiscoverySourceIconURLs,
} from "./RSSDiscoveryCards";
import { RSSRemoteImage } from "./RSSRemoteImage";
import type { RSSDiscoveryRoute, RSSEntry, RSSPreviewResult, RSSSubscription } from "./types";

const route: RSSDiscoveryRoute = {
  id: "rsshub:bilibili/ranking/0",
  title: "Bilibili ranking",
  url: "rsshub://bilibili/ranking/0",
  provider: "rsshub",
  description: "Latest videos from a creator",
  sourceId: "bilibili",
  sourceName: "Bilibili",
  sourceUrl: "https://www.bilibili.com",
  siteUrl: "https://www.bilibili.com",
  routePath: "bilibili/ranking/0",
  examplePath: "bilibili/ranking/0",
  categories: ["multimedia"],
  heat: 128,
  language: "zh-CN",
  region: "CN",
  viewType: "video",
  parameters: [],
  needsParameters: false,
  requiresConfig: true,
  requiresPuppeteer: true,
};

const subscription: RSSSubscription = {
  id: "subscription-1",
  workspaceId: "rss-default",
  feedUrl: "rsshub://bilibili/ranking/0/",
  title: "Bilibili ranking",
  viewType: "video",
  enabled: true,
  unreadCount: 0,
  createdAt: "2026-07-13T00:00:00Z",
  updatedAt: "2026-07-13T00:00:00Z",
  revision: 1,
};

const localEntry = {
  id: "entry-1",
  subscriptionId: subscription.id,
  externalId: "external-1",
  url: "https://example.com/post#comments",
  title: "First local result",
  summary: "A matching entry stored in the local RSS library.",
  kind: "article",
  imageUrls: [],
  media: [],
  stateRevision: 1,
  revision: 1,
  createdAt: "2026-07-13T00:00:00Z",
  modifiedAt: "2026-07-13T00:00:00Z",
} satisfies RSSEntry;

const t = (key: string) => ({
  "xiadown.rss.discoveryCategoryMultimedia": "Multimedia",
  "xiadown.rss.categoryDescription": "Curated routes",
  "xiadown.rss.routes": "Routes",
  "xiadown.rss.subscribed": "Subscribed",
  "xiadown.rss.requiresConfig": "Needs configuration",
  "xiadown.rss.requiresPuppeteer": "Needs browser session",
}[key] ?? key);

describe("RSS subscription discovery", () => {
  test("reuses successful previews and permits pending subscriptions after preview failure", async () => {
    const {
      buildRSSAddSubscriptionRequest,
      buildRSSSubscriptionUpdateRequest,
    } = await import("./subscription-dialog-utils");
    const preview: RSSPreviewResult = {
      subscription,
      entries: [localEntry],
      resolvedUrl: "https://example.com/feed.xml",
      previewToken: "preview-token-1",
    };

    expect(buildRSSAddSubscriptionRequest(
      "https://example.com/feed.xml",
      "article",
      preview,
    )).toEqual({
      url: "https://example.com/feed.xml",
      viewType: "article",
      title: "Bilibili ranking",
      previewToken: "preview-token-1",
      allowPending: true,
    });
    expect(buildRSSAddSubscriptionRequest(
      "https://unavailable.example/feed.xml",
      "auto",
      null,
    )).toEqual({
      url: "https://unavailable.example/feed.xml",
      viewType: "auto",
      allowPending: true,
    });
    expect(buildRSSAddSubscriptionRequest(
      "https://example.com/feed.xml",
      "video",
      preview,
      "  Custom title  ",
    )).toMatchObject({
      title: "Custom title",
      viewType: "video",
      previewToken: "preview-token-1",
    });
    expect(buildRSSSubscriptionUpdateRequest(subscription, {
      title: "  Renamed feed  ",
      viewType: subscription.viewType,
      enabled: subscription.enabled,
    })).toEqual({ id: subscription.id, title: "Renamed feed" });
    expect(buildRSSSubscriptionUpdateRequest(subscription, {
      title: subscription.title,
      viewType: "article",
      enabled: false,
      categoryId: "category-reading",
    })).toEqual({
      id: subscription.id,
      viewType: "article",
      enabled: false,
      categoryId: "category-reading",
    });
    expect(buildRSSSubscriptionUpdateRequest({
      ...subscription,
      categoryId: "category-reading",
    }, {
      title: subscription.title,
      viewType: subscription.viewType,
      enabled: subscription.enabled,
      categoryId: "",
    })).toEqual({ id: subscription.id, categoryId: "" });
  });

  test("keeps Wails preview failures short and hides runtime payload details", async () => {
    const { rssPreviewErrorText } = await import("./subscription-dialog-utils");
    const hint = "Try again, or subscribe while RSS loads in the background.";
    const runtimeFailure = new Error(
      `Couldn't preview this feed ${JSON.stringify({
        message: "RSS fetch failed: https://feeds.example/private?q=secret: Get request: context deadline exceeded",
        cause: { internal: "network stack" },
        kind: "RuntimeError",
      })}`,
    );

    const normalized = rssPreviewErrorText(runtimeFailure, hint);
    expect(normalized).toBe(hint);
    expect(normalized).not.toContain("RuntimeError");
    expect(normalized).not.toContain("cause");
    expect(normalized).not.toContain("feeds.example");

    expect(rssPreviewErrorText(
      new Error(`transport failed ${JSON.stringify({
        message: "RSS fetch failed: https://feeds.example/rss: HTTP status 404",
        cause: {},
        kind: "RuntimeError",
      })}`),
      hint,
    )).toBe(`HTTP 404 · ${hint}`);
  });

  test("distinguishes catalog queries from previewable addresses and preserves canonical RSSHub routes", () => {
    expect(isRSSDiscoveryAddress("Bilibili videos")).toBe(false);
    expect(isRSSDiscoveryAddress("rsshub://bilibili/ranking/0")).toBe(true);
    expect(isRSSDiscoveryAddress("/bilibili/ranking/0")).toBe(true);
    expect(isRSSDiscoveryAddress("feed://example.com/rss.xml")).toBe(true);
    expect(canonicalizeRSSHubInput("/bilibili/ranking/0")).toBe("rsshub://bilibili/ranking/0");
    expect(canonicalizeRSSHubInput("feed://example.com/rss.xml")).toBe("https://example.com/rss.xml");
  });

  test("maps app locales to catalog language identifiers", () => {
    expect(catalogLanguageForLocale("ja-JP")).toBe("ja");
    expect(catalogLanguageForLocale("ko_KR")).toBe("ko");
    expect(catalogLanguageForLocale("es-419")).toBe("es");
    expect(catalogLanguageForLocale("zh-HK")).toBe("zh-TW");
    expect(catalogLanguageForLocale("en-US")).toBe("en");
    expect(catalogLanguageForLocale("pt-BR")).toBe("");
    expect(catalogLanguageForLocale("id-ID")).toBe("");
    expect(catalogLanguageForLocale("vi-VN")).toBe("");
    expect(catalogLanguageForLocale("fr-FR")).toBe("");
  });

  test("normalizes discovery pagination without leaking empty filters", () => {
    expect(normalizeDiscoveryRequest({
      query: "  video  ",
      categoryId: " ",
      language: " ja ",
      sort: "title",
      offset: -4,
      limit: 36.8,
    })).toEqual({ query: "video", language: "ja", sort: "title", offset: 0, limit: 36 });
  });

  test("renders category and route cards with discovery state", () => {
    const categoryMarkup = renderToStaticMarkup(
      <RSSDiscoveryCategoryGrid
        categories={[{ id: "multimedia", count: 42, examples: ["YouTube"], iconUrl: "", iconLabel: "" }]}
        language="en"
        onSelect={() => undefined}
        t={t}
      />,
    );
    expect(categoryMarkup).toContain("🎬");
    expect(categoryMarkup).toContain("Multimedia");
    expect(categoryMarkup).toContain("42 Routes");

    const routeMarkup = renderToStaticMarkup(
      <RSSDiscoveryRouteGrid language="en" routes={[route]} subscriptions={[subscription]} t={t} onPreview={() => undefined} />,
    );
    expect(routeMarkup).toContain("Bilibili ranking");
    expect(routeMarkup).toContain("Subscribed");
    expect(routeMarkup).toContain("Needs configuration");
    expect(routeMarkup).toContain("Needs browser session");
    expect(routeMarkup).toContain('data-subscribed="true"');
    expect(routeMarkup).toContain("rss-discovery-route-icon__initials");
    expect(routeMarkup).toContain(">BI</span>");
    expect(routeMarkup).not.toContain("<img");
    expect(routeMarkup).not.toContain(route.siteUrl);
  });

  test("creates stable local source marks without using catalog URLs", () => {
    expect(rssDiscoveryRouteInitials(route)).toBe("BI");
    expect(rssDiscoveryRouteInitials({ ...route, sourceName: "YouTube" })).toBe("YT");
    expect(rssDiscoveryRouteInitials({ ...route, sourceName: "The Verge" })).toBe("TV");
    expect(rssDiscoveryRouteInitials({ ...route, sourceName: "\u54d4\u54e9\u54d4\u54e9" })).toBe("\u54d4\u54e9");
    expect(rssDiscoveryRouteInitials({ ...route, sourceName: " ", sourceId: "bilibili" })).toBe("BI");
    expect(rssDiscoveryRouteInitials({ ...route, sourceName: "", sourceId: "" })).toBe("");

    const emojiMarkup = renderToStaticMarkup(
      <RSSDiscoveryRouteIcon route={{ ...route, sourceName: "", sourceId: "" }} />,
    );
    expect(emojiMarkup).toContain("rss-discovery-route-icon__emoji");
    expect(emojiMarkup).toContain("🎬");
    expect(emojiMarkup).not.toContain("<img");

    const rssMarkup = renderToStaticMarkup(
      <RSSDiscoveryRouteIcon route={{ ...route, sourceName: "", sourceId: "", categories: ["unknown"] }} />,
    );
    expect(rssMarkup).toContain("<svg");
    expect(rssMarkup).not.toContain("<img");
  });

  test("keeps projected images hidden behind a skeleton until they load", () => {
    const token = "a".repeat(64);
    const projected = `http://127.0.0.1:43127/_xiadown/${token}/api/rss/subscriptions/subscription-1/icon`;
    const loadingMarkup = renderToStaticMarkup(
      <RSSRemoteImage alt="" fallback={<span className="fallback" />} src={projected} />,
    );
    expect(loadingMarkup).toContain("rss-remote-image--probe");
    expect(loadingMarkup).toContain('data-rss-image-state="loading"');
    expect(loadingMarkup).toContain("rss-image-skeleton");

    const rejectedMarkup = renderToStaticMarkup(
      <RSSRemoteImage alt="" src="https://example.com/unprojected.jpg" />,
    );
    expect(rejectedMarkup).not.toContain("<img");
    expect(rejectedMarkup).toContain('data-rss-image-state="empty"');
  });

  test("uses taxonomy emoji and renders source favicons only through projected RSS resources", () => {
    const token = "a".repeat(64);
    const categoryIcon = `http://127.0.0.1:43127/_xiadown/${token}/api/rss/discovery/categories/multimedia/icon`;
    const routeIcon = `http://127.0.0.1:43127/_xiadown/${token}/api/rss/discovery/routes/rsshub:bilibili-ranking/icon`;
    const categoryMarkup = renderToStaticMarkup(
      <RSSDiscoveryCategoryGrid
        categories={[{ id: "multimedia", count: 42, examples: [], iconUrl: categoryIcon, iconLabel: "Multimedia" }]}
        language="en"
        onSelect={() => undefined}
        t={t}
      />,
    );
    const routeMarkup = renderToStaticMarkup(
      <RSSDiscoveryRouteGrid
        language="en"
        routes={[{ ...route, iconUrl: routeIcon }]}
        subscriptions={[]}
        t={t}
        onPreview={() => undefined}
      />,
    );

    expect(categoryMarkup).toContain("🎬");
    expect(categoryMarkup).not.toContain("<img");
    expect(categoryMarkup).not.toContain(categoryIcon);
    expect(routeMarkup).toContain("<img");
    expect(routeMarkup).not.toContain("src=");
    expect([...rssDiscoverySourceIconURLs([{ ...route, iconUrl: routeIcon }]).values()]).toEqual([routeIcon]);
    expect(routeMarkup).toContain("rss-discovery-route-card__footer");
  });

  test("coalesces projected favicon candidates for routes from the same source", () => {
    const token = "a".repeat(64);
    const firstIcon = `http://127.0.0.1:43127/_xiadown/${token}/api/rss/discovery/routes/rsshub:bilibili-ranking/icon`;
    const secondIcon = `http://127.0.0.1:43127/_xiadown/${token}/api/rss/discovery/routes/rsshub:bilibili-user/icon`;
    const routes = [
      { ...route, iconUrl: firstIcon },
      { ...route, id: "rsshub:bilibili-user", title: "Bilibili user", iconUrl: secondIcon },
    ];
    const markup = renderToStaticMarkup(
      <RSSDiscoveryRouteGrid
        language="en"
        routes={routes}
        subscriptions={[]}
        t={t}
        onPreview={() => undefined}
      />,
    );

    expect([...rssDiscoverySourceIconURLs(routes).values()]).toEqual([firstIcon]);
    expect(markup).not.toContain(firstIcon);
    expect(markup).not.toContain(secondIcon);
  });

  test("matches canonical subscriptions and deduplicates paginated routes", () => {
    expect(rssDiscoveryRouteSubscribed(route, [subscription])).toBe(true);
    expect(mergeRSSDiscoveryPages([route, { ...route, title: "Updated title" }])).toEqual([
      { ...route, title: "Updated title" },
    ]);
  });

  test("keeps local search relevance order while deduplicating entry ids and canonical URLs", () => {
    expect(mergeRSSLocalSearchEntries([
      localEntry,
      { ...localEntry, title: "Duplicate id" },
      { ...localEntry, id: "entry-2", externalId: "external-2", url: "https://example.com/post" },
      { ...localEntry, id: "entry-3", externalId: "external-3", url: "https://example.com/next" },
    ]).map((item) => item.id)).toEqual(["entry-1", "entry-3"]);
  });

  test("renders local search pages beyond 36 entries while a remote error remains isolated", async () => {
    const {
      RSSLocalEntryResults,
      RSSRemoteDiscoveryResults,
    } = await import("./RSSAddSubscriptionPage");
    const firstPage = Array.from({ length: 36 }, (_, index) => ({
      ...localEntry,
      id: `local-${index + 1}`,
      externalId: `local-${index + 1}`,
      url: `https://example.com/posts/${index + 1}`,
      title: `Local result ${index + 1}`,
    }));
    const secondPage = Array.from({ length: 4 }, (_, index) => ({
      ...localEntry,
      id: `local-${index + 37}`,
      externalId: `local-${index + 37}`,
      url: `https://example.com/posts/${index + 37}`,
      title: `Local result ${index + 37}`,
    }));
    const entries = mergeRSSLocalEntryPages([
      { items: firstPage },
      { items: secondPage },
    ]);

    const markup = renderToStaticMarkup(
      <div>
        <RSSLocalEntryResults
          entries={entries}
          error={null}
          hasNextPage={true}
          language="en-US"
          loading={false}
          loadingMore={false}
          subscriptions={[subscription]}
          t={t}
          total={80}
          onLoadMore={() => undefined}
          onOpen={() => undefined}
          onRetry={() => undefined}
        />
        <RSSRemoteDiscoveryResults
          error={new Error("Remote catalog unavailable")}
          hasNextPage={false}
          language="en-US"
          loading={false}
          loadingMore={false}
          routes={[]}
          subscriptions={[subscription]}
          t={t}
          onLoadMore={() => undefined}
          onPreview={() => undefined}
          onRetry={() => undefined}
        />
      </div>,
    );

    expect(entries).toHaveLength(40);
    expect((markup.match(/data-entry-id=/g) ?? [])).toHaveLength(40);
    expect(markup).toContain("Local result 40");
    expect(markup).toContain("xiadown.rss.loadMore");
    expect(markup).toContain("Remote catalog unavailable");
  });

  test("keeps remote discovery results visible when local search fails", async () => {
    const { RSSLocalEntryResults, RSSRemoteDiscoveryResults } = await import("./RSSAddSubscriptionPage");
    const markup = renderToStaticMarkup(
      <div>
        <RSSLocalEntryResults
          entries={[]}
          error={new Error("Local library unavailable")}
          hasNextPage={false}
          language="en-US"
          loading={false}
          loadingMore={false}
          subscriptions={[subscription]}
          t={t}
          total={0}
          onLoadMore={() => undefined}
          onRetry={() => undefined}
        />
        <RSSRemoteDiscoveryResults
          error={null}
          fetchedAt="2026-07-13T00:00:00Z"
          hasNextPage={true}
          language="en-US"
          loading={false}
          loadingMore={false}
          routes={[route]}
          sourceUrl="https://github.com/DIYgod/RSSHub"
          subscriptions={[]}
          t={t}
          total={72}
          onLoadMore={() => undefined}
          onPreview={() => undefined}
          onRetry={() => undefined}
        />
      </div>,
    );

    expect(markup).toContain("Local library unavailable");
    expect(markup).toContain("Bilibili ranking");
    expect(markup).toContain("72");
    expect(markup).toContain("xiadown.rss.loadMore");
  });

  test("broadly matches existing subscriptions by any token and ranks complete phrase matches first", () => {
    const subscriptions = [
      { ...subscription, id: "one", title: "Daily video archive", feedUrl: "https://example.com/archive.xml", unreadCount: 2 },
      { ...subscription, id: "two", title: "Bilibili creator videos", feedUrl: "https://example.com/bili.xml", unreadCount: 1 },
      { ...subscription, id: "three", title: "Unrelated reading", feedUrl: "https://example.com/reading.xml", unreadCount: 8 },
    ];
    expect(searchRSSSubscriptions(subscriptions, "bilibili video").map((item) => item.id)).toEqual([
      "two",
      "one",
    ]);
    expect(searchRSSSubscriptions(subscriptions, "  ")).toEqual([]);
  });

  test("builds concrete canonical routes only from entered values, never examples", () => {
    const dynamicRoute: RSSDiscoveryRoute = {
      ...route,
      id: "rsshub:youtube/user/:id",
      routePath: "youtube/user/:id",
      examplePath: "youtube/user/@official-example",
      url: "rsshub://youtube/user/:id",
      needsParameters: true,
      parameters: [{
        name: "id",
        description: "Channel handle",
        defaultValue: null,
        exampleValue: "@official-example",
        optional: false,
        catchAll: false,
        type: "text",
        options: [],
      }],
    };
    expect(initialRSSDiscoveryParameterValues(dynamicRoute)).toEqual({ id: "" });
    expect(() => buildRSSDiscoveryRouteURL(dynamicRoute, { id: "" })).toThrow(RSSDiscoveryRouteConfigurationError);
    expect(buildRSSDiscoveryRouteURL(dynamicRoute, { id: "@my channel" })).toBe("rsshub://youtube/user/%40my%20channel");
    expect(rssDiscoveryRouteSubscribed(dynamicRoute, [{ ...subscription, feedUrl: "rsshub://youtube/user/%40my%20channel" }])).toBe(true);
    expect(rssDiscoveryRouteSubscribed(dynamicRoute, [{ ...subscription, feedUrl: dynamicRoute.url }])).toBe(false);
  });

  test("prefills declared defaults and enforces structured options", () => {
    const optionRoute: RSSDiscoveryRoute = {
      ...route,
      routePath: "example/list/:mode",
      url: "rsshub://example/list/:mode",
      needsParameters: true,
      parameters: [{
        name: "mode",
        description: "List mode",
        defaultValue: "latest",
        exampleValue: "popular",
        optional: false,
        catchAll: false,
        type: "string",
        options: [
          { value: "latest", label: "Latest" },
          { value: "popular", label: "Popular" },
        ],
      }],
    };
    expect(initialRSSDiscoveryParameterValues(optionRoute)).toEqual({ mode: "latest" });
    expect(buildRSSDiscoveryRouteURL(optionRoute, { mode: "latest" })).toBe("rsshub://example/list/latest");
    expect(() => buildRSSDiscoveryRouteURL(optionRoute, { mode: "unsupported" })).toThrow();
  });

  test("accepts the same hyphenated parameter names as the backend catalog parser", () => {
    const hyphenatedRoute: RSSDiscoveryRoute = {
      ...route,
      routePath: "example/user/:user-id",
      url: "rsshub://example/user/:user-id",
      needsParameters: true,
      parameters: [{
        name: "user-id",
        description: "User identifier",
        defaultValue: null,
        exampleValue: "official-example",
        optional: false,
        catchAll: false,
        type: "text",
        options: [],
      }],
    };
    expect(buildRSSDiscoveryRouteURL(hyphenatedRoute, { "user-id": "my account" })).toBe(
      "rsshub://example/user/my%20account",
    );
    expect(rssDiscoveryRouteMatchesFeedURL(hyphenatedRoute, "rsshub://example/user/my%20account")).toBe(true);
  });

  test("encodes catch-all segments and rejects optional gaps and naked wildcards", () => {
    const catchAllRoute: RSSDiscoveryRoute = {
      ...route,
      routePath: "mail/imap/:email/:folder{.+}?/:mode?",
      url: "rsshub://mail/imap/:email/:folder{.+}?/:mode?",
      needsParameters: true,
      parameters: [
        { name: "email", description: "", defaultValue: null, exampleValue: "", optional: false, catchAll: false, type: "text", options: [] },
        { name: "folder", description: "", defaultValue: null, exampleValue: "", optional: true, catchAll: true, type: "text", options: [] },
        { name: "mode", description: "", defaultValue: null, exampleValue: "", optional: true, catchAll: false, type: "text", options: [] },
      ],
    };
    expect(buildRSSDiscoveryRouteURL(catchAllRoute, { email: "me@example.com", folder: "News/\u4ea7\u54c1", mode: "full" })).toBe(
      "rsshub://mail/imap/me%40example.com/News/%E4%BA%A7%E5%93%81/full",
    );
    expect(() => buildRSSDiscoveryRouteURL(catchAllRoute, { email: "me@example.com", folder: "", mode: "full" })).toThrow();
    expect(() => buildRSSDiscoveryRouteURL({ ...catchAllRoute, routePath: "broken/*" }, {})).toThrow();
    expect(rssDiscoveryRouteMatchesFeedURL(catchAllRoute, "rsshub://mail/imap/me%40example.com/News/Tech/full")).toBe(true);
    expect(rssDiscoveryRouteMatchesFeedURL(catchAllRoute, "rsshub://mail/imap/me%40example.com")).toBe(true);
    expect(rssDiscoveryRouteMatchesFeedURL(catchAllRoute, catchAllRoute.url)).toBe(false);
  });

  test("formats catalog numbers and dates with the UI locale", () => {
    expect(formatRSSDiscoveryNumber(12345, "en-US")).toBe("12,345");
    expect(formatRSSDiscoveryNumber(12345, "de-DE")).toBe("12.345");
    expect(formatRSSDiscoveryDate("2026-07-13T00:00:00Z", "en-US")).toContain("2026");
  });

  test("uses the shared focus-trapping dialog and never previews a dynamic template directly", async () => {
    const [dialogSource, dialogUtilsSource, pageSource, workspacePageSource] = await Promise.all([
      Bun.file(new URL("./RSSSubscriptionDialog.tsx", import.meta.url)).text(),
      Bun.file(new URL("./subscription-dialog-utils.ts", import.meta.url)).text(),
      Bun.file(new URL("./RSSAddSubscriptionPage.tsx", import.meta.url)).text(),
      Bun.file(new URL("./RSSWorkspacePage.tsx", import.meta.url)).text(),
    ]);
    expect(dialogSource).toContain('from "@/shared/ui/dialog"');
    expect(dialogSource).toContain("<Dialog open");
    expect(dialogSource).not.toContain("overlayClassName=");
    expect(dialogSource).not.toContain("unstyled");
    expect(dialogSource).toContain("onOpenAutoFocus");
    expect(dialogSource).toContain("onCloseAutoFocus");
    expect(dialogSource).toContain('aria-live="polite"');
    expect(dialogSource).toContain("setPreviewAttempt");
    expect(dialogSource).toContain("setPreviewError(rssPreviewErrorText(");
    expect(dialogSource).toContain('t("xiadown.rss.previewFailureHint")');
    expect(dialogSource).toContain('props.target.kind === "edit"');
    expect(dialogSource).toContain("await update.mutateAsync(buildRSSSubscriptionUpdateRequest(");
    expect(dialogSource).toContain("if (!open && !update.isPending && !changed) onClose()");
    expect(dialogSource).toContain("{update.error ? (");
    expect(dialogSource).not.toContain("export function buildRSSAddSubscriptionRequest");
    expect(dialogSource).not.toContain("export function rssPreviewErrorText");
    expect(dialogUtilsSource).toContain("previewToken: preview.previewToken");
    expect(dialogUtilsSource).toContain("allowPending: true");
    expect(dialogSource).toContain("disabled={!title.trim() || add.isPending || subscribed}");
    expect(dialogSource).not.toContain("disabled={!preview || add.isPending || subscribed}");
    expect(dialogSource).not.toContain('window.addEventListener("keydown"');
    expect(pageSource).toContain('url: route.needsParameters ? "" : route.url');
    expect(pageSource).toContain('language: isSearch ? undefined : browseLanguage');
    expect(pageSource).toContain('data-discovery-mode={mode}');
    expect(pageSource).toContain("useRSSEntriesInfinite(");
    expect(pageSource).toContain("mergeRSSLocalEntryPages");
    expect(pageSource).not.toContain("export function mergeRSSLocalEntryPages");
    expect(pageSource).toContain("onOpenEntry");
    expect(pageSource).toContain("RSSLocalEntryResults");
    expect(pageSource).toContain("DreamSegmentSwitch");
    expect(pageSource).toContain('value: "local"');
    expect(pageSource).toContain('value: "subscriptions"');
    expect(pageSource).toContain('value: "discovery"');
    expect(pageSource).toContain("<WorkspaceSearchControl");
    expect(pageSource).toContain('from "@/shared/ui/workspace-search-control"');
    expect(pageSource).toContain('from "@/shared/ui/workspace-page"');
    expect(pageSource).toContain("<WorkspacePage");
    expect(pageSource).toContain("<WorkspacePageTopBar");
    expect(pageSource).toContain("<WorkspacePageContent");
    expect(pageSource).toContain('recipe: isSearch ? "search" : "collection"');
    expect(pageSource).toContain('heading: "assistive"');
    expect(pageSource).toContain('footer: "none"');
    expect(pageSource).not.toContain('<h1 className="sr-only">');
    expect(pageSource).not.toContain("app-station-search-header__title");
    expect(pageSource).not.toContain("rss-discovery-content-search");
    expect(pageSource).not.toContain("rss-discovery-hero");
    expect(pageSource).not.toContain("rss-discovery-section-heading");
    expect(workspacePageSource).toContain("RSS_WORKSPACE_ROUTE_IDS.discoverySearch");
    expect(workspacePageSource).toContain("RSS_WORKSPACE_ROUTE_IDS.discoveryBrowse");
    expect(workspacePageSource).toContain("RSS_WORKSPACE_ROUTE_IDS.addSubscription");
    expect(workspacePageSource).toContain('? "browse"');
  });

  test("preserves the RSSHub product name in Simplified and Traditional Chinese", async () => {
    const localeURLs = [
      new URL("../../shared/i18n/locales/zh-CN.json", import.meta.url),
      new URL("../../shared/i18n/locales/zh-TW.json", import.meta.url),
    ];
    const rssHubKeys = [
      "categoryDescription",
      "puppeteerPreviewHint",
    ];

    for (const localeURL of localeURLs) {
      const locale = JSON.parse(await Bun.file(localeURL).text()) as {
        xiadown: { rss: Record<string, string> };
      };
      for (const key of rssHubKeys) {
        expect(locale.xiadown.rss[key]).toContain("RSSHub");
      }
    }
  });
});
