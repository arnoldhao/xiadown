import type {
  PDFDocumentLoadingTask,
  PDFDocumentProxy,
  PDFPageProxy,
  RenderTask,
} from "pdfjs-dist";
import * as React from "react";

import { LibraryArtwork } from "./LibraryArtwork";
import { loadLibraryPdfRuntime } from "./library-pdf-runtime";
import type { LibraryCardPreview, LibraryWorkspaceItem } from "./types";

const CARD_PREVIEW_ROOT_MARGIN = "320px 0px";
const PDF_THUMBNAIL_MAX_WIDTH = 384;
const PDF_THUMBNAIL_MAX_HEIGHT = 480;
const PDF_THUMBNAIL_MEMORY_LIMIT = 24;
const PDF_THUMBNAIL_DISK_LIMIT = 96;
const PDF_THUMBNAIL_MAX_BLOB_BYTES = 1 << 20;
const PDF_THUMBNAIL_DISK_MAX_AGE_MS = 90 * 24 * 60 * 60 * 1_000;
const PDF_THUMBNAIL_CACHE_NAME = "xiadown-library-card-previews-v1";
const PDF_THUMBNAIL_CACHE_ORIGIN = "https://card-preview-cache.xiadown.invalid";

interface LogPreviewPayload {
  lines: string[];
  truncated: boolean;
}

const pdfMemoryCache = new Map<string, Blob>();
let pdfQueueTail: Promise<void> = Promise.resolve();
let pdfDiskCachePruned = false;
let pdfDiskCacheWrites = 0;

type LogQueueEntry = {
  signal: AbortSignal;
  run: () => Promise<LogPreviewPayload>;
  resolve: (value: LogPreviewPayload) => void;
  reject: (reason?: unknown) => void;
};

const logQueue: LogQueueEntry[] = [];
let activeLogRequests = 0;

function abortError() {
  return new DOMException("Preview request was cancelled", "AbortError");
}

function useNearViewport(enabled: boolean) {
  const [node, setNode] = React.useState<HTMLSpanElement | null>(null);
  const [nearViewport, setNearViewport] = React.useState(false);

  React.useEffect(() => {
    if (!enabled || nearViewport || !node) return;
    if (typeof IntersectionObserver === "undefined") {
      setNearViewport(true);
      return;
    }
    const observer = new IntersectionObserver((entries) => {
      if (!entries.some((entry) => entry.isIntersecting)) return;
      setNearViewport(true);
      observer.disconnect();
    }, { rootMargin: CARD_PREVIEW_ROOT_MARGIN });
    observer.observe(node);
    return () => observer.disconnect();
  }, [enabled, nearViewport, node]);

  return { nearViewport, setNode };
}

function cachePdfInMemory(key: string, blob: Blob) {
  pdfMemoryCache.delete(key);
  pdfMemoryCache.set(key, blob);
  while (pdfMemoryCache.size > PDF_THUMBNAIL_MEMORY_LIMIT) {
    const oldest = pdfMemoryCache.keys().next().value;
    if (typeof oldest !== "string") break;
    pdfMemoryCache.delete(oldest);
  }
}

function pdfCacheRequest(key: string) {
  return new Request(
    `${PDF_THUMBNAIL_CACHE_ORIGIN}/v1/pdf/${encodeURIComponent(key)}`,
    { method: "GET" },
  );
}

async function openPdfDiskCache() {
  if (typeof caches === "undefined") return undefined;
  try {
    const cache = await caches.open(PDF_THUMBNAIL_CACHE_NAME);
    if (!pdfDiskCachePruned) {
      pdfDiskCachePruned = true;
      void prunePdfDiskCache(cache);
    }
    return cache;
  } catch {
    return undefined;
  }
}

async function prunePdfDiskCache(cache: Cache) {
  try {
    const now = Date.now();
    const records = await Promise.all((await cache.keys()).map(async (request) => {
      const response = await cache.match(request);
      const cachedAt = Number(response?.headers.get("X-XiaDown-Cached-At") ?? 0);
      return { request, cachedAt: Number.isFinite(cachedAt) ? cachedAt : 0 };
    }));
    records.sort((left, right) => right.cachedAt - left.cachedAt);
    await Promise.all(records.map((record, index) => {
      const expired = record.cachedAt <= 0 ||
        now - record.cachedAt > PDF_THUMBNAIL_DISK_MAX_AGE_MS;
      return expired || index >= PDF_THUMBNAIL_DISK_LIMIT
        ? cache.delete(record.request)
        : Promise.resolve(false);
    }));
  } catch {
    // Cache Storage is an optimization. A failed prune must not block preview.
  }
}

async function readCachedPdfThumbnail(key: string) {
  const memory = pdfMemoryCache.get(key);
  if (memory) {
    cachePdfInMemory(key, memory);
    return memory;
  }
  const cache = await openPdfDiskCache();
  if (!cache) return undefined;
  try {
    const response = await cache.match(pdfCacheRequest(key));
    if (!response?.ok) return undefined;
    const blob = await response.blob();
    if (
      !blob.size ||
      blob.size > PDF_THUMBNAIL_MAX_BLOB_BYTES ||
      !blob.type.startsWith("image/")
    ) {
      return undefined;
    }
    cachePdfInMemory(key, blob);
    return blob;
  } catch {
    return undefined;
  }
}

async function writeCachedPdfThumbnail(key: string, blob: Blob) {
  cachePdfInMemory(key, blob);
  if (blob.size > PDF_THUMBNAIL_MAX_BLOB_BYTES) return;
  const cache = await openPdfDiskCache();
  if (!cache) return;
  try {
    await cache.put(
      pdfCacheRequest(key),
      new Response(blob, {
        headers: {
          "Content-Type": blob.type || "image/jpeg",
          "X-XiaDown-Cached-At": String(Date.now()),
        },
      }),
    );
    pdfDiskCacheWrites++;
    if (pdfDiskCacheWrites % 12 === 0) {
      await prunePdfDiskCache(cache);
    }
  } catch {
    // Memory caching remains available when the WebView cache is unavailable.
  }
}

function canvasBlob(canvas: HTMLCanvasElement) {
  return new Promise<Blob>((resolve, reject) => {
    canvas.toBlob((blob) => {
      if (blob) {
        resolve(blob);
      } else {
        reject(new Error("PDF thumbnail encoding failed"));
      }
    }, "image/jpeg", 0.82);
  });
}

async function renderPdfThumbnail(sourceURL: string, signal: AbortSignal) {
  if (signal.aborted) throw abortError();
  const pdfjs = await loadLibraryPdfRuntime();
  if (signal.aborted) throw abortError();

  let loadingTask: PDFDocumentLoadingTask | undefined;
  let document: PDFDocumentProxy | undefined;
  let page: PDFPageProxy | undefined;
  let renderTask: RenderTask | undefined;
  let destroyPromise: Promise<void> | undefined;
  const destroyLoadingTask = () => {
    if (!loadingTask) return Promise.resolve();
    destroyPromise ??= loadingTask.destroy();
    return destroyPromise;
  };
  const abort = () => {
    renderTask?.cancel();
    void destroyLoadingTask();
  };
  signal.addEventListener("abort", abort, { once: true });
  try {
    loadingTask = pdfjs.getDocument({
      url: sourceURL,
      disableAutoFetch: true,
      disableStream: true,
      rangeChunkSize: 64 * 1_024,
    });
    if (signal.aborted) {
      await destroyLoadingTask();
      throw abortError();
    }
    document = await loadingTask.promise;
    if (signal.aborted) throw abortError();
    page = await document.getPage(1);
    const unscaled = page.getViewport({ scale: 1 });
    const scale = Math.min(
      PDF_THUMBNAIL_MAX_WIDTH / Math.max(1, unscaled.width),
      PDF_THUMBNAIL_MAX_HEIGHT / Math.max(1, unscaled.height),
    );
    const viewport = page.getViewport({ scale: Math.max(0.01, scale) });
    const outputScale = Math.min(
      2,
      Math.max(1, globalThis.devicePixelRatio || 1),
    );
    const canvas = documentOwner().createElement("canvas");
    canvas.width = Math.max(1, Math.floor(viewport.width * outputScale));
    canvas.height = Math.max(1, Math.floor(viewport.height * outputScale));
    const context = canvas.getContext("2d", { alpha: false });
    if (!context) throw new Error("PDF thumbnail canvas is unavailable");
    context.fillStyle = "#ffffff";
    context.fillRect(0, 0, canvas.width, canvas.height);
    renderTask = page.render({
      canvas,
      canvasContext: context,
      viewport,
      background: "#ffffff",
      transform: outputScale === 1
        ? undefined
        : [outputScale, 0, 0, outputScale, 0, 0],
    });
    await renderTask.promise;
    if (signal.aborted) throw abortError();
    return await canvasBlob(canvas);
  } finally {
    signal.removeEventListener("abort", abort);
    page?.cleanup();
    try {
      if (document) await document.cleanup();
    } finally {
      await destroyLoadingTask();
    }
  }
}

function documentOwner() {
  if (typeof document === "undefined") {
    throw new Error("PDF thumbnail document is unavailable");
  }
  return document;
}

async function loadPdfThumbnail(
  preview: LibraryCardPreview,
  signal: AbortSignal,
) {
  const cached = await readCachedPdfThumbnail(preview.cacheKey);
  if (cached) return cached;
  if (signal.aborted) throw abortError();

  const queued = pdfQueueTail.then(async () => {
    if (signal.aborted) throw abortError();
    const secondCacheCheck = await readCachedPdfThumbnail(preview.cacheKey);
    if (secondCacheCheck) return secondCacheCheck;
    const blob = await renderPdfThumbnail(preview.sourceURL, signal);
    await writeCachedPdfThumbnail(preview.cacheKey, blob);
    return blob;
  });
  pdfQueueTail = queued.then(() => undefined, () => undefined);
  return queued;
}

function drainLogQueue() {
  while (activeLogRequests < 2 && logQueue.length > 0) {
    const entry = logQueue.shift();
    if (!entry) return;
    if (entry.signal.aborted) {
      entry.reject(abortError());
      continue;
    }
    activeLogRequests++;
    void entry.run()
      .then(entry.resolve, entry.reject)
      .finally(() => {
        activeLogRequests--;
        drainLogQueue();
      });
  }
}

function queueLogPreview(
  preview: LibraryCardPreview,
  signal: AbortSignal,
) {
  return new Promise<LogPreviewPayload>((resolve, reject) => {
    logQueue.push({
      signal,
      resolve,
      reject,
      run: async () => {
        const response = await fetch(preview.sourceURL, {
          cache: "default",
          credentials: "same-origin",
          headers: { Accept: "application/json" },
          signal,
        });
        if (!response.ok) {
          throw new Error(`LOG preview request failed (${response.status})`);
        }
        const payload = await response.json() as Partial<LogPreviewPayload>;
        const lines = Array.isArray(payload.lines)
          ? payload.lines
              .filter((line): line is string => typeof line === "string")
              .slice(0, 7)
          : [];
        return { lines, truncated: Boolean(payload.truncated) };
      },
    });
    drainLogQueue();
  });
}

export interface LibraryCardArtworkProps {
  item: LibraryWorkspaceItem;
}

/**
 * Library-only derived preview boundary. It keeps the normal artwork fallback
 * mounted until a bounded PDF page or LOG excerpt is ready.
 */
export function LibraryCardArtwork(props: LibraryCardArtworkProps) {
  const preview = props.item.cardPreview;
  const { nearViewport, setNode } = useNearViewport(Boolean(preview));
  const [pdfURL, setPdfURL] = React.useState("");
  const [logPreview, setLogPreview] = React.useState<LogPreviewPayload>();

  React.useEffect(() => {
    setPdfURL("");
    setLogPreview(undefined);
    if (!preview || !nearViewport) return;
    const controller = new AbortController();
    let objectURL = "";

    if (preview.kind === "pdf") {
      void loadPdfThumbnail(preview, controller.signal)
        .then((blob) => {
          if (controller.signal.aborted) return;
          objectURL = URL.createObjectURL(blob);
          setPdfURL(objectURL);
        })
        .catch(() => {
          // The document placeholder remains the stable failure state.
        });
    } else {
      void queueLogPreview(preview, controller.signal)
        .then((payload) => {
          if (!controller.signal.aborted) setLogPreview(payload);
        })
        .catch(() => {
          // The document placeholder remains the stable failure state.
        });
    }
    return () => {
      controller.abort();
      if (objectURL) URL.revokeObjectURL(objectURL);
    };
  }, [nearViewport, preview]);

  const ready = Boolean(pdfURL || logPreview?.lines.length);
  return (
    <span
      className="app-library-card-preview"
      data-preview-kind={preview?.kind}
      data-preview-ready={ready ? "true" : "false"}
      ref={setNode}
    >
      <LibraryArtwork
        src={props.item.coverURL}
        fallbackSrc={props.item.fallbackCoverURL}
        category={props.item.category}
        otherGroup={props.item.otherGroup}
        alt=""
      />
      {pdfURL ? (
        <img
          alt=""
          className="app-library-card-preview__pdf"
          decoding="async"
          src={pdfURL}
        />
      ) : null}
      {logPreview?.lines.length ? (
        <span className="app-library-card-preview__log">
          <span className="app-library-card-preview__log-chrome" aria-hidden="true">
            <i />
            <i />
            <i />
          </span>
          <span className="app-library-card-preview__log-lines">
            {logPreview.lines.map((line, index) => (
              <span
                className="app-library-card-preview__log-line"
                key={`${index}-${line}`}
              >
                {line}
              </span>
            ))}
          </span>
          {logPreview.truncated ? (
            <span className="app-library-card-preview__log-fade" />
          ) : null}
        </span>
      ) : null}
    </span>
  );
}
