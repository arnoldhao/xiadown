import * as React from "react";

import { cn } from "@/lib/utils";

export const LISTEN_INFINITE_SCROLL_ROOT_MARGIN = "0px 0px 320px 0px";

type InfiniteScrollReadiness = {
  enabled: boolean;
  loading: boolean;
  continuation: string;
};

export type ListenInfiniteScrollGate = {
  setVisible: (visible: boolean) => void;
  tryAcquire: (readiness: InfiniteScrollReadiness) => boolean;
};

/**
 * Keeps observer delivery details out of pagination state. A continuation can
 * only be acquired once; a new continuation may load immediately while the
 * sentinel remains visible so a short first page can fill the scroll viewport.
 */
export function createListenInfiniteScrollGate(): ListenInfiniteScrollGate {
  let visible = false;
  const acquiredContinuations = new Set<string>();

  return {
    setVisible(nextVisible) {
      visible = nextVisible;
    },
    tryAcquire({ enabled, loading, continuation }) {
      const normalizedContinuation = continuation.trim();
      if (
        !visible ||
        !enabled ||
        loading ||
        !normalizedContinuation ||
        acquiredContinuations.has(normalizedContinuation)
      ) {
        return false;
      }
      acquiredContinuations.add(normalizedContinuation);
      return true;
    },
  };
}

function findNearestScrollRoot(node: HTMLElement): Element | null {
  if (
    typeof window === "undefined" ||
    typeof window.getComputedStyle !== "function"
  ) {
    return null;
  }

  let ancestor = node.parentElement;
  while (ancestor) {
    const style = window.getComputedStyle(ancestor);
    const overflow = `${style.overflow} ${style.overflowY}`;
    if (/(auto|scroll|overlay)/u.test(overflow)) {
      return ancestor;
    }
    ancestor = ancestor.parentElement;
  }
  return null;
}

export type ListenInfiniteScrollSentinelProps = {
  continuation: string;
  enabled: boolean;
  loading: boolean;
  onLoadMore: () => void;
  /** `undefined` discovers the nearest scrolling ancestor; `null` uses the viewport. */
  root?: Element | null;
  rootMargin?: string;
  threshold?: number;
  className?: string;
};

export function ListenInfiniteScrollSentinel({
  continuation,
  enabled,
  loading,
  onLoadMore,
  root,
  rootMargin = LISTEN_INFINITE_SCROLL_ROOT_MARGIN,
  threshold = 0,
  className,
}: ListenInfiniteScrollSentinelProps) {
  const sentinelRef = React.useRef<HTMLDivElement | null>(null);
  const gateRef = React.useRef<ListenInfiniteScrollGate>();
  const currentRequestRef = React.useRef({
    continuation,
    enabled,
    loading,
    onLoadMore,
  });

  if (!gateRef.current) {
    gateRef.current = createListenInfiniteScrollGate();
  }
  currentRequestRef.current = {
    continuation,
    enabled,
    loading,
    onLoadMore,
  };

  const requestNextPage = React.useCallback(() => {
    const request = currentRequestRef.current;
    if (gateRef.current?.tryAcquire(request)) {
      request.onLoadMore();
    }
  }, []);

  React.useEffect(() => {
    const node = sentinelRef.current;
    if (!node || typeof IntersectionObserver === "undefined") {
      return;
    }

    const observer = new IntersectionObserver(
      ([entry]) => {
        const visible = Boolean(
          entry && (entry.isIntersecting || entry.intersectionRatio > 0),
        );
        gateRef.current?.setVisible(visible);
        if (visible) {
          requestNextPage();
        }
      },
      {
        root: root === undefined ? findNearestScrollRoot(node) : root,
        rootMargin,
        threshold,
      },
    );

    observer.observe(node);
    return () => observer.disconnect();
  }, [requestNextPage, root, rootMargin, threshold]);

  // Readiness often changes after the initial intersection event (for example,
  // when the first page finishes and supplies its continuation). Wait one
  // animation frame so IntersectionObserver can first report whether the newly
  // appended content moved the sentinel away from the scroll threshold.
  React.useEffect(() => {
    if (typeof requestAnimationFrame !== "function") {
      requestNextPage();
      return;
    }
    const frame = requestAnimationFrame(requestNextPage);
    return () => cancelAnimationFrame(frame);
  }, [continuation, enabled, loading, onLoadMore, requestNextPage]);

  return (
    <div
      ref={sentinelRef}
      aria-hidden="true"
      className={cn("h-px w-full pointer-events-none", className)}
      data-listen-infinite-scroll-sentinel="true"
    />
  );
}
