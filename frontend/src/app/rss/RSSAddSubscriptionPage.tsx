import {
  ArrowLeft,
  Check,
  LoaderCircle,
  Plus,
  RefreshCcw,
  Rss,
  Search,
  X,
} from "lucide-react";
import * as React from "react";

import {
  useRSSDiscovery,
  useRSSDiscoveryInfinite,
  useRSSEntriesInfinite,
  useRSSRefreshDiscovery,
} from "./api";
import {
  canonicalizeRSSHubInput,
  catalogLanguageForLocale,
  formatRSSDiscoveryDate,
  formatRSSDiscoveryNumber,
  isRSSDiscoveryAddress,
  mergeRSSDiscoveryPages,
  mergeRSSLocalEntryPages,
  rssDiscoveryCategoryLabel,
  searchRSSSubscriptions,
} from "./discovery-utils";
import { controlledRSSResourceURL } from "./remote-resource";
import {
  RSSDiscoveryCategoryGrid,
  RSSDiscoveryRouteGrid,
} from "./RSSDiscoveryCards";
import {
  RSSSubscriptionDialog,
  type RSSSubscriptionDialogTarget,
} from "./RSSSubscriptionDialog";
import { RSSRemoteImage } from "./RSSRemoteImage";
import type {
  RSSDiscoverySort,
  RSSDiscoveryRoute,
  RSSEntry,
  RSSSubscription,
} from "./types";
import { useI18n, type TFunction } from "@/shared/i18n";
import { Button } from "@/shared/ui/button";
import { DreamSegmentSwitch } from "@/shared/ui/dream-segment-switch";
import { Select } from "@/shared/ui/select";
import { WorkspaceSearchControl } from "@/shared/ui/workspace-search-control";
import {
  WorkspacePrimaryHeaderAction as RSSHeaderAction,
  WorkspacePrimaryHeaderActionGroup,
} from "@/shared/ui/workspace-primary-header-action";
import {
  WorkspacePage,
  WorkspacePageContent,
  WorkspacePageTopBar,
  defineWorkspacePageContract,
  isWorkspacePageHeaderScrolled,
  type WorkspacePageHeaderScrollState,
} from "@/shared/ui/workspace-page";

export type RSSSubscriptionDiscoveryMode = "search" | "browse";
type RSSSearchResultTab = "local" | "subscriptions" | "discovery";

export interface RSSAddSubscriptionPageProps {
  mode: RSSSubscriptionDiscoveryMode;
  reserveWindowControls: boolean;
  subscriptions: readonly RSSSubscription[];
  onAdded: (subscription: RSSSubscription) => void;
  /** Opens an entry returned by the local half of unified Search. */
  onOpenEntry?: (entry: RSSEntry) => void;
}

const DISCOVERY_PAGE_SIZE = 36;
const LOCAL_SEARCH_PAGE_SIZE = 36;
const CATALOG_LANGUAGE_OPTIONS = [
  "",
  "zh-CN",
  "zh-TW",
  "en",
  "ja",
  "ko",
  "es",
  "pt-BR",
  "id-ID",
  "vi-VN",
  "fr-FR",
  "de",
  "ru",
] as const;

export function RSSAddSubscriptionPage({
  mode,
  reserveWindowControls,
  subscriptions,
  onAdded,
  onOpenEntry,
}: RSSAddSubscriptionPageProps) {
  const { t, language } = useI18n();
  const isSearch = mode === "search";
  const [input, setInput] = React.useState("");
  const [searchQuery, setSearchQuery] = React.useState("");
  const [selectedCategoryId, setSelectedCategoryId] = React.useState<string | null>(null);
  const [categoryQuery, setCategoryQuery] = React.useState("");
  const [browseLanguage, setBrowseLanguage] = React.useState<string>(() => catalogLanguageForLocale(language));
  const [sort, setSort] = React.useState<RSSDiscoverySort>("popular");
  const [searchResultTab, setSearchResultTab] = React.useState<RSSSearchResultTab>("discovery");
  const [previewTarget, setPreviewTarget] = React.useState<RSSSubscriptionDialogTarget | null>(null);
  const [headerScrollState, setHeaderScrollState] = React.useState({
    mode,
    scrolled: false,
  });
  const searchHeaderMaterialState: WorkspacePageHeaderScrollState =
    headerScrollState.mode === mode && headerScrollState.scrolled
      ? "scrolled"
      : "top";
  const handleDiscoveryScroll = React.useCallback<
    React.UIEventHandler<HTMLDivElement>
  >((event) => {
    const scrolled = isWorkspacePageHeaderScrolled(
      event.currentTarget.scrollTop,
    );
    setHeaderScrollState((current) =>
      current.mode === mode && current.scrolled === scrolled
        ? current
        : { mode, scrolled });
  }, [mode]);
  React.useLayoutEffect(() => {
    setHeaderScrollState({ mode, scrolled: false });
  }, [mode]);

  const debouncedSearchInput = useDebouncedValue(input, 300);
  React.useEffect(() => {
    if (!isSearch) return;
    const value = debouncedSearchInput.trim();
    setSearchQuery(value && !isRSSDiscoveryAddress(value) ? value : "");
  }, [debouncedSearchInput, isSearch]);

  const refreshDiscovery = useRSSRefreshDiscovery();
  const discoveryHome = useRSSDiscovery({
    language: browseLanguage,
    sort: "popular",
    limit: 1,
  }, !isSearch);
  const debouncedCategoryQuery = useDebouncedValue(categoryQuery, 250);
  const activeQuery = isSearch ? searchQuery : debouncedCategoryQuery;
  const discoveryRoutes = useRSSDiscoveryInfinite(
    {
      categoryId: isSearch
        ? undefined
        : selectedCategoryId && selectedCategoryId !== "all"
          ? selectedCategoryId
          : undefined,
      query: activeQuery,
      // Search intentionally spans the complete catalog. Browse keeps a
      // user-controlled language filter so categories remain manageable.
      language: isSearch ? undefined : browseLanguage,
      sort,
      limit: DISCOVERY_PAGE_SIZE,
    },
    isSearch ? Boolean(searchQuery) : selectedCategoryId !== null,
  );
  const localEntriesQuery = useRSSEntriesInfinite(
    { query: searchQuery, limit: LOCAL_SEARCH_PAGE_SIZE },
    isSearch && Boolean(searchQuery),
  );
  const categories = discoveryHome.data?.categories ?? [];
  const routes = mergeRSSDiscoveryPages(
    discoveryRoutes.data?.pages.flatMap((page) => page.routes) ?? [],
  );
  const resultMeta = discoveryRoutes.data?.pages[0];
  const existingMatches = React.useMemo(
    () => isSearch ? searchRSSSubscriptions(subscriptions, searchQuery) : [],
    [isSearch, searchQuery, subscriptions],
  );
  const localEntries = React.useMemo(
    () => mergeRSSLocalEntryPages(localEntriesQuery.data?.pages ?? []),
    [localEntriesQuery.data?.pages],
  );
  const localResultCount = localEntriesQuery.data?.pages[0]?.total ?? localEntries.length;
  const discoveryResultCount = resultMeta?.filteredRouteCount ?? routes.length;
  const selectedCategoryTitle = selectedCategoryId === "all"
    ? t("xiadown.rss.discoveryCategoryAll")
    : selectedCategoryId
      ? rssDiscoveryCategoryLabel(selectedCategoryId, t)
      : t("xiadown.rss.browseCategories");
  const pageContract = defineWorkspacePageContract({
    presentation: "primary",
    recipe: isSearch ? "search" : "collection",
    routeLabel: isSearch
      ? t("xiadown.rss.discoverySearch")
      : selectedCategoryTitle,
    topBar: isSearch ? "search" : "actions",
    heading: "assistive",
    contentLayout: isSearch ? "list" : "card-grid",
    footer: "none",
    scroll: "content",
    density: isSearch ? "regular" : "comfortable",
    immersion: "standard",
  });

  const submitSearch = () => {
    const value = input.trim();
    if (!value) return;
    if (isRSSDiscoveryAddress(value)) {
      setSearchQuery("");
      setPreviewTarget({
        kind: "subscribe",
        url: canonicalizeRSSHubInput(value),
      });
      return;
    }
    setSearchQuery(value);
  };

  const selectCategory = (categoryId: string) => {
    setCategoryQuery("");
    setSelectedCategoryId(categoryId);
  };

  return (
    <WorkspacePage
      className="rss-workspace-page app-dream-window"
      contract={pageContract}
      data-discovery-mode={mode}
    >
      {isSearch ? (
        <WorkspacePageTopBar
          className="rss-page-heading rss-discovery-heading rss-discovery-search-heading app-station-search-header"
          reserveWindowControls={reserveWindowControls}
          scrollMaterialState={searchHeaderMaterialState}
        />
      ) : (
        <WorkspacePageTopBar
          className="rss-page-heading rss-discovery-heading rss-discovery-browse-heading"
          data-category-selected={selectedCategoryId !== null || undefined}
          reserveWindowControls={reserveWindowControls}
        >
          <WorkspacePrimaryHeaderActionGroup
            className="rss-page-heading__actions rss-discovery-heading__actions"
            label={pageContract.routeLabel}
          >
            {selectedCategoryId !== null ? (
              <RSSHeaderAction
                label={t("xiadown.rss.backToCategories")}
                onClick={() => {
                  setSelectedCategoryId(null);
                  setCategoryQuery("");
                }}
              >
                <ArrowLeft />
              </RSSHeaderAction>
            ) : null}
            <DiscoveryLanguageSelect
              uiLanguage={language}
              t={t}
              value={browseLanguage}
              onChange={setBrowseLanguage}
            />
            {selectedCategoryId !== null ? (
              <label className="rss-discovery-select">
                <span>{t("xiadown.rss.sortBy")}</span>
                <Select onChange={(event) => setSort(event.currentTarget.value as RSSDiscoverySort)} value={sort}>
                  <option value="popular">{t("xiadown.rss.sortPopular")}</option>
                  <option value="title">{t("xiadown.rss.sortTitle")}</option>
                </Select>
              </label>
            ) : (
              <RSSHeaderAction
                disabled={refreshDiscovery.isPending}
                label={t("xiadown.rss.refreshCatalog")}
                onClick={() => refreshDiscovery.mutate({ language: browseLanguage, sort: "popular", limit: 1 })}
              >
                <RefreshCcw className={refreshDiscovery.isPending ? "app-motion-spin" : undefined} />
              </RSSHeaderAction>
            )}
          </WorkspacePrimaryHeaderActionGroup>
          {selectedCategoryId !== null ? (
            <WorkspacePrimaryHeaderActionGroup
              label={t("xiadown.rss.categorySearchPlaceholder")}
            >
              <label className="rss-discovery-category-search rss-discovery-category-search--heading">
                <Search aria-hidden="true" />
                <input
                  aria-label={t("xiadown.rss.categorySearchPlaceholder")}
                  onChange={(event) => setCategoryQuery(event.currentTarget.value)}
                  placeholder={t("xiadown.rss.categorySearchPlaceholder")}
                  value={categoryQuery}
                />
                {categoryQuery ? (
                  <button aria-label={t("xiadown.rss.clearSearch")} onClick={() => setCategoryQuery("")} type="button">
                    <X />
                  </button>
                ) : null}
              </label>
            </WorkspacePrimaryHeaderActionGroup>
          ) : null}
        </WorkspacePageTopBar>
      )}

      <WorkspacePageContent
        className="rss-discovery-shell"
        onScroll={isSearch ? handleDiscoveryScroll : undefined}
      >
        {isSearch ? (
          <>
            <WorkspaceSearchControl
              clearLabel={t("xiadown.rss.clearSearch")}
              onClear={() => {
                setSearchQuery("");
                setSearchResultTab("discovery");
              }}
              onSubmit={submitSearch}
              onValueChange={setInput}
              placeholder={t("xiadown.rss.discoverySearchPlaceholder")}
              submitLabel={isRSSDiscoveryAddress(input) ? t("xiadown.rss.preview") : t("xiadown.workspace.search")}
              value={input}
            />
            {searchQuery ? (
            <section className="rss-discovery-search-results" aria-labelledby="rss-discovery-search-results-title">
              <h2 className="sr-only" id="rss-discovery-search-results-title">{t("xiadown.rss.searchResults")}</h2>
              <div className="rss-discovery-results-toolbar">
                <DreamSegmentSwitch
                  ariaLabel={t("xiadown.rss.searchResults")}
                  className="rss-discovery-result-tabs"
                  items={[
                    { value: "local", label: `${t("xiadown.workspace.local")} ${formatRSSDiscoveryNumber(localResultCount, language)}` },
                    { value: "subscriptions", label: `${t("xiadown.rss.yourSubscriptions")} ${formatRSSDiscoveryNumber(existingMatches.length, language)}` },
                    { value: "discovery", label: `${t("xiadown.rss.discovery")} ${formatRSSDiscoveryNumber(discoveryResultCount, language)}` },
                  ]}
                  value={searchResultTab}
                  onValueChange={setSearchResultTab}
                />
                {searchResultTab === "discovery" ? (
                  <div className="rss-discovery-section-actions">
                    <label className="rss-discovery-select">
                      <span>{t("xiadown.rss.sortBy")}</span>
                      <Select onChange={(event) => setSort(event.currentTarget.value as RSSDiscoverySort)} value={sort}>
                        <option value="popular">{t("xiadown.rss.sortPopular")}</option>
                        <option value="title">{t("xiadown.rss.sortTitle")}</option>
                      </Select>
                    </label>
                    <Button
                      disabled={refreshDiscovery.isPending}
                      onClick={() => refreshDiscovery.mutate(
                        { query: searchQuery, sort, limit: DISCOVERY_PAGE_SIZE },
                        { onSuccess: () => void discoveryRoutes.refetch() },
                      )}
                      size="sm"
                      type="button"
                      variant="outline"
                    >
                      <RefreshCcw className={refreshDiscovery.isPending ? "app-motion-spin" : undefined} />
                      {t("xiadown.rss.refreshCatalog")}
                    </Button>
                  </div>
                ) : searchResultTab === "local" ? (
                  <Button
                    disabled={localEntriesQuery.isFetching}
                    onClick={() => void localEntriesQuery.refetch()}
                    size="sm"
                    type="button"
                    variant="outline"
                  >
                    <RefreshCcw className={localEntriesQuery.isFetching ? "app-motion-spin" : undefined} />
                    {t("xiadown.listen.refresh")}
                  </Button>
                ) : null}
              </div>

              {searchResultTab === "local" ? (
                <>
                  <RSSLocalEntryResults
                    entries={localEntries}
                    error={localEntriesQuery.error}
                    hasNextPage={Boolean(localEntriesQuery.hasNextPage)}
                    language={language}
                    loading={localEntriesQuery.isPending}
                    loadingMore={localEntriesQuery.isFetchingNextPage}
                    showHeading={false}
                    subscriptions={subscriptions}
                    t={t}
                    total={localResultCount}
                    onLoadMore={() => void localEntriesQuery.fetchNextPage()}
                    onOpen={onOpenEntry}
                    onRetry={() => void localEntriesQuery.refetch()}
                  />
                  {!localEntriesQuery.isPending && !localEntriesQuery.isError && localEntries.length === 0 ? <RSSSearchEmpty t={t} /> : null}
                </>
              ) : searchResultTab === "subscriptions" ? (
                existingMatches.length > 0 ? (
                  <ExistingSubscriptionResults
                    language={language}
                    showHeading={false}
                    subscriptions={existingMatches}
                    t={t}
                    onOpen={onAdded}
                  />
                ) : <RSSSearchEmpty t={t} />
              ) : (
                <>
                  <RSSRemoteDiscoveryResults
                    error={discoveryRoutes.error}
                    fetchedAt={resultMeta?.fetchedAt}
                    hasNextPage={Boolean(discoveryRoutes.hasNextPage)}
                    language={language}
                    loading={discoveryRoutes.isPending}
                    loadingMore={discoveryRoutes.isFetchingNextPage}
                    routes={routes}
                    showHeading={false}
                    sourceUrl={resultMeta?.sourceUrl}
                    subscriptions={subscriptions}
                    t={t}
                    total={resultMeta?.filteredRouteCount}
                    onLoadMore={() => void discoveryRoutes.fetchNextPage()}
                    onPreview={(route) => setPreviewTarget({
                      kind: "subscribe",
                      url: route.needsParameters ? "" : route.url,
                      route,
                    })}
                    onRetry={() => void discoveryRoutes.refetch()}
                  />
                  {!discoveryRoutes.isPending && !discoveryRoutes.isError && routes.length === 0 ? <RSSSearchEmpty t={t} /> : null}
                </>
              )}
            </section>
            ) : null}
          </>
        ) : selectedCategoryId === null ? (
          <section className="rss-discovery-catalog rss-discovery-catalog--root" aria-labelledby="rss-discovery-catalog-title">
            <h2 className="sr-only" id="rss-discovery-catalog-title">{t("xiadown.rss.browseCategories")}</h2>
            {discoveryHome.isPending ? (
              <DiscoverySkeleton count={8} />
            ) : discoveryHome.isError ? (
              <DiscoveryError error={discoveryHome.error} t={t} onRetry={() => void discoveryHome.refetch()} />
            ) : (
              <RSSDiscoveryCategoryGrid categories={categories} language={language} t={t} onSelect={selectCategory} />
            )}
            {discoveryHome.data ? (
              <DiscoverySourceFooter
                count={discoveryHome.data.totalRouteCount}
                fetchedAt={discoveryHome.data.fetchedAt}
                language={language}
                sourceUrl={discoveryHome.data.sourceUrl}
                t={t}
              />
            ) : null}
          </section>
        ) : (
          <section className="rss-discovery-category" aria-labelledby="rss-discovery-category-title">
            <h2 className="sr-only" id="rss-discovery-category-title">{selectedCategoryTitle}</h2>

            {discoveryRoutes.isPending ? (
              <DiscoverySkeleton count={9} />
            ) : discoveryRoutes.isError ? (
              <DiscoveryError error={discoveryRoutes.error} t={t} onRetry={() => void discoveryRoutes.refetch()} />
            ) : routes.length === 0 ? (
              <div className="rss-state-surface">
                <Search />
                <strong>{t("xiadown.rss.noDiscoveryMatches")}</strong>
                <span>{t("xiadown.rss.noDiscoveryMatchesDescription")}</span>
              </div>
            ) : (
              <>
                <RSSDiscoveryRouteGrid
                  routes={routes}
                  language={language}
                  subscriptions={subscriptions}
                  t={t}
                  onPreview={(route) => setPreviewTarget({
                    kind: "subscribe",
                    url: route.needsParameters ? "" : route.url,
                    route,
                  })}
                />
                <DiscoveryLoadMore query={discoveryRoutes} t={t} />
              </>
            )}
          </section>
        )}
      </WorkspacePageContent>

      {previewTarget ? (
        <RSSSubscriptionDialog
          subscriptions={subscriptions}
          target={previewTarget}
          onAdded={onAdded}
          onClose={() => setPreviewTarget(null)}
        />
      ) : null}
    </WorkspacePage>
  );
}

export function RSSLocalEntryResults({
  entries,
  error,
  hasNextPage,
  language,
  loading,
  loadingMore,
  showHeading = true,
  subscriptions,
  t,
  total,
  onLoadMore,
  onOpen,
  onRetry,
}: {
  entries: readonly RSSEntry[];
  error: unknown;
  hasNextPage: boolean;
  language: string;
  loading: boolean;
  loadingMore: boolean;
  showHeading?: boolean;
  subscriptions: readonly RSSSubscription[];
  t: TFunction;
  total: number;
  onLoadMore: () => void;
  onOpen?: (entry: RSSEntry) => void;
  onRetry: () => void;
}) {
  const sourceByID = new Map(subscriptions.map((subscription) => [subscription.id, subscription]));
  if (!loading && !error && entries.length === 0) return null;
  const initialError = error && entries.length === 0;

  return (
    <section className="rss-discovery-existing" aria-labelledby={showHeading ? "rss-discovery-local-title" : undefined}>
      {showHeading ? <header>
        <h3 id="rss-discovery-local-title">{t("xiadown.workspace.local")}</h3>
        {!loading && !initialError ? <span>{formatRSSDiscoveryNumber(total, language)}</span> : null}
      </header> : null}
      {loading ? (
        <DiscoverySkeleton count={3} />
      ) : initialError ? (
        <div className="rss-state-surface rss-state-surface--error">
          <Search />
          <strong>{t("xiadown.rss.entryLoadFailed")}</strong>
          <span>{errorText(error)}</span>
          <Button onClick={onRetry} type="button" variant="outline">
            <RefreshCcw />{t("xiadown.rss.tryAgain")}
          </Button>
        </div>
      ) : (
        <>
          <div className="rss-discovery-route-grid">
            {entries.map((entry) => {
              const source = sourceByID.get(entry.subscriptionId);
              const publishedAt = entry.publishedAt || entry.sourceUpdatedAt || entry.modifiedAt;
              const title = entry.title || entry.url || source?.title || source?.feedUrl || entry.id;
              return (
                <button
                  aria-label={title}
                  className="rss-discovery-route-card app-dream-card app-motion-surface"
                  data-entry-id={entry.id}
                  key={entry.id}
                  onClick={() => onOpen?.(entry)}
                  type="button"
                >
                  <RSSLocalEntryIcon src={controlledRSSResourceURL(source?.iconUrl)} />
                  <span className="rss-discovery-route-card__copy">
                    <span className="rss-discovery-route-card__title">
                      <strong>{title}</strong>
                      <mark>{localEntryKindLabel(entry, t)}</mark>
                    </span>
                    <small>{entry.summary || entry.author || entry.url || source?.feedUrl}</small>
                    <span className="rss-discovery-route-card__meta">
                      {source ? <em>{source.title || source.feedUrl}</em> : null}
                      {entry.author ? <em>{entry.author}</em> : null}
                      {publishedAt ? <em>{formatRSSDiscoveryDate(publishedAt, language)}</em> : null}
                      {!entry.readAt ? <em>{t("xiadown.rss.unread")}</em> : null}
                    </span>
                  </span>
                </button>
              );
            })}
          </div>
          {error ? <DiscoveryPaginationError error={error} t={t} onRetry={onRetry} /> : null}
        </>
      )}
      {!loading && !initialError ? (
        <RSSSearchLoadMore
          hasNextPage={hasNextPage}
          loading={loadingMore}
          t={t}
          onLoadMore={onLoadMore}
        />
      ) : null}
    </section>
  );
}

export function RSSRemoteDiscoveryResults({
  error,
  fetchedAt,
  hasNextPage,
  language,
  loading,
  loadingMore,
  routes,
  showHeading = true,
  sourceUrl,
  subscriptions,
  t,
  total,
  onLoadMore,
  onPreview,
  onRetry,
}: {
  error: unknown;
  fetchedAt?: string;
  hasNextPage: boolean;
  language: string;
  loading: boolean;
  loadingMore: boolean;
  routes: readonly RSSDiscoveryRoute[];
  showHeading?: boolean;
  sourceUrl?: string;
  subscriptions: readonly RSSSubscription[];
  t: TFunction;
  total?: number;
  onLoadMore: () => void;
  onPreview: (route: RSSDiscoveryRoute) => void;
  onRetry: () => void;
}) {
  return (
    <section className="rss-discovery-existing" aria-labelledby={showHeading ? "rss-discovery-remote-title" : undefined}>
      {showHeading ? <header>
        <h3 id="rss-discovery-remote-title">{t("xiadown.rss.discovery")}</h3>
        {total !== undefined ? <span>{formatRSSDiscoveryNumber(total, language)}</span> : null}
      </header> : null}
      {loading ? (
        <DiscoverySkeleton count={9} />
      ) : error && routes.length === 0 ? (
        <DiscoveryError error={error} t={t} onRetry={onRetry} />
      ) : routes.length > 0 ? (
        <>
          <RSSDiscoveryRouteGrid
            routes={routes}
            language={language}
            subscriptions={subscriptions}
            t={t}
            onPreview={onPreview}
          />
          {error ? <DiscoveryPaginationError error={error} t={t} onRetry={onRetry} /> : null}
          <RSSSearchLoadMore
            hasNextPage={hasNextPage}
            loading={loadingMore}
            t={t}
            onLoadMore={onLoadMore}
          />
        </>
      ) : null}
      {total !== undefined && fetchedAt && sourceUrl ? (
        <DiscoverySourceFooter
          count={total}
          fetchedAt={fetchedAt}
          language={language}
          sourceUrl={sourceUrl}
          t={t}
        />
      ) : null}
    </section>
  );
}

function RSSSearchLoadMore({
  hasNextPage,
  loading,
  t,
  onLoadMore,
}: {
  hasNextPage: boolean;
  loading: boolean;
  t: TFunction;
  onLoadMore: () => void;
}) {
  if (!hasNextPage) return null;
  return (
    <div className="rss-discovery-load-more">
      <Button disabled={loading} onClick={onLoadMore} type="button" variant="outline">
        {loading ? <LoaderCircle className="app-motion-spin" /> : <Plus />}
        {t("xiadown.rss.loadMore")}
      </Button>
    </div>
  );
}

function DiscoveryPaginationError({ error, t, onRetry }: { error: unknown; t: TFunction; onRetry: () => void }) {
  return (
    <div className="rss-state-surface rss-state-surface--error rss-state-surface--pagination">
      <strong>{t("xiadown.rss.entryLoadFailed")}</strong>
      <span>{errorText(error)}</span>
      <Button onClick={onRetry} type="button" variant="outline">
        <RefreshCcw />{t("xiadown.rss.tryAgain")}
      </Button>
    </div>
  );
}

function RSSLocalEntryIcon({ src }: { src: string }) {
  return (
    <span aria-hidden="true" className="rss-favicon rss-discovery-route-icon">
      <RSSRemoteImage alt="" fallback={<Rss />} loading="lazy" src={src} />
    </span>
  );
}

function localEntryKindLabel(entry: RSSEntry, t: TFunction) {
  switch (entry.kind) {
    case "article": return t("xiadown.rss.articles");
    case "social": return t("xiadown.rss.socialMedia");
    case "image": return t("xiadown.workspace.images");
    case "video": return t("xiadown.rss.videos");
  }
}

function ExistingSubscriptionResults({
  subscriptions,
  language,
  showHeading = true,
  t,
  onOpen,
}: {
  subscriptions: readonly RSSSubscription[];
  language: string;
  showHeading?: boolean;
  t: TFunction;
  onOpen: (subscription: RSSSubscription) => void;
}) {
  return (
    <section className="rss-discovery-existing" aria-labelledby={showHeading ? "rss-discovery-existing-title" : undefined}>
      {showHeading ? <header>
        <h3 id="rss-discovery-existing-title">{t("xiadown.rss.yourSubscriptions")}</h3>
        <span>{formatRSSDiscoveryNumber(subscriptions.length, language)}</span>
      </header> : null}
      <div className="rss-discovery-route-grid">
        {subscriptions.map((subscription) => (
          <button
            className="rss-discovery-route-card app-dream-card app-motion-surface"
            data-subscribed="true"
            key={subscription.id}
            onClick={() => onOpen(subscription)}
            type="button"
          >
            <RSSLocalEntryIcon src={controlledRSSResourceURL(subscription.iconUrl)} />
            <span className="rss-discovery-route-card__copy">
              <span className="rss-discovery-route-card__title">
                <strong>{subscription.title || subscription.feedUrl}</strong>
                <mark><Check />{t("xiadown.rss.subscribed")}</mark>
              </span>
              <small>{subscription.description || subscription.feedUrl}</small>
              <span className="rss-discovery-route-card__meta">
                <em>{t("xiadown.rss.openSubscription")}</em>
                {subscription.unreadCount > 0 ? <em>{formatRSSDiscoveryNumber(subscription.unreadCount, language)} {t("xiadown.rss.unread")}</em> : null}
              </span>
            </span>
          </button>
        ))}
      </div>
    </section>
  );
}

function RSSSearchEmpty({ t }: { t: TFunction }) {
  return (
    <div className="rss-state-surface rss-discovery-search-empty">
      <Search />
      <strong>{t("xiadown.rss.noDiscoveryMatches")}</strong>
      <span>{t("xiadown.rss.noDiscoveryMatchesDescription")}</span>
    </div>
  );
}

function DiscoveryLoadMore({ query, t }: {
  query: ReturnType<typeof useRSSDiscoveryInfinite>;
  t: TFunction;
}) {
  if (!query.hasNextPage) return null;
  return (
    <div className="rss-discovery-load-more">
      <Button
        disabled={query.isFetchingNextPage}
        onClick={() => void query.fetchNextPage()}
        type="button"
        variant="outline"
      >
        {query.isFetchingNextPage ? <LoaderCircle className="app-motion-spin" /> : <Plus />}
        {t("xiadown.rss.loadMore")}
      </Button>
    </div>
  );
}

function DiscoveryLanguageSelect({
  uiLanguage,
  t,
  value,
  onChange,
}: {
  uiLanguage: string;
  t: TFunction;
  value: string;
  onChange: (value: string) => void;
}) {
  return (
    <label className="rss-discovery-select">
      <span>{t("xiadown.rss.language")}</span>
      <Select onChange={(event) => onChange(event.currentTarget.value)} value={value}>
        <option value="">{t("xiadown.rss.allLanguages")}</option>
        {CATALOG_LANGUAGE_OPTIONS.filter(Boolean).map((option) => (
          <option key={option} value={option}>{catalogLanguageLabel(option, uiLanguage)}</option>
        ))}
      </Select>
    </label>
  );
}

function DiscoverySkeleton({ count }: { count: number }) {
  return <div aria-hidden="true" className="rss-discovery-skeleton-grid">{Array.from({ length: count }, (_, index) => <span key={index} />)}</div>;
}

function DiscoveryError({ error, t, onRetry }: { error: unknown; t: TFunction; onRetry: () => void }) {
  return <div className="rss-state-surface rss-state-surface--error"><Rss /><strong>{t("xiadown.rss.catalogUnavailable")}</strong><span>{errorText(error)}</span><Button onClick={onRetry} type="button" variant="outline"><RefreshCcw />{t("xiadown.rss.tryAgain")}</Button></div>;
}

function DiscoverySourceFooter({ count, fetchedAt, language, sourceUrl, t }: { count: number; fetchedAt: string; language: string; sourceUrl: string; t: TFunction }) {
  let source = sourceUrl;
  try {
    source = new URL(sourceUrl).hostname;
  } catch {
    // Preserve a non-URL source label.
  }
  return <footer className="rss-discovery-source"><span>{formatRSSDiscoveryNumber(count, language)} {t("xiadown.rss.routes")}</span><span>{source}</span>{fetchedAt ? <time dateTime={fetchedAt}>{formatRSSDiscoveryDate(fetchedAt, language)}</time> : null}</footer>;
}

function catalogLanguageLabel(value: string, uiLanguage: string) {
  try {
    return new Intl.DisplayNames([uiLanguage], { type: "language" }).of(value) || value;
  } catch {
    return value;
  }
}

function useDebouncedValue(value: string, delay: number) {
  const [debounced, setDebounced] = React.useState(value);
  React.useEffect(() => {
    const timeout = window.setTimeout(() => setDebounced(value), delay);
    return () => window.clearTimeout(timeout);
  }, [delay, value]);
  return debounced;
}

function errorText(error: unknown) {
  return error instanceof Error ? error.message : String(error ?? "");
}
