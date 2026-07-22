import {
  ArrowLeft,
  ArrowUp,
  BookOpenText,
  CalendarDays,
  CheckCheck,
  CheckCircle2,
  ChevronLeft,
  ChevronRight,
  Circle,
  Copy,
  Download,
  Ellipsis,
  Eye,
  EyeOff,
  ExternalLink,
  FileDown,
  FolderCog,
  Headphones,
  Image as ImageIcon,
  Keyboard,
  LoaderCircle,
  Pencil,
  Play,
  RefreshCcw,
  Rss,
  Search,
  Share2,
  SlidersHorizontal,
  Square,
  Star,
  ThumbsUp,
  Trash2,
  Upload,
  Volume2,
} from "lucide-react";
import { useQueryClient } from "@tanstack/react-query";
import * as React from "react";

import {
  rssQueryKeys,
  saveRSSEntryImage,
  useRSSAddSubscription,
  useRSSBackfillHistory,
  useRSSDeleteSubscription,
  useRSSEntry,
  useRSSEntriesInfinite,
  useRSSMarkAllRead,
  useRSSRefresh,
  useRSSSetEntryState,
  useRSSUpdateSubscription,
  isMalformedRSSDynamicRouteID,
  parseRSSCategoryRouteID,
  parseRSSCollectionRouteID,
  reconcileRSSBulkSelection,
  runRSSBulkAction,
  setRSSVisibleSelection,
  toggleRSSBulkSelection,
  type RSSEntry,
  type RSSEntryKind,
  type RSSCategory,
  type RSSCollection,
  type RSSListEntriesRequest,
  type RSSEntryStateField,
  type RSSSetEntryStateRequest,
  type RSSSubscription,
  type RSSViewType,
} from "@/app/rss";
import { YouTubeVideoCard } from "@/app/youtube/YouTubeWorkspacePage";
import {
  formatYouTubePublishedLabel,
  formatYouTubeViewCount,
} from "@/app/youtube/page-state";
import type { YouTubeWorkspaceVideo } from "@/app/youtube/types";
import { RSSAddSubscriptionPage } from "./RSSAddSubscriptionPage";
import { RSSBilibiliPlayback } from "./RSSBilibiliPlayback";
import {
  resolveRSSBilibiliDisplayMetadata,
  type RSSBilibiliPageMetadata,
} from "./bilibili-page-metadata";
import { RSSRemoteImage } from "./RSSRemoteImage";
import { RSSOrganizationManager } from "./RSSOrganizationManager";
import { RSSSubscriptionDialog } from "./RSSSubscriptionDialog";
import { RSSSiteVideoPlayback } from "./RSSSiteVideoPlayback";
import {
  RSS_VIDEO_IFRAME_PERMISSIONS,
  RSSWebVideoPlayback,
} from "./RSSWebVideoPlayback";
import { RSSYouTubePlayback } from "./RSSYouTubePlayback";
import {
  createRSSHistorySentinelGate,
  RSS_HISTORY_BACKFILL_PAGE_BUDGET,
  RSS_PENDING_HYDRATION_REFETCH_INTERVAL_MS,
  RSS_PENDING_HYDRATION_REFETCH_WINDOW_MS,
  rssBackfillFailureMessage,
  rssBackfillRequestForEntries,
  rssBackfillRequestKey,
  rssHistoryCollectionMetric,
  rssHistorySessionShouldStop,
  rssShouldFastPollPendingSubscription,
  rssSubscriptionHistoryReady,
  type RSSHistorySentinelGate,
} from "./history-backfill";
import { controlledRSSResourceURL } from "./remote-resource";
import {
  buildRSSScrollCacheKey,
  readRSSSelectedEntryID,
  readRSSScrollOffset,
  writeRSSSelectedEntryID,
  writeRSSScrollOffset,
} from "./session-cache";
import { rssFeedAddressSubscribed } from "./discovery-utils";
import {
  exportRSSSubscriptionsToOPML,
  parseRSSSubscriptionsFromOPML,
  RSS_OPML_MAX_SOURCE_BYTES,
} from "./opml";
import {
  applyRSSStateToEntry,
  boundedRSSEntryImages,
  buildRSSArticlePrintDocument,
  DEFAULT_RSS_READER_PREFERENCES,
  filterAndSortRSSSubscriptions,
  mergeRSSEntryPages,
  readRSSReaderLayoutMessage,
  readRSSReaderImageContextMessage,
  readRSSReaderLinkMessage,
  readRSSReaderOutlineMessage,
  readRSSReaderPreferences,
  readRSSReaderSelectionMessage,
  readRSSReaderWheelMessage,
  resolveRSSReaderOutlineProgress,
  resolveRSSReaderOutlineMarkers,
  resolveRSSAudioPresentation,
  resolveRSSReaderDocumentSnapshot,
  resolveRSSWorkspaceShortcut,
  readRSSIdentityScopedBoolean,
  rssEntryImageCandidates,
  rssReaderVideoDownloadURL,
  rssReaderSpeechText,
  rssReaderScrollFraction,
  rssReaderVideoEmbedURL,
  rssReaderWheelPixels,
  resolveRSSCollectionPresentation,
  setRSSIdentityScopedBoolean,
  toggleRSSIdentityScopedBoolean,
  writeRSSReaderPreferences,
  type RSSCollectionRoute,
  type RSSAudioPresentation,
  type RSSIdentityScopedBoolean,
  type RSSReaderDocumentSnapshot,
  type RSSReaderPreferences,
  type RSSReaderOutlineItem,
  type RSSSubscriptionSort,
  type RSSReaderVideoEmbedLayout,
} from "./workspace-utils";
import {
  buildRSSArticleProgressStateRequest,
  buildRSSReadStateRequest,
  buildRSSStarredStateRequest,
  buildRSSVideoProgressStateRequest,
} from "./state-utils";
import {
  buildRSSVideoBatchDownloadTarget,
  buildRSSVideoBatchDownloadTargets,
  resolveRSSBilibiliPlaybackIdentity,
  resolveRSSVideoExperience,
  shouldUseRSSVideoCollectionPresentation,
  shouldUseRSSVideoLayoutPresentation,
  type RSSVideoBatchDownloadTarget,
} from "./video-platform";
import {
  parseRSSSubscriptionRouteId,
  RSS_WORKSPACE_ROUTE_IDS,
} from "./rss-routes";
import { cn } from "@/lib/utils";
import { useI18n } from "@/shared/i18n";
import { openExternalURL } from "@/shared/query/system";
import { Button } from "@/shared/ui/button";
import { DreamInlineSwitch } from "@/shared/ui/dream-inline-switch";
import { SecondaryReveal } from "@/shared/ui/secondary-reveal";
import {
  WorkspacePrimaryHeaderAction as RSSHeaderAction,
  WorkspacePrimaryHeaderActionGroup,
  WorkspacePrimaryHeaderMenuContent,
} from "@/shared/ui/workspace-primary-header-action";
import {
  WorkspacePage,
  WorkspacePageContent,
  WorkspacePageTopBar,
  defineWorkspacePageContract,
  isWorkspacePageHeaderScrolled,
  type WorkspacePageContentLayout,
  type WorkspacePageHeaderScrollState,
} from "@/shared/ui/workspace-page";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogScrollArea,
  DialogTitle,
} from "@/shared/ui/dialog";
import { Input } from "@/shared/ui/input";
import { Select } from "@/shared/ui/select";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/shared/ui/dropdown-menu";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/shared/ui/tooltip";

import "./rss-workspace.css";

export interface RSSWorkspacePageProps {
  active: boolean;
  routeId: string;
  subscriptions: readonly RSSSubscription[];
  categories?: readonly RSSCategory[];
  collections?: readonly RSSCollection[];
  reserveWindowControls?: boolean;
  onNavigate: (routeId: string) => void;
  onDownload: (
    url: string | undefined,
    entry: RSSEntry,
    batchTargets?: readonly RSSVideoBatchDownloadTarget[],
  ) => void;
}

const RSS_ENTRY_PAGE_SIZE = 80;
const RSS_OPML_FILE_TYPES = ".opml,.xml,application/xml,text/xml";

export function RSSWorkspacePage({
  active,
  routeId,
  subscriptions,
  categories = [],
  collections = [],
  reserveWindowControls = false,
  onNavigate,
  onDownload,
}: RSSWorkspacePageProps) {
  const { t, language } = useI18n();
  const subscriptionId = parseRSSSubscriptionRouteId(routeId);
  const subscription = subscriptions.find((item) => item.id === subscriptionId);
  const categoryId = parseRSSCategoryRouteID(routeId);
  const category = categories.find((item) => item.id === categoryId);
  const collectionId = parseRSSCollectionRouteID(routeId);
  const collection = collections.find((item) => item.id === collectionId);
  const malformedDynamicRoute = isMalformedRSSDynamicRouteID(routeId);
  const unresolvedDynamicRoute = Boolean(
    (subscriptionId && !subscription) ||
    (categoryId && !category) ||
    (collectionId && !collection),
  );
  const invalidDynamicRoute = malformedDynamicRoute || unresolvedDynamicRoute;
  const [selectedEntryState, setSelectedEntryState] = React.useState({
    cacheKey: "",
    entryId: "",
  });
  const [focusedEntryState, setFocusedEntryState] = React.useState<{
    routeId: string;
    entry: RSSEntry | null;
  }>({ routeId: "", entry: null });
  const [imageViewerState, setImageViewerState] = React.useState({
    routeId: "",
    entryId: "",
  });
  const routeIdRef = React.useRef(routeId);
  routeIdRef.current = routeId;
  const focusedEntry = focusedEntryState.routeId === routeId
    ? focusedEntryState.entry
    : null;
  const imageViewerEntryId = imageViewerState.routeId === routeId
    ? imageViewerState.entryId
    : "";
  const setFocusedEntry = React.useCallback(
    (update: React.SetStateAction<RSSEntry | null>) => {
      setFocusedEntryState((current) => {
        if (routeIdRef.current !== routeId) return current;
        const currentEntry = current.routeId === routeId ? current.entry : null;
        const entry = typeof update === "function"
          ? update(currentEntry)
          : update;
        return { routeId, entry };
      });
    },
    [routeId],
  );
  const [unreadFilterState, setUnreadFilterState] =
    React.useState<RSSIdentityScopedBoolean>({ identity: "", enabled: false });
  const [readingModeState, setReadingModeState] =
    React.useState<RSSIdentityScopedBoolean>({ identity: "", enabled: false });
  const [viewOriginalState, setViewOriginalState] =
    React.useState<RSSIdentityScopedBoolean>({ identity: "", enabled: false });
  const [readerPreferences, setReaderPreferences] =
    React.useState<RSSReaderPreferences>(() => readRSSReaderPreferences());
  const updateReaderPreferences = React.useCallback((next: RSSReaderPreferences) => {
    setReaderPreferences(next);
    writeRSSReaderPreferences(next);
  }, []);
  const unreadOnly = readRSSIdentityScopedBoolean(unreadFilterState, routeId);
  const collectionRoute = collection
    ? collection.viewType === "auto" ? "all" : collection.viewType
    : resolveCollectionRoute(routeId, subscription);
  const presentationRoute = React.useMemo(
    () => resolveRSSCollectionPresentation(collectionRoute, subscription),
    [collectionRoute, subscription],
  );
  const collectionCacheKey = buildRSSScrollCacheKey({
    routeId,
    presentation: presentationRoute,
    subscriptionId,
    filter: unreadOnly ? "unread" : "all",
  });
  const selectedEntryId = selectedEntryState.cacheKey === collectionCacheKey
    ? selectedEntryState.entryId
    : readRSSSelectedEntryID(collectionCacheKey);
  const setSelectedEntryId = React.useCallback((entryId: string) => {
    writeRSSSelectedEntryID(collectionCacheKey, entryId);
    setSelectedEntryState({ cacheKey: collectionCacheKey, entryId });
  }, [collectionCacheKey]);
  const entriesRequest = React.useMemo(
    () => ({
      ...(subscriptionId ? { subscriptionId } : {}),
      ...(collectionId ? { collectionId } : {}),
      ...(categoryId ? { categoryId } : {}),
      ...(!subscriptionId && !collectionId && !categoryId && collectionRoute !== "all"
        ? { kind: collectionRoute as RSSEntryKind }
        : {}),
      ...(routeId === RSS_WORKSPACE_ROUTE_IDS.starred
        ? { starredOnly: true }
        : {}),
      ...(unreadOnly ? { unreadOnly: true } : {}),
      limit: RSS_ENTRY_PAGE_SIZE,
    }),
    [categoryId, collectionId, collectionRoute, routeId, subscriptionId, unreadOnly],
  );
  const isCollection =
    routeId !== RSS_WORKSPACE_ROUTE_IDS.search &&
    routeId !== RSS_WORKSPACE_ROUTE_IDS.addSubscription &&
    routeId !== RSS_WORKSPACE_ROUTE_IDS.discoverySearch &&
    routeId !== RSS_WORKSPACE_ROUTE_IDS.discoveryBrowse &&
    routeId !== RSS_WORKSPACE_ROUTE_IDS.manageSubscriptions;
  const collectionEnabled = active && isCollection && !invalidDynamicRoute;
  const entriesQuery = useRSSEntriesInfinite(entriesRequest, collectionEnabled);
  const loadedEntries = React.useMemo(
    () => mergeRSSEntryPages(entriesQuery.data?.pages ?? []),
    [entriesQuery.data?.pages],
  );
  const entries = loadedEntries;
  const loadedVideoBatchTargets = React.useMemo(
    () => buildRSSVideoBatchDownloadTargets(entries),
    [entries],
  );
  const effectivePresentationRoute =
    presentationRoute === "video" &&
    !shouldUseRSSVideoCollectionPresentation(entries)
      ? "article"
      : presentationRoute;
  const subscriptionHistoryReady = rssSubscriptionHistoryReady(
    subscriptionId,
    subscription?.lastSuccessAt,
  );
  useRSSPendingSubscriptionHydration({
    enabled: collectionEnabled,
    subscriptionId,
    lastSuccessAt: subscription?.lastSuccessAt,
    visibleEntries: entries.length,
    refetch: entriesQuery.refetch,
  });
  const historyPagination = useRSSHistoryPagination(
    entriesQuery,
    entriesRequest,
    collectionEnabled && subscriptionHistoryReady,
  );
  const refresh = useRSSRefresh();
  const refreshCurrentCollection = React.useCallback(() => {
    refresh.mutate(subscriptionId ? { id: subscriptionId } : {});
  }, [refresh, subscriptionId]);
  const markAllRead = useRSSMarkAllRead();
  const setState = useRSSSetEntryState();
  const collectionScroll = useRSSScrollRestoration(
    collectionCacheKey,
    entries.length,
  );
  const {
    scrollMaterialState: collectionPaneScrollMaterialState,
    onScrollStateChange: setCollectionPaneHeaderScrolled,
  } = useRSSScrollMaterialState(collectionCacheKey);
  const handleCollectionPaneScroll = React.useCallback<
    React.UIEventHandler<HTMLDivElement>
  >((event) => {
    collectionScroll.onScroll(event);
    setCollectionPaneHeaderScrolled(
      isWorkspacePageHeaderScrolled(event.currentTarget.scrollTop),
    );
  }, [collectionScroll.onScroll, setCollectionPaneHeaderScrolled]);
  React.useLayoutEffect(() => {
    setCollectionPaneHeaderScrolled(
      isWorkspacePageHeaderScrolled(readRSSScrollOffset(collectionCacheKey)),
    );
  }, [collectionCacheKey, setCollectionPaneHeaderScrolled]);
  const subscriptionById = React.useMemo(
    () => new Map(subscriptions.map((item) => [item.id, item] as const)),
    [subscriptions],
  );
  const selectedListEntry =
    entries.find((item) => item.id === selectedEntryId) ?? null;
  const resolvedFocusedEntry = focusedEntry
    ? entries.find((item) => item.id === focusedEntry.id) ?? focusedEntry
    : null;
  const detailFallbackEntry = resolvedFocusedEntry ?? selectedListEntry;
  const detailEntryID = detailFallbackEntry?.id || "";
  const detailContentRevision = detailFallbackEntry?.revision ?? 0;
  const readingMode = readRSSIdentityScopedBoolean(
    readingModeState,
    detailEntryID,
  );
  const viewOriginal = readRSSIdentityScopedBoolean(
    viewOriginalState,
    detailEntryID,
  );
  const detailScrollIdentity = [
    routeId,
    detailEntryID,
    viewOriginal ? "original" : "rendered",
  ].join("\u001f");
  const {
    scrollMaterialState: detailScrollMaterialState,
    onScrollStateChange: setDetailHeaderScrolled,
  } = useRSSScrollMaterialState(detailScrollIdentity);
  const setReadingMode = React.useCallback((enabled: boolean) => {
    setReadingModeState(setRSSIdentityScopedBoolean(detailEntryID, enabled));
  }, [detailEntryID]);
  const setViewOriginal = React.useCallback((enabled: boolean) => {
    setViewOriginalState(setRSSIdentityScopedBoolean(detailEntryID, enabled));
  }, [detailEntryID]);
  const detailQuery = useRSSEntry(
    detailEntryID,
    detailContentRevision,
    active && Boolean(detailEntryID),
  );

  React.useEffect(() => {
    if (
      effectivePresentationRoute === "social" ||
      effectivePresentationRoute === "image" ||
      effectivePresentationRoute === "video"
    ) {
      return;
    }
    if (selectedEntryId && entries.some((item) => item.id === selectedEntryId)) {
      return;
    }
    setSelectedEntryId(entries[0]?.id ?? "");
  }, [effectivePresentationRoute, entries, selectedEntryId, setSelectedEntryId]);

  const openEntry = React.useCallback((entry: RSSEntry, focus?: boolean) => {
    setSelectedEntryId(entry.id);
    if (focus) {
      setFocusedEntry(entry);
    }
    if (entry.kind === "article" && focus !== true) {
      setViewOriginalState(setRSSIdentityScopedBoolean(
        entry.id,
        readerPreferences.openMode === "original",
      ));
    }
    if (readerPreferences.autoMarkRead && !entry.readAt) {
      setState.mutate(buildRSSReadStateRequest(entry, true), {
        onSuccess: (state) => {
          setFocusedEntry((current) =>
            current?.id === state.entryId
              ? applyRSSStateToEntry(current, state)
              : current,
          );
        },
      });
    }
  }, [readerPreferences.autoMarkRead, readerPreferences.openMode, setFocusedEntry, setSelectedEntryId, setState]);

  React.useEffect(() => {
    if (!active) return;
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.defaultPrevented || rssKeyboardTargetIsEditable(event.target)) return;
      const shortcut = resolveRSSWorkspaceShortcut(event);
      if (!shortcut) return;
      const current = resolvedFocusedEntry ?? selectedListEntry;
      if (shortcut === "close-entry") {
        if (!resolvedFocusedEntry) return;
        event.preventDefault();
        setFocusedEntry(null);
        return;
      }
      if (shortcut === "previous-entry" || shortcut === "next-entry") {
        if (entries.length === 0) return;
        const currentIndex = current
          ? entries.findIndex((entry) => entry.id === current.id)
          : -1;
        const offset = shortcut === "previous-entry" ? -1 : 1;
        const fallbackIndex = offset < 0 ? entries.length - 1 : 0;
        const nextIndex = currentIndex < 0
          ? fallbackIndex
          : Math.max(0, Math.min(entries.length - 1, currentIndex + offset));
        const next = entries[nextIndex];
        if (!next || next.id === current?.id) return;
        event.preventDefault();
        openEntry(next, Boolean(resolvedFocusedEntry));
        return;
      }
      if (shortcut === "open-entry") {
        if (!current || resolvedFocusedEntry) return;
        event.preventDefault();
        document.querySelector<HTMLElement>(".rss-entry-detail-pane")
          ?.focus({ preventScroll: true });
        return;
      }
      if (shortcut === "toggle-read" && current) {
        event.preventDefault();
        setState.mutate(buildRSSReadStateRequest(current, !Boolean(current.readAt)));
        return;
      }
      if (shortcut === "toggle-starred" && current) {
        event.preventDefault();
        setState.mutate(buildRSSStarredStateRequest(current, !Boolean(current.starredAt)));
        return;
      }
      if (shortcut === "toggle-unread-filter" && isCollection) {
        event.preventDefault();
        setUnreadFilterState((currentState) =>
          toggleRSSIdentityScopedBoolean(currentState, routeId));
        return;
      }
      if (shortcut === "refresh" && collectionEnabled && !refresh.isPending) {
        event.preventDefault();
        refreshCurrentCollection();
      }
    };
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [
    active,
    entries,
    collectionEnabled,
    openEntry,
    refresh,
    refreshCurrentCollection,
    resolvedFocusedEntry,
    routeId,
    selectedListEntry,
    setFocusedEntry,
    setState,
  ]);

  if (!active) {
    return null;
  }

  if (
    !focusedEntry &&
    (routeId === RSS_WORKSPACE_ROUTE_IDS.search ||
    routeId === RSS_WORKSPACE_ROUTE_IDS.addSubscription ||
    routeId === RSS_WORKSPACE_ROUTE_IDS.discoverySearch ||
    routeId === RSS_WORKSPACE_ROUTE_IDS.discoveryBrowse)
  ) {
    return (
      <RSSAddSubscriptionPage
        mode={
          routeId === RSS_WORKSPACE_ROUTE_IDS.discoveryBrowse
            ? "browse"
            : "search"
        }
        onAdded={(item) => onNavigate(`subscription:${encodeURIComponent(item.id)}`)}
        onOpenEntry={(entry) => openEntry(entry, true)}
        reserveWindowControls={reserveWindowControls}
        subscriptions={subscriptions}
      />
    );
  }

  if (routeId === RSS_WORKSPACE_ROUTE_IDS.manageSubscriptions) {
    return (
      <RSSSubscriptionManagementPage
        categories={categories}
        collections={collections}
        subscriptions={subscriptions}
        reserveWindowControls={reserveWindowControls}
      />
    );
  }

  if (invalidDynamicRoute) {
    const invalidRouteContract = defineWorkspacePageContract({
      presentation: "primary",
      recipe: "feed",
      routeLabel: t("xiadown.rss.emptyTitle"),
      topBar: "none",
      heading: "assistive",
      contentLayout: "list",
      footer: "none",
      scroll: "content",
      density: "regular",
      immersion: "standard",
    });
    return (
      <WorkspacePage
        className="rss-workspace-page app-dream-window"
        contract={invalidRouteContract}
      >
        <WorkspacePageContent>
          <div className="rss-state-surface">
            <Rss />
            <strong>{t("xiadown.rss.emptyTitle")}</strong>
            <span>{t("xiadown.rss.emptyDescription")}</span>
          </div>
        </WorkspacePageContent>
      </WorkspacePage>
    );
  }

  const title = collectionTitle(
    routeId,
    subscription,
    category,
    collection,
    t,
  );
  const collectionContentLayout: WorkspacePageContentLayout =
    effectivePresentationRoute === "social"
      ? "feed"
      : effectivePresentationRoute === "image" || effectivePresentationRoute === "video"
        ? "card-grid"
        : "split";
  const collectionContract = defineWorkspacePageContract({
    presentation: "primary",
    recipe: "feed",
    routeLabel: title,
    topBar: "actions",
    heading: "assistive",
    contentLayout: collectionContentLayout,
    footer: "none",
    scroll: collectionContentLayout === "split" ? "panes" : "content",
    density: "compact",
    immersion: "standard",
  });
  const total = entriesQuery.data?.pages[0]?.total ?? entries.length;
  const collectionToolbarOwnsTrailingEdge =
    effectivePresentationRoute === "social" ||
    effectivePresentationRoute === "image" ||
    effectivePresentationRoute === "video";
  const markAllReadRequest = {
    ...(subscriptionId ? { subscriptionId } : {}),
    ...(collectionId ? { collectionId } : {}),
    ...(categoryId ? { categoryId } : {}),
    ...(!subscriptionId && !collectionId && !categoryId && collectionRoute !== "all"
      ? { kind: collectionRoute as RSSEntryKind }
      : {}),
    ...(routeId === RSS_WORKSPACE_ROUTE_IDS.starred
      ? { starredOnly: true }
      : {}),
  };
  const toolbar = (
    <RSSCollectionToolbar
      count={total}
      loading={refresh.isPending}
      markingRead={markAllRead.isPending}
      scrollMaterialState={
        collectionContentLayout === "split"
          ? collectionPaneScrollMaterialState
          : undefined
      }
      reserveWindowControls={
        collectionToolbarOwnsTrailingEdge && reserveWindowControls
      }
      unreadOnly={unreadOnly}
      onMarkAllRead={() => markAllRead.mutate(markAllReadRequest)}
      onRefresh={refreshCurrentCollection}
      onToggleUnread={() => setUnreadFilterState((current) =>
        toggleRSSIdentityScopedBoolean(current, routeId))}
      downloadLoadedCount={
        effectivePresentationRoute === "video" && loadedVideoBatchTargets.length > 1
          ? loadedVideoBatchTargets.length
          : undefined
      }
      onDownloadLoaded={
        effectivePresentationRoute === "video" && loadedVideoBatchTargets.length > 1
          ? () => {
              const firstTarget = loadedVideoBatchTargets[0];
              const firstEntry = entries.find(
                (entry) => entry.id === firstTarget?.entryId,
              );
              if (!firstEntry) return;
              onDownload(
                buildRSSVideoBatchDownloadTarget(entries),
                firstEntry,
                loadedVideoBatchTargets,
              );
            }
          : undefined
      }
    />
  );

  const updateFocusedEntry = (request: RSSSetEntryStateRequest) => {
    setState.mutate(request, {
      onSuccess: (state) => {
        setFocusedEntry((current) =>
          current?.id === state.entryId
            ? applyRSSStateToEntry(current, state)
            : current,
        );
      },
    });
  };
  const renderEntryDetailBody = (entry: RSSEntry, focused = false) => {
    const source = subscriptionById.get(entry.subscriptionId);
    return (
      <RSSEntryDetail
        entry={entry}
        headingLevel={focused ? 1 : 2}
        key={entry.id}
        language={language}
        onBack={focused ? () => setFocusedEntry(null) : undefined}
        onDownload={onDownload}
        onReadChange={(read) => updateFocusedEntry(buildRSSReadStateRequest(entry, read))}
        onScrollStateChange={setDetailHeaderScrolled}
        onStarChange={(starred) => updateFocusedEntry(buildRSSStarredStateRequest(entry, starred))}
        readingMode={readingMode}
        readerPreferences={readerPreferences}
        reserveWindowControls={reserveWindowControls}
        source={source}
        videoPresentation={shouldUseRSSVideoLayoutPresentation(
          presentationRoute,
          entry,
        )}
        viewOriginal={viewOriginal}
      />
    );
  };
  const renderEntryDetailToolbar = (entry: RSSEntry, focused = false) => {
    const source = subscriptionById.get(entry.subscriptionId);
    return (
      <RSSDetailToolbar
        entry={entry}
        overlaysReader={
          !viewOriginal && entry.kind !== "social" && entry.kind !== "image"
        }
        onBack={focused ? () => setFocusedEntry(null) : undefined}
        onExportPDF={() => printRSSArticle(entry, source, language)}
        onReadChange={(read) => updateFocusedEntry(buildRSSReadStateRequest(entry, read))}
        onStarChange={(starred) => updateFocusedEntry(buildRSSStarredStateRequest(entry, starred))}
        readingMode={readingMode}
        readerPreferences={readerPreferences}
        reserveWindowControls={reserveWindowControls}
        scrollMaterialState={detailScrollMaterialState}
        source={source}
        viewOriginal={viewOriginal}
        onReadingModeChange={setReadingMode}
        onReaderPreferencesChange={updateReaderPreferences}
        onViewOriginalChange={setViewOriginal}
      />
    );
  };
  const renderEntryDetailSurface = (fallbackEntry: RSSEntry, focused = false) => {
    const toolbarEntry = detailQuery.data ?? fallbackEntry;
    const videoPresentation = shouldUseRSSVideoLayoutPresentation(
      presentationRoute,
      toolbarEntry,
    );
    const back = focused ? () => setFocusedEntry(null) : undefined;
    return (
      <>
        {videoPresentation ? (
          detailQuery.data ? null : (
            <RSSDetailPlaceholderToolbar
              onBack={back}
              reserveWindowControls={reserveWindowControls}
              title={toolbarEntry.title}
            />
          )
        ) : renderEntryDetailToolbar(toolbarEntry, focused)}
        <RSSDetailHydrationSurface
          headingLevel={focused ? 1 : 2}
          query={detailQuery}
          title={toolbarEntry.title}
        >
          {(entry) => renderEntryDetailBody(entry, focused)}
        </RSSDetailHydrationSurface>
      </>
    );
  };

  if (resolvedFocusedEntry) {
    const focusedVideoPresentation = shouldUseRSSVideoLayoutPresentation(
      presentationRoute,
      resolvedFocusedEntry,
    );
    const detailContract = defineWorkspacePageContract({
      presentation: "primary",
      recipe: "detail",
      routeLabel: resolvedFocusedEntry.title,
      topBar: focusedVideoPresentation ? "host-owned" : "actions",
      heading: "host-owned",
      contentLayout: focusedVideoPresentation
        ? "canvas"
        : resolvedFocusedEntry.kind === "image"
          ? "card-grid"
          : "feed",
      footer: "none",
      scroll: "host",
      density: "regular",
      immersion: focusedVideoPresentation ? "edge-to-edge" : "standard",
    });
    return (
      <WorkspacePage
        className={cn(
          "rss-workspace-page rss-workspace-page--focused app-dream-window",
          focusedVideoPresentation
            && "youtube-workspace-page app-workspace-primary-subpane",
        )}
        contract={detailContract}
        data-reserve-window-controls={reserveWindowControls ? "true" : "false"}
      >
        <WorkspacePageContent className="rss-focused-entry">
          <div className="rss-focused-entry__content">
            {renderEntryDetailSurface(resolvedFocusedEntry, true)}
          </div>
        </WorkspacePageContent>
      </WorkspacePage>
    );
  }

  if (effectivePresentationRoute === "social") {
    return (
      <WorkspacePage
        className="rss-workspace-page app-dream-window"
        contract={collectionContract}
        data-reserve-window-controls={reserveWindowControls ? "true" : "false"}
      >
        {toolbar}
        <WorkspacePageContent
          className="rss-social-stream"
          onScroll={collectionScroll.onScroll}
          ref={collectionScroll.ref}
        >
          <RSSQuerySurface pagination={historyPagination} query={entriesQuery}>
            {entries.map((entry) => (
              <RSSSocialCard
                entry={entry}
                key={entry.id}
                language={language}
                source={subscriptionById.get(entry.subscriptionId)}
                onClick={() => openEntry(entry, true)}
              />
            ))}
            <RSSHistorySentinel pagination={historyPagination} />
          </RSSQuerySurface>
        </WorkspacePageContent>
      </WorkspacePage>
    );
  }

  if (effectivePresentationRoute === "image") {
    return (
      <WorkspacePage
        className="rss-workspace-page app-dream-window"
        contract={collectionContract}
        data-reserve-window-controls={reserveWindowControls ? "true" : "false"}
      >
        {toolbar}
        <WorkspacePageContent
          className="rss-image-scroll"
          onScroll={collectionScroll.onScroll}
          ref={collectionScroll.ref}
        >
          <RSSQuerySurface pagination={historyPagination} query={entriesQuery}>
            <div className="rss-image-masonry">
              {entries.map((entry) => (
                <button
                  className="rss-image-tile"
                  key={entry.id}
                  onClick={() => {
                    openEntry(entry);
                    setImageViewerState({ routeId, entryId: entry.id });
                  }}
                  type="button"
                >
                  <RSSRemoteImage
                    alt=""
                    fallback={<span className="rss-image-tile__placeholder"><ImageIcon /></span>}
                    loading="lazy"
                    sources={boundedRSSEntryImages(entry)}
                  />
                  <span className="rss-image-tile__caption">
                    <RSSFavicon source={subscriptionById.get(entry.subscriptionId)} />
                    <span>
                      <strong>{entry.title}</strong>
                      <small>{sourceLine(entry, subscriptionById.get(entry.subscriptionId), language)}</small>
                    </span>
                    <UnreadDot read={Boolean(entry.readAt)} />
                  </span>
                </button>
              ))}
              <RSSHistorySentinel pagination={historyPagination} />
            </div>
          </RSSQuerySurface>
        </WorkspacePageContent>
        <RSSImageLightbox
          currentEntryId={imageViewerEntryId}
          entries={entries}
          language={language}
          sourceById={subscriptionById}
          onEntryChange={(entry) => {
            openEntry(entry);
            setImageViewerState({ routeId, entryId: entry.id });
          }}
          onOpenChange={(open) => {
            if (!open) setImageViewerState({ routeId: "", entryId: "" });
          }}
        />
      </WorkspacePage>
    );
  }

  if (effectivePresentationRoute === "video") {
    return (
      <WorkspacePage
        className="rss-workspace-page app-dream-window"
        contract={collectionContract}
        data-reserve-window-controls={reserveWindowControls ? "true" : "false"}
      >
        {toolbar}
        <WorkspacePageContent
          className="youtube-workspace-scroll rss-video-browse-scroll"
          onScroll={collectionScroll.onScroll}
          ref={collectionScroll.ref}
        >
          <RSSQuerySurface pagination={historyPagination} query={entriesQuery}>
            <div className="youtube-workspace-grid" aria-busy={entriesQuery.isFetching}>
              {entries.map((entry) => {
                const source = subscriptionById.get(entry.subscriptionId);
                return (
                  <YouTubeVideoCard
                    fallbackChannel={source?.title || entry.author || "RSS"}
                    key={entry.id}
                    locale={language}
                    metadataPrefix={<UnreadDot read={Boolean(entry.readAt)} />}
                    opening={false}
                    selected={false}
                    thumbnail={(
                      <RSSRemoteImage
                        alt=""
                        draggable={false}
                        fallback={<span className="rss-video-thumbnail-placeholder"><Play /></span>}
                        loading="lazy"
                        sources={rssEntryImageCandidates(entry)}
                      />
                    )}
                    video={rssEntryToYouTubeBrowseCard(entry, source, language)}
                    onOpen={() => openEntry(entry, true)}
                  />
                );
              })}
              <RSSHistorySentinel pagination={historyPagination} />
            </div>
          </RSSQuerySurface>
        </WorkspacePageContent>
      </WorkspacePage>
    );
  }

  const articleList = effectivePresentationRoute === "article";
  return (
    <WorkspacePage
      className="rss-workspace-page app-dream-window"
      contract={collectionContract}
      data-reserve-window-controls={reserveWindowControls ? "true" : "false"}
    >
      <WorkspacePageContent className="rss-feed-page-content">
        <div className="rss-split-view">
          <section className="rss-collection-list-pane app-workspace-primary-subpane app-workspace-primary-subpane--leading">
            {toolbar}
            <RSSQuerySurface pagination={historyPagination} query={entriesQuery}>
              <div
                className={cn(
                  "rss-entry-list app-dream-selection-list",
                  articleList && "rss-entry-list--articles",
                )}
                data-scroll-owner="true"
                onScroll={handleCollectionPaneScroll}
                ref={collectionScroll.ref}
              >
                {entries.map((entry) => {
                  const audio = resolveRSSAudioPresentation(entry);
                  return (
                    <button
                      aria-current={entry.id === selectedEntryId ? "true" : undefined}
                      className={cn(
                        "rss-entry-row app-dream-selection-item",
                        articleList && "rss-entry-row--article",
                        audio && "rss-entry-row--audio",
                      )}
                      key={entry.id}
                      onClick={() => openEntry(entry)}
                      type="button"
                    >
                      <span className="rss-entry-row__read-slot">
                        <UnreadDot read={Boolean(entry.readAt)} />
                      </span>
                      <RSSFavicon source={subscriptionById.get(entry.subscriptionId)} />
                      {articleList ? (
                        <span className="rss-entry-row__article-text">
                          <small>
                            {audio ? <Headphones aria-hidden="true" /> : null}
                            {sourceLine(entry, subscriptionById.get(entry.subscriptionId), language)}
                          </small>
                          <strong>{entry.title}</strong>
                          <span>{entry.summary || htmlToText(entry.contentHtml)}</span>
                        </span>
                      ) : (
                        <strong className="rss-entry-row__compact-title">{entry.title}</strong>
                      )}
                      {articleList && rssEntryImageCandidates(entry).length > 0 ? (
                        <RSSRemoteImage
                          className="rss-entry-row__thumbnail"
                          alt=""
                          fallback={<span className="rss-entry-row__thumbnail rss-entry-row__thumbnail--placeholder"><ImageIcon /></span>}
                          loading="lazy"
                          sources={rssEntryImageCandidates(entry)}
                        />
                      ) : null}
                      {!articleList ? <time>{relativeTime(entry.publishedAt || entry.sourceUpdatedAt || entry.createdAt, language)}</time> : null}
                    </button>
                  );
                })}
                <RSSHistorySentinel pagination={historyPagination} />
              </div>
            </RSSQuerySurface>
          </section>
          <section
            className="rss-entry-detail-pane app-workspace-primary-subpane"
            tabIndex={-1}
          >
            {selectedListEntry ? (
              renderEntryDetailSurface(selectedListEntry)
            ) : (
              <>
                <RSSDetailPlaceholderToolbar
                  reserveWindowControls={reserveWindowControls}
                  title={t("xiadown.rss.selectEntry")}
                />
                <RSSEmptyDetail />
              </>
            )}
          </section>
        </div>
      </WorkspacePageContent>
    </WorkspacePage>
  );
}

function RSSImageLightbox({
  currentEntryId,
  entries,
  language,
  sourceById,
  onEntryChange,
  onOpenChange,
}: {
  currentEntryId: string;
  entries: readonly RSSEntry[];
  language: string;
  sourceById: ReadonlyMap<string, RSSSubscription>;
  onEntryChange: (entry: RSSEntry) => void;
  onOpenChange: (open: boolean) => void;
}) {
  const { t } = useI18n();
  const currentIndex = entries.findIndex((entry) => entry.id === currentEntryId);
  const entry = currentIndex >= 0 ? entries[currentIndex] : null;
  const move = React.useCallback((offset: number) => {
    if (currentIndex < 0) return;
    const next = entries[currentIndex + offset];
    if (next) onEntryChange(next);
  }, [currentIndex, entries, onEntryChange]);
  const handleKeyDown = (event: React.KeyboardEvent<HTMLDivElement>) => {
    if (event.altKey || event.ctrlKey || event.metaKey) return;
    if (event.key === "ArrowLeft") {
      event.preventDefault();
      move(-1);
    } else if (event.key === "ArrowRight") {
      event.preventDefault();
      move(1);
    }
  };
  const candidates = entry ? boundedRSSEntryImages(entry) : [];
  const source = entry ? sourceById.get(entry.subscriptionId) : undefined;

  return (
    <Dialog open={Boolean(entry)} onOpenChange={onOpenChange}>
      {entry ? (
        <DialogContent
          className="rss-image-lightbox"
          onKeyDown={handleKeyDown}
        >
          <DialogTitle className="sr-only">{entry.title}</DialogTitle>
          <DialogDescription className="sr-only">
            {sourceLine(entry, source, language)}
          </DialogDescription>
          <Button
            aria-label={t("xiadown.actions.previous")}
            className="rss-image-lightbox__navigation rss-image-lightbox__navigation--previous"
            disabled={currentIndex <= 0}
            onClick={() => move(-1)}
            shape="circle"
            size="compactIcon"
            type="button"
            variant="ghost"
          >
            <ChevronLeft />
          </Button>
          <figure key={entry.id} className="rss-image-lightbox__figure">
            <div className="rss-image-lightbox__media">
              <RSSRemoteImage
                alt={entry.title}
                decoding="async"
                fallback={<span className="rss-image-lightbox__placeholder"><ImageIcon /></span>}
                loading="eager"
                sources={candidates}
              />
            </div>
            <figcaption aria-live="polite">
              <span>
                <strong>{entry.title}</strong>
                <small>{sourceLine(entry, source, language)}</small>
              </span>
              <span className="rss-image-lightbox__count">
                {currentIndex + 1} / {entries.length}
              </span>
            </figcaption>
          </figure>
          <Button
            aria-label={t("xiadown.actions.next")}
            className="rss-image-lightbox__navigation rss-image-lightbox__navigation--next"
            disabled={currentIndex >= entries.length - 1}
            onClick={() => move(1)}
            shape="circle"
            size="compactIcon"
            type="button"
            variant="ghost"
          >
            <ChevronRight />
          </Button>
        </DialogContent>
      ) : null}
    </Dialog>
  );
}

function useRSSScrollRestoration(cacheKey: string, contentVersion: number) {
  const nodeRef = React.useRef<HTMLDivElement | null>(null);
  const keyRef = React.useRef(cacheKey);
  const restoreFrameRef = React.useRef<number | null>(null);

  const ref = React.useCallback<React.RefCallback<HTMLDivElement>>((node) => {
    if (nodeRef.current) {
      writeRSSScrollOffset(keyRef.current, nodeRef.current.scrollTop);
    }
    if (restoreFrameRef.current !== null) {
      window.cancelAnimationFrame(restoreFrameRef.current);
      restoreFrameRef.current = null;
    }

    nodeRef.current = node;
    keyRef.current = cacheKey;
    if (!node) {
      return;
    }

    const savedOffset = readRSSScrollOffset(cacheKey);
    node.scrollTop = savedOffset;
    restoreFrameRef.current = window.requestAnimationFrame(() => {
      restoreFrameRef.current = null;
      if (
        nodeRef.current === node &&
        keyRef.current === cacheKey &&
        readRSSScrollOffset(cacheKey) === savedOffset
      ) {
        node.scrollTop = savedOffset;
      }
    });
  }, [cacheKey]);

  React.useLayoutEffect(() => {
    const node = nodeRef.current;
    if (!node || keyRef.current !== cacheKey) {
      return;
    }
    node.scrollTop = readRSSScrollOffset(cacheKey);
  }, [cacheKey, contentVersion]);

  const onScroll = React.useCallback<React.UIEventHandler<HTMLDivElement>>(
    (event) => {
      writeRSSScrollOffset(cacheKey, event.currentTarget.scrollTop);
    },
    [cacheKey],
  );

  return { onScroll, ref };
}

function useRSSScrollMaterialState(identity: string) {
  const [state, setState] = React.useState({
    identity: "",
    scrolled: false,
  });
  const onScrollStateChange = React.useCallback((scrolled: boolean) => {
    setState((current) =>
      current.identity === identity && current.scrolled === scrolled
        ? current
        : { identity, scrolled });
  }, [identity]);
  const scrollMaterialState: WorkspacePageHeaderScrollState =
    state.identity === identity && state.scrolled ? "scrolled" : "top";
  return { onScrollStateChange, scrollMaterialState };
}

function RSSCollectionToolbar({
  count,
  loading,
  markingRead,
  reserveWindowControls,
  scrollMaterialState,
  unreadOnly,
  downloadLoadedCount,
  onMarkAllRead,
  onDownloadLoaded,
  onRefresh,
  onToggleUnread,
}: {
  count: number;
  loading: boolean;
  markingRead: boolean;
  reserveWindowControls: boolean;
  scrollMaterialState?: WorkspacePageHeaderScrollState;
  unreadOnly: boolean;
  downloadLoadedCount?: number;
  onMarkAllRead?: () => void;
  onDownloadLoaded?: () => void;
  onRefresh: () => void;
  onToggleUnread: () => void;
}) {
  const { t } = useI18n();
  return (
    <WorkspacePageTopBar
      className="rss-workspace-toolbar"
      reserveWindowControls={reserveWindowControls}
      scrollMaterialState={scrollMaterialState}
    >
      <WorkspacePrimaryHeaderActionGroup label={t("xiadown.actions.view")}>
        <RSSHeaderAction
          disabled={loading}
          label={t("xiadown.listen.refresh")}
          onClick={onRefresh}
        >
          <RefreshCcw className={cn(loading && "app-motion-spin")} />
        </RSSHeaderAction>
        <RSSHeaderAction
          aria-pressed={unreadOnly}
          label={t(unreadOnly ? "xiadown.rss.showAll" : "xiadown.rss.showUnread")}
          onClick={onToggleUnread}
        >
          {unreadOnly ? <Eye /> : <EyeOff />}
        </RSSHeaderAction>
      </WorkspacePrimaryHeaderActionGroup>
      <WorkspacePrimaryHeaderActionGroup label={t("xiadown.workspace.more")}>
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <RSSHeaderAction label={t("xiadown.workspace.more")}>
              <Ellipsis />
            </RSSHeaderAction>
          </DropdownMenuTrigger>
          <WorkspacePrimaryHeaderMenuContent className="rss-dropdown-menu">
            {onDownloadLoaded && downloadLoadedCount ? (
              <DropdownMenuItem onSelect={onDownloadLoaded}>
                <Download />
                {t("xiadown.dialogs.batchDownload")} · {downloadLoadedCount}
              </DropdownMenuItem>
            ) : null}
            <DropdownMenuItem
              disabled={!onMarkAllRead || markingRead || count === 0}
              onSelect={onMarkAllRead}
            >
              <CheckCheck className={cn(markingRead && "app-motion-pulse")} />
              {t("xiadown.rss.markAllRead")}
            </DropdownMenuItem>
          </WorkspacePrimaryHeaderMenuContent>
        </DropdownMenu>
      </WorkspacePrimaryHeaderActionGroup>
    </WorkspacePageTopBar>
  );
}

function RSSDetailPlaceholderToolbar({
  title,
  reserveWindowControls,
  scrollMaterialState = "top",
  onBack,
}: {
  title: string;
  reserveWindowControls: boolean;
  scrollMaterialState?: WorkspacePageHeaderScrollState;
  onBack?: () => void;
}) {
  const { t } = useI18n();
  return (
    <WorkspacePageTopBar
      actionsLabel={title}
      className="rss-detail-toolbar rss-detail-toolbar--placeholder"
      reserveWindowControls={reserveWindowControls}
      scrollMaterialState={scrollMaterialState}
    >
      {onBack ? (
        <RSSHeaderAction label={t("xiadown.rss.back")} onClick={onBack}>
          <ArrowLeft />
        </RSSHeaderAction>
      ) : null}
      <strong title={title}>{title}</strong>
    </WorkspacePageTopBar>
  );
}

type RSSEntriesInfiniteQuery = ReturnType<typeof useRSSEntriesInfinite>;

function useRSSPendingSubscriptionHydration({
  enabled,
  subscriptionId,
  lastSuccessAt,
  visibleEntries,
  refetch,
}: {
  enabled: boolean;
  subscriptionId: string | null | undefined;
  lastSuccessAt: string | undefined;
  visibleEntries: number;
  refetch: RSSEntriesInfiniteQuery["refetch"];
}) {
  const sessionKey = enabled ? subscriptionId?.trim() || "" : "";
  const sessionRef = React.useRef<{ key: string; deadline: number }>();
  if (!sessionKey) {
    sessionRef.current = undefined;
  } else if (sessionRef.current?.key !== sessionKey) {
    sessionRef.current = {
      key: sessionKey,
      deadline: Date.now() + RSS_PENDING_HYDRATION_REFETCH_WINDOW_MS,
    };
  }
  const deadline = sessionRef.current?.deadline ?? 0;

  React.useEffect(() => {
    if (!rssShouldFastPollPendingSubscription({
      enabled,
      subscriptionId,
      lastSuccessAt,
      visibleEntries,
      now: Date.now(),
      deadline,
    })) return;

    let cancelled = false;
    let timer: ReturnType<typeof setTimeout> | undefined;
    const schedule = () => {
      const remaining = deadline - Date.now();
      if (cancelled || remaining <= RSS_PENDING_HYDRATION_REFETCH_INTERVAL_MS) return;
      timer = setTimeout(() => {
        if (cancelled || !rssShouldFastPollPendingSubscription({
          enabled,
          subscriptionId,
          lastSuccessAt,
          visibleEntries,
          now: Date.now(),
          deadline,
        })) return;
        void refetch()
          .catch(() => {
            // The normal query error surface/default polling remains active.
          })
          .finally(schedule);
      }, RSS_PENDING_HYDRATION_REFETCH_INTERVAL_MS);
    };
    schedule();
    return () => {
      cancelled = true;
      if (timer !== undefined) clearTimeout(timer);
    };
  }, [deadline, enabled, lastSuccessAt, refetch, subscriptionId, visibleEntries]);
}

interface RSSHistoryPaginationState {
  key: string;
  stopped: boolean;
  error: unknown;
  backfillPages: number;
}

interface RSSHistoryPaginationController {
  advance: (force?: boolean) => Promise<void>;
  continuation: string;
  done: boolean;
  error: unknown;
  isBusy: boolean;
}

function useRSSHistoryPagination(
  query: RSSEntriesInfiniteQuery,
  entriesRequest: RSSListEntriesRequest,
  enabled: boolean,
): RSSHistoryPaginationController {
  const queryClient = useQueryClient();
  const backfill = useRSSBackfillHistory();
  const backfillRequest = React.useMemo(
    () => rssBackfillRequestForEntries(entriesRequest),
    [entriesRequest],
  );
  const requestKey = rssBackfillRequestKey(backfillRequest);
  const sessionKey = `${enabled ? "active" : "inactive"}:${requestKey}`;
  const entriesQueryKey = React.useMemo(
    () => rssQueryKeys.entriesInfinite({
      ...entriesRequest,
      offset: entriesRequest.offset ?? 0,
    }),
    [entriesRequest],
  );
  const generationRef = React.useRef(sessionKey);
  generationRef.current = sessionKey;
  const inFlightRef = React.useRef(false);
  const [settledGeneration, setSettledGeneration] = React.useState(0);
  const [historyState, setHistoryState] = React.useState<RSSHistoryPaginationState>(() => ({
    key: sessionKey,
    stopped: false,
    error: null,
    backfillPages: 0,
  }));
  const historyStateRef = React.useRef(historyState);
  const commitHistoryState = React.useCallback((next: RSSHistoryPaginationState) => {
    historyStateRef.current = next;
    setHistoryState(next);
  }, []);
  const settleAdvance = React.useCallback(() => {
    inFlightRef.current = false;
    // Always publish the settle edge. If the route changed while an old query
    // was in flight, the new route otherwise has no render that can observe
    // inFlightRef becoming false.
    setSettledGeneration((current) => current + 1);
  }, []);

  React.useEffect(() => {
    const next = {
      key: sessionKey,
      stopped: false,
      error: null,
      backfillPages: 0,
    };
    historyStateRef.current = next;
    setHistoryState(next);
    setSettledGeneration(0);
    backfill.reset();
  }, [sessionKey]);

  const advance = React.useCallback(async (force = false) => {
    if (
      !enabled ||
      inFlightRef.current ||
      backfill.isPending ||
      query.isLoading ||
      query.isFetching
    ) return;
    const activeKey = sessionKey;
    const state = historyStateRef.current.key === activeKey
      ? historyStateRef.current
      : { key: activeKey, stopped: false, error: null, backfillPages: 0 };

    if (query.hasNextPage) {
      if (query.isFetchNextPageError && !force) return;
      inFlightRef.current = true;
      try {
        await query.fetchNextPage();
      } catch {
        // React Query retains the page error for the sentinel's explicit retry.
      } finally {
        settleAdvance();
      }
      return;
    }

    if (
      backfillRequest === null ||
      state.backfillPages >= RSS_HISTORY_BACKFILL_PAGE_BUDGET ||
      (state.stopped && !force)
    ) return;
    const before = rssHistoryCollectionMetric(query.data?.pages);
    const attemptedPages = state.backfillPages + 1;
    inFlightRef.current = true;
    commitHistoryState({
      key: activeKey,
      stopped: false,
      error: null,
      backfillPages: state.backfillPages,
    });
    try {
      const result = await backfill.mutateAsync(backfillRequest);
      if (generationRef.current !== activeKey) return;
      const refreshed = queryClient.getQueryData<
        NonNullable<RSSEntriesInfiniteQuery["data"]>
      >(entriesQueryKey);
      const after = rssHistoryCollectionMetric(refreshed?.pages);
      const message = rssBackfillFailureMessage(result);
      commitHistoryState({
        key: activeKey,
        stopped: rssHistorySessionShouldStop(
          result,
          before,
          after,
          attemptedPages,
        ),
        error: message ? new Error(message) : null,
        backfillPages: attemptedPages,
      });
    } catch (error) {
      if (generationRef.current === activeKey) {
        commitHistoryState({
          key: activeKey,
          stopped: true,
          error,
          backfillPages: attemptedPages,
        });
      }
    } finally {
      settleAdvance();
    }
  }, [
    backfill.isPending,
    backfill.mutateAsync,
    backfillRequest,
    commitHistoryState,
    enabled,
    entriesQueryKey,
    query.data?.pages,
    query.fetchNextPage,
    query.hasNextPage,
    query.isFetchNextPageError,
    query.isFetching,
    query.isLoading,
    queryClient,
    sessionKey,
    settleAdvance,
  ]);

  const currentState = historyState.key === sessionKey
    ? historyState
    : { key: sessionKey, stopped: false, error: null, backfillPages: 0 };
  const localError = query.isFetchNextPageError ? query.error : null;
  const error = localError || currentState.error;
  const budgetExhausted =
    currentState.backfillPages >= RSS_HISTORY_BACKFILL_PAGE_BUDGET;
  return {
    advance,
    continuation: `${sessionKey}:${settledGeneration}`,
    done:
      !enabled ||
      (!query.hasNextPage &&
        (backfillRequest === null || currentState.stopped || budgetExhausted) &&
        (!error || budgetExhausted)),
    error,
    isBusy: query.isFetching || backfill.isPending || inFlightRef.current,
  };
}

function RSSQuerySurface({
  query,
  pagination,
  children,
}: {
  query: RSSEntriesInfiniteQuery;
  pagination: RSSHistoryPaginationController;
  children: React.ReactNode;
}) {
  const { t } = useI18n();
  if (query.isLoading && !query.data) {
    return <div className="rss-state-surface"><LoaderCircle className="app-motion-spin" /><span>{t("xiadown.rss.loading")}</span></div>;
  }
  if (query.isError && !query.data) {
    return (
      <div className="rss-state-surface rss-state-surface--error">
        <Rss />
        <strong>{t("xiadown.rss.loadFailed")}</strong>
        <span>{errorText(query.error)}</span>
        <Button onClick={() => void query.refetch()} type="button" variant="outline">{t("xiadown.listen.retry")}</Button>
      </div>
    );
  }
  if (mergeRSSEntryPages(query.data?.pages ?? []).length === 0) {
    return (
      <div className="rss-state-surface">
        <Rss />
        <strong>{t("xiadown.rss.emptyTitle")}</strong>
        <span>{t("xiadown.rss.emptyDescription")}</span>
        <RSSHistorySentinel pagination={pagination} />
      </div>
    );
  }
  return <>{children}</>;
}

function RSSHistorySentinel({
  pagination,
}: {
  pagination: RSSHistoryPaginationController;
}) {
  const { t } = useI18n();
  const sentinelRef = React.useRef<HTMLDivElement | null>(null);
  const observe = !pagination.done && !pagination.error;
  const gateRef = React.useRef<RSSHistorySentinelGate>();
  const requestRef = React.useRef({
    advance: pagination.advance,
    busy: pagination.isBusy,
    continuation: pagination.continuation,
    enabled: observe,
  });
  if (!gateRef.current) gateRef.current = createRSSHistorySentinelGate();
  requestRef.current = {
    advance: pagination.advance,
    busy: pagination.isBusy,
    continuation: pagination.continuation,
    enabled: observe,
  };
  const requestNextPage = React.useCallback(() => {
    const request = requestRef.current;
    if (gateRef.current?.tryAcquire(request)) {
      void request.advance();
    }
  }, []);

  React.useEffect(() => {
    const node = sentinelRef.current;
    if (!node) return;
    if (typeof IntersectionObserver === "undefined") {
      gateRef.current?.setVisible(true);
      requestNextPage();
      return () => gateRef.current?.setVisible(false);
    }
    const observer = new IntersectionObserver((entries) => {
      const visible = entries.some(
        (entry) => entry.isIntersecting || entry.intersectionRatio > 0,
      );
      gateRef.current?.setVisible(visible);
      if (visible) requestNextPage();
    }, {
      rootMargin: "640px 0px",
      threshold: 0.01,
    });
    observer.observe(node);
    return () => {
      gateRef.current?.setVisible(false);
      observer.disconnect();
    };
  }, [requestNextPage]);

  React.useEffect(() => {
    if (typeof requestAnimationFrame !== "function") {
      requestNextPage();
      return;
    }
    const frame = requestAnimationFrame(requestNextPage);
    return () => cancelAnimationFrame(frame);
  }, [observe, pagination.continuation, pagination.isBusy, requestNextPage]);

  if (pagination.done) return null;
  return (
    <div
      aria-busy={pagination.isBusy || undefined}
      aria-label={t("xiadown.rss.loadMore")}
      className="rss-entry-load-more"
      data-error={Boolean(pagination.error) || undefined}
      ref={sentinelRef}
      role="status"
    >
      {pagination.error ? (
        <small role="alert">
          <strong>{t("xiadown.rss.loadMoreFailed")}</strong>
          <span>{errorText(pagination.error)}</span>
        </small>
      ) : null}
      {pagination.error ? (
        <Button
          disabled={pagination.isBusy}
          onClick={() => void pagination.advance(true)}
          type="button"
          variant="outline"
        >
          {t("xiadown.listen.retry")}
        </Button>
      ) : pagination.isBusy ? <LoaderCircle className="app-motion-spin" /> : null}
    </div>
  );
}

interface RSSEntryDetailProps {
  entry: RSSEntry;
  /**
   * A focused entry owns the page heading. An entry rendered beside a feed
   * lives below the feed's assistive h1 and therefore starts at h2.
   */
  headingLevel: 1 | 2;
  source?: RSSSubscription;
  language: string;
  onBack?: () => void;
  onDownload: (
    url: string | undefined,
    entry: RSSEntry,
    batchTargets?: readonly RSSVideoBatchDownloadTarget[],
  ) => void;
  onReadChange: (read: boolean) => void;
  onScrollStateChange: (scrolled: boolean) => void;
  onStarChange: (starred: boolean) => void;
  readingMode: boolean;
  readerPreferences: RSSReaderPreferences;
  reserveWindowControls: boolean;
  videoPresentation: boolean;
  viewOriginal: boolean;
}

function RSSEntryDetail(props: RSSEntryDetailProps) {
  switch (props.entry.kind) {
    case "video":
      return props.videoPresentation
        ? <RSSVideoDetail {...props} />
        : <RSSArticleDetail {...props} />;
    case "social":
      return <RSSSocialDetail {...props} />;
    case "image":
      return <RSSImageDetail {...props} />;
    default:
      return <RSSArticleDetail {...props} />;
  }
}

type RSSDetailToolbarAction =
  | "read"
  | "star"
  | "readingMode"
  | "readAloud"
  | "readerSettings"
  | "source"
  | "share";

const RSS_DETAIL_TOOLBAR_STORAGE_KEY = "xiadown:rss:detail-toolbar:v2";
const RSS_DETAIL_TOOLBAR_ACTIONS: readonly RSSDetailToolbarAction[] = [
  "read",
  "star",
  "readingMode",
  "readAloud",
  "readerSettings",
  "source",
  "share",
];

function RSSDetailToolbar({
  entry,
  overlaysReader,
  source,
  reserveWindowControls,
  readingMode,
  readerPreferences,
  scrollMaterialState,
  viewOriginal,
  onBack,
  onExportPDF,
  onReadChange,
  onStarChange,
  onReadingModeChange,
  onReaderPreferencesChange,
  onViewOriginalChange,
}: {
  entry: RSSEntry;
  overlaysReader: boolean;
  source?: RSSSubscription;
  reserveWindowControls: boolean;
  readingMode: boolean;
  readerPreferences: RSSReaderPreferences;
  scrollMaterialState: WorkspacePageHeaderScrollState;
  viewOriginal: boolean;
  onBack?: () => void;
  onExportPDF: () => void;
  onReadChange: (read: boolean) => void;
  onStarChange: (starred: boolean) => void;
  onReadingModeChange: (enabled: boolean) => void;
  onReaderPreferencesChange: (preferences: RSSReaderPreferences) => void;
  onViewOriginalChange: (enabled: boolean) => void;
}) {
  const { t, language } = useI18n();
  const [customizing, setCustomizing] = React.useState(false);
  const [preferencesOpen, setPreferencesOpen] = React.useState(false);
  const [shortcutsOpen, setShortcutsOpen] = React.useState(false);
  const [visibleActions, setVisibleActions] = React.useState<Set<RSSDetailToolbarAction>>(
    () => readRSSDetailToolbarActions(),
  );
  const [status, setStatus] = React.useState("");
  const speech = useRSSReaderSpeech(entry, language);
  const visible = (action: RSSDetailToolbarAction) => visibleActions.has(action);
  const writeClipboardText = async (value: string, success: string) => {
    if (!value) return;
    try {
      await navigator.clipboard.writeText(value);
      setStatus(success);
    } catch (error) {
      setStatus(errorText(error));
    }
  };
  const toggleAction = (action: RSSDetailToolbarAction) => {
    setVisibleActions((current) => {
      const next = new Set(current);
      if (next.has(action)) next.delete(action);
      else next.add(action);
      writeRSSDetailToolbarActions(next);
      return next;
    });
  };
  const share = async () => {
    try {
      await shareRSSEntry(entry, source);
      setStatus(t("xiadown.rss.shared"));
    } catch (error) {
      setStatus(errorText(error));
    }
  };

  return (
    <>
      <WorkspacePageTopBar
        actionsLabel={entry.title}
        className="rss-detail-toolbar"
        data-rss-reader-overlay={overlaysReader || undefined}
        reserveWindowControls={reserveWindowControls}
        scrollMaterialState={scrollMaterialState}
      >
        {onBack ? (
          <WorkspacePrimaryHeaderActionGroup label={t("xiadown.rss.back")}>
            <RSSHeaderAction label={t("xiadown.rss.back")} onClick={onBack}>
              <ArrowLeft />
            </RSSHeaderAction>
          </WorkspacePrimaryHeaderActionGroup>
        ) : null}
        <WorkspacePrimaryHeaderActionGroup label={t("xiadown.actions.view")}>
          {visible("read") ? (
            <RSSHeaderAction
              aria-pressed={Boolean(entry.readAt)}
              label={t(entry.readAt ? "xiadown.rss.markUnread" : "xiadown.rss.markRead")}
              onClick={() => onReadChange(!Boolean(entry.readAt))}
            >
              {entry.readAt ? <Circle /> : <CheckCircle2 />}
            </RSSHeaderAction>
          ) : null}
          {visible("star") ? (
            <RSSHeaderAction
              aria-pressed={Boolean(entry.starredAt)}
              label={t(entry.starredAt ? "xiadown.rss.unstar" : "xiadown.rss.star")}
              onClick={() => onStarChange(!Boolean(entry.starredAt))}
            >
              <Star fill={entry.starredAt ? "currentColor" : "none"} />
            </RSSHeaderAction>
          ) : null}
          {visible("readingMode") ? (
            <RSSHeaderAction
              aria-pressed={readingMode}
              disabled={entry.kind !== "article"}
              label={t("xiadown.rss.readingMode")}
              onClick={() => onReadingModeChange(!readingMode)}
            >
              <BookOpenText />
            </RSSHeaderAction>
          ) : null}
          {visible("readAloud") ? (
            <RSSHeaderAction
              aria-pressed={speech.speaking}
              disabled={!speech.supported}
              label={t(speech.speaking ? "xiadown.rss.stopReadingAloud" : "xiadown.rss.readAloud")}
              onClick={speech.toggle}
            >
              {speech.speaking ? <Square /> : <Volume2 />}
            </RSSHeaderAction>
          ) : null}
          {visible("readerSettings") ? (
            <RSSHeaderAction
              label={t("xiadown.rss.readerPreferences")}
              onClick={() => setPreferencesOpen(true)}
            >
              <SlidersHorizontal />
            </RSSHeaderAction>
          ) : null}
          {visible("source") ? (
            <RSSHeaderAction
              aria-pressed={viewOriginal}
              disabled={!entry.url}
              label={t("xiadown.rss.viewOriginal")}
              onClick={() => onViewOriginalChange(!viewOriginal)}
            >
              <ExternalLink />
            </RSSHeaderAction>
          ) : null}
          {visible("share") ? (
            <RSSHeaderAction
              disabled={!entry.url}
              label={t("xiadown.rss.share")}
              onClick={() => void share()}
            >
              <Share2 />
            </RSSHeaderAction>
          ) : null}
        </WorkspacePrimaryHeaderActionGroup>
        <WorkspacePrimaryHeaderActionGroup label={t("xiadown.workspace.more")}>
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <RSSHeaderAction label={t("xiadown.workspace.more")}>
                <Ellipsis />
              </RSSHeaderAction>
            </DropdownMenuTrigger>
            <WorkspacePrimaryHeaderMenuContent className="rss-dropdown-menu">
              <DropdownMenuItem
                disabled={!entry.url}
                onSelect={() => void openExternalURL(entry.url || "")}
              >
                <ExternalLink /> {t("xiadown.rss.openInBrowser")}
              </DropdownMenuItem>
              <DropdownMenuItem
                disabled={!entry.url}
                onSelect={() => void writeClipboardText(entry.url || "", t("xiadown.rss.linkCopied"))}
              >
                <Copy /> {t("xiadown.rss.copyLink")}
              </DropdownMenuItem>
              <DropdownMenuItem
                onSelect={() => void writeClipboardText(entry.title, t("xiadown.rss.titleCopied"))}
              >
                <Copy /> {t("xiadown.rss.copyTitle")}
              </DropdownMenuItem>
              <DropdownMenuItem
                disabled={entry.kind !== "article"}
                onSelect={onExportPDF}
              >
                <FileDown /> {t("xiadown.rss.exportPDF")}
              </DropdownMenuItem>
              <DropdownMenuItem onSelect={() => setShortcutsOpen(true)}>
                <Keyboard /> {t("xiadown.rss.keyboardShortcuts")}
              </DropdownMenuItem>
              <DropdownMenuSeparator />
              <DropdownMenuItem onSelect={() => setCustomizing(true)}>
                <SlidersHorizontal /> {t("xiadown.rss.customizeToolbar")}
              </DropdownMenuItem>
            </WorkspacePrimaryHeaderMenuContent>
          </DropdownMenu>
        </WorkspacePrimaryHeaderActionGroup>
        <span className="sr-only" aria-live="polite">{status}</span>
      </WorkspacePageTopBar>
      <Dialog open={customizing} onOpenChange={setCustomizing}>
        <DialogContent className="rss-toolbar-dialog">
          <DialogTitle>{t("xiadown.rss.customizeToolbar")}</DialogTitle>
          <DialogDescription>{t("xiadown.rss.customizeToolbarDescription")}</DialogDescription>
          <div className="rss-toolbar-dialog__options">
            {RSS_DETAIL_TOOLBAR_ACTIONS.map((action) => (
              <label key={action}>
                <input
                  checked={visibleActions.has(action)}
                  onChange={() => toggleAction(action)}
                  type="checkbox"
                />
                <span>{rssDetailToolbarActionLabel(action, t, entry)}</span>
              </label>
            ))}
          </div>
          <DialogClose asChild>
            <Button type="button">{t("xiadown.rss.done")}</Button>
          </DialogClose>
        </DialogContent>
      </Dialog>
      <RSSReaderPreferencesDialog
        open={preferencesOpen}
        preferences={readerPreferences}
        onChange={onReaderPreferencesChange}
        onOpenChange={setPreferencesOpen}
      />
      <RSSKeyboardShortcutsDialog
        open={shortcutsOpen}
        onOpenChange={setShortcutsOpen}
      />
    </>
  );
}

function RSSReaderPreferencesDialog({
  open,
  preferences,
  onChange,
  onOpenChange,
}: {
  open: boolean;
  preferences: RSSReaderPreferences;
  onChange: (preferences: RSSReaderPreferences) => void;
  onOpenChange: (open: boolean) => void;
}) {
  const { t } = useI18n();
  const patchPreferences = (patch: Partial<RSSReaderPreferences>) => {
    onChange({ ...preferences, ...patch });
  };
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="rss-reader-preferences-dialog">
        <DialogTitle>{t("xiadown.rss.readerPreferences")}</DialogTitle>
        <DialogDescription>
          {t("xiadown.rss.readerPreferencesDescription")}
        </DialogDescription>
        <div className="rss-reader-preferences-dialog__settings">
          <div className="rss-reader-preferences-dialog__row">
            <span>
              <strong>{t("xiadown.rss.autoMarkRead")}</strong>
              <small>{t("xiadown.rss.autoMarkReadDescription")}</small>
            </span>
            <DreamInlineSwitch
              ariaLabel={t("xiadown.rss.autoMarkRead")}
              checked={preferences.autoMarkRead}
              onCheckedChange={(checked) => patchPreferences({ autoMarkRead: checked })}
            />
          </div>
          <label className="rss-reader-preferences-dialog__row">
            <span><strong>{t("xiadown.rss.openBehavior")}</strong></span>
            <Select
              aria-label={t("xiadown.rss.openBehavior")}
              onChange={(event) => patchPreferences({
                openMode: event.currentTarget.value as RSSReaderPreferences["openMode"],
              })}
              value={preferences.openMode}
            >
              <option value="reader">{t("xiadown.rss.openReader")}</option>
              <option value="original">{t("xiadown.rss.openOriginal")}</option>
            </Select>
          </label>
          <label className="rss-reader-preferences-dialog__row">
            <span><strong>{t("xiadown.rss.readerFontSize")}</strong></span>
            <Select
              aria-label={t("xiadown.rss.readerFontSize")}
              onChange={(event) => patchPreferences({
                fontSize: event.currentTarget.value as RSSReaderPreferences["fontSize"],
              })}
              value={preferences.fontSize}
            >
              <option value="small">{t("xiadown.rss.readerFontSmall")}</option>
              <option value="medium">{t("xiadown.rss.readerFontMedium")}</option>
              <option value="large">{t("xiadown.rss.readerFontLarge")}</option>
            </Select>
          </label>
          <label className="rss-reader-preferences-dialog__row">
            <span><strong>{t("xiadown.rss.readerDensity")}</strong></span>
            <Select
              aria-label={t("xiadown.rss.readerDensity")}
              onChange={(event) => patchPreferences({
                density: event.currentTarget.value as RSSReaderPreferences["density"],
              })}
              value={preferences.density}
            >
              <option value="compact">{t("xiadown.rss.readerDensityCompact")}</option>
              <option value="comfortable">{t("xiadown.rss.readerDensityComfortable")}</option>
              <option value="relaxed">{t("xiadown.rss.readerDensityRelaxed")}</option>
            </Select>
          </label>
        </div>
        <div className="rss-reader-preferences-dialog__actions">
          <Button
            onClick={() => onChange({ ...DEFAULT_RSS_READER_PREFERENCES })}
            type="button"
            variant="ghost"
          >
            {t("xiadown.actions.reset")}
          </Button>
          <DialogClose asChild>
            <Button type="button">{t("xiadown.rss.done")}</Button>
          </DialogClose>
        </div>
      </DialogContent>
    </Dialog>
  );
}

function RSSKeyboardShortcutsDialog({
  open,
  onOpenChange,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const { t } = useI18n();
  const shortcuts = [
    { keys: "J / K", label: t("xiadown.rss.shortcutNextPrevious") },
    { keys: "Enter", label: t("xiadown.rss.shortcutOpen") },
    { keys: "Esc", label: t("xiadown.rss.shortcutClose") },
    { keys: "M", label: t("xiadown.rss.shortcutToggleRead") },
    { keys: "S", label: t("xiadown.rss.shortcutToggleStarred") },
    { keys: "U", label: t("xiadown.rss.shortcutToggleUnread") },
    { keys: "R", label: t("xiadown.rss.shortcutRefresh") },
  ];
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="rss-keyboard-shortcuts-dialog">
        <DialogTitle>{t("xiadown.rss.keyboardShortcuts")}</DialogTitle>
        <DialogDescription>{t("xiadown.rss.keyboardShortcutsDescription")}</DialogDescription>
        <dl>
          {shortcuts.map((shortcut) => (
            <div key={shortcut.keys}>
              <dt><kbd>{shortcut.keys}</kbd></dt>
              <dd>{shortcut.label}</dd>
            </div>
          ))}
        </dl>
        <DialogClose asChild>
          <Button type="button">{t("xiadown.rss.done")}</Button>
        </DialogClose>
      </DialogContent>
    </Dialog>
  );
}

function useRSSReaderSpeech(
  entry: RSSEntry,
  language: string,
) {
  const [speaking, setSpeaking] = React.useState(false);
  const utteranceRef = React.useRef<SpeechSynthesisUtterance | null>(null);
  const text = React.useMemo(() => rssReaderSpeechText(entry), [
    entry.contentHtml,
    entry.summary,
    entry.title,
  ]);
  const supported = Boolean(text) &&
    typeof window !== "undefined" &&
    "speechSynthesis" in window &&
    typeof SpeechSynthesisUtterance !== "undefined";

  const stop = React.useCallback(() => {
    if (typeof window !== "undefined" && "speechSynthesis" in window) {
      window.speechSynthesis.cancel();
    }
    utteranceRef.current = null;
    setSpeaking(false);
  }, []);

  React.useEffect(() => {
    if (utteranceRef.current) stop();
  }, [entry.id, stop]);

  React.useEffect(() => () => {
    const utterance = utteranceRef.current;
    if (utterance) {
      utterance.onend = null;
      utterance.onerror = null;
      window.speechSynthesis.cancel();
    }
  }, []);

  const toggle = React.useCallback(() => {
    if (!supported) return;
    if (utteranceRef.current) {
      stop();
      return;
    }
    window.speechSynthesis.cancel();
    const utterance = new SpeechSynthesisUtterance(text);
    utterance.lang = language;
    utterance.rate = 1;
    utterance.onend = () => {
      if (utteranceRef.current !== utterance) return;
      utteranceRef.current = null;
      setSpeaking(false);
    };
    utterance.onerror = utterance.onend;
    utteranceRef.current = utterance;
    setSpeaking(true);
    try {
      window.speechSynthesis.speak(utterance);
    } catch {
      utteranceRef.current = null;
      setSpeaking(false);
    }
  }, [language, stop, supported, text]);

  return { speaking, supported, toggle };
}

function readRSSDetailToolbarActions() {
  if (typeof window === "undefined") return new Set(RSS_DETAIL_TOOLBAR_ACTIONS);
  try {
    const parsed = JSON.parse(window.localStorage.getItem(RSS_DETAIL_TOOLBAR_STORAGE_KEY) || "null");
    if (!Array.isArray(parsed)) return new Set(RSS_DETAIL_TOOLBAR_ACTIONS);
    const known = parsed.filter((item): item is RSSDetailToolbarAction =>
      RSS_DETAIL_TOOLBAR_ACTIONS.includes(item as RSSDetailToolbarAction),
    );
    return new Set(known);
  } catch {
    return new Set(RSS_DETAIL_TOOLBAR_ACTIONS);
  }
}

function writeRSSDetailToolbarActions(actions: ReadonlySet<RSSDetailToolbarAction>) {
  try {
    window.localStorage.setItem(
      RSS_DETAIL_TOOLBAR_STORAGE_KEY,
      JSON.stringify(RSS_DETAIL_TOOLBAR_ACTIONS.filter((action) => actions.has(action))),
    );
  } catch {
    // Customization remains active for this session if storage is unavailable.
  }
}

function rssDetailToolbarActionLabel(
  action: RSSDetailToolbarAction,
  t: (key: string) => string,
  entry: RSSEntry,
) {
  switch (action) {
    case "read": return t(entry.readAt ? "xiadown.rss.markUnread" : "xiadown.rss.markRead");
    case "star": return t(entry.starredAt ? "xiadown.rss.unstar" : "xiadown.rss.star");
    case "readingMode": return t("xiadown.rss.readingMode");
    case "readAloud": return t("xiadown.rss.readAloud");
    case "readerSettings": return t("xiadown.rss.readerPreferences");
    case "source": return t("xiadown.rss.viewOriginal");
    case "share": return t("xiadown.rss.share");
  }
}

async function shareRSSEntry(
  entry: RSSEntry,
  source?: RSSSubscription,
  preferredURL = "",
) {
  const url = preferredURL || entry.url || source?.siteUrl || "";
  if (!url) return;
  if (typeof navigator.share === "function") {
    await navigator.share({ title: entry.title, text: entry.summary, url });
    return;
  }
  await navigator.clipboard.writeText(url);
}

type RSSEntryQuery = ReturnType<typeof useRSSEntry>;

function RSSDetailHydrationSurface({
  headingLevel,
  query,
  title,
  children,
}: {
  headingLevel: 1 | 2;
  query: RSSEntryQuery;
  title: string;
  children: (entry: RSSEntry) => React.ReactNode;
}) {
  const { t } = useI18n();
  const Heading = headingLevel === 1 ? "h1" : "h2";
  const fallbackHeading = <Heading className="sr-only">{title}</Heading>;
  if (query.isLoading && !query.data) {
    return (
      <div className="rss-state-surface">
        {fallbackHeading}
        <LoaderCircle className="app-motion-spin" />
        <span>{t("xiadown.rss.loading")}</span>
      </div>
    );
  }
  if (query.isError && !query.data) {
    return (
      <div className="rss-state-surface rss-state-surface--error">
        {fallbackHeading}
        <Rss />
        <strong>{t("xiadown.rss.entryLoadFailed")}</strong>
        <span>{errorText(query.error)}</span>
        <Button onClick={() => void query.refetch()} type="button" variant="outline">
          {t("xiadown.listen.retry")}
        </Button>
      </div>
    );
  }
  return query.data
    ? <>{children(query.data)}</>
    : <>{fallbackHeading}<RSSEmptyDetail /></>;
}

function RSSArticleDetail(props: RSSEntryDetailProps) {
  React.useEffect(() => {
    if (props.viewOriginal && props.entry.url) {
      // Cross-origin iframe scrolling cannot bubble to the parent. Keep the
      // Title material visible so original pages still retain the same chrome.
      props.onScrollStateChange(true);
    }
  }, [props.entry.id, props.onScrollStateChange, props.viewOriginal]);
  if (props.viewOriginal && props.entry.url) {
    const Heading = props.headingLevel === 1 ? "h1" : "h2";
    return (
      <div className="rss-original-view">
        <Heading className="sr-only">{props.entry.title}</Heading>
        <iframe
          data-scroll-owner="host"
          referrerPolicy="no-referrer"
          sandbox=""
          src={props.entry.url}
          title={props.entry.title}
        />
      </div>
    );
  }
  return <RSSArticleReader {...props} />;
}

function resolveRSSReaderEmbedDownloadTarget(
  entry: RSSEntry,
  embed: RSSReaderVideoEmbedLayout,
) {
  const targetURL = rssReaderVideoDownloadURL(embed);
  const playbackURL = rssReaderVideoEmbedURL(embed);
  if (!targetURL || !playbackURL) return "";
  const experience = resolveRSSVideoExperience({
    ...entry,
    kind: "video",
    platform: embed.provider,
    platformVideoId: embed.videoId,
    url: targetURL,
    playbackUrl: playbackURL,
    downloadTarget: targetURL,
    mediaUrl: "",
    mediaType: "",
    media: [],
  });
  return experience.mode === "unavailable"
    ? ""
    : experience.targetUrl || "";
}

function RSSArticleReader(props: RSSEntryDetailProps) {
  const { entry, source } = props;
  const { t } = useI18n();
  const readerTheme = useRSSReaderTheme();
  const audio = resolveRSSAudioPresentation(entry);
  const readerRef = React.useRef<HTMLElement | null>(null);
  const frameRef = React.useRef<HTMLIFrameElement | null>(null);
  const documentSnapshotRef = React.useRef<RSSReaderDocumentSnapshot | null>(null);
  const documentSnapshot = resolveRSSReaderDocumentSnapshot(
    documentSnapshotRef.current,
    entry,
    readerTheme,
    source?.siteUrl,
    props.readerPreferences,
  );
  documentSnapshotRef.current = documentSnapshot;
  const documentSnapshotKeyRef = React.useRef(documentSnapshot.key);
  documentSnapshotKeyRef.current = documentSnapshot.key;
  const savedFraction =
    entry.articleProgress?.contentRevision === entry.revision
      ? Math.max(0, Math.min(1, entry.articleProgress.fraction))
      : 0;
  const [frameHeight, setFrameHeight] = React.useState(0);
  const frameHeightRef = React.useRef(0);
  const [progress, setProgress] = React.useState(savedFraction);
  const [outline, setOutline] = React.useState<RSSReaderOutlineItem[]>([]);
  const [readerVideoEmbedState, setReaderVideoEmbedState] = React.useState<{
    documentKey: string;
    items: RSSReaderVideoEmbedLayout[];
  }>({ documentKey: documentSnapshot.key, items: [] });
  const [outlineProgress, setOutlineProgress] = React.useState({
    activeIndex: -1,
    sectionFraction: 0,
  });
  const [readerSelectionActive, setReaderSelectionActive] = React.useState(false);
  const [imageContextTarget, setImageContextTarget] = React.useState<{
    slot: string;
    alt: string;
    x: number;
    y: number;
  } | null>(null);
  const [imageSaveStatus, setImageSaveStatus] = React.useState("");
  const restoredDocumentKeyRef = React.useRef("");
  const lastLayoutSequenceRef = React.useRef({ documentId: "", sequence: 0 });
  const latestProgressRef = React.useRef(savedFraction);
  const latestAnchorRef = React.useRef(entry.articleProgress?.anchor || "");
  const progressTimerRef = React.useRef<number | null>(null);
  const writeProgress = useCoalescedRSSStateWriter(entry, "articleProgress");

  React.useLayoutEffect(() => {
    restoredDocumentKeyRef.current = "";
    latestProgressRef.current = savedFraction;
    props.onScrollStateChange(false);
    setProgress(savedFraction);
    frameHeightRef.current = 0;
    setFrameHeight(0);
    setOutline([]);
    setReaderVideoEmbedState({ documentKey: documentSnapshot.key, items: [] });
    setOutlineProgress({ activeIndex: -1, sectionFraction: 0 });
    setReaderSelectionActive(false);
    setImageContextTarget(null);
    setImageSaveStatus("");
    latestAnchorRef.current = entry.articleProgress?.anchor || "";
  }, [documentSnapshot.key, props.onScrollStateChange]);

  React.useEffect(() => {
    let selectionAnimationFrame = 0;
    let selectionPointerY: number | null = null;
    const stopSelectionAutoScroll = () => {
      selectionPointerY = null;
      if (selectionAnimationFrame) {
        window.cancelAnimationFrame(selectionAnimationFrame);
        selectionAnimationFrame = 0;
      }
    };
    const endReaderSelectionInteraction = () => {
      setReaderSelectionActive(false);
      stopSelectionAutoScroll();
    };
    const scrollSelectionAtReaderEdge = () => {
      selectionAnimationFrame = 0;
      const reader = readerRef.current;
      if (!reader || selectionPointerY === null) return;
      const bounds = reader.getBoundingClientRect();
      const edgeSize = Math.min(64, Math.max(36, bounds.height * 0.12));
      let distance = 0;
      if (selectionPointerY < bounds.top + edgeSize) {
        distance = selectionPointerY - (bounds.top + edgeSize);
      } else if (selectionPointerY > bounds.bottom - edgeSize) {
        distance = selectionPointerY - (bounds.bottom - edgeSize);
      }
      if (!distance) return;
      const strength = Math.min(1, Math.abs(distance) / edgeSize);
      const delta = Math.sign(distance) * Math.ceil(2 + strength * 14);
      const previousScrollTop = reader.scrollTop;
      reader.scrollTop += delta;
      if (reader.scrollTop !== previousScrollTop) {
        selectionAnimationFrame = window.requestAnimationFrame(
          scrollSelectionAtReaderEdge,
        );
      }
    };
    const scheduleSelectionAutoScroll = () => {
      if (!selectionAnimationFrame) {
        selectionAnimationFrame = window.requestAnimationFrame(
          scrollSelectionAtReaderEdge,
        );
      }
    };
    const receiveReaderMessage = (event: MessageEvent<unknown>) => {
      if (event.source !== frameRef.current?.contentWindow) return;
      const layout = readRSSReaderLayoutMessage(
        event.data,
        entry.id,
        documentSnapshotRef.current?.documentId,
      );
      if (layout) {
        const layoutDocumentId = layout.documentId || "";
        if (lastLayoutSequenceRef.current.documentId !== layoutDocumentId) {
          lastLayoutSequenceRef.current = {
            documentId: layoutDocumentId,
            sequence: 0,
          };
        }
        if (
          layout.sequence !== undefined &&
          layout.sequence <= lastLayoutSequenceRef.current.sequence
        ) {
          return;
        }
        if (layout.sequence !== undefined) {
          lastLayoutSequenceRef.current.sequence = layout.sequence;
        }
        setReaderVideoEmbedState({
          documentKey: documentSnapshotKeyRef.current,
          items: layout.embeds ?? [],
        });
        const reader = readerRef.current;
        const frame = frameRef.current;
        const currentScrollTop = reader?.scrollTop ?? 0;
        const previousMaximum = reader
          ? Math.max(0, reader.scrollHeight - reader.clientHeight)
          : 0;
        const wasAtBottom = Boolean(
          reader &&
          previousMaximum > 1 &&
          previousMaximum - currentScrollTop <= 1,
        );
        const readerBounds = reader?.getBoundingClientRect();
        const frameBounds = frame?.getBoundingClientRect();
        const canPreserveViewport = Boolean(
          reader &&
          frame &&
          readerBounds &&
          frameBounds &&
          frameHeightRef.current > 0 &&
          restoredDocumentKeyRef.current === documentSnapshotKeyRef.current &&
          frameBounds.bottom > readerBounds.top &&
          frameBounds.top < readerBounds.bottom &&
          layout.shifts &&
          layout.shifts.length > 0,
        );
        let anchoredDelta = 0;
        if (canPreserveViewport && readerBounds && frameBounds) {
          let effectiveViewportTop = Math.max(
            0,
            readerBounds.top - frameBounds.top,
          );
          for (const shift of layout.shifts ?? []) {
            if (shift.bottom > effectiveViewportTop) continue;
            anchoredDelta += shift.delta;
            effectiveViewportTop += shift.delta;
          }
        }
        if (frame) frame.style.height = `${layout.height}px`;
        frameHeightRef.current = layout.height;
        setFrameHeight(layout.height);
        if (canPreserveViewport && reader) {
          const nextMaximum = Math.max(
            0,
            reader.scrollHeight - reader.clientHeight,
          );
          reader.scrollTop = wasAtBottom
            ? nextMaximum
            : Math.max(
                0,
                Math.min(currentScrollTop + anchoredDelta, nextMaximum),
              );
        }
        return;
      }
      const imageContext = readRSSReaderImageContextMessage(
        event.data,
        entry.id,
        documentSnapshotRef.current?.documentId || "",
      );
      if (imageContext) {
        const frameBounds = frameRef.current?.getBoundingClientRect();
        if (!frameBounds) return;
        setImageSaveStatus("");
        setImageContextTarget({
          slot: imageContext.slot,
          alt: imageContext.alt,
          x: Math.max(
            0,
            Math.min(window.innerWidth - 1, frameBounds.left + imageContext.clientX),
          ),
          y: Math.max(
            0,
            Math.min(window.innerHeight - 1, frameBounds.top + imageContext.clientY),
          ),
        });
        return;
      }
      const link = readRSSReaderLinkMessage(event.data, entry.id);
      if (link) {
        void openExternalURL(link.url).catch(() => {});
        return;
      }
      const nextOutline = readRSSReaderOutlineMessage(event.data, entry.id);
      if (nextOutline) {
        setOutline(nextOutline.items);
        return;
      }
      const selection = readRSSReaderSelectionMessage(event.data, entry.id);
      if (selection) {
        if (!selection.active) {
          endReaderSelectionInteraction();
          return;
        }
        setReaderSelectionActive(true);
        const readerBounds = readerRef.current?.getBoundingClientRect();
        if (!readerBounds) return;
        const frameBounds = frameRef.current?.getBoundingClientRect();
        const frameClientY = frameBounds
          ? frameBounds.top + selection.clientY
          : Number.NaN;
        const screenClientY = selection.screenY - window.screenY;
        const screenCoordinateIsUsable =
          selection.screenY > 0 &&
          screenClientY >= readerBounds.top - 128 &&
          screenClientY <= readerBounds.bottom + 128;
        selectionPointerY = Number.isFinite(frameClientY)
          ? frameClientY
          : screenCoordinateIsUsable
            ? screenClientY
            : null;
        if (selectionPointerY === null) return;
        scheduleSelectionAutoScroll();
        return;
      }
      const wheel = readRSSReaderWheelMessage(event.data, entry.id);
      const reader = readerRef.current;
      if (!wheel || !reader) return;
      reader.scrollBy({
        top: rssReaderWheelPixels(wheel, reader.clientHeight),
        behavior: "auto",
      });
    };
    window.addEventListener("message", receiveReaderMessage);
    window.addEventListener("pointerup", endReaderSelectionInteraction, true);
    window.addEventListener("pointercancel", endReaderSelectionInteraction, true);
    window.addEventListener("blur", endReaderSelectionInteraction);
    return () => {
      stopSelectionAutoScroll();
      window.removeEventListener("message", receiveReaderMessage);
      window.removeEventListener("pointerup", endReaderSelectionInteraction, true);
      window.removeEventListener("pointercancel", endReaderSelectionInteraction, true);
      window.removeEventListener("blur", endReaderSelectionInteraction);
    };
  }, [entry.id]);

  const readerOutline = React.useMemo(
    () => outline.length > 0 ? outline : buildRSSReaderFallbackOutline(frameHeight),
    [frameHeight, outline],
  );
  const outlineMarkers = React.useMemo(
    () => resolveRSSReaderOutlineMarkers(
      readerOutline,
      frameHeight,
      outlineProgress,
    ),
    [frameHeight, outlineProgress, readerOutline],
  );
  const titledOutlineItems = React.useMemo(
    () => outline
      .map((item, index) => ({ item, index }))
      .filter(({ item }) => Boolean(item.title.trim())),
    [outline],
  );

  const updateOutlineProgress = React.useCallback((reader: HTMLElement) => {
    const frameTop = frameRef.current?.offsetTop ?? 0;
    const activationTop = reader.scrollTop + reader.clientHeight / 3 - frameTop;
    const next = resolveRSSReaderOutlineProgress(
      readerOutline,
      activationTop,
      frameHeight,
      {
        atDocumentEnd: rssReaderScrollFraction(
          reader.scrollTop,
          reader.scrollHeight,
          reader.clientHeight,
        ) === 1,
      },
    );
    latestAnchorRef.current = outline.length > 0 && next.activeIndex >= 0
      ? readerOutline[next.activeIndex]?.id || ""
      : "";
    setOutlineProgress(next);
  }, [frameHeight, outline.length, readerOutline]);

  const scrollToOutlineItem = React.useCallback((item: RSSReaderOutlineItem) => {
    const reader = readerRef.current;
    if (!reader) return;
    const frameTop = frameRef.current?.offsetTop ?? 0;
    reader.scrollTo({
      top: Math.max(0, frameTop + item.top - reader.clientHeight / 3),
      behavior: preferredRSSReaderScrollBehavior(),
    });
  }, []);

  React.useLayoutEffect(() => {
    const reader = readerRef.current;
    if (!reader || readerOutline.length === 0) return;
    const animationFrame = window.requestAnimationFrame(() => updateOutlineProgress(reader));
    return () => window.cancelAnimationFrame(animationFrame);
  }, [readerOutline, updateOutlineProgress]);

  React.useLayoutEffect(() => {
    const reader = readerRef.current;
    if (
      !reader ||
      frameHeight <= 0 ||
      restoredDocumentKeyRef.current === documentSnapshot.key
    ) {
      return;
    }
    const animationFrame = window.requestAnimationFrame(() => {
      const maximum = Math.max(0, reader.scrollHeight - reader.clientHeight);
      reader.scrollTop = maximum * savedFraction;
      const restored = rssReaderScrollFraction(
        reader.scrollTop,
        reader.scrollHeight,
        reader.clientHeight,
      );
      restoredDocumentKeyRef.current = documentSnapshot.key;
      latestProgressRef.current = restored;
      setProgress(restored);
      updateOutlineProgress(reader);
      props.onScrollStateChange(
        isWorkspacePageHeaderScrolled(reader.scrollTop),
      );
    });
    return () => window.cancelAnimationFrame(animationFrame);
  }, [
    documentSnapshot.key,
    frameHeight,
    props.onScrollStateChange,
    savedFraction,
    updateOutlineProgress,
  ]);

  const flushProgress = React.useCallback(() => {
    if (progressTimerRef.current !== null) {
      window.clearTimeout(progressTimerRef.current);
      progressTimerRef.current = null;
    }
    const nextFraction = latestProgressRef.current;
    const nextAnchor = latestAnchorRef.current;
    writeProgress((current) => {
      const saved = current.articleProgress;
      if (
        saved?.contentRevision === current.revision &&
        Math.abs(saved.fraction - nextFraction) < 0.005 &&
        (saved.anchor || "") === nextAnchor
      ) {
        return null;
      }
      return buildRSSArticleProgressStateRequest(current, nextFraction, nextAnchor);
    });
  }, [writeProgress]);

  const persistProgress = React.useCallback((fraction: number) => {
    latestProgressRef.current = fraction;
    if (progressTimerRef.current !== null) return;
    progressTimerRef.current = window.setTimeout(flushProgress, 500);
  }, [flushProgress]);

  React.useEffect(() => () => {
    if (progressTimerRef.current !== null) flushProgress();
  }, [entry.id, flushProgress]);

  const handleReaderScroll = (event: React.UIEvent<HTMLElement>) => {
    const reader = event.currentTarget;
    const fraction = rssReaderScrollFraction(
      reader.scrollTop,
      reader.scrollHeight,
      reader.clientHeight,
    );
    latestProgressRef.current = fraction;
    setProgress(fraction);
    updateOutlineProgress(reader);
    props.onScrollStateChange(
      isWorkspacePageHeaderScrolled(reader.scrollTop),
    );
    if (restoredDocumentKeyRef.current === documentSnapshot.key) {
      persistProgress(fraction);
    }
  };

  const readerVideoEmbeds =
    readerVideoEmbedState.documentKey === documentSnapshot.key
      ? readerVideoEmbedState.items
      : [];

  const saveReaderImage = async () => {
    const target = imageContextTarget;
    if (!target) return;
    setImageContextTarget(null);
    setImageSaveStatus("");
    try {
      const result = await saveRSSEntryImage({
        entryId: entry.id,
        slot: target.slot,
        suggestedName: target.alt || entry.title || "image",
        dialogTitle: t("xiadown.rss.saveImage"),
        filterName: t("xiadown.workspace.images"),
        buttonText: t("xiadown.actions.save"),
      });
      if (result.saved) setImageSaveStatus(t("xiadown.rss.imageSaved"));
    } catch (error) {
      setImageSaveStatus(errorText(error));
    }
  };

  return (
    <article
      className="rss-reader"
      data-audio={audio ? "true" : undefined}
      data-scroll-owner="true"
      data-reading-mode={props.readingMode || undefined}
      onScroll={handleReaderScroll}
      ref={readerRef}
    >
      <div className="rss-reader__main">
        <RSSEntryHeader {...props} />
        {audio ? <RSSAudioPlayer entry={entry} audio={audio} /> : null}
        <div className="rss-reader__document-stack">
          <iframe
            className="rss-reader__document"
            ref={frameRef}
            referrerPolicy="no-referrer"
            sandbox="allow-scripts"
            scrolling="no"
            srcDoc={documentSnapshot.document}
            style={{ height: `${frameHeight || 1}px` }}
            title={entry.title}
          />
          {readerVideoEmbeds.map((embed, index) => {
            const downloadTarget = resolveRSSReaderEmbedDownloadTarget(entry, embed);
            return (
              <React.Fragment key={`${embed.provider}:${embed.videoId}:${index}`}>
                <iframe
                  allow={RSS_VIDEO_IFRAME_PERMISSIONS}
                  allowFullScreen
                  className="rss-reader__embedded-video"
                  loading="lazy"
                  referrerPolicy="no-referrer"
                  sandbox="allow-scripts allow-same-origin allow-presentation"
                  src={rssReaderVideoEmbedURL(embed)}
                  style={{
                    height: `${embed.height}px`,
                    left: `${embed.left}px`,
                    top: `${embed.top}px`,
                    width: `${embed.width}px`,
                  }}
                  title={embed.title || entry.title}
                />
                {downloadTarget ? (
                  <div
                    className="rss-reader__embedded-video-action"
                    style={{
                      height: `${embed.action.height}px`,
                      left: `${embed.action.left}px`,
                      top: `${embed.action.top}px`,
                      width: `${embed.action.width}px`,
                    }}
                  >
                    <Button
                      aria-label={`${t("xiadown.actions.download")}: ${embed.title || entry.title}`}
                      onClick={() => props.onDownload(downloadTarget, entry)}
                      size="sm"
                      type="button"
                      variant="outline"
                    >
                      <Download aria-hidden="true" />
                      {t("xiadown.actions.download")}
                    </Button>
                  </div>
                ) : null}
              </React.Fragment>
            );
          })}
        </div>
      </div>
      <aside
        className="rss-reader-progress"
        data-selection-active={readerSelectionActive || undefined}
      >
        <SecondaryReveal
          ariaLabel={t("xiadown.rss.readingProgress")}
          className="rss-reader-progress__sheet"
          closeDelay={120}
          content={({ close }) => (
            <nav
              aria-label={t("xiadown.rss.readingProgress")}
              className="rss-reader-progress__sheet-nav"
              onKeyDown={(event) => {
                if (event.key !== "Tab") return;
                const buttons = Array.from(
                  event.currentTarget.querySelectorAll<HTMLButtonElement>(
                    "button:not(:disabled)",
                  ),
                );
                const boundary = event.shiftKey
                  ? buttons[0]
                  : buttons[buttons.length - 1];
                if (event.target !== boundary) return;
                event.preventDefault();
                close();
              }}
            >
              <div className="rss-reader-progress__sheet-header">
                <strong>{t("xiadown.rss.readingProgress")}</strong>
                <span>{Math.round(progress * 100)}%</span>
              </div>
              {titledOutlineItems.length > 0 ? (
                <ol className="rss-reader-progress__sheet-list">
                  {titledOutlineItems.map(({ item, index }) => (
                    <li key={item.id}>
                      <button
                        aria-current={index === outlineProgress.activeIndex
                          ? "location"
                          : undefined}
                        data-depth={item.depth}
                        onClick={() => {
                          scrollToOutlineItem(item);
                          close();
                        }}
                        type="button"
                      >
                        {item.title}
                      </button>
                    </li>
                  ))}
                </ol>
              ) : (
                <p className="rss-reader-progress__sheet-empty">{entry.title}</p>
              )}
            </nav>
          )}
          key={readerSelectionActive ? "selection-active" : "interactive"}
          openDelay={0}
          pinOnClick={false}
          sideOffset={8}
        >
          {({ anchorProps, triggerProps }) => {
            return (
              <div
                {...anchorProps}
                className="rss-reader-progress__disclosure"
              >
                <nav
                  aria-label={t("xiadown.rss.readingProgress")}
                  className="rss-reader-progress__outline"
                >
                  {outlineMarkers.map((marker) => (
                    <button
                      aria-current={marker.active
                        ? "location"
                        : undefined}
                      aria-label={marker.item.title || t("xiadown.rss.readingProgress")}
                      className="rss-reader-progress__segment"
                      data-active={marker.active || undefined}
                      key={`${marker.item.id}:${marker.endIndex}`}
                      onClick={() => scrollToOutlineItem(marker.item)}
                      style={{
                        "--rss-toc-progress": `${marker.fillFraction * 100}%`,
                        "--rss-toc-width": `${marker.widthFraction * 100}%`,
                      } as React.CSSProperties}
                      type="button"
                    >
                      <span className="rss-reader-progress__track" aria-hidden="true">
                        <span />
                      </span>
                    </button>
                  ))}
                </nav>
                <button
                  {...triggerProps}
                  aria-label={`${t("xiadown.rss.readingProgress")} ${Math.round(progress * 100)}%`}
                  className="rss-reader-progress__value"
                  onClick={(event) => {
                    triggerProps.onClick?.(event);
                    if (event.detail === 0) {
                      focusRSSReaderProgressSheet(event.currentTarget, true);
                    }
                  }}
                  onKeyDown={(event) => {
                    if (
                      event.key === "Tab" &&
                      !event.shiftKey &&
                      focusRSSReaderProgressSheet(event.currentTarget)
                    ) {
                      event.preventDefault();
                    }
                  }}
                  type="button"
                >
                  {Math.round(progress * 100)}%
                </button>
              </div>
            );
          }}
        </SecondaryReveal>
        <RSSHeaderAction
          className="rss-reader-progress__back-to-top"
          label={t("xiadown.rss.backToTop")}
          onClick={() => readerRef.current?.scrollTo({
            top: 0,
            behavior: preferredRSSReaderScrollBehavior(),
          })}
        >
          <ArrowUp />
        </RSSHeaderAction>
      </aside>
      <DropdownMenu
        modal={false}
        open={Boolean(imageContextTarget)}
        onOpenChange={(open) => {
          if (!open) setImageContextTarget(null);
        }}
      >
        {imageContextTarget ? (
          <DropdownMenuTrigger asChild>
            <button
              aria-hidden="true"
              className="app-rss-subscription-context-menu__anchor"
              style={{ left: imageContextTarget.x, top: imageContextTarget.y }}
              tabIndex={-1}
              type="button"
            />
          </DropdownMenuTrigger>
        ) : null}
        <DropdownMenuContent
          align="start"
          aria-label={t("xiadown.rss.saveImage")}
          className="rss-dropdown-menu"
          onCloseAutoFocus={(event) => {
            event.preventDefault();
            frameRef.current?.focus();
          }}
          side="bottom"
          sideOffset={2}
        >
          <DropdownMenuItem onSelect={() => void saveReaderImage()}>
            <Download /> {t("xiadown.rss.saveImage")}
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
      <span aria-live="polite" className="sr-only">{imageSaveStatus}</span>
    </article>
  );
}

function RSSAudioPlayer({
  entry,
  audio,
}: {
  entry: RSSEntry;
  audio: RSSAudioPresentation;
}) {
  const { t } = useI18n();
  const images = rssEntryImageCandidates(entry);
  const duration = audio.durationMs > 0
    ? formatRSSVideoDuration(audio.durationMs / 1_000)
    : "";
  return (
    <section
      aria-label={t("xiadown.rss.podcastEpisode")}
      className="rss-audio-player"
    >
      <div className="rss-audio-player__cover" aria-hidden="true">
        <RSSRemoteImage
          alt=""
          fallback={<Headphones />}
          loading="eager"
          sources={images}
        />
      </div>
      <div className="rss-audio-player__body">
        <span className="rss-audio-player__eyebrow">
          <Headphones aria-hidden="true" />
          {t("xiadown.rss.podcastEpisode")}
          {duration ? <small>{duration}</small> : null}
        </span>
        <audio
          controls
          key={`${audio.url}:${audio.mimeType}`}
          preload="metadata"
        >
          <source src={audio.url} type={audio.mimeType} />
        </audio>
      </div>
    </section>
  );
}

function buildRSSReaderFallbackOutline(frameHeight: number): RSSReaderOutlineItem[] {
  if (!Number.isFinite(frameHeight) || frameHeight < 1) return [];
  const sectionCount = Math.max(3, Math.min(10, Math.ceil(frameHeight / 1_200)));
  return Array.from({ length: sectionCount }, (_, index) => ({
    id: `rss-section-${index + 1}`,
    title: "",
    depth: 1 as const,
    top: Math.round((frameHeight * index) / sectionCount),
  }));
}

function preferredRSSReaderScrollBehavior(): ScrollBehavior {
  if (
    typeof window !== "undefined" &&
    typeof window.matchMedia === "function" &&
    window.matchMedia("(prefers-reduced-motion: reduce)").matches
  ) {
    return "auto";
  }
  return "smooth";
}

function focusRSSReaderProgressSheet(
  trigger: HTMLButtonElement,
  deferred = false,
) {
  const contentId = trigger.getAttribute("aria-controls");
  const focusFirstChapter = () => {
    if (!contentId) return false;
    const firstChapter = document
      .getElementById(contentId)
      ?.querySelector<HTMLButtonElement>("button:not(:disabled)");
    firstChapter?.focus({ preventScroll: true });
    return Boolean(firstChapter);
  };
  if (deferred) {
    window.setTimeout(focusFirstChapter, 0);
    return true;
  }
  return focusFirstChapter();
}

function RSSSocialDetail(props: RSSEntryDetailProps) {
  React.useEffect(() => {
    props.onScrollStateChange(false);
  }, [props.entry.id, props.onScrollStateChange]);
  return (
    <article
      className="rss-detail-card rss-detail-card--social"
      data-scroll-owner="true"
      onScroll={(event) => props.onScrollStateChange(
        isWorkspacePageHeaderScrolled(event.currentTarget.scrollTop),
      )}
    >
      <RSSEntryHeader {...props} />
      <p>{htmlToText(props.entry.contentHtml) || props.entry.summary}</p>
      <RSSMediaGallery entry={props.entry} />
    </article>
  );
}

function RSSImageDetail(props: RSSEntryDetailProps) {
  const images = boundedRSSEntryImages(props.entry);
  React.useEffect(() => {
    props.onScrollStateChange(false);
  }, [props.entry.id, props.onScrollStateChange]);
  return (
    <article
      className="rss-detail-card rss-detail-card--images"
      data-scroll-owner="true"
      onScroll={(event) => props.onScrollStateChange(
        isWorkspacePageHeaderScrolled(event.currentTarget.scrollTop),
      )}
    >
      <RSSEntryHeader {...props} />
      <div className="rss-detail-images">
        {images.map((image) => <RSSRemoteImage alt="" key={image} src={image} />)}
      </div>
      {props.entry.summary ? <p>{props.entry.summary}</p> : null}
    </article>
  );
}

function RSSVideoDetail(props: RSSEntryDetailProps) {
  const { t } = useI18n();
  const { entry } = props;
  const experience = resolveRSSVideoExperience(entry);
  const [bilibiliMetadata, setBilibiliMetadata] =
    React.useState<RSSBilibiliPageMetadata | null>(null);
  const [siteMenuOpen, setSiteMenuOpen] = React.useState(false);
  const writeProgress = useCoalescedRSSStateWriter(entry, "videoProgressSeconds");
  const reportProgress = React.useCallback((currentTime: number, duration: number) => {
    writeProgress((current) => {
      if (
        Math.abs((current.videoProgressSeconds || 0) - currentTime) < 1 &&
        Math.abs((current.videoDurationSeconds || 0) - duration) < 1
      ) {
        return null;
      }
      return buildRSSVideoProgressStateRequest(current, currentTime, duration);
    });
  }, [writeProgress]);
  const youtubeNative = experience.mode === "youtube-native";
  const bilibiliIdentity = experience.mode === "bilibili-native"
    ? resolveRSSBilibiliPlaybackIdentity(entry)
    : null;
  const bilibiliVideoID = bilibiliIdentity?.platformVideoId || "";
  const displayMetadata = resolveRSSBilibiliDisplayMetadata(
    entry,
    props.source,
    experience.mode === "bilibili-native" ? bilibiliMetadata : null,
    bilibiliVideoID,
  );
  const publishedDisplay = displayMetadata.publishedAt
    ? formatYouTubePublishedLabel(displayMetadata.publishedAt, props.language)
    : "";
  const targetURL = experience.targetUrl || entry.url || "";
  const receiveBilibiliMetadata = React.useCallback(
    (metadata: RSSBilibiliPageMetadata) => setBilibiliMetadata(metadata),
    [],
  );
  const Heading = props.headingLevel === 1 ? "h1" : "h2";
  React.useEffect(() => setSiteMenuOpen(false), [entry.id]);
  return (
    <article
      className="youtube-workspace-watch-page rss-video-watch-page"
      data-reserve-window-controls={props.reserveWindowControls ? "true" : "false"}
    >
      <header
        className="youtube-workspace-watch-header rss-video-watch-header wails-drag"
        data-has-back={props.onBack ? "true" : "false"}
      >
        {props.onBack ? (
          <RSSHeaderAction
            className="youtube-workspace-watch-back wails-no-drag"
            label={t("xiadown.rss.back")}
            onClick={props.onBack}
          >
            <ArrowLeft />
          </RSSHeaderAction>
        ) : null}
        <div className="youtube-workspace-watch-info">
          <Heading title={entry.title}>{entry.title}</Heading>
          <div className="youtube-workspace-watch-byline">
            {displayMetadata.publisher ? (
              <span className="youtube-workspace-watch-uploader">
                <RSSFavicon source={props.source} />
                <strong>{displayMetadata.publisher}</strong>
              </span>
            ) : null}
            <span className="youtube-workspace-watch-stats">
              {publishedDisplay ? (
                <span title={t("xiadown.youtube.published")}>
                  <CalendarDays aria-hidden="true" />
                  <time dateTime={displayMetadata.publishedAt}>{publishedDisplay}</time>
                </span>
              ) : null}
              {displayMetadata.viewCount > 0 ? (
                <span title={t("xiadown.youtube.views")}>
                  <Eye aria-hidden="true" />
                  {formatYouTubeViewCount(displayMetadata.viewCount, props.language)}
                </span>
              ) : null}
              {displayMetadata.likeCount > 0 ? (
                <span title={t("xiadown.youtube.likes")}>
                  <ThumbsUp aria-hidden="true" />
                  {formatYouTubeViewCount(displayMetadata.likeCount, props.language)}
                </span>
              ) : null}
            </span>
            <RSSVideoMoreMenu
              entry={entry}
              source={props.source}
              targetURL={targetURL}
              onDownload={() => props.onDownload(experience.targetUrl, entry)}
              onOpenChange={experience.mode === "site" ? setSiteMenuOpen : undefined}
            />
          </div>
        </div>
      </header>
      {youtubeNative ? (
        <RSSYouTubePlayback
          active
          entry={entry}
          onDownload={() => props.onDownload(experience.targetUrl, entry)}
          onProgress={reportProgress}
        />
      ) : experience.mode === "bilibili-native" && bilibiliVideoID ? (
        <RSSBilibiliPlayback
          active
          entry={entry}
          platformVideoId={bilibiliVideoID}
          onDownload={() => props.onDownload(experience.targetUrl, entry)}
          onMetadata={receiveBilibiliMetadata}
          onProgress={reportProgress}
        />
      ) : experience.mode === "site" ? (
        <RSSSiteVideoPlayback
          active
          entry={entry}
          experience={experience}
          geometrySuspended={siteMenuOpen}
          onDownload={experience.targetUrl
            ? () => props.onDownload(experience.targetUrl, entry)
            : undefined}
        />
      ) : (
        <RSSWebVideoPlayback
          entry={entry}
          experience={experience}
          onDownload={experience.targetUrl
            ? () => props.onDownload(experience.targetUrl, entry)
            : undefined}
          onProgress={reportProgress}
        />
      )}
    </article>
  );
}

function RSSVideoMoreMenu({
  entry,
  source,
  targetURL,
  onDownload,
  onOpenChange,
}: {
  entry: RSSEntry;
  source?: RSSSubscription;
  targetURL: string;
  onDownload: () => void;
  onOpenChange?: (open: boolean) => void;
}) {
  const { t } = useI18n();
  const [status, setStatus] = React.useState("");
  const copyLink = async () => {
    if (!targetURL) return;
    try {
      await navigator.clipboard.writeText(targetURL);
      setStatus(t("xiadown.rss.linkCopied"));
    } catch (error) {
      setStatus(errorText(error));
    }
  };
  const copyTitle = async () => {
    if (!entry.title) return;
    try {
      await navigator.clipboard.writeText(entry.title);
      setStatus(t("xiadown.rss.titleCopied"));
    } catch (error) {
      setStatus(errorText(error));
    }
  };
  const share = async () => {
    try {
      await shareRSSEntry(entry, source, targetURL);
      setStatus(t("xiadown.rss.shared"));
    } catch (error) {
      setStatus(errorText(error));
    }
  };
  const moreLabel = t("xiadown.workspace.more");

  return (
    <>
      <DropdownMenu onOpenChange={onOpenChange}>
        <Tooltip>
          <TooltipTrigger asChild>
            <DropdownMenuTrigger asChild>
              <Button
                aria-label={moreLabel}
                className="youtube-workspace-watch-more wails-no-drag"
                shape="circle"
                size="compactIcon"
                title={moreLabel}
                type="button"
                variant="ghost"
              >
                <Ellipsis aria-hidden="true" />
              </Button>
            </DropdownMenuTrigger>
          </TooltipTrigger>
          <TooltipContent side="bottom">{moreLabel}</TooltipContent>
        </Tooltip>
        <DropdownMenuContent
          align="center"
          className="youtube-workspace-watch-more-menu rss-dropdown-menu"
          side="bottom"
        >
          <DropdownMenuItem onSelect={onDownload}>
            <Download /> {t("xiadown.actions.download")}
          </DropdownMenuItem>
          <DropdownMenuSeparator />
          <DropdownMenuItem
            disabled={!targetURL}
            onSelect={() => void openExternalURL(targetURL)}
          >
            <ExternalLink /> {t("xiadown.rss.openInBrowser")}
          </DropdownMenuItem>
          <DropdownMenuItem disabled={!targetURL} onSelect={() => void copyLink()}>
            <Copy /> {t("xiadown.rss.copyLink")}
          </DropdownMenuItem>
          <DropdownMenuItem disabled={!entry.title} onSelect={() => void copyTitle()}>
            <Copy /> {t("xiadown.rss.copyTitle")}
          </DropdownMenuItem>
          <DropdownMenuItem disabled={!targetURL} onSelect={() => void share()}>
            <Share2 /> {t("xiadown.rss.share")}
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
      <span aria-live="polite" className="sr-only">{status}</span>
    </>
  );
}

function RSSEntryHeader({
  entry,
  headingLevel,
  source,
  language,
}: RSSEntryDetailProps) {
  const publishedAt = entry.publishedAt || entry.sourceUpdatedAt || entry.createdAt;
  const Heading = headingLevel === 1 ? "h1" : "h2";
  return (
    <header className="rss-entry-header">
      <Heading>{entry.title}</Heading>
      <div className="rss-entry-header__source">
        <RSSFavicon source={source} />
        <strong>{source?.title || "RSS"}</strong>
        {source?.title && entry.author && entry.author !== source.title ? (
          <span>· {entry.author}</span>
        ) : null}
        <time dateTime={publishedAt} title={absoluteDateTime(publishedAt, language)}>
          · {relativeTime(publishedAt, language)}
        </time>
      </div>
    </header>
  );
}

function RSSSocialCard({ entry, source, language, onClick }: { entry: RSSEntry; source?: RSSSubscription; language: string; onClick: () => void }) {
  return (
    <button className="rss-social-card" onClick={onClick} type="button">
      <header><RSSFavicon source={source} /><span><strong>{entry.author || source?.title || "RSS"}</strong><small>{relativeTime(entry.publishedAt || entry.createdAt, language)}</small></span><UnreadDot read={Boolean(entry.readAt)} /></header>
      <p>{htmlToText(entry.contentHtml) || entry.summary || entry.title}</p>
      <RSSMediaGallery entry={entry} />
    </button>
  );
}

function RSSMediaGallery({ entry }: { entry: RSSEntry }) {
  const images = boundedRSSEntryImages(entry, 4);
  if (images.length === 0) {
    return null;
  }
  return <div className="rss-media-gallery" data-count={images.length}>{images.map((image) => <RSSRemoteImage alt="" key={image} loading="lazy" src={image} />)}</div>;
}

interface RSSOPMLImportSummary {
  imported: number;
  skipped: number;
  failures: string[];
}

function RSSSubscriptionManagementPage({
  subscriptions,
  categories,
  collections,
  reserveWindowControls,
}: {
  subscriptions: readonly RSSSubscription[];
  categories: readonly RSSCategory[];
  collections: readonly RSSCollection[];
  reserveWindowControls: boolean;
}) {
  const { t, language } = useI18n();
  const add = useRSSAddSubscription();
  const update = useRSSUpdateSubscription();
  const remove = useRSSDeleteSubscription();
  const refresh = useRSSRefresh();
  const markAllRead = useRSSMarkAllRead();
  const importInputRef = React.useRef<HTMLInputElement | null>(null);
  const managementSearchRef = React.useRef<HTMLInputElement | null>(null);
  const managementSurfaceRef = React.useRef<HTMLElement | null>(null);
  const organizationReturnFocusTargetRef = React.useRef<HTMLButtonElement | null>(null);
  const editReturnFocusTargetRef = React.useRef<HTMLButtonElement | null>(null);
  const deleteReturnFocusTargetRef = React.useRef<HTMLButtonElement | null>(null);
  const bulkDeleteReturnFocusTargetRef = React.useRef<HTMLButtonElement | null>(null);
  const [search, setSearch] = React.useState("");
  const [sort, setSort] = React.useState<RSSSubscriptionSort>("title");
  const [editingSubscription, setEditingSubscription] =
    React.useState<RSSSubscription | null>(null);
  const [organizationOpen, setOrganizationOpen] = React.useState(false);
  const [pendingDelete, setPendingDelete] = React.useState<RSSSubscription | null>(null);
  const [pendingBulkDeleteIDs, setPendingBulkDeleteIDs] = React.useState<string[]>([]);
  const [bulkDeleteFailures, setBulkDeleteFailures] = React.useState<
    Array<{ id: string; error: unknown }>
  >([]);
  const [selectedSubscriptionIDs, setSelectedSubscriptionIDs] =
    React.useState<Set<string>>(() => new Set());
  const [bulkAction, setBulkAction] = React.useState<
    "mark-read" | "enable" | "pause" | "delete" | ""
  >("");
  const [actionError, setActionError] = React.useState("");
  const [opmlBusy, setOPMLBusy] = React.useState<"import" | "export" | "">("");
  const [opmlSummary, setOPMLSummary] = React.useState<RSSOPMLImportSummary | null>(null);
  const visibleSubscriptions = React.useMemo(
    () => filterAndSortRSSSubscriptions(subscriptions, search, sort, language),
    [language, search, sort, subscriptions],
  );
  const visibleSubscriptionIDs = React.useMemo(
    () => visibleSubscriptions.map((subscription) => subscription.id),
    [visibleSubscriptions],
  );
  const selectedVisibleIDs = React.useMemo(
    () => visibleSubscriptionIDs.filter((id) => selectedSubscriptionIDs.has(id)),
    [selectedSubscriptionIDs, visibleSubscriptionIDs],
  );
  const selectedVisibleCount = selectedVisibleIDs.length;
  const allVisibleSelected = visibleSubscriptionIDs.length > 0 &&
    selectedVisibleCount === visibleSubscriptionIDs.length;
  React.useEffect(() => {
    setSelectedSubscriptionIDs((current) => reconcileRSSBulkSelection(
      current,
      visibleSubscriptionIDs,
    ));
  }, [visibleSubscriptionIDs]);
  const managementContract = defineWorkspacePageContract({
    presentation: "primary",
    recipe: "collection",
    routeLabel: t("xiadown.rss.manageSubscriptions"),
    topBar: "actions",
    heading: "assistive",
    contentLayout: "list",
    footer: "none",
    scroll: "content",
    density: "regular",
    immersion: "standard",
  });
  const managementScroll = useRSSScrollRestoration(
    buildRSSScrollCacheKey({
      routeId: RSS_WORKSPACE_ROUTE_IDS.manageSubscriptions,
      presentation: "subscriptions",
    }),
    visibleSubscriptions.length,
  );
  const runAction = async (action: () => Promise<unknown>) => {
    setActionError("");
    try {
      await action();
      return true;
    } catch (error) {
      setActionError(errorText(error));
      return false;
    }
  };
  const runBulkSubscriptionAction = async (
    action: Exclude<typeof bulkAction, "">,
    worker: (id: string) => Promise<unknown>,
    ids = selectedVisibleIDs,
  ) => {
    if (ids.length === 0 || bulkAction) return null;
    setActionError("");
    setBulkAction(action);
    try {
      const result = await runRSSBulkAction(ids, worker);
      setSelectedSubscriptionIDs(new Set(
        result.failures.map((failure) => failure.id),
      ));
      if (result.failures.length > 0) {
        setActionError(t("xiadown.rss.bulkActionFailed"));
      }
      return result;
    } finally {
      setBulkAction("");
    }
  };
  const importOPML = async (file: File) => {
    setActionError("");
    setOPMLSummary(null);
    setOPMLBusy("import");
    try {
      if (file.size > RSS_OPML_MAX_SOURCE_BYTES) {
        throw new Error("invalid_opml");
      }
      const items = parseRSSSubscriptionsFromOPML(await file.text());
      if (items.length === 0) throw new Error(t("xiadown.rss.opmlNoFeeds"));
      const knownSubscriptions = [...subscriptions];
      let imported = 0;
      let skipped = 0;
      const failures: string[] = [];
      for (const item of items) {
        if (rssFeedAddressSubscribed(item.url, knownSubscriptions)) {
          skipped += 1;
          continue;
        }
        try {
          const added = await add.mutateAsync(item);
          knownSubscriptions.push(added);
          imported += 1;
        } catch (error) {
          failures.push(`${item.title || item.url}: ${errorText(error)}`);
        }
      }
      setOPMLSummary({ imported, skipped, failures });
    } catch (error) {
      setActionError(errorText(error) === "invalid_opml" ? t("xiadown.rss.opmlInvalid") : errorText(error));
    } finally {
      setOPMLBusy("");
    }
  };
  const exportOPML = async () => {
    setActionError("");
    setOPMLBusy("export");
    try {
      const date = new Date().toISOString().slice(0, 10);
      await saveRSSOPMLFile(`xiadown-rss-${date}.opml`, exportRSSSubscriptionsToOPML(subscriptions));
    } catch (error) {
      setActionError(errorText(error));
    } finally {
      setOPMLBusy("");
    }
  };
  return (
    <WorkspacePage
      className="rss-workspace-page app-dream-window"
      contract={managementContract}
    >
      <WorkspacePageTopBar
        className="rss-page-heading"
        reserveWindowControls={reserveWindowControls}
      >
        <WorkspacePrimaryHeaderActionGroup label={managementContract.routeLabel}>
          <RSSHeaderAction
            disabled={Boolean(opmlBusy)}
            label={t("xiadown.rss.importOPML")}
            onClick={() => importInputRef.current?.click()}
          >
            {opmlBusy === "import" ? (
              <LoaderCircle aria-hidden="true" className="app-motion-spin" />
            ) : (
              <Upload aria-hidden="true" />
            )}
          </RSSHeaderAction>
          <RSSHeaderAction
            disabled={subscriptions.length === 0 || Boolean(opmlBusy)}
            label={t("xiadown.rss.exportOPML")}
            onClick={() => void exportOPML()}
          >
            {opmlBusy === "export" ? (
              <LoaderCircle aria-hidden="true" className="app-motion-spin" />
            ) : (
              <Download aria-hidden="true" />
            )}
          </RSSHeaderAction>
          <RSSHeaderAction
            label={t("xiadown.rss.organizationTitle")}
            onClick={(event) => {
              organizationReturnFocusTargetRef.current = event.currentTarget;
              setOrganizationOpen(true);
            }}
          >
            <FolderCog aria-hidden="true" />
          </RSSHeaderAction>
        </WorkspacePrimaryHeaderActionGroup>
        <WorkspacePrimaryHeaderActionGroup label={t("xiadown.rss.bulkActions")}>
          <RSSHeaderAction
            disabled={selectedVisibleCount === 0 || Boolean(bulkAction)}
            label={t("xiadown.rss.markAllRead")}
            onClick={() => void runBulkSubscriptionAction(
              "mark-read",
              (id) => markAllRead.mutateAsync({ subscriptionId: id }),
            )}
          >
            {bulkAction === "mark-read" ? (
              <LoaderCircle className="app-motion-spin" />
            ) : (
              <CheckCheck aria-hidden="true" />
            )}
          </RSSHeaderAction>
          <RSSHeaderAction
            disabled={selectedVisibleCount === 0 || Boolean(bulkAction)}
            label={t("xiadown.rss.enable")}
            onClick={() => void runBulkSubscriptionAction(
              "enable",
              (id) => update.mutateAsync({ id, enabled: true }),
            )}
          >
            <CheckCircle2 aria-hidden="true" />
          </RSSHeaderAction>
          <RSSHeaderAction
            disabled={selectedVisibleCount === 0 || Boolean(bulkAction)}
            label={t("xiadown.rss.pause")}
            onClick={() => void runBulkSubscriptionAction(
              "pause",
              (id) => update.mutateAsync({ id, enabled: false }),
            )}
          >
            <Square aria-hidden="true" />
          </RSSHeaderAction>
          <RSSHeaderAction
            disabled={selectedVisibleCount === 0 || Boolean(bulkAction)}
            label={t("xiadown.rss.unsubscribe")}
            onClick={(event) => {
              bulkDeleteReturnFocusTargetRef.current = event.currentTarget;
              setBulkDeleteFailures([]);
              setPendingBulkDeleteIDs(selectedVisibleIDs);
            }}
          >
            <Trash2 aria-hidden="true" />
          </RSSHeaderAction>
        </WorkspacePrimaryHeaderActionGroup>
        <WorkspacePrimaryHeaderActionGroup label={t("xiadown.listen.refresh")}>
          <RSSHeaderAction
            disabled={refresh.isPending || Boolean(opmlBusy)}
            label={t("xiadown.listen.refresh")}
            onClick={() => void runAction(() => refresh.mutateAsync({}))}
          >
            <RefreshCcw className={cn(refresh.isPending && "app-motion-spin")} />
          </RSSHeaderAction>
        </WorkspacePrimaryHeaderActionGroup>
      </WorkspacePageTopBar>
      <WorkspacePageContent
        className="rss-subscription-management-scroll"
        onScroll={managementScroll.onScroll}
        ref={managementScroll.ref}
      >
        <input
          accept={RSS_OPML_FILE_TYPES}
          className="sr-only"
          ref={importInputRef}
          type="file"
          onChange={(event) => {
            const input = event.currentTarget;
            const file = input.files?.[0];
            input.value = "";
            if (file) void importOPML(file);
          }}
        />
        {actionError ? (
          <p
            className="rss-management-error rss-management-feedback app-dream-status-message"
            data-intent="danger"
            role="alert"
          >
            {actionError}
          </p>
        ) : null}
        {opmlSummary ? <RSSOPMLImportResult summary={opmlSummary} /> : null}
        <section
          className="rss-subscription-management"
          ref={managementSurfaceRef}
          tabIndex={-1}
        >
          {subscriptions.length > 0 ? (
            <div className="rss-subscription-controls">
              <label className="rss-subscription-selection">
                <input
                  aria-label={t("xiadown.rss.selectVisibleSubscriptions")}
                  checked={allVisibleSelected}
                  onChange={(event) => setSelectedSubscriptionIDs((current) =>
                    setRSSVisibleSelection(
                      current,
                      visibleSubscriptionIDs,
                      event.currentTarget.checked,
                    ))}
                  ref={(node) => {
                    if (node) {
                      node.indeterminate = selectedVisibleCount > 0 &&
                        !allVisibleSelected;
                    }
                  }}
                  type="checkbox"
                />
                <span>
                  {t("xiadown.rss.selectedSubscriptions")} · {selectedVisibleCount}
                </span>
              </label>
              <label className="rss-subscription-search">
                <Search aria-hidden="true" />
                <Input
                  aria-label={t("xiadown.rss.managementSearchPlaceholder")}
                  onChange={(event) => setSearch(event.currentTarget.value)}
                  placeholder={t("xiadown.rss.managementSearchPlaceholder")}
                  ref={managementSearchRef}
                  type="search"
                  value={search}
                />
              </label>
              <label className="rss-discovery-select">
                <span>{t("xiadown.rss.sortBy")}</span>
                <Select
                  onChange={(event) => setSort(
                    event.currentTarget.value as RSSSubscriptionSort,
                  )}
                  value={sort}
                >
                  <option value="title">{t("xiadown.rss.sortTitle")}</option>
                  <option value="updated">{t("xiadown.rss.sortUpdated")}</option>
                  <option value="unread">{t("xiadown.rss.sortUnread")}</option>
                </Select>
              </label>
            </div>
          ) : null}
          {subscriptions.length === 0 ? (
            <div className="rss-state-surface"><Rss /><strong>{t("xiadown.rss.noSubscriptions")}</strong></div>
          ) : visibleSubscriptions.length === 0 ? (
            <div className="rss-state-surface"><Search /><strong>{t("xiadown.rss.noManagementMatches")}</strong></div>
          ) : (
            <div className="rss-subscription-table app-dream-card app-motion-surface">
              {visibleSubscriptions.map((item) => (
                <article className="rss-subscription-row" key={item.id}>
                  <label className="rss-subscription-row__selection">
                    <input
                      aria-label={`${t("xiadown.rss.selectSubscription")} ${item.title || item.feedUrl}`}
                      checked={selectedSubscriptionIDs.has(item.id)}
                      onChange={() => setSelectedSubscriptionIDs((current) =>
                        toggleRSSBulkSelection(current, item.id))}
                      type="checkbox"
                    />
                  </label>
                  <RSSFavicon source={item} />
                  <div className="rss-subscription-row__identity">
                    <span className="rss-subscription-row__title">
                      <strong>{item.title}</strong>
                      <Button
                        aria-label={t("xiadown.rss.editSubscription")}
                        onClick={(event) => {
                          editReturnFocusTargetRef.current = event.currentTarget;
                          setEditingSubscription(item);
                        }}
                        size="icon"
                        title={t("xiadown.rss.editSubscription")}
                        type="button"
                        variant="ghost"
                      >
                        <Pencil />
                      </Button>
                    </span>
                    <span>{item.feedUrl}</span>
                    {item.lastError ? <small>{item.lastError}</small> : null}
                  </div>
                  <Select aria-label={t("xiadown.rss.viewType")} disabled={update.isPending} onChange={(event) => void runAction(() => update.mutateAsync({ id: item.id, viewType: event.currentTarget.value as RSSViewType }))} value={item.viewType}>
                    {viewTypeOptions(t).map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}
                  </Select>
                  <span className="rss-subscription-row__updated">{item.lastSuccessAt ? relativeTime(item.lastSuccessAt, language) : "—"}</span>
                  <Button disabled={update.isPending} onClick={() => void runAction(() => update.mutateAsync({ id: item.id, enabled: !item.enabled }))} type="button" variant="outline">{item.enabled ? t("xiadown.rss.pause") : t("xiadown.rss.enable")}</Button>
                  <Button aria-label={t("xiadown.rss.refreshFeed")} disabled={refresh.isPending} onClick={() => void runAction(() => refresh.mutateAsync({ id: item.id }))} size="icon" title={t("xiadown.rss.refreshFeed")} type="button" variant="ghost"><RefreshCcw className={cn(refresh.isPending && "app-motion-spin")} /></Button>
                  <Button
                    aria-label={t("xiadown.rss.deleteSubscription")}
                    onClick={(event) => {
                      deleteReturnFocusTargetRef.current = event.currentTarget;
                      setPendingDelete(item);
                    }}
                    size="icon"
                    title={t("xiadown.rss.deleteSubscription")}
                    type="button"
                    variant="ghost"
                  >
                    <Trash2 />
                  </Button>
                </article>
              ))}
            </div>
          )}
        </section>
      </WorkspacePageContent>
      <Dialog open={organizationOpen} onOpenChange={setOrganizationOpen}>
        <DialogContent
          className="rss-organization-dialog"
          onCloseAutoFocus={(event) => {
            const target = organizationReturnFocusTargetRef.current;
            organizationReturnFocusTargetRef.current = null;
            if (!target?.isConnected) return;
            event.preventDefault();
            target.focus();
          }}
        >
          <DialogTitle>{t("xiadown.rss.organizationTitle")}</DialogTitle>
          <DialogDescription className="rss-organization-dialog__description">
            <span>{t("xiadown.rss.organizationDescription")}</span>
            <span>
              {t("xiadown.rss.selectedSubscriptions")} · {selectedSubscriptionIDs.size}
            </span>
          </DialogDescription>
          <DialogScrollArea className="rss-organization-dialog__scroll">
            <RSSOrganizationManager
              categories={categories}
              collections={collections}
              onSelectionChange={setSelectedSubscriptionIDs}
              selectedSubscriptionIDs={selectedSubscriptionIDs}
              subscriptions={subscriptions}
            />
          </DialogScrollArea>
        </DialogContent>
      </Dialog>
      {editingSubscription ? (
        <RSSSubscriptionDialog
          categories={categories}
          subscriptions={subscriptions}
          returnFocusTarget={editReturnFocusTargetRef.current}
          target={{ kind: "edit", subscription: editingSubscription }}
          onClose={() => setEditingSubscription(null)}
        />
      ) : null}
      <Dialog open={Boolean(pendingDelete)} onOpenChange={(open) => { if (!open && !remove.isPending) setPendingDelete(null); }}>
        <DialogContent
          className="rss-confirm-dialog"
          onCloseAutoFocus={(event) => {
            const preferredTarget = deleteReturnFocusTargetRef.current;
            deleteReturnFocusTargetRef.current = null;
            const target = preferredTarget?.isConnected
              ? preferredTarget
              : managementSearchRef.current?.isConnected
                ? managementSearchRef.current
                : managementSurfaceRef.current;
            if (!target) return;
            event.preventDefault();
            target.focus();
          }}
          showCloseButton={false}
        >
          <DialogTitle>{t("xiadown.rss.deleteSubscription")}</DialogTitle>
          <DialogDescription className="rss-confirm-dialog__description">
            <span>{t("xiadown.rss.deleteConfirm")}</span>
            {pendingDelete ? (
              <strong>{pendingDelete.title || pendingDelete.feedUrl}</strong>
            ) : null}
          </DialogDescription>
          <div className="rss-confirm-dialog__actions">
            <DialogClose asChild><Button disabled={remove.isPending} type="button" variant="outline">{t("xiadown.rss.cancel")}</Button></DialogClose>
            <Button
              disabled={!pendingDelete || remove.isPending}
              onClick={() => {
                if (!pendingDelete) return;
                void runAction(() => remove.mutateAsync({ id: pendingDelete.id })).then((removed) => {
                  if (removed) setPendingDelete(null);
                });
              }}
              type="button"
              variant="destructive"
            >
              {remove.isPending ? <LoaderCircle className="app-motion-spin" /> : <Trash2 />}
              {t("xiadown.rss.deleteSubscription")}
            </Button>
          </div>
        </DialogContent>
      </Dialog>
      <Dialog
        open={pendingBulkDeleteIDs.length > 0}
        onOpenChange={(open) => {
          if (!open && bulkAction !== "delete") {
            setPendingBulkDeleteIDs([]);
            setBulkDeleteFailures([]);
          }
        }}
      >
        <DialogContent
          className="rss-confirm-dialog"
          onCloseAutoFocus={(event) => {
            const preferredTarget = bulkDeleteReturnFocusTargetRef.current;
            bulkDeleteReturnFocusTargetRef.current = null;
            const target = preferredTarget?.isConnected && !preferredTarget.disabled
              ? preferredTarget
              : managementSearchRef.current?.isConnected
                ? managementSearchRef.current
                : managementSurfaceRef.current;
            if (!target) return;
            event.preventDefault();
            target.focus();
          }}
          showCloseButton={false}
        >
          <DialogTitle>{t("xiadown.rss.unsubscribeSelected")}</DialogTitle>
          <DialogDescription>
            {t("xiadown.rss.unsubscribeSelectedDescription")} · {pendingBulkDeleteIDs.length}
          </DialogDescription>
          {bulkDeleteFailures.length > 0 ? (
            <div aria-live="assertive" className="rss-form-error" role="alert">
              <strong>{t("xiadown.rss.bulkActionFailed")}</strong>
              <ul>
                {bulkDeleteFailures.slice(0, 6).map((failure) => {
                  const failedSubscription = subscriptions.find(
                    (subscription) => subscription.id === failure.id,
                  );
                  return (
                    <li key={failure.id}>
                      {failedSubscription?.title || failedSubscription?.feedUrl || failure.id}
                    </li>
                  );
                })}
                {bulkDeleteFailures.length > 6 ? (
                  <li>+{bulkDeleteFailures.length - 6}</li>
                ) : null}
              </ul>
            </div>
          ) : null}
          <div className="rss-confirm-dialog__actions">
            <DialogClose asChild>
              <Button disabled={bulkAction === "delete"} type="button" variant="outline">
                {t("xiadown.rss.cancel")}
              </Button>
            </DialogClose>
            <Button
              disabled={bulkAction === "delete"}
              onClick={() => void runBulkSubscriptionAction(
                "delete",
                (id) => remove.mutateAsync({ id }),
                pendingBulkDeleteIDs,
              ).then((result) => {
                if (!result) return;
                setBulkDeleteFailures(result.failures);
                setPendingBulkDeleteIDs(
                  result.failures.map((failure) => failure.id),
                );
              })}
              type="button"
              variant="destructive"
            >
              {bulkAction === "delete" ? <LoaderCircle className="app-motion-spin" /> : <Trash2 />}
              {t("xiadown.rss.unsubscribe")}
            </Button>
          </div>
        </DialogContent>
      </Dialog>
    </WorkspacePage>
  );
}

function RSSOPMLImportResult({ summary }: { summary: RSSOPMLImportSummary }) {
  const { t } = useI18n();
  return (
    <section aria-live="polite" className="rss-opml-result rss-management-feedback" role="status">
      <strong>{t("xiadown.rss.opmlImportComplete")}</strong>
      <span>
        {t("xiadown.rss.opmlImported")}: {summary.imported} · {t("xiadown.rss.opmlSkipped")}: {summary.skipped} · {t("xiadown.rss.opmlFailed")}: {summary.failures.length}
      </span>
      {summary.failures.length > 0 ? (
        <details>
          <summary>{t("xiadown.rss.opmlFailureDetails")}</summary>
          <ul>{summary.failures.map((failure) => <li key={failure}>{failure}</li>)}</ul>
        </details>
      ) : null}
    </section>
  );
}

function RSSFavicon({ source }: { source?: RSSSubscription }) {
  return <span className="rss-favicon"><RSSRemoteImage alt="" fallback={<Rss />} sources={[source?.iconUrl]} /></span>;
}

function firstControlledRSSResourceURL(candidates: readonly (string | undefined)[]) {
  for (const candidate of candidates) {
    const controlled = controlledRSSResourceURL(candidate);
    if (controlled) return controlled;
  }
  return "";
}

function UnreadDot({ read }: { read: boolean }) {
  return <span aria-hidden="true" className="rss-unread-dot" data-read={read || undefined} />;
}

function RSSEmptyDetail() {
  const { t } = useI18n();
  return <div className="rss-state-surface"><Rss /><span>{t("xiadown.rss.selectEntry")}</span></div>;
}

function resolveCollectionRoute(routeId: string, subscription?: RSSSubscription): RSSCollectionRoute {
  if (subscription) {
    return subscription.viewType === "auto" ? "all" : subscription.viewType;
  }
  switch (routeId) {
    case RSS_WORKSPACE_ROUTE_IDS.articles:
      return "article";
    case RSS_WORKSPACE_ROUTE_IDS.social:
      return "social";
    case RSS_WORKSPACE_ROUTE_IDS.images:
      return "image";
    case RSS_WORKSPACE_ROUTE_IDS.videos:
      return "video";
    default:
      return "all";
  }
}

function collectionTitle(
  routeId: string,
  subscription: RSSSubscription | undefined,
  category: RSSCategory | undefined,
  collection: RSSCollection | undefined,
  t: (key: string) => string,
) {
  if (subscription) return subscription.title;
  if (category) return category.title;
  if (collection) return collection.title;
  switch (routeId) {
    case RSS_WORKSPACE_ROUTE_IDS.search: return t("xiadown.workspace.search");
    case RSS_WORKSPACE_ROUTE_IDS.articles: return t("xiadown.rss.articles");
    case RSS_WORKSPACE_ROUTE_IDS.social: return t("xiadown.rss.socialMedia");
    case RSS_WORKSPACE_ROUTE_IDS.images: return t("xiadown.workspace.images");
    case RSS_WORKSPACE_ROUTE_IDS.videos: return t("xiadown.rss.videos");
    case RSS_WORKSPACE_ROUTE_IDS.starred: return t("xiadown.rss.favorites");
    default: return t("xiadown.workspace.all");
  }
}

function sourceLine(entry: RSSEntry, source: RSSSubscription | undefined, language: string) {
  return [source?.title || entry.author, relativeTime(entry.publishedAt || entry.sourceUpdatedAt || entry.createdAt, language)].filter(Boolean).join(" · ");
}

function rssEntryToYouTubeBrowseCard(
  entry: RSSEntry,
  source: RSSSubscription | undefined,
  language: string,
): YouTubeWorkspaceVideo {
  const durationSeconds = Math.max(
    0,
    entry.videoDurationSeconds ||
      (entry.media.find((item) => (item.durationMs || 0) > 0)?.durationMs || 0) / 1_000,
  );
  return {
    itemKind: "video",
    videoId: entry.platformVideoId || entry.externalId || entry.id,
    title: entry.title,
    channel: sourceLine(entry, source, language),
    thumbnailUrl: firstControlledRSSResourceURL(rssEntryImageCandidates(entry)) || undefined,
    durationSeconds: durationSeconds || undefined,
    durationLabel: durationSeconds ? formatRSSVideoDuration(durationSeconds) : undefined,
    publishedLabel: entry.publishedAt || entry.sourceUpdatedAt || entry.createdAt,
    webUrl: entry.url || entry.downloadTarget || entry.mediaUrl || "",
  };
}

function formatRSSVideoDuration(value: number) {
  const seconds = Math.max(0, Math.floor(value));
  const hours = Math.floor(seconds / 3_600);
  const minutes = Math.floor((seconds % 3_600) / 60);
  const remainder = seconds % 60;
  return hours > 0
    ? `${hours}:${String(minutes).padStart(2, "0")}:${String(remainder).padStart(2, "0")}`
    : `${minutes}:${String(remainder).padStart(2, "0")}`;
}

function relativeTime(value: string | undefined, language: string) {
  if (!value) return "";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "";
  const seconds = Math.round((date.getTime() - Date.now()) / 1000);
  const ranges: Array<[Intl.RelativeTimeFormatUnit, number]> = [["year", 31_536_000], ["month", 2_592_000], ["week", 604_800], ["day", 86_400], ["hour", 3_600], ["minute", 60]];
  const formatter = new Intl.RelativeTimeFormat(language, { numeric: "auto" });
  for (const [unit, size] of ranges) {
    if (Math.abs(seconds) >= size) return formatter.format(Math.round(seconds / size), unit);
  }
  return formatter.format(seconds, "second");
}

function absoluteDateTime(value: string | undefined, language: string) {
  if (!value) return "";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "";
  return new Intl.DateTimeFormat(language, {
    dateStyle: "medium",
    timeStyle: "short",
  }).format(date);
}

function htmlToText(markup?: string) {
  if (!markup) return "";
  if (typeof DOMParser === "undefined") return markup.replace(/<[^>]+>/g, " ").replace(/\s+/g, " ").trim();
  return new DOMParser().parseFromString(markup, "text/html").body.textContent?.replace(/\s+/g, " ").trim() || "";
}

function rssKeyboardTargetIsEditable(target: EventTarget | null) {
  if (!(target instanceof Element)) return false;
  return Boolean(target.closest(
    "input,textarea,select,button,a,[contenteditable='true'],[role='dialog'],[role='menu']",
  ));
}

function printRSSArticle(
  entry: RSSEntry,
  source: RSSSubscription | undefined,
  language: string,
) {
  if (entry.kind !== "article" || typeof document === "undefined") return;
  const frame = document.createElement("iframe");
  frame.className = "rss-article-print-frame";
  frame.setAttribute("aria-hidden", "true");
  frame.setAttribute("sandbox", "allow-modals allow-same-origin");
  frame.tabIndex = -1;
  frame.title = entry.title;
  let cleanupTimer: number | null = null;
  const cleanup = () => {
    if (cleanupTimer !== null) window.clearTimeout(cleanupTimer);
    cleanupTimer = null;
    frame.remove();
  };
  frame.addEventListener("load", () => {
    const printWindow = frame.contentWindow;
    if (!printWindow) {
      cleanup();
      return;
    }
    printWindow.addEventListener("afterprint", cleanup, { once: true });
    cleanupTimer = window.setTimeout(cleanup, 60_000);
    try {
      printWindow.focus();
      printWindow.print();
    } catch {
      cleanup();
    }
  }, { once: true });
  frame.srcdoc = buildRSSArticlePrintDocument(
    entry,
    currentRSSReaderTheme(),
    source?.siteUrl,
    {
      source: source?.title,
      author: entry.author,
      published: absoluteDateTime(
        entry.publishedAt || entry.sourceUpdatedAt || entry.createdAt,
        language,
      ),
    },
  );
  document.body.append(frame);
}

function viewTypeOptions(t: (key: string) => string): Array<{ value: RSSViewType; label: string }> {
  return [
    { value: "auto", label: t("xiadown.rss.viewAuto") },
    { value: "article", label: t("xiadown.rss.articles") },
    { value: "social", label: t("xiadown.rss.socialMedia") },
    { value: "image", label: t("xiadown.workspace.images") },
    { value: "video", label: t("xiadown.rss.videos") },
  ];
}

function useRSSReaderTheme() {
  const [theme, setTheme] = React.useState<"light" | "dark">(() => currentRSSReaderTheme());
  React.useEffect(() => {
    const root = document.documentElement;
    const update = () => setTheme(currentRSSReaderTheme());
    const observer = new MutationObserver(update);
    observer.observe(root, { attributes: true, attributeFilter: ["class", "style"] });
    update();
    return () => observer.disconnect();
  }, []);
  return theme;
}

type RSSStateRequestFactory = (
  current: RSSEntry,
) => RSSSetEntryStateRequest | null;

interface RSSPendingStateWrite {
  entryId: string;
  factory: RSSStateRequestFactory;
}

/** Serializes and coalesces high-frequency reader/player progress writes. */
function useCoalescedRSSStateWriter(
  entry: RSSEntry,
  field: RSSEntryStateField,
) {
  const mutation = useRSSSetEntryState();
  const mutationRef = React.useRef(mutation.mutateAsync);
  const entryRef = React.useRef(entry);
  const entryIdRef = React.useRef(entry.id);
  const pendingRef = React.useRef<RSSPendingStateWrite | null>(null);
  const runningRef = React.useRef(false);

  mutationRef.current = mutation.mutateAsync;
  if (entryIdRef.current !== entry.id) {
    entryIdRef.current = entry.id;
    pendingRef.current = null;
    entryRef.current = entry;
  } else if (entry.stateRevision >= entryRef.current.stateRevision) {
    entryRef.current = entry;
  }

  const drain = React.useCallback(() => {
    if (runningRef.current) return;
    runningRef.current = true;
    void (async () => {
      try {
        while (pendingRef.current) {
          const pending = pendingRef.current;
          pendingRef.current = null;
          if (
            pending.entryId !== entryIdRef.current ||
            entryRef.current.id !== pending.entryId
          ) {
            continue;
          }
          const request = pending.factory(entryRef.current);
          if (
            !request ||
            request.id !== pending.entryId ||
            request.field !== field
          ) {
            continue;
          }
          try {
            const state = await mutationRef.current(request);
            if (
              state.entryId === entryIdRef.current &&
              state.entryId === pending.entryId
            ) {
              entryRef.current = applyRSSStateToEntry(entryRef.current, state);
            }
          } catch {
            // The mutation hook invalidates stale caches. A later progress
            // event starts a new coalesced write with the hydrated revision.
          }
        }
      } finally {
        runningRef.current = false;
        if (pendingRef.current) drain();
      }
    })();
  }, [field]);

  return React.useCallback((factory: RSSStateRequestFactory) => {
    const expectedEntryId = entry.id;
    if (
      entryIdRef.current !== expectedEntryId ||
      entryRef.current.id !== expectedEntryId
    ) {
      return;
    }
    pendingRef.current = { entryId: expectedEntryId, factory };
    drain();
  }, [drain, entry.id]);
}

function currentRSSReaderTheme(): "light" | "dark" {
  if (typeof document === "undefined") return "light";
  return document.documentElement.classList.contains("dark")
    ? "dark"
    : "light";
}

async function saveRSSOPMLFile(filename: string, contents: string) {
  const picker = (window as Window & {
    showSaveFilePicker?: (options: {
      suggestedName: string;
      types: Array<{ description: string; accept: Record<string, string[]> }>;
    }) => Promise<{
      createWritable: () => Promise<{
        write: (value: string) => Promise<void>;
        close: () => Promise<void>;
      }>;
    }>;
  }).showSaveFilePicker;
  if (picker) {
    try {
      const handle = await picker({
        suggestedName: filename,
        types: [{ description: "OPML", accept: { "text/x-opml": [".opml", ".xml"] } }],
      });
      const writable = await handle.createWritable();
      await writable.write(contents);
      await writable.close();
      return;
    } catch (error) {
      if (error instanceof DOMException && error.name === "AbortError") return;
      throw error;
    }
  }
  const url = URL.createObjectURL(new Blob([contents], { type: "text/x-opml;charset=utf-8" }));
  const anchor = document.createElement("a");
  anchor.download = filename;
  anchor.href = url;
  anchor.hidden = true;
  document.body.append(anchor);
  anchor.click();
  anchor.remove();
  window.setTimeout(() => URL.revokeObjectURL(url), 0);
}

function errorText(error: unknown) {
  return error instanceof Error ? error.message : String(error ?? "");
}
