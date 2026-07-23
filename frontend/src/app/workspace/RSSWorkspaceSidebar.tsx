import type {
  WideWorkspaceSidebarProps,
  WorkspaceSidebarItemCatalog,
  WorkspaceSidebarNavigationItem,
  WorkspaceSidebarRegionCatalog,
  WorkspaceSidebarSection,
  WorkspaceSidebarSectionCatalog,
} from "@/app/workspace/navigation-types";
import { WorkspaceSidebarNavigation } from "@/app/workspace/WorkspaceSidebarNavigation";
import {
  controlledRSSResourceURL,
} from "@/app/rss/remote-resource";
import type {
  RSSCategory,
  RSSCollection,
  RSSSubscription,
} from "@/app/rss/types";
import {
  createRSSCategoryRouteID,
  createRSSCollectionRouteID,
  createRSSSubscriptionRouteId,
  isRSSSubscriptionContextMenuKey,
  RSS_WORKSPACE_ROUTE_IDS,
} from "@/app/rss/rss-routes";
import { cn } from "@/lib/utils";
import { useI18n } from "@/shared/i18n";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/shared/ui/dropdown-menu";
import {
  CheckCheck,
  ChevronDown,
  ChevronRight,
  Folder,
  ListFilter,
  Pencil,
  Rss,
  Trash2,
} from "lucide-react";
import { Button } from "@/shared/ui/button";
import * as React from "react";

export interface RSSWorkspaceSidebarCatalog
  extends WorkspaceSidebarRegionCatalog {
  unreadLabel: string;
  sections: {
    collections: WorkspaceSidebarSectionCatalog;
    discovery: WorkspaceSidebarSectionCatalog;
    subscriptions: WorkspaceSidebarSectionCatalog;
  };
  routes: Record<
    Exclude<
      keyof typeof RSS_WORKSPACE_ROUTE_IDS,
      "addSubscription" | "discoverySearch"
    >,
    WorkspaceSidebarItemCatalog
  >;
}

export type RSSWorkspaceCountedRoute =
  | "all"
  | "articles"
  | "social"
  | "images"
  | "videos"
  | "starred";

export type RSSWorkspaceCollectionUnreadCounts = Partial<
  Record<RSSWorkspaceCountedRoute, number>
>;

export interface RSSWorkspaceSidebarProps extends WideWorkspaceSidebarProps {
  catalog: RSSWorkspaceSidebarCatalog;
  /** Unread totals for collection routes. Missing and zero values stay hidden. */
  collectionUnreadCounts?: RSSWorkspaceCollectionUnreadCounts;
  subscriptions: readonly RSSSubscription[];
  categories?: readonly RSSCategory[];
  collections?: readonly RSSCollection[];
  markAllReadPending?: boolean;
  onEditSubscription?: (
    subscription: RSSSubscription,
    returnFocusTarget: HTMLButtonElement | null,
  ) => void;
  onMarkSubscriptionRead?: (subscription: RSSSubscription) => void;
  onUnsubscribe?: (
    subscription: RSSSubscription,
    returnFocusTarget: HTMLButtonElement | null,
  ) => void;
}

export function RSSWorkspaceSidebar({
  catalog,
  collectionUnreadCounts,
  subscriptions,
  categories = [],
  collections = [],
  controlPanel,
  className,
  activeRouteId,
  markAllReadPending = false,
  onNavigate,
  onEditSubscription,
  onMarkSubscriptionRead,
  onUnsubscribe,
  ...props
}: RSSWorkspaceSidebarProps) {
  const { t } = useI18n();
  const [collapsedFolderIds, setCollapsedFolderIds] = React.useState<Set<string>>(
    () => new Set(),
  );
  const [contextTarget, setContextTarget] = React.useState<{
    subscriptionId: string;
    x: number;
    y: number;
  } | null>(null);
  const returnFocusRef = React.useRef<HTMLButtonElement | null>(null);
  const contextSubscription = contextTarget
    ? subscriptions.find((item) => item.id === contextTarget.subscriptionId) ?? null
    : null;

  React.useEffect(() => {
    if (contextTarget && !contextSubscription) setContextTarget(null);
  }, [contextSubscription, contextTarget]);

  const openContextMenu = React.useCallback((
    subscription: RSSSubscription,
    point: { x: number; y: number },
    returnFocus: HTMLButtonElement,
  ) => {
    returnFocusRef.current = returnFocus;
    setContextTarget({ subscriptionId: subscription.id, ...point });
  }, []);

  const item = (
    route: keyof RSSWorkspaceSidebarCatalog["routes"],
  ): WorkspaceSidebarNavigationItem => {
    const catalogItem = catalog.routes[route];
    const unreadCount = isRSSWorkspaceCountedRoute(route)
      ? normalizeUnreadCount(collectionUnreadCounts?.[route])
      : 0;
    return {
      routeId: RSS_WORKSPACE_ROUTE_IDS[route],
      ...catalogItem,
      ariaLabel:
        unreadCount > 0
          ? `${catalogItem.ariaLabel ?? catalogItem.label}, ${unreadCount} ${catalog.unreadLabel}`
          : catalogItem.ariaLabel,
      badge: unreadCount > 0 ? formatUnreadCount(unreadCount) : catalogItem.badge,
    };
  };

  const subscriptionItems = subscriptions.map(
    (subscription): WorkspaceSidebarNavigationItem => {
      const iconURL = controlledRSSResourceURL(subscription.iconUrl);
      return {
        routeId: createRSSSubscriptionRouteId(subscription.id),
        label: subscription.title || subscription.feedUrl,
        tooltip: subscription.title || subscription.feedUrl,
        ariaLabel: subscription.unreadCount > 0
          ? `${subscription.title || subscription.feedUrl}, ${subscription.unreadCount} ${catalog.unreadLabel}`
          : subscription.title || subscription.feedUrl,
        icon: (
          <span className="app-rss-workspace-sidebar__favicon">
            <Rss className="app-rss-workspace-sidebar__favicon-fallback" />
            {iconURL ? (
              <img
                alt=""
                loading="lazy"
                onError={(event) => {
                  event.currentTarget.hidden = true;
                }}
                referrerPolicy="no-referrer"
                src={iconURL}
              />
            ) : null}
          </span>
        ),
        badge:
          subscription.unreadCount > 0
            ? formatUnreadCount(subscription.unreadCount)
            : undefined,
        onContextMenu: (event) => {
          event.preventDefault();
          event.stopPropagation();
          openContextMenu(
            subscription,
            { x: event.clientX, y: event.clientY },
            event.currentTarget,
          );
        },
        onKeyDown: (event) => {
          if (!isRSSSubscriptionContextMenuKey(event.key, event.shiftKey)) return;
          event.preventDefault();
          event.stopPropagation();
          const rect = event.currentTarget.getBoundingClientRect();
          openContextMenu(
            subscription,
            { x: rect.left + rect.width / 2, y: rect.bottom },
            event.currentTarget,
          );
        },
      };
    },
  );
  const subscriptionsByCategory = React.useMemo(() => {
    const grouped = new Map<string, RSSSubscription[]>();
    const knownCategoryIDs = new Set(categories.map((category) => category.id));
    for (const subscription of subscriptions) {
      const candidate = subscription.categoryId?.trim() || "";
      const categoryID = knownCategoryIDs.has(candidate) ? candidate : "";
      const members = grouped.get(categoryID) ?? [];
      members.push(subscription);
      grouped.set(categoryID, members);
    }
    return grouped;
  }, [categories, subscriptions]);
  const subscriptionItemsByID = new Map(
    subscriptions.map((subscription, index) => [
      subscription.id,
      subscriptionItems[index],
    ] as const),
  );
  const categorySections: WorkspaceSidebarSection[] = [...categories]
    .sort((left, right) => left.sortOrder - right.sortOrder ||
      left.title.localeCompare(right.title))
    .map((category) => {
      const collapsed = collapsedFolderIds.has(category.id);
      const folderActionLabel = `${t(
        collapsed ? "xiadown.rss.expandFolder" : "xiadown.rss.collapseFolder",
      )} · ${category.title}`;
      const categoryItems = (subscriptionsByCategory.get(category.id) ?? [])
        .map((subscription) => subscriptionItemsByID.get(subscription.id))
        .filter((entry): entry is WorkspaceSidebarNavigationItem => Boolean(entry));
      return {
        id: `subscriptions-${category.id}`,
        label: category.title,
        labelRouteId: createRSSCategoryRouteID(category.id),
        labelIcon: <Folder />,
        action: (
          <Button
            aria-label={folderActionLabel}
            onClick={() => {
              setCollapsedFolderIds((current) => {
                const next = new Set(current);
                if (collapsed) next.delete(category.id);
                else next.add(category.id);
                return next;
              });
            }}
            size="icon"
            title={folderActionLabel}
            type="button"
            variant="ghost"
          >
            {collapsed ? <ChevronRight /> : <ChevronDown />}
          </Button>
        ),
        items: collapsed ? [] : categoryItems,
      };
    });
  const uncategorizedItems = (subscriptionsByCategory.get("") ?? [])
    .map((subscription) => subscriptionItemsByID.get(subscription.id))
    .filter((entry): entry is WorkspaceSidebarNavigationItem => Boolean(entry));
  const collectionItems: WorkspaceSidebarNavigationItem[] = collections.map((collection) => ({
    routeId: createRSSCollectionRouteID(collection.id),
    label: collection.title,
    tooltip: collection.description || collection.title,
    icon: <ListFilter />,
    badge: collection.unreadCount > 0
      ? formatUnreadCount(collection.unreadCount)
      : undefined,
    ariaLabel: collection.unreadCount > 0
      ? `${collection.title}, ${collection.unreadCount} ${catalog.unreadLabel}`
      : collection.title,
  }));

  const sections: WorkspaceSidebarSection[] = [
    {
      id: "primary",
      items: [
        item("search"),
        item("all"),
        item("articles"),
        item("social"),
        item("images"),
        item("videos"),
        item("starred"),
      ],
    },
    {
      id: "discovery",
      label: catalog.sections.discovery.label,
      items: [item("manageSubscriptions"), item("discoveryBrowse")],
    },
    ...(collectionItems.length > 0
      ? [{
          id: "collections",
          label: catalog.sections.collections.label,
          items: collectionItems,
        }]
      : []),
    ...categorySections,
    {
      id: "subscriptions",
      label: categories.length > 0
        ? t("xiadown.rss.uncategorized")
        : catalog.sections.subscriptions.label,
      items: uncategorizedItems,
    },
  ];

  return (
    <>
      <WorkspaceSidebarNavigation
        {...props}
        activeRouteId={activeRouteId}
        ariaLabel={catalog.sidebarAriaLabel}
        className={cn("app-rss-workspace-sidebar", className)}
        controlPanel={controlPanel}
        onNavigate={onNavigate}
        sections={sections}
      />
      <DropdownMenu
        modal={false}
        open={Boolean(contextTarget)}
        onOpenChange={(open) => { if (!open) setContextTarget(null); }}
      >
        {contextTarget ? (
          <DropdownMenuTrigger asChild>
            <button
              aria-hidden="true"
              className="app-rss-subscription-context-menu__anchor"
              style={{ left: contextTarget.x, top: contextTarget.y }}
              tabIndex={-1}
              type="button"
            />
          </DropdownMenuTrigger>
        ) : null}
        <DropdownMenuContent
          align="start"
          aria-label={contextSubscription?.title || catalog.sidebarAriaLabel}
          className="app-rss-subscription-context-menu"
          onCloseAutoFocus={(event) => {
            event.preventDefault();
            const fallback = document.querySelector<HTMLButtonElement>(
              'button[data-route-id="all"]',
            );
            const target = returnFocusRef.current?.isConnected
              ? returnFocusRef.current
              : fallback;
            target?.focus();
          }}
          side="bottom"
          sideOffset={2}
        >
          <DropdownMenuItem
            disabled={!contextSubscription || !onMarkSubscriptionRead || markAllReadPending}
            onSelect={() => {
              const subscription = contextSubscription;
              setContextTarget(null);
              if (!subscription) return;
              onMarkSubscriptionRead?.(subscription);
            }}
          >
            <CheckCheck />
            {t("xiadown.rss.markAllRead")}
          </DropdownMenuItem>
          <DropdownMenuItem
            disabled={!contextSubscription || !onEditSubscription}
            onSelect={() => {
              const subscription = contextSubscription;
              setContextTarget(null);
              if (subscription) {
                onEditSubscription?.(subscription, returnFocusRef.current);
              }
            }}
          >
            <Pencil />
            {t("xiadown.rss.editSubscription")}
          </DropdownMenuItem>
          <DropdownMenuSeparator />
          <DropdownMenuItem
            className="app-menu-item--destructive"
            disabled={!contextSubscription || !onUnsubscribe}
            onSelect={() => {
              const subscription = contextSubscription;
              setContextTarget(null);
              if (subscription) {
                onUnsubscribe?.(subscription, returnFocusRef.current);
              }
            }}
          >
            <Trash2 />
            {t("xiadown.rss.unsubscribe")}
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
    </>
  );
}

function formatUnreadCount(count: number) {
  return count > 999 ? "999+" : String(count);
}

function normalizeUnreadCount(value: number | undefined) {
  return typeof value === "number" && Number.isFinite(value) && value > 0
    ? Math.floor(value)
    : 0;
}

function isRSSWorkspaceCountedRoute(
  route: keyof RSSWorkspaceSidebarCatalog["routes"],
): route is RSSWorkspaceCountedRoute {
  return (
    route === "all" ||
    route === "articles" ||
    route === "social" ||
    route === "images" ||
    route === "videos" ||
    route === "starred"
  );
}
