import {
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
  type HTMLAttributes,
  type ReactNode,
} from "react";

import type { CompanionDestination } from "@/app/workspace/types";
import type { CompanionPresentation } from "@/app/workspace/layout";
import { useWorkspaceSurfaceStyle } from "@/app/workspace/AppShell";
import { cn } from "@/lib/utils";
import { GlassSurface } from "@/shared/ui/glass-surface";
import { isWorkspacePageHeaderScrolled } from "@/shared/ui/workspace-page";

export type CompanionScrollChrome = "auto" | "off";

const useIsomorphicLayoutEffect =
  typeof window === "undefined" ? useEffect : useLayoutEffect;

const COMPANION_HEADER_INSET = 48;
const COMPANION_FOOTER_INSET = 60;

function stableCompanionContextValue(value: unknown): unknown {
  if (Array.isArray(value)) {
    return value.map(stableCompanionContextValue);
  }
  if (value && typeof value === "object") {
    return Object.fromEntries(
      Object.keys(value)
        .sort()
        .map((key) => [
          key,
          stableCompanionContextValue((value as Record<string, unknown>)[key]),
        ]),
    );
  }
  return value;
}

export function resolveCompanionScrollChromeKey(
  destination: CompanionDestination | null | undefined,
): string {
  if (!destination) {
    return "";
  }
  const scope =
    destination.scope.kind === "global"
      ? ["global"]
      : destination.scope.kind === "workspace"
        ? ["workspace", destination.scope.workspaceId]
        : [
            "route",
            destination.scope.workspaceId,
            destination.scope.routeId,
          ];
  return JSON.stringify([
    destination.id,
    scope,
    stableCompanionContextValue(destination.context ?? {}),
  ]);
}

export interface CompanionPanelProps
  extends Omit<HTMLAttributes<HTMLElement>, "title"> {
  open: boolean;
  destination?: CompanionDestination | null;
  keepMounted?: boolean;
  presentation?: CompanionPresentation;
  header?: ReactNode;
  title?: ReactNode;
  actions?: ReactNode;
  footer?: ReactNode;
  /**
   * Tracks only descendants that explicitly name the active destination via
   * `data-companion-scroll-owner`. Fullscreen/player hosts must opt out.
   */
  scrollChrome?: CompanionScrollChrome;
}

export function CompanionPanel({
  open,
  destination,
  keepMounted = true,
  presentation = "docked",
  header,
  title,
  actions,
  footer,
  scrollChrome = "auto",
  className,
  children,
  ...props
}: CompanionPanelProps) {
  const surfaceStyle = useWorkspaceSurfaceStyle();
  const panelRef = useRef<HTMLElement>(null);
  const headerRef = useRef<HTMLDivElement>(null);
  const footerRef = useRef<HTMLDivElement>(null);
  const hasFooter = footer != null;
  const destinationId = destination?.id ?? "";
  const scrollChromeKey = resolveCompanionScrollChromeKey(destination);
  const scrollChromeActive =
    scrollChrome === "auto" && open && destinationId.length > 0;
  const [scrollSnapshot, setScrollSnapshot] = useState({
    key: scrollChromeKey,
    scrolled: false,
  });
  const headerIsScrolled =
    scrollChromeActive &&
    scrollSnapshot.key === scrollChromeKey &&
    scrollSnapshot.scrolled;
  useIsomorphicLayoutEffect(() => {
    const panel = panelRef.current;
    if (!panel || !scrollChromeActive) {
      setScrollSnapshot({
        key: scrollChromeKey,
        scrolled: false,
      });
      return;
    }

    let observedOwner: HTMLElement | null = null;
    const readInset = (
      element: HTMLElement | null,
      fallback: number,
    ): number => {
      const measured = element?.getBoundingClientRect().height ?? 0;
      return Number.isFinite(measured) && measured > 0 ? measured : fallback;
    };
    const writeInset = (property: string, value: number) => {
      const next = `${Math.ceil(value)}px`;
      if (panel.style.getPropertyValue(property) !== next) {
        panel.style.setProperty(property, next);
      }
    };
    const update = (owner: HTMLElement | null) => {
      const scrollTop = owner?.scrollTop ?? 0;
      const headerInset = readInset(headerRef.current, COMPANION_HEADER_INSET);
      const footerInset = hasFooter
        ? readInset(footerRef.current, COMPANION_FOOTER_INSET)
        : 0;
      writeInset("--app-workspace-companion-header-inset", headerInset);
      writeInset("--app-workspace-companion-footer-inset", footerInset);

      const scrolled = isWorkspacePageHeaderScrolled(scrollTop);
      setScrollSnapshot((current) =>
        current.key === scrollChromeKey &&
        current.scrolled === scrolled
          ? current
          : { key: scrollChromeKey, scrolled },
      );
    };
    let resizeObserver: ResizeObserver;
    const syncActiveOwner = () => {
      const owner = Array.from(
        panel.querySelectorAll<HTMLElement>("[data-companion-scroll-owner]"),
      ).find(
        (candidate) =>
          candidate.dataset.companionScrollOwner === destinationId,
      );
      if (observedOwner !== owner) {
        if (observedOwner) {
          resizeObserver.unobserve(observedOwner);
        }
        observedOwner = owner ?? null;
        if (observedOwner) {
          resizeObserver.observe(observedOwner);
        }
      }
      update(owner ?? null);
    };
    resizeObserver = new ResizeObserver(syncActiveOwner);
    const handleScroll = (event: Event) => {
      const owner = event.target as HTMLElement;
      if (owner.dataset.companionScrollOwner === destinationId) {
        update(owner);
      }
    };

    resizeObserver.observe(panel);
    if (headerRef.current) {
      resizeObserver.observe(headerRef.current);
    }
    if (footerRef.current) {
      resizeObserver.observe(footerRef.current);
    }
    syncActiveOwner();
    panel.addEventListener("scroll", handleScroll, true);
    window.addEventListener("resize", syncActiveOwner);
    const ownerObserver = new MutationObserver(syncActiveOwner);
    ownerObserver.observe(panel, { childList: true, subtree: true });
    const frame = window.requestAnimationFrame(syncActiveOwner);
    return () => {
      window.cancelAnimationFrame(frame);
      ownerObserver.disconnect();
      resizeObserver.disconnect();
      panel.removeEventListener("scroll", handleScroll, true);
      window.removeEventListener("resize", syncActiveOwner);
    };
  }, [destinationId, hasFooter, scrollChromeActive, scrollChromeKey]);

  if (!keepMounted && !open) {
    return null;
  }

  const glassHost = presentation === "overlay" || surfaceStyle === "glass";

  const panelHeader =
    header ??
    (title || actions ? (
      <div className="app-workspace-companion__titlebar">
        <div className="app-workspace-companion__title">{title}</div>
        {actions ? (
          <div className="app-workspace-companion__actions">{actions}</div>
        ) : null}
      </div>
    ) : null);

  return (
    <aside
      {...props}
      ref={panelRef}
      aria-hidden={!open}
      className={cn("app-workspace-companion", className)}
      data-destination={destination?.id}
      data-glass-host={glassHost ? "true" : "false"}
      data-open={open}
      data-presentation={presentation}
      data-scroll-chrome={scrollChromeActive ? "active" : "off"}
      data-scroll-state={headerIsScrolled ? "scrolled" : "top"}
      data-surface-role={presentation === "overlay" ? "overlay" : "chrome"}
      data-surface-style={surfaceStyle}
      data-has-header={panelHeader ? "true" : "false"}
      data-has-footer={hasFooter ? "true" : "false"}
      hidden={!open}
    >
      {glassHost ? (
        <GlassSurface
          aria-hidden="true"
          className="app-workspace-chrome-material"
          data-glass-role="companion"
          elevation={presentation === "overlay" ? "floating" : "embedded"}
          shape="panel"
          surfaceRole={presentation === "overlay" ? "overlay" : "chrome"}
          tint="neutral"
        />
      ) : null}
      {panelHeader ? (
        <div
          ref={headerRef}
          className="app-workspace-companion__header wails-drag"
        >
          <GlassSurface
            aria-hidden="true"
            className="app-workspace-companion__header-material"
            data-glass-role="header"
            elevation="embedded"
            shape="panel"
            surfaceRole="chrome"
            tint="neutral"
          />
          {panelHeader}
        </div>
      ) : null}
      <div className="app-workspace-companion__content">{children}</div>
      {footer ? (
        <div ref={footerRef} className="app-workspace-companion__footer">
          {footer}
        </div>
      ) : null}
    </aside>
  );
}
