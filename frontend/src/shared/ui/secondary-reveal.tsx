import * as React from "react";
import { createPortal } from "react-dom";

import { cn } from "@/lib/utils";
import {
  GlassSurface,
  type GlassTint,
} from "@/shared/ui/glass-surface";

export const SECONDARY_REVEAL_OPEN_DELAY = 300;
export const SECONDARY_REVEAL_CLOSE_DELAY = 180;

type SecondaryRevealPosition = {
  left: number;
  side: "left" | "right";
  top: number;
};

export type SecondaryRevealAnchorProps = React.HTMLAttributes<HTMLDivElement> & {
  ref: React.RefCallback<HTMLDivElement>;
  "data-secondary-reveal-open"?: "true";
  "data-secondary-reveal-pinned"?: "true";
};

export type SecondaryRevealTriggerProps =
  React.ButtonHTMLAttributes<HTMLButtonElement> & {
    ref: React.RefCallback<HTMLButtonElement>;
  };

export type SecondaryRevealRenderState = {
  anchorProps: SecondaryRevealAnchorProps;
  close: () => void;
  open: boolean;
  pinned: boolean;
  triggerProps: SecondaryRevealTriggerProps;
};

export interface SecondaryRevealProps {
  ariaLabel: string;
  children: (state: SecondaryRevealRenderState) => React.ReactElement;
  className?: string;
  closeDelay?: number;
  content:
    | React.ReactNode
    | ((state: Pick<SecondaryRevealRenderState, "close" | "open" | "pinned">) => React.ReactNode);
  /**
   * When supplied, the trigger keeps its normal one-click action instead of
   * pinning the preview. Hover and focus can still disclose the preview.
   */
  onActivate?: () => void;
  openDelay?: number;
  /**
   * Defaults to the touch-friendly click-to-pin behavior. Set to false for
   * pointer-adjacent inspectors that must follow hover and disappear when the
   * pointer leaves; keyboard focus still opens the disclosure.
   */
  pinOnClick?: boolean;
  sideOffset?: number;
  tint?: GlassTint;
}

function containsEventTarget(
  node: HTMLElement | null,
  target: EventTarget | null,
) {
  return target instanceof Node && Boolean(node?.contains(target));
}

const useIsomorphicLayoutEffect =
  typeof window === "undefined" ? React.useEffect : React.useLayoutEffect;

/**
 * A non-modal, secondary disclosure for compact controls. Hover is deliberately
 * delayed, while focus and click remain deterministic keyboard/touch paths.
 */
export function SecondaryReveal({
  ariaLabel,
  children,
  className,
  closeDelay = SECONDARY_REVEAL_CLOSE_DELAY,
  content,
  onActivate,
  openDelay = SECONDARY_REVEAL_OPEN_DELAY,
  pinOnClick = true,
  sideOffset = 10,
  tint = "neutral",
}: SecondaryRevealProps) {
  const contentId = React.useId();
  const anchorRef = React.useRef<HTMLDivElement | null>(null);
  const triggerRef = React.useRef<HTMLButtonElement | null>(null);
  const positionerRef = React.useRef<HTMLDivElement | null>(null);
  const panelRef = React.useRef<HTMLDivElement | null>(null);
  const openTimerRef = React.useRef<ReturnType<typeof setTimeout> | null>(null);
  const closeTimerRef = React.useRef<ReturnType<typeof setTimeout> | null>(null);
  const anchorHoveredRef = React.useRef(false);
  const panelHoveredRef = React.useRef(false);
  const focusWithinRef = React.useRef(false);
  const pinnedRef = React.useRef(false);
  const suppressNextFocusOpenRef = React.useRef(false);
  const [open, setOpen] = React.useState(false);
  const [pinned, setPinned] = React.useState(false);
  const [position, setPosition] = React.useState<SecondaryRevealPosition | null>(
    null,
  );

  const clearOpenTimer = React.useCallback(() => {
    if (openTimerRef.current !== null) {
      clearTimeout(openTimerRef.current);
      openTimerRef.current = null;
    }
  }, []);

  const clearCloseTimer = React.useCallback(() => {
    if (closeTimerRef.current !== null) {
      clearTimeout(closeTimerRef.current);
      closeTimerRef.current = null;
    }
  }, []);

  const setPinnedState = React.useCallback((next: boolean) => {
    pinnedRef.current = next;
    setPinned(next);
  }, []);

  const closeReveal = React.useCallback(
    (restoreFocus = false) => {
      clearOpenTimer();
      clearCloseTimer();
      setPinnedState(false);
      panelHoveredRef.current = false;
      focusWithinRef.current = false;
      setOpen(false);
      setPosition(null);
      if (restoreFocus) {
        suppressNextFocusOpenRef.current = true;
        triggerRef.current?.focus({ preventScroll: true });
        window.requestAnimationFrame(() => {
          suppressNextFocusOpenRef.current = false;
        });
      }
    },
    [clearCloseTimer, clearOpenTimer, setPinnedState],
  );

  const scheduleClose = React.useCallback(() => {
    clearOpenTimer();
    clearCloseTimer();
    if (pinnedRef.current) {
      return;
    }
    closeTimerRef.current = setTimeout(() => {
      closeTimerRef.current = null;
      if (
        !pinnedRef.current &&
        !anchorHoveredRef.current &&
        !panelHoveredRef.current &&
        !focusWithinRef.current
      ) {
        setOpen(false);
        setPosition(null);
      }
    }, closeDelay);
  }, [clearCloseTimer, clearOpenTimer, closeDelay]);

  const openImmediately = React.useCallback(() => {
    clearOpenTimer();
    clearCloseTimer();
    setOpen(true);
  }, [clearCloseTimer, clearOpenTimer]);

  const scheduleOpen = React.useCallback(() => {
    clearCloseTimer();
    if (open || openTimerRef.current !== null) {
      return;
    }
    openTimerRef.current = setTimeout(() => {
      openTimerRef.current = null;
      setOpen(true);
    }, openDelay);
  }, [clearCloseTimer, open, openDelay]);

  const updatePosition = React.useCallback(() => {
    const anchor = anchorRef.current;
    const positioner = positionerRef.current;
    if (!anchor || !positioner || typeof window === "undefined") {
      return;
    }

    const anchorRect = anchor.getBoundingClientRect();
    const panelWidth = positioner.offsetWidth;
    const panelHeight = positioner.offsetHeight;
    const viewportInset = 8;
    const roomOnRight = window.innerWidth - anchorRect.right;
    const roomOnLeft = anchorRect.left;
    const side =
      roomOnRight >= panelWidth + viewportInset || roomOnRight >= roomOnLeft
        ? "right"
        : "left";
    const preferredLeft =
      side === "right"
        ? anchorRect.right
        : anchorRect.left - panelWidth;
    const preferredTop =
      anchorRect.top + anchorRect.height / 2 - panelHeight / 2;

    setPosition({
      side,
      left: Math.max(
        viewportInset,
        Math.min(preferredLeft, window.innerWidth - panelWidth - viewportInset),
      ),
      top: Math.max(
        viewportInset,
        Math.min(preferredTop, window.innerHeight - panelHeight - viewportInset),
      ),
    });
  }, []);

  useIsomorphicLayoutEffect(() => {
    if (!open || typeof window === "undefined") {
      return;
    }
    updatePosition();
    window.addEventListener("resize", updatePosition);
    window.addEventListener("scroll", updatePosition, true);
    return () => {
      window.removeEventListener("resize", updatePosition);
      window.removeEventListener("scroll", updatePosition, true);
    };
  }, [open, updatePosition]);

  React.useEffect(() => {
    if (!open || typeof document === "undefined") {
      return;
    }

    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        event.preventDefault();
        closeReveal(true);
      }
    };
    const handlePointerDown = (event: PointerEvent) => {
      if (
        !containsEventTarget(anchorRef.current, event.target) &&
        !containsEventTarget(positionerRef.current, event.target)
      ) {
        closeReveal(false);
      }
    };
    // Some WKWebView/CDP mouse paths only dispatch `mousemove` when the
    // pointer is repositioned. Keep the hover state in sync from the actual
    // event target so those paths still dismiss a hover-only reveal.
    const handleMouseMove = (event: MouseEvent) => {
      const overAnchor = containsEventTarget(anchorRef.current, event.target);
      const overPanel = containsEventTarget(positionerRef.current, event.target);

      if (overAnchor || overPanel) {
        anchorHoveredRef.current = overAnchor;
        panelHoveredRef.current = overPanel;
        clearCloseTimer();
        return;
      }

      const leftReveal =
        anchorHoveredRef.current || panelHoveredRef.current;
      anchorHoveredRef.current = false;
      panelHoveredRef.current = false;
      if (leftReveal) {
        scheduleClose();
      }
    };
    document.addEventListener("keydown", handleKeyDown, true);
    document.addEventListener("pointerdown", handlePointerDown, true);
    document.addEventListener("mousemove", handleMouseMove, true);
    return () => {
      document.removeEventListener("keydown", handleKeyDown, true);
      document.removeEventListener("pointerdown", handlePointerDown, true);
      document.removeEventListener("mousemove", handleMouseMove, true);
    };
  }, [clearCloseTimer, closeReveal, open, scheduleClose]);

  React.useEffect(
    () => () => {
      clearOpenTimer();
      clearCloseTimer();
    },
    [clearCloseTimer, clearOpenTimer],
  );

  const anchorProps: SecondaryRevealAnchorProps = {
    ref: (node) => {
      anchorRef.current = node;
    },
    "data-secondary-reveal-open": open ? "true" : undefined,
    "data-secondary-reveal-pinned": pinned ? "true" : undefined,
    onPointerEnter: (event) => {
      if (event.pointerType === "touch") {
        return;
      }
      anchorHoveredRef.current = true;
      scheduleOpen();
    },
    onPointerLeave: (event) => {
      if (event.pointerType === "touch") {
        return;
      }
      anchorHoveredRef.current = false;
      scheduleClose();
    },
    // WKWebView can omit pointerType/capability media-query information even
    // for a desktop mouse. Mouse events provide a reliable fallback.
    onMouseEnter: () => {
      anchorHoveredRef.current = true;
      scheduleOpen();
    },
    onMouseLeave: () => {
      anchorHoveredRef.current = false;
      scheduleClose();
    },
    // `mouseenter` is not emitted by every WKWebView mouse-injection path.
    // `mouseover` covers a real boundary crossing, while `mousemove` covers
    // pointer repositioning that only produces motion events. The containment
    // guard prevents transitions between descendants from toggling the reveal.
    onMouseOver: (event) => {
      if (containsEventTarget(event.currentTarget, event.relatedTarget)) {
        return;
      }
      anchorHoveredRef.current = true;
      scheduleOpen();
    },
    onMouseOut: (event) => {
      if (containsEventTarget(event.currentTarget, event.relatedTarget)) {
        return;
      }
      anchorHoveredRef.current = false;
      scheduleClose();
    },
    onMouseMove: () => {
      anchorHoveredRef.current = true;
      scheduleOpen();
    },
    onFocusCapture: (event) => {
      if (!containsEventTarget(triggerRef.current, event.target)) {
        return;
      }
      if (suppressNextFocusOpenRef.current) {
        suppressNextFocusOpenRef.current = false;
        return;
      }
      focusWithinRef.current = true;
      openImmediately();
    },
    onBlurCapture: () => {
      requestAnimationFrame(() => {
        const activeElement = document.activeElement;
        focusWithinRef.current =
          containsEventTarget(triggerRef.current, activeElement) ||
          containsEventTarget(panelRef.current, activeElement);
        if (!focusWithinRef.current) {
          scheduleClose();
        }
      });
    },
  };

  const triggerProps: SecondaryRevealTriggerProps = {
    ref: (node) => {
      triggerRef.current = node;
    },
    "aria-controls": contentId,
    "aria-expanded": open,
    "aria-haspopup": "dialog",
    onMouseDown: (event) => {
      // A pointer click normally moves focus to the trigger. In hover-only
      // mode that focus would keep the panel alive after pointer leave and
      // make it look pinned. Keyboard focus remains unaffected.
      if (!pinOnClick && event.detail > 0) {
        event.preventDefault();
      }
    },
    onClick: () => {
      if (onActivate) {
        closeReveal(false);
        onActivate();
        return;
      }
      if (!pinOnClick) {
        setPinnedState(false);
        openImmediately();
        return;
      }
      if (pinnedRef.current) {
        setPinnedState(false);
        scheduleClose();
        return;
      }
      setPinnedState(true);
      openImmediately();
    },
  };

  const renderState: SecondaryRevealRenderState = {
    anchorProps,
    close: () => closeReveal(true),
    open,
    pinned,
    triggerProps,
  };

  const portal =
    open && typeof document !== "undefined"
      ? createPortal(
          <div
            ref={positionerRef}
            className="app-secondary-reveal__positioner"
            data-positioned={position ? "true" : "false"}
            data-side={position?.side ?? "right"}
            onPointerEnter={(event) => {
              if (event.pointerType === "touch") {
                return;
              }
              panelHoveredRef.current = true;
              clearCloseTimer();
            }}
            onPointerLeave={(event) => {
              if (event.pointerType === "touch") {
                return;
              }
              panelHoveredRef.current = false;
              scheduleClose();
            }}
            onMouseEnter={() => {
              panelHoveredRef.current = true;
              clearCloseTimer();
            }}
            onMouseLeave={() => {
              panelHoveredRef.current = false;
              scheduleClose();
            }}
            onMouseOver={(event) => {
              if (containsEventTarget(event.currentTarget, event.relatedTarget)) {
                return;
              }
              panelHoveredRef.current = true;
              clearCloseTimer();
            }}
            onMouseOut={(event) => {
              if (containsEventTarget(event.currentTarget, event.relatedTarget)) {
                return;
              }
              panelHoveredRef.current = false;
              scheduleClose();
            }}
            onMouseMove={() => {
              panelHoveredRef.current = true;
              clearCloseTimer();
            }}
            onFocusCapture={() => {
              focusWithinRef.current = true;
              clearCloseTimer();
            }}
            onBlurCapture={() => {
              requestAnimationFrame(() => {
                const activeElement = document.activeElement;
                focusWithinRef.current =
                  containsEventTarget(anchorRef.current, activeElement) ||
                  containsEventTarget(panelRef.current, activeElement);
                if (!focusWithinRef.current) {
                  scheduleClose();
                }
              });
            }}
            style={{
              "--app-secondary-reveal-side-offset": `${sideOffset}px`,
              left: position?.left ?? 0,
              top: position?.top ?? 0,
            } as React.CSSProperties}
          >
            <GlassSurface
              ref={panelRef}
              id={contentId}
              role="dialog"
              aria-label={ariaLabel}
              aria-modal={false}
              className={cn(
                "app-secondary-reveal__panel app-motion-surface",
                className,
              )}
              elevation="floating"
              shape="panel"
              surfaceRole="overlay"
              tint={tint}
            >
              {typeof content === "function"
                ? content({ close: renderState.close, open, pinned })
                : content}
            </GlassSurface>
          </div>,
          document.body,
        )
      : null;

  return (
    <>
      {children(renderState)}
      {portal}
    </>
  );
}
