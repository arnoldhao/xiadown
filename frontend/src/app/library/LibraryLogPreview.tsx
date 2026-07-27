import {
  ChevronDown,
  ChevronUp,
  Eye,
  LoaderCircle,
  RotateCcw,
} from "lucide-react";
import * as React from "react";

import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from "@/shared/ui/dialog";

import { IpodControlWheel } from "./LibraryIpodPreview";
import type { LibraryWorkspaceLabels } from "./types";

const INLINE_LOG_LINE_COUNT = 5;

interface LogSummaryPayload {
  lines: string[];
  truncated: boolean;
}

interface LogTextPayload {
  text: string;
  truncated: boolean;
}

function logDetailURL(sourceURL: string) {
  try {
    const url = new URL(sourceURL);
    url.searchParams.set("detail", "1");
    return url.toString();
  } catch {
    return `${sourceURL}${sourceURL.includes("?") ? "&" : "?"}detail=1`;
  }
}

async function fetchLogPayload<T>(url: string, signal: AbortSignal) {
  const response = await fetch(url, { signal });
  if (!response.ok) throw new Error(`Log preview failed (${response.status})`);
  return await response.json() as T;
}

export interface LibraryLogPreviewProps {
  labels: LibraryWorkspaceLabels;
  sourceURL: string;
  title: string;
}

export function LibraryLogPreview(props: LibraryLogPreviewProps) {
  const sourceURL = props.sourceURL.trim();
  const [summary, setSummary] = React.useState<LogSummaryPayload>({
    lines: [],
    truncated: false,
  });
  const [lineIndex, setLineIndex] = React.useState(0);
  const [loading, setLoading] = React.useState(Boolean(sourceURL));
  const [loadError, setLoadError] = React.useState("");
  const [dialogOpen, setDialogOpen] = React.useState(false);
  const [detail, setDetail] = React.useState<LogTextPayload | null>(null);
  const [detailLoading, setDetailLoading] = React.useState(false);
  const [detailError, setDetailError] = React.useState("");

  React.useEffect(() => {
    const controller = new AbortController();
    setSummary({ lines: [], truncated: false });
    setLineIndex(0);
    setLoading(Boolean(sourceURL));
    setLoadError("");
    setDialogOpen(false);
    setDetail(null);
    setDetailLoading(false);
    setDetailError("");
    if (!sourceURL) {
      setLoading(false);
      return () => controller.abort();
    }
    void fetchLogPayload<LogSummaryPayload>(sourceURL, controller.signal)
      .then((payload) => {
        if (controller.signal.aborted) return;
        const lines = Array.isArray(payload.lines)
          ? payload.lines.filter((line): line is string => typeof line === "string")
          : [];
        setSummary({ lines, truncated: Boolean(payload.truncated) });
        setLineIndex(Math.max(0, lines.length - INLINE_LOG_LINE_COUNT));
        setLoading(false);
      })
      .catch((error: unknown) => {
        if (controller.signal.aborted) return;
        setLoading(false);
        setLoadError(
          error instanceof Error ? error.message : props.labels.loadFailed,
        );
      });
    return () => controller.abort();
  }, [props.labels.loadFailed, sourceURL]);

  React.useEffect(() => {
    if (!dialogOpen || !sourceURL || detail) return;
    const controller = new AbortController();
    setDetailLoading(true);
    setDetailError("");
    void fetchLogPayload<LogTextPayload>(
      logDetailURL(sourceURL),
      controller.signal,
    )
      .then((payload) => {
        if (controller.signal.aborted) return;
        setDetail({
          text: typeof payload.text === "string" ? payload.text : "",
          truncated: Boolean(payload.truncated),
        });
        setDetailLoading(false);
      })
      .catch((error: unknown) => {
        if (controller.signal.aborted) return;
        setDetailLoading(false);
        setDetailError(
          error instanceof Error ? error.message : props.labels.loadFailed,
        );
      });
    return () => controller.abort();
  }, [
    detail,
    dialogOpen,
    props.labels.loadFailed,
    sourceURL,
  ]);

  const maxLineIndex = Math.max(
    0,
    summary.lines.length - INLINE_LOG_LINE_COUNT,
  );
  const visibleLines = summary.lines.slice(
    lineIndex,
    lineIndex + INLINE_LOG_LINE_COUNT,
  );
  const lineProgress = maxLineIndex > 0
    ? (lineIndex / maxLineIndex) * 100
    : 100;
  const rangeLabel = summary.lines.length > 0
    ? `${lineIndex + 1}–${Math.min(
        summary.lines.length,
        lineIndex + INLINE_LOG_LINE_COUNT,
      )} / ${summary.lines.length}`
    : "–";
  const setSafeLineIndex = (value: number) => {
    setLineIndex(Math.min(maxLineIndex, Math.max(0, value)));
  };

  return (
    <>
      <div
        className="app-library-ipod app-library-log-ipod"
        data-loading={loading ? "true" : undefined}
        data-media-kind="log"
      >
        <div className="app-library-ipod__screen">
          <div className="app-library-ipod__display app-library-log-ipod__display">
            <div className="app-library-log-ipod__chrome" aria-hidden="true">
              <i /><i /><i />
              <span>LOG</span>
            </div>
            <pre aria-label={`${props.labels.preview}: ${props.title}`}>
              {visibleLines.join("\n")}
            </pre>
            {loading ? (
              <LoaderCircle
                aria-hidden="true"
                className="app-library-ipod__loading app-motion-spin"
              />
            ) : null}
          </div>
          <div className="app-library-ipod__range">
            <input
              aria-label={props.labels.preview}
              aria-valuetext={rangeLabel}
              disabled={maxLineIndex === 0}
              max={maxLineIndex}
              min={0}
              onChange={(event) =>
                setSafeLineIndex(Number(event.currentTarget.value))}
              step={1}
              style={{
                "--app-library-ipod-range-value": `${lineProgress}%`,
              } as React.CSSProperties}
              type="range"
              value={lineIndex}
            />
            <div>
              <span>{rangeLabel}</span>
              <span>{summary.truncated ? "… LOG" : "LOG"}</span>
            </div>
          </div>
        </div>
        <IpodControlWheel
          bottomDisabled={lineIndex === maxLineIndex}
          bottomIcon={<RotateCcw aria-hidden="true" size={18} />}
          bottomLabel={props.labels.reset}
          leftDisabled={lineIndex <= 0}
          leftIcon={<ChevronUp aria-hidden="true" size={18} />}
          leftLabel={props.labels.previousPage}
          onBottom={() => setSafeLineIndex(maxLineIndex)}
          onLeft={() => setSafeLineIndex(lineIndex - 1)}
          onRight={() => setSafeLineIndex(lineIndex + 1)}
          onTop={() => setDialogOpen(true)}
          rightDisabled={lineIndex >= maxLineIndex}
          rightIcon={<ChevronDown aria-hidden="true" size={18} />}
          rightLabel={props.labels.nextPage}
          topDisabled={!sourceURL || Boolean(loadError)}
          topIcon={<Eye aria-hidden="true" size={18} />}
          topLabel={props.labels.preview}
        />
        {loadError ? (
          <p className="app-library-ipod__error" role="alert" title={loadError}>
            {props.labels.loadFailed}
          </p>
        ) : null}
      </div>

      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent className="app-library-log-dialog app-media-preview-dialog min-w-0 max-w-none">
          <DialogHeader className="app-media-preview-dialog-header app-library-log-dialog__header">
            <DialogTitle
              className="app-library-log-dialog__title"
              title={props.title}
            >
              {props.title}
            </DialogTitle>
          </DialogHeader>
          <div
            aria-busy={detailLoading}
            aria-label={`${props.labels.preview}: ${props.title}`}
            className="app-library-log-dialog__stage app-media-preview-dialog-stage"
          >
            {detailLoading ? (
              <span className="app-library-log-dialog__loading" role="status">
                <LoaderCircle aria-hidden="true" className="app-motion-spin" />
                <span className="sr-only">{props.labels.loading}</span>
              </span>
            ) : detailError ? (
              <p className="app-library-log-dialog__error" role="alert">
                {props.labels.loadFailed}
              </p>
            ) : (
              <pre>{detail?.truncated ? `…\n${detail.text}` : detail?.text}</pre>
            )}
          </div>
        </DialogContent>
      </Dialog>
    </>
  );
}
