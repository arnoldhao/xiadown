import { describe, expect, test } from "bun:test";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderToStaticMarkup } from "react-dom/server";

import {
  RSSOrganizationManager,
} from "./RSSOrganizationManager";
import {
  moveOrganizationID,
  selectionAfterRSSBulkAction,
} from "./rss-organization-utils";
import type {
  RSSCategory,
  RSSCollection,
  RSSSubscription,
} from "./types";

const subscription: RSSSubscription = {
  id: "subscription-1",
  workspaceId: "rss-default",
  feedUrl: "https://example.com/feed.xml",
  title: "Example feed",
  viewType: "article",
  enabled: true,
  unreadCount: 2,
  createdAt: "2026-07-17T00:00:00Z",
  updatedAt: "2026-07-17T00:00:00Z",
  revision: 1,
};

const category: RSSCategory = {
  id: "category-1",
  workspaceId: "rss-default",
  title: "Reading",
  sortOrder: 0,
  subscriptionCount: 1,
  unreadCount: 2,
  createdAt: "2026-07-17T00:00:00Z",
  updatedAt: "2026-07-17T00:00:00Z",
  revision: 1,
};

const collection: RSSCollection = {
  id: "collection-1",
  workspaceId: "rss-default",
  title: "Daily",
  kind: "subscriptions",
  viewType: "auto",
  sortOrder: 0,
  itemCount: 1,
  unreadCount: 2,
  createdAt: "2026-07-17T00:00:00Z",
  updatedAt: "2026-07-17T00:00:00Z",
  revision: 1,
};

describe("RSS organization manager", () => {
  test("renders compact accessible controls for every organization type", () => {
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    });
    const html = renderToStaticMarkup(
      <QueryClientProvider client={queryClient}>
        <RSSOrganizationManager
          categories={[category]}
          collections={[collection]}
          onSelectionChange={() => undefined}
          selectedSubscriptionIDs={new Set([subscription.id])}
          subscriptions={[subscription]}
        />
      </QueryClientProvider>,
    );

    expect(html).toContain("rss-organization-manager");
    expect(html).toContain("Organize RSS");
    expect(html).toContain("Create category");
    expect(html).toContain("Create list");
    queryClient.clear();
  });

  test("reorders without mutation and retains only failed bulk selections", () => {
    const order = ["one", "two", "three"];
    expect(moveOrganizationID(order, "two", -1)).toEqual([
      "two",
      "one",
      "three",
    ]);
    expect(moveOrganizationID(order, "one", -1)).toEqual(order);
    expect(order).toEqual(["one", "two", "three"]);
    expect([...selectionAfterRSSBulkAction({
      requested: 3,
      succeededIDs: ["one", "three"],
      failures: [{ id: "two", error: new Error("failed") }],
    })]).toEqual(["two"]);
  });

  test("keeps mutations on the shared API and destructive actions behind a dialog", async () => {
    const sourceCode = await Bun.file(
      new URL("./RSSOrganizationManager.tsx", import.meta.url),
    ).text();
    const [style, rssAppearance] = await Promise.all([
      Bun.file(new URL("./rss-organization-manager.css", import.meta.url)).text(),
      Bun.file(
        new URL("../../shared/styles/dream/rss.css", import.meta.url),
      ).text(),
    ]);

    for (const hook of [
      "useRSSCreateCategory",
      "useRSSUpdateCategory",
      "useRSSDeleteCategory",
      "useRSSReorderCategories",
      "useRSSCreateCollection",
      "useRSSAddCollectionItems",
    ]) {
      expect(sourceCode).toContain(hook);
    }
    expect(sourceCode).toContain("<DialogContent");
    expect(sourceCode).toContain("runRSSBulkAction");
    expect(sourceCode).not.toMatch(/^export function (moveOrganizationID|selectionAfterRSSBulkAction)/m);
    expect(sourceCode).toContain('import "./rss-organization-manager.css"');
    expect(style).toContain(".rss-organization-manager__grid");
    expect(style).toContain("container-type: inline-size");
    expect(rssAppearance).toContain("color: var(--app-status-tone-error)");
    expect(rssAppearance).not.toContain("--app-danger");
    expect(rssAppearance).not.toMatch(/#[0-9a-f]{3,8}\b/i);
    expect(style).toContain("@container rss-organization-manager (max-width: 540px)");
  });
});
