import * as React from "react";

import { cn } from "@/lib/utils";
import { GlassSurface } from "@/shared/ui/glass-surface";

const useIsomorphicLayoutEffect =
  typeof window === "undefined" ? React.useEffect : React.useLayoutEffect;

export type WorkspacePagePresentation =
  | "primary"
  | "companion"
  | "fullscreen"
  | "standalone-window";

export type WorkspacePageRecipe =
  | "browse"
  | "collection"
  | "feed"
  | "detail"
  | "search"
  | "operational"
  | "settings"
  | "custom";

export type WorkspacePageTopBarKind =
  | "drag"
  | "actions"
  | "search"
  | "navigation"
  | "none"
  | "host-owned"
  | "custom";

export type WorkspacePageHeadingKind =
  | "display"
  | "assistive"
  | "hero"
  | "host-owned";

export type WorkspacePageContentLayout =
  | "shelves"
  | "card-grid"
  | "list"
  | "feed"
  | "split"
  | "form"
  | "canvas"
  | "custom";

export type WorkspacePageFooterKind =
  | "none"
  | "pagination"
  | "commands"
  | "status"
  | "host-owned"
  | "overlay";

export type WorkspacePageScrollOwner =
  | "content"
  | "panes"
  | "host"
  | "none";

export type WorkspacePageDensity = "comfortable" | "regular" | "compact";

export type WorkspacePageImmersion =
  | "standard"
  | "edge-to-edge"
  | "auto-hiding-chrome";

export type WorkspacePageHeaderLayer = "layered" | "flow" | "absent";

export type WorkspacePageHeaderScrollState = "top" | "scrolled";

export type WorkspacePageFooterLayer = "layered" | "flow" | "absent";

export const WORKSPACE_PAGE_HEADER_SCROLL_THRESHOLD = 8;
export const WORKSPACE_PAGE_FOOTER_SCROLL_THRESHOLD = 8;

interface WorkspacePageContractBase {
  presentation: WorkspacePagePresentation;
  routeLabel: string;
  heading: WorkspacePageHeadingKind;
  footer: WorkspacePageFooterKind;
  scroll: WorkspacePageScrollOwner;
  density: WorkspacePageDensity;
  immersion: WorkspacePageImmersion;
}

type StandardWorkspacePageContract = WorkspacePageContractBase & {
  recipe: Exclude<WorkspacePageRecipe, "custom">;
  topBar: Exclude<WorkspacePageTopBarKind, "custom">;
  contentLayout: Exclude<WorkspacePageContentLayout, "custom">;
  customContractId?: never;
};

type CustomWorkspacePageContract = WorkspacePageContractBase &
  (
    | {
        recipe: "custom";
        topBar: WorkspacePageTopBarKind;
        contentLayout: WorkspacePageContentLayout;
      }
    | {
        recipe: Exclude<WorkspacePageRecipe, "custom">;
        topBar: "custom";
        contentLayout: WorkspacePageContentLayout;
      }
    | {
        recipe: Exclude<WorkspacePageRecipe, "custom">;
        topBar: Exclude<WorkspacePageTopBarKind, "custom">;
        contentLayout: "custom";
      }
  ) & {
    /** Names the reviewed exception so `custom` never becomes an anonymous escape hatch. */
    customContractId: string;
  };

/**
 * The complete, station-independent anatomy of a XiaDown page.
 *
 * Stations choose a recipe and provide content. This contract, the shared
 * primitives below, and Dream CSS own chrome, headings, scrolling, and footer
 * geometry. A custom recipe/TopBar/layout must include `customContractId`.
 */
export type WorkspacePageContract =
  | StandardWorkspacePageContract
  | CustomWorkspacePageContract;

/**
 * Only ordinary Primary pages with one shared content scroller participate in
 * scroll-aware chrome. Search, custom, split, canvas, host-scrolled, and
 * immersive presentations keep their reviewed page-specific geometry.
 */
export function resolveWorkspacePageHeaderLayer(
  contract: WorkspacePageContract,
): WorkspacePageHeaderLayer {
  if (contract.topBar === "none" || contract.topBar === "host-owned") {
    return "absent";
  }

  const usesSpecialGeometry =
    contract.presentation !== "primary" ||
    contract.recipe === "custom" ||
    contract.recipe === "search" ||
    contract.topBar === "custom" ||
    contract.topBar === "search" ||
    contract.contentLayout === "custom" ||
    contract.contentLayout === "split" ||
    contract.contentLayout === "canvas";

  return !usesSpecialGeometry &&
    contract.scroll === "content" &&
    contract.immersion === "standard"
    ? "layered"
    : "flow";
}

/**
 * A true Primary page Footer is persistent chrome, independent from whether
 * the route uses the standard or special Search Header. Only pages with the
 * shared Content scroll owner can safely pass content beneath this layer.
 * Split, canvas, host-owned, custom, and explicitly overlaid footers retain
 * their reviewed geometry.
 */
export function resolveWorkspacePageFooterLayer(
  contract: WorkspacePageContract,
): WorkspacePageFooterLayer {
  if (contract.footer === "none" || contract.footer === "host-owned") {
    return "absent";
  }

  const usesSpecialGeometry =
    contract.presentation !== "primary" ||
    contract.recipe === "custom" ||
    contract.topBar === "custom" ||
    contract.contentLayout === "custom" ||
    contract.contentLayout === "split" ||
    contract.contentLayout === "canvas" ||
    contract.scroll !== "content" ||
    contract.immersion !== "standard" ||
    contract.footer === "overlay";

  return usesSpecialGeometry ? "flow" : "layered";
}

export function isWorkspacePageHeaderScrolled(scrollTop: number): boolean {
  return (
    Number.isFinite(scrollTop) &&
    scrollTop > WORKSPACE_PAGE_HEADER_SCROLL_THRESHOLD
  );
}

export function isWorkspacePageFooterCovered(
  scrollTop: number,
  scrollHeight: number,
  clientHeight: number,
  endInset = 0,
): boolean {
  if (
    !Number.isFinite(scrollTop) ||
    !Number.isFinite(scrollHeight) ||
    !Number.isFinite(clientHeight) ||
    !Number.isFinite(endInset)
  ) {
    return false;
  }

  return (
    Math.max(
      0,
      scrollHeight - clientHeight - scrollTop - Math.max(0, endInset),
    ) >
    WORKSPACE_PAGE_FOOTER_SCROLL_THRESHOLD
  );
}

function readWorkspacePageFooterEndInset(owner: HTMLElement): number {
  const style = window.getComputedStyle(owner);
  const inset = Number.parseFloat(style.paddingBlockEnd || style.paddingBottom);
  return Number.isFinite(inset) ? Math.max(0, inset) : 0;
}

export function assertWorkspacePageContract(
  contract: WorkspacePageContract,
): void {
  if (!contract.routeLabel.trim()) {
    throw new Error("WorkspacePage contract requires a non-empty routeLabel.");
  }

  const usesCustomContract =
    contract.recipe === "custom" ||
    contract.topBar === "custom" ||
    contract.contentLayout === "custom";

  if (usesCustomContract && !contract.customContractId?.trim()) {
    throw new Error(
      "A custom WorkspacePage recipe, TopBar, or content layout requires customContractId.",
    );
  }
}

/** Preserves literal values while validating a reusable page contract. */
export function defineWorkspacePageContract<
  const Contract extends WorkspacePageContract,
>(contract: Contract): Contract {
  assertWorkspacePageContract(contract);
  return contract;
}

interface WorkspacePageContextValue {
  contract: WorkspacePageContract;
  headerLayer: WorkspacePageHeaderLayer;
  footerLayer: WorkspacePageFooterLayer;
  headingId: string;
  setContentScrollOwner: (node: HTMLDivElement | null) => void;
}

const WorkspacePageContext = React.createContext<
  WorkspacePageContextValue | undefined
>(undefined);

function useWorkspacePageContext(componentName: string) {
  const context = React.useContext(WorkspacePageContext);
  if (!context) {
    throw new Error(`${componentName} must be rendered inside WorkspacePage.`);
  }
  return context;
}

export interface WorkspacePageProps
  extends Omit<React.HTMLAttributes<HTMLElement>, "title"> {
  as?: "section" | "div";
  contract: WorkspacePageContract;
}

/**
 * Root page region. It publishes the contract as stable data attributes and
 * supplies the accessible-name relationship used by WorkspacePageContent.
 */
export const WorkspacePage = React.forwardRef<
  HTMLElement,
  WorkspacePageProps
>(function WorkspacePage(
  {
    as = "section",
    contract,
    className,
    children,
    "aria-label": ariaLabel,
    "aria-labelledby": ariaLabelledBy,
    onScrollCapture,
    ...props
  },
  ref,
) {
  assertWorkspacePageContract(contract);
  const generatedHeadingId = React.useId();
  const headingId = `workspace-page-heading-${generatedHeadingId.replace(/:/g, "")}`;
  const Component = as as React.ElementType;
  const headingIsHostOwned = contract.heading === "host-owned";
  const headerLayer = resolveWorkspacePageHeaderLayer(contract);
  const footerLayer = resolveWorkspacePageFooterLayer(contract);
  const contentScrollOwnerRef = React.useRef<HTMLDivElement | null>(null);
  const [contentScrollOwner, setContentScrollOwnerNode] =
    React.useState<HTMLDivElement | null>(null);
  const setContentScrollOwner = React.useCallback(
    (node: HTMLDivElement | null) => {
      contentScrollOwnerRef.current = node;
      setContentScrollOwnerNode((current) =>
        current === node ? current : node,
      );
    },
    [],
  );
  const headerStateKey = [
    contract.presentation,
    contract.routeLabel,
    contract.recipe,
    contract.topBar,
    contract.contentLayout,
    contract.scroll,
    contract.immersion,
  ].join("\u001f");
  const footerStateKey = `${headerStateKey}\u001f${contract.footer}`;
  const [headerScrollState, setHeaderScrollState] = React.useState({
    key: headerStateKey,
    scrolled: false,
  });
  const [footerScrollState, setFooterScrollState] = React.useState({
    key: footerStateKey,
    covered: false,
  });
  const headerIsScrolled =
    headerLayer === "layered" &&
    headerScrollState.key === headerStateKey &&
    headerScrollState.scrolled;
  const footerIsCovered =
    footerLayer === "layered" &&
    footerScrollState.key === footerStateKey &&
    footerScrollState.covered;

  useIsomorphicLayoutEffect(() => {
    if (headerLayer !== "layered" && footerLayer !== "layered") {
      return;
    }

    const sync = () => {
      const owner = contentScrollOwnerRef.current;
      if (headerLayer === "layered") {
        const scrolled = isWorkspacePageHeaderScrolled(owner?.scrollTop ?? 0);
        setHeaderScrollState((current) =>
          current.key === headerStateKey && current.scrolled === scrolled
            ? current
            : { key: headerStateKey, scrolled },
        );
      }
      if (footerLayer === "layered") {
        const covered = isWorkspacePageFooterCovered(
          owner?.scrollTop ?? 0,
          owner?.scrollHeight ?? 0,
          owner?.clientHeight ?? 0,
          owner ? readWorkspacePageFooterEndInset(owner) : 0,
        );
        setFooterScrollState((current) =>
          current.key === footerStateKey && current.covered === covered
            ? current
            : { key: footerStateKey, covered },
        );
      }
    };

    sync();
    const frame = window.requestAnimationFrame(sync);
    const owner = contentScrollOwnerRef.current;
    const resizeObserver =
      footerLayer === "layered" &&
      owner &&
      typeof ResizeObserver !== "undefined"
        ? new ResizeObserver(sync)
        : null;
    const mutationObserver =
      footerLayer === "layered" &&
      owner &&
      typeof MutationObserver !== "undefined"
        ? new MutationObserver(sync)
        : null;
    if (owner) {
      resizeObserver?.observe(owner);
      mutationObserver?.observe(owner, {
        childList: true,
        characterData: true,
        subtree: true,
      });
    }
    if (footerLayer === "layered") {
      window.addEventListener("resize", sync);
    }

    return () => {
      window.cancelAnimationFrame(frame);
      resizeObserver?.disconnect();
      mutationObserver?.disconnect();
      if (footerLayer === "layered") {
        window.removeEventListener("resize", sync);
      }
    };
  }, [
    contentScrollOwner,
    footerLayer,
    footerStateKey,
    headerLayer,
    headerStateKey,
  ]);

  const handleScrollCapture = React.useCallback(
    (event: React.UIEvent<HTMLElement>) => {
      onScrollCapture?.(event);

      if (headerLayer !== "layered" && footerLayer !== "layered") {
        return;
      }

      const target = event.target as HTMLElement;
      if (
        target.parentElement !== event.currentTarget ||
        target.dataset.scrollOwner !== "true" ||
        !target.classList.contains("app-workspace-page__content")
      ) {
        return;
      }

      if (headerLayer === "layered") {
        const scrolled = isWorkspacePageHeaderScrolled(target.scrollTop);
        setHeaderScrollState((current) =>
          current.key === headerStateKey && current.scrolled === scrolled
            ? current
            : { key: headerStateKey, scrolled },
        );
      }
      if (footerLayer === "layered") {
        const covered = isWorkspacePageFooterCovered(
          target.scrollTop,
          target.scrollHeight,
          target.clientHeight,
          readWorkspacePageFooterEndInset(target),
        );
        setFooterScrollState((current) =>
          current.key === footerStateKey && current.covered === covered
            ? current
            : { key: footerStateKey, covered },
        );
      }
    },
    [
      footerLayer,
      footerStateKey,
      headerLayer,
      headerStateKey,
      onScrollCapture,
    ],
  );

  return (
    <WorkspacePageContext.Provider
      value={{
        contract,
        headerLayer,
        footerLayer,
        headingId,
        setContentScrollOwner,
      }}
    >
      <Component
        {...props}
        ref={ref}
        aria-label={
          headingIsHostOwned ? (ariaLabel ?? contract.routeLabel) : ariaLabel
        }
        aria-labelledby={
          headingIsHostOwned
            ? ariaLabelledBy
            : (ariaLabelledBy ?? headingId)
        }
        className={cn("app-workspace-page", className)}
        data-page-content-layout={contract.contentLayout}
        data-page-density={contract.density}
        data-page-footer={contract.footer}
        data-page-footer-layer={footerLayer}
        data-page-footer-state={
          footerLayer === "layered"
            ? footerIsCovered
              ? "content"
              : "end"
            : undefined
        }
        data-page-header-layer={headerLayer}
        data-page-header-state={headerIsScrolled ? "scrolled" : "top"}
        data-page-heading={contract.heading}
        data-page-immersion={contract.immersion}
        data-page-presentation={contract.presentation}
        data-page-recipe={contract.recipe}
        data-page-scroll={contract.scroll}
        data-page-topbar={contract.topBar}
        data-page-custom-contract={contract.customContractId}
        onScrollCapture={handleScrollCapture}
      >
        {children}
      </Component>
    </WorkspacePageContext.Provider>
  );
});

WorkspacePage.displayName = "WorkspacePage";

export interface WorkspacePageTopBarProps
  extends React.HTMLAttributes<HTMLElement> {
  /** True only while Primary owns the native Windows caption controls. */
  reserveWindowControls?: boolean;
  /** Accessible name for the action group; defaults to the route label. */
  actionsLabel?: string;
  /**
   * Pane- and host-scrolled pages may publish their real scroll state so the
   * canonical Title material behaves like a standard Content-scrolled page.
   */
  scrollMaterialState?: WorkspacePageHeaderScrollState;
}

export function WorkspacePageTopBarMaterial({
  scrollState,
}: {
  scrollState?: WorkspacePageHeaderScrollState;
}) {
  return (
    <GlassSurface
      aria-hidden="true"
      className="app-workspace-page__topbar-material"
      data-glass-role="header"
      data-page-scroll-material-state={scrollState}
      elevation="embedded"
      shape="panel"
      surfaceRole="chrome"
      tint="neutral"
    />
  );
}

/**
 * Canonical fixed page rail. Product actions are automatically excluded from
 * native dragging and the trailing native-caption safe area is always present.
 */
export const WorkspacePageTopBar = React.forwardRef<
  HTMLElement,
  WorkspacePageTopBarProps
>(function WorkspacePageTopBar(
  {
    actionsLabel,
    reserveWindowControls = false,
    scrollMaterialState,
    className,
    children,
    ...props
  },
  ref,
) {
  const { contract, headerLayer } = useWorkspacePageContext(
    "WorkspacePageTopBar",
  );

  if (contract.topBar === "none" || contract.topBar === "host-owned") {
    return null;
  }

  return (
    <header
      {...props}
      ref={ref}
      className={cn(
        "app-workspace-page__topbar app-workspace-primary-header wails-drag",
        className,
      )}
      data-page-scroll-material-state={scrollMaterialState}
      data-page-topbar={contract.topBar}
      data-window-controls={reserveWindowControls || undefined}
    >
      {headerLayer === "layered" || scrollMaterialState !== undefined ? (
        <WorkspacePageTopBarMaterial scrollState={scrollMaterialState} />
      ) : null}
      {children != null ? (
        <div
          aria-label={actionsLabel ?? contract.routeLabel}
          className="app-workspace-page__topbar-actions app-workspace-primary-header__actions wails-no-drag"
          role="group"
        >
          {children}
        </div>
      ) : null}
      <span
        aria-hidden="true"
        className="app-workspace-page__topbar-drag-region wails-drag"
      />
      <span
        aria-hidden="true"
        className="app-workspace-page__topbar-safe-area app-workspace-primary-header__safe-area"
      />
    </header>
  );
});

WorkspacePageTopBar.displayName = "WorkspacePageTopBar";

export interface WorkspacePageHeadingProps
  extends Omit<React.HTMLAttributes<HTMLDivElement>, "children"> {
  actions?: React.ReactNode;
  description?: React.ReactNode;
  descriptionClassName?: string;
  titleClassName?: string;
}

/**
 * Contract-owned route heading. `WorkspacePageContent` renders it once, so a
 * page switches between visible and assistive headings without duplicating h1.
 */
export function WorkspacePageHeading({
  actions,
  className,
  description,
  descriptionClassName,
  titleClassName,
  ...props
}: WorkspacePageHeadingProps) {
  const { contract, headingId } = useWorkspacePageContext(
    "WorkspacePageHeading",
  );

  if (contract.heading === "host-owned") {
    return null;
  }

  if (contract.heading === "assistive") {
    return (
      <h1 id={headingId} className={cn("app-visually-hidden", titleClassName)}>
        {contract.routeLabel}
      </h1>
    );
  }

  return (
    <div
      {...props}
      className={cn("app-workspace-page__heading", className)}
      data-page-heading={contract.heading}
    >
      <div className="app-workspace-page__heading-text">
        <h1
          id={headingId}
          className={cn("app-workspace-page__heading-title", titleClassName)}
        >
          {contract.routeLabel}
        </h1>
        {description != null ? (
          <div
            className={cn(
              "app-workspace-page__heading-description",
              descriptionClassName,
            )}
          >
            {description}
          </div>
        ) : null}
      </div>
      {actions != null ? (
        <div className="app-workspace-page__heading-actions">
          {actions}
        </div>
      ) : null}
    </div>
  );
}

export interface WorkspacePageContentProps
  extends React.HTMLAttributes<HTMLDivElement> {
  headingActions?: React.ReactNode;
  headingClassName?: string;
  headingDescription?: React.ReactNode;
  headingDescriptionClassName?: string;
  headingTitleClassName?: string;
}

/** Main route content and, when applicable, its sole h1. */
export const WorkspacePageContent = React.forwardRef<
  HTMLDivElement,
  WorkspacePageContentProps
>(function WorkspacePageContent(
  {
    className,
    children,
    headingActions,
    headingClassName,
    headingDescription,
    headingDescriptionClassName,
    headingTitleClassName,
    ...props
  },
  ref,
) {
  const { contract, setContentScrollOwner } = useWorkspacePageContext(
    "WorkspacePageContent",
  );
  const setContentRef = React.useCallback(
    (node: HTMLDivElement | null) => {
      if (typeof ref === "function") {
        ref(node);
      } else if (ref) {
        ref.current = node;
      }
      setContentScrollOwner(contract.scroll === "content" ? node : null);
    },
    [contract.scroll, ref, setContentScrollOwner],
  );
  return (
    <div
      {...props}
      ref={setContentRef}
      className={cn("app-workspace-page__content", className)}
      data-page-content-layout={contract.contentLayout}
      data-page-scroll={contract.scroll}
      data-scroll-owner={contract.scroll === "content" ? "true" : undefined}
    >
      <WorkspacePageHeading
        actions={headingActions}
        className={headingClassName}
        description={headingDescription}
        descriptionClassName={headingDescriptionClassName}
        titleClassName={headingTitleClassName}
      />
      {children}
    </div>
  );
});

WorkspacePageContent.displayName = "WorkspacePageContent";

export interface WorkspacePageFooterProps
  extends React.HTMLAttributes<HTMLElement> {}

/** A true page footer (pagination, commands, status, or an overlay). */
export const WorkspacePageFooter = React.forwardRef<
  HTMLElement,
  WorkspacePageFooterProps
>(function WorkspacePageFooter({ className, children, ...props }, ref) {
  const { contract, footerLayer } = useWorkspacePageContext(
    "WorkspacePageFooter",
  );

  if (contract.footer === "none" || contract.footer === "host-owned") {
    return null;
  }

  return (
    <footer
      {...props}
      ref={ref}
      className={cn("app-workspace-page__footer", className)}
      data-page-footer={contract.footer}
    >
      {footerLayer === "layered" ? (
        <GlassSurface
          aria-hidden="true"
          className="app-workspace-page__footer-material"
          data-glass-role="footer"
          elevation="embedded"
          shape="panel"
          surfaceRole="chrome"
          tint="neutral"
        />
      ) : null}
      {children}
    </footer>
  );
});

WorkspacePageFooter.displayName = "WorkspacePageFooter";
