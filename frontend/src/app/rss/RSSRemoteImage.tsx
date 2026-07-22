import { Image as ImageIcon } from "lucide-react";
import * as React from "react";

import { cn } from "@/lib/utils";

import { controlledRSSResourceURL } from "./remote-resource";
import {
  getRSSImageSessionState,
  markRSSImageFailed,
  markRSSImageLoaded,
} from "./session-cache";

type RSSRemoteImageState = "loading" | "failed" | "empty";

/**
 * Renders only loopback-projected RSS media. The real image stays invisible
 * until it has decoded successfully, so WebKit never exposes its broken-image
 * chrome while candidates are loading or being retried.
 */
export function RSSRemoteImage({
  src,
  sources,
  fallback = null,
  onError,
  onLoad,
  loading = "lazy",
  decoding = "async",
  className,
  ...props
}: Omit<React.ImgHTMLAttributes<HTMLImageElement>, "src"> & {
  src?: string;
  sources?: readonly (string | undefined)[];
  fallback?: React.ReactNode;
}) {
  const candidates = React.useMemo(
    () => [...new Set([src, ...(sources ?? [])]
      .map((candidate) => controlledRSSResourceURL(candidate))
      .filter(Boolean))],
    [sources, src],
  );
  const candidateKey = candidates.join("\u0000");

  return (
    <RSSRemoteImageCandidates
      {...props}
      candidates={candidates}
      className={className}
      decoding={decoding}
      fallback={fallback}
      key={candidateKey}
      loading={loading}
      onError={onError}
      onLoad={onLoad}
    />
  );
}

function RSSRemoteImageCandidates({
  candidates,
  fallback,
  onError,
  onLoad,
  loading,
  decoding,
  className,
  ...props
}: Omit<React.ImgHTMLAttributes<HTMLImageElement>, "src"> & {
  candidates: readonly string[];
  fallback: React.ReactNode;
}) {
  const initial = React.useMemo(
    () => resolveRSSImageCandidate(candidates),
    [candidates],
  );
  const [candidateIndex, setCandidateIndex] = React.useState(initial.index);
  const [loadedCandidate, setLoadedCandidate] = React.useState(
    initial.loaded ? candidates[initial.index] ?? "" : "",
  );
  const [retryAt, setRetryAt] = React.useState(initial.retryAt);
  const [requested, setRequested] = React.useState(loading !== "lazy");
  const probeRef = React.useRef<HTMLImageElement | null>(null);

  const controlled = candidates[candidateIndex] || "";
  const loaded = Boolean(
    requested && controlled && loadedCandidate === controlled,
  );
  const state: RSSRemoteImageState = controlled
    ? "loading"
    : candidates.length > 0
      ? "failed"
      : "empty";

  React.useEffect(() => {
    if (loading !== "lazy" || typeof IntersectionObserver === "undefined") {
      setRequested(true);
      return;
    }
    const probe = probeRef.current;
    const target = probe?.parentElement;
    if (!target) {
      setRequested(true);
      return;
    }
    const observer = new IntersectionObserver((entries) => {
      if (!entries.some((entry) => entry.isIntersecting)) return;
      setRequested(true);
      observer.disconnect();
    }, { rootMargin: "320px 0px" });
    observer.observe(target);
    return () => observer.disconnect();
  }, [loading]);

  React.useEffect(() => {
    if (controlled || retryAt === undefined) {
      return;
    }
    const timer = window.setTimeout(() => {
      const next = resolveRSSImageCandidate(candidates);
      setCandidateIndex(next.index);
      setLoadedCandidate(next.loaded ? candidates[next.index] ?? "" : "");
      setRetryAt(next.retryAt);
    }, Math.max(0, retryAt - Date.now()) + 1);
    return () => window.clearTimeout(timer);
  }, [candidates, controlled, retryAt]);

  return (
    <>
      {controlled ? (
        <img
          {...props}
          className={cn(className, !loaded && "rss-remote-image--probe")}
          data-rss-image-state={loaded ? "loaded" : "loading"}
          decoding={decoding}
          loading="eager"
          onLoad={(event) => {
            markRSSImageLoaded(controlled);
            setLoadedCandidate(controlled);
            setRetryAt(undefined);
            onLoad?.(event);
          }}
          onError={(event) => {
            // Hide the browser's broken-image chrome before React advances to
            // another candidate (or leaves the skeleton in place).
            event.currentTarget.classList.add("rss-remote-image--probe");
            event.currentTarget.dataset.rssImageState = "loading";
            markRSSImageFailed(controlled);
            setLoadedCandidate("");
            const next = resolveRSSImageCandidate(candidates);
            setCandidateIndex(next.index);
            setLoadedCandidate(next.loaded ? candidates[next.index] ?? "" : "");
            setRetryAt(next.retryAt);
            if (next.index >= candidates.length) {
              onError?.(event);
            }
          }}
          referrerPolicy="no-referrer"
          ref={probeRef}
          src={requested ? controlled : undefined}
        />
      ) : null}
      {!loaded ? (
        <RSSImageSkeleton className={className} fallback={fallback} state={state} />
      ) : null}
    </>
  );
}

function resolveRSSImageCandidate(
  candidates: readonly string[],
  now = Date.now(),
) {
  let firstUnknown = -1;
  let retryAt: number | undefined;

  for (let index = 0; index < candidates.length; index += 1) {
    const state = getRSSImageSessionState(candidates[index], now);
    if (state?.status === "loaded") {
      return { index, loaded: true, retryAt: undefined };
    }
    if (!state && firstUnknown < 0) {
      firstUnknown = index;
    }
    if (state?.status === "failed") {
      retryAt = Math.min(retryAt ?? state.retryAt, state.retryAt);
    }
  }

  if (firstUnknown >= 0) {
    return { index: firstUnknown, loaded: false, retryAt: undefined };
  }
  return {
    index: candidates.length,
    loaded: false,
    retryAt,
  };
}

function RSSImageSkeleton({
  className,
  fallback,
  state,
}: {
  className?: string;
  fallback: React.ReactNode;
  state: RSSRemoteImageState;
}) {
  if (
    React.isValidElement<{ className?: string }>(fallback) &&
    typeof fallback.type === "string" &&
    fallback.type !== "svg"
  ) {
    return React.cloneElement(fallback, {
      className: cn(fallback.props.className, "rss-image-skeleton"),
      "data-rss-image-state": state,
    } as React.HTMLAttributes<HTMLElement>);
  }

  return (
    <span
      aria-hidden="true"
      className={cn("rss-image-skeleton rss-image-skeleton--generic", className && `${className}--skeleton`)}
      data-rss-image-state={state}
    >
      {fallback || <ImageIcon />}
    </span>
  );
}
