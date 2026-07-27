import {
  ChevronLeft,
  ChevronRight,
  Eye,
  LoaderCircle,
  RotateCcw,
  ZoomIn,
  ZoomOut,
} from "lucide-react";
import type {
  PDFDocumentLoadingTask,
  PDFDocumentProxy,
  PDFPageProxy,
  RenderTask,
} from "pdfjs-dist";
import * as React from "react";

import { Button } from "@/shared/ui/button";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from "@/shared/ui/dialog";

import { IpodControlWheel } from "./LibraryIpodPreview";
import {
  capturePointerZoomAnchor,
  restorePointerZoomAnchor,
  zoomAfterWheel,
  type PointerZoomAnchor,
} from "./library-pointer-zoom";
import { loadLibraryPdfRuntime } from "./library-pdf-runtime";
import type { LibraryWorkspaceLabels } from "./types";

const MIN_PDF_ZOOM = 0.6;
const MAX_PDF_ZOOM = 2.5;
const PDF_ZOOM_STEP = 0.2;

export function clampPdfZoom(value: number) {
  if (!Number.isFinite(value)) return 1;
  return Math.min(MAX_PDF_ZOOM, Math.max(MIN_PDF_ZOOM, value));
}

export function pdfZoomAfterWheel(
  current: number,
  deltaY: number,
  deltaMode = 0,
  viewportHeight = 1,
) {
  return zoomAfterWheel(
    current,
    deltaY,
    deltaMode,
    viewportHeight,
    clampPdfZoom,
  );
}

interface PdfPageStageProps {
  document: PDFDocumentProxy | null;
  externalStageRef?: React.MutableRefObject<HTMLDivElement | null>;
  labels: LibraryWorkspaceLabels;
  loadError: string;
  loadingDocument: boolean;
  onCanvasLayout?: () => void;
  onWheel?: React.WheelEventHandler<HTMLDivElement>;
  pageNumber: number;
  presentation?: "ipod" | "dialog";
  title: string;
  zoom: number;
}

function PdfPageStage(props: PdfPageStageProps) {
  const stageRef = React.useRef<HTMLDivElement | null>(null);
  const canvasRef = React.useRef<HTMLCanvasElement | null>(null);
  const [stageWidth, setStageWidth] = React.useState(0);
  const [stageHeight, setStageHeight] = React.useState(0);
  const [renderingPage, setRenderingPage] = React.useState(false);
  const [renderError, setRenderError] = React.useState("");

  React.useEffect(() => {
    const stage = stageRef.current;
    if (!stage) return;
    const measure = () => {
      setStageWidth(Math.max(0, stage.clientWidth));
      setStageHeight(Math.max(0, stage.clientHeight));
    };
    measure();
    if (typeof ResizeObserver === "undefined") return;
    const observer = new ResizeObserver(measure);
    observer.observe(stage);
    return () => observer.disconnect();
  }, []);

  React.useEffect(() => {
    const canvas = canvasRef.current;
    if (!props.document || !canvas || stageWidth <= 0) return;
    let disposed = false;
    let page: PDFPageProxy | undefined;
    let renderTask: RenderTask | undefined;
    setRenderingPage(true);
    setRenderError("");

    void props.document.getPage(props.pageNumber)
      .then((loadedPage) => {
        if (disposed) return;
        page = loadedPage;
        const unscaled = page.getViewport({ scale: 1 });
        const availableWidth = Math.max(1, stageWidth - 24);
        const widthScale = availableWidth / Math.max(1, unscaled.width);
        const fitScale = props.presentation === "ipod" && stageHeight > 0
          ? Math.min(
              widthScale,
              Math.max(1, stageHeight - 16) / Math.max(1, unscaled.height),
            )
          : widthScale;
        const viewport = page.getViewport({ scale: fitScale * props.zoom });
        const outputScale = Math.min(2, Math.max(1, window.devicePixelRatio || 1));
        canvas.width = Math.max(1, Math.floor(viewport.width * outputScale));
        canvas.height = Math.max(1, Math.floor(viewport.height * outputScale));
        canvas.style.width = `${Math.floor(viewport.width)}px`;
        canvas.style.height = `${Math.floor(viewport.height)}px`;
        props.onCanvasLayout?.();
        renderTask = page.render({
          canvas,
          viewport,
          transform: outputScale === 1
            ? undefined
            : [outputScale, 0, 0, outputScale, 0, 0],
        });
        return renderTask.promise;
      })
      .then(() => {
        if (!disposed) setRenderingPage(false);
      })
      .catch((error: unknown) => {
        if (
          disposed ||
          (error instanceof Error && error.name === "RenderingCancelledException")
        ) {
          return;
        }
        setRenderingPage(false);
        setRenderError(
          error instanceof Error ? error.message : props.labels.loadFailed,
        );
      });

    return () => {
      disposed = true;
      renderTask?.cancel();
      page?.cleanup();
    };
  }, [
    props.document,
    props.labels.loadFailed,
    props.onCanvasLayout,
    props.pageNumber,
    props.presentation,
    props.zoom,
    stageHeight,
    stageWidth,
  ]);

  const error = props.loadError || renderError;
  const loading = props.loadingDocument || renderingPage;
  const pageCount = props.document?.numPages ?? 0;

  return (
    <div
      aria-busy={loading}
      aria-label={`${props.labels.preview}: ${props.title}`}
      className={[
        "app-library-pdf__stage",
        props.presentation === "dialog"
          ? "app-library-pdf-dialog__stage app-media-preview-dialog-stage"
          : props.presentation === "ipod"
            ? "app-library-pdf__stage--ipod"
            : "",
      ].filter(Boolean).join(" ")}
      data-loading={loading ? "true" : undefined}
      onWheel={props.onWheel}
      ref={(node) => {
        stageRef.current = node;
        if (props.externalStageRef) props.externalStageRef.current = node;
      }}
    >
      <canvas
        aria-label={
          pageCount > 0
            ? props.labels.pageOf(props.pageNumber, pageCount)
            : props.title
        }
        ref={canvasRef}
      />
      {loading ? (
        <span className="app-library-pdf__loading" role="status">
          <LoaderCircle aria-hidden="true" className="app-motion-spin" />
          <span className="sr-only">{props.labels.loading}</span>
        </span>
      ) : null}
      {error ? (
        <p className="app-library-pdf__error" role="alert" title={error}>
          {props.labels.loadFailed}
        </p>
      ) : null}
    </div>
  );
}

interface PdfToolbarProps {
  labels: LibraryWorkspaceLabels;
  pageCount: number;
  pageNumber: number;
  setPageNumber: (value: number) => void;
  setZoom: (value: number) => void;
  zoom: number;
}

function PdfToolbar(props: PdfToolbarProps) {
  const setPage = (value: number) => {
    props.setPageNumber(
      Math.min(Math.max(1, value), Math.max(1, props.pageCount)),
    );
  };
  const setSafeZoom = (value: number) => props.setZoom(clampPdfZoom(value));
  const zoomLabel = `${Math.round(props.zoom * 100)}%`;

  return (
    <div className="app-library-pdf__toolbar">
      <div className="app-library-pdf__page-controls">
        <Button
          aria-label={props.labels.previousPage}
          disabled={props.pageNumber <= 1 || props.pageCount === 0}
          onClick={() => setPage(props.pageNumber - 1)}
          shape="square"
          size="compactIcon"
          type="button"
          variant="ghost"
        >
          <ChevronLeft aria-hidden="true" />
        </Button>
        <span aria-live="polite">
          {props.pageCount > 0
            ? props.labels.pageOf(props.pageNumber, props.pageCount)
            : "–"}
        </span>
        <Button
          aria-label={props.labels.nextPage}
          disabled={
            props.pageNumber >= props.pageCount || props.pageCount === 0
          }
          onClick={() => setPage(props.pageNumber + 1)}
          shape="square"
          size="compactIcon"
          type="button"
          variant="ghost"
        >
          <ChevronRight aria-hidden="true" />
        </Button>
      </div>
      <div className="app-library-pdf__toolbar-actions">
        <div className="app-library-pdf__zoom-controls">
          <Button
            aria-label={`${props.labels.size} −`}
            disabled={props.zoom <= MIN_PDF_ZOOM}
            onClick={() => setSafeZoom(props.zoom - PDF_ZOOM_STEP)}
            shape="square"
            size="compactIcon"
            type="button"
            variant="ghost"
          >
            <ZoomOut aria-hidden="true" />
          </Button>
          <button
            aria-label={props.labels.reset}
            className="app-library-pdf__zoom-value"
            onClick={() => props.setZoom(1)}
            title={props.labels.reset}
            type="button"
          >
            <RotateCcw aria-hidden="true" />
            {zoomLabel}
          </button>
          <Button
            aria-label={`${props.labels.size} +`}
            disabled={props.zoom >= MAX_PDF_ZOOM}
            onClick={() => setSafeZoom(props.zoom + PDF_ZOOM_STEP)}
            shape="square"
            size="compactIcon"
            type="button"
            variant="ghost"
          >
            <ZoomIn aria-hidden="true" />
          </Button>
        </div>
      </div>
    </div>
  );
}

export interface LibraryPdfPreviewProps {
  labels: LibraryWorkspaceLabels;
  sourceURL: string;
  title: string;
}

export function LibraryPdfPreview(props: LibraryPdfPreviewProps) {
  const dialogStageRef = React.useRef<HTMLDivElement | null>(null);
  const dialogAnchorRef = React.useRef<PointerZoomAnchor | null>(null);
  const [document, setDocument] = React.useState<PDFDocumentProxy | null>(null);
  const [pageNumber, setPageNumber] = React.useState(1);
  const [dialogOpen, setDialogOpen] = React.useState(false);
  const [dialogPageNumber, setDialogPageNumber] = React.useState(1);
  const [dialogZoom, setDialogZoom] = React.useState(1);
  const [loadingDocument, setLoadingDocument] = React.useState(true);
  const [loadError, setLoadError] = React.useState("");

  React.useEffect(() => {
    const sourceURL = props.sourceURL.trim();
    let disposed = false;
    let loadingTask: PDFDocumentLoadingTask | undefined;

    setDocument(null);
    setPageNumber(1);
    setDialogOpen(false);
    setDialogPageNumber(1);
    setDialogZoom(1);
    dialogAnchorRef.current = null;
    setLoadError("");
    setLoadingDocument(Boolean(sourceURL));
    if (!sourceURL) {
      setLoadingDocument(false);
      return;
    }

    void loadLibraryPdfRuntime()
      .then((pdfjs) => {
        if (disposed) return null;
        loadingTask = pdfjs.getDocument({ url: sourceURL });
        return loadingTask.promise;
      })
      .then((loaded) => {
        if (!loaded) return;
        if (disposed) {
          if (loadingTask) void loadingTask.destroy();
          return;
        }
        setDocument(loaded);
        setLoadingDocument(false);
      })
      .catch((error: unknown) => {
        if (disposed) return;
        setLoadingDocument(false);
        setLoadError(
          error instanceof Error ? error.message : props.labels.loadFailed,
        );
      });

    return () => {
      disposed = true;
      if (loadingTask) void loadingTask.destroy();
    };
  }, [props.labels.loadFailed, props.sourceURL]);

  const pageCount = document?.numPages ?? 0;
  const pageProgress = pageCount <= 1
    ? 100
    : ((pageNumber - 1) / (pageCount - 1)) * 100;
  const setInlinePage = (value: number) => {
    setPageNumber(Math.min(Math.max(1, value), Math.max(1, pageCount)));
  };
  const openDialog = () => {
    setDialogPageNumber(pageNumber);
    setDialogZoom(1);
    dialogAnchorRef.current = null;
    setDialogOpen(true);
  };
  const captureDialogAnchor = React.useCallback((
    clientX: number,
    clientY: number,
  ) => {
    const stage = dialogStageRef.current;
    if (!stage) return;
    dialogAnchorRef.current = capturePointerZoomAnchor(
      stage,
      clientX,
      clientY,
    );
  }, []);
  const captureDialogCenter = React.useCallback(() => {
    const stage = dialogStageRef.current;
    if (!stage) return;
    const rect = stage.getBoundingClientRect();
    captureDialogAnchor(
      rect.left + stage.clientWidth / 2,
      rect.top + stage.clientHeight / 2,
    );
  }, [captureDialogAnchor]);
  const restoreDialogAnchor = React.useCallback((clear: boolean) => {
    const stage = dialogStageRef.current;
    const anchor = dialogAnchorRef.current;
    if (!stage || !anchor) return;
    restorePointerZoomAnchor(stage, anchor);
    if (clear) dialogAnchorRef.current = null;
  }, []);
  const updateDialogZoom = React.useCallback((value: number) => {
    const next = clampPdfZoom(value);
    if (next === dialogZoom) return;
    captureDialogCenter();
    setDialogZoom(next);
  }, [captureDialogCenter, dialogZoom]);
  const handleDialogWheel = React.useCallback((
    event: React.WheelEvent<HTMLDivElement>,
  ) => {
    event.preventDefault();
    const next = pdfZoomAfterWheel(
      dialogZoom,
      event.deltaY,
      event.deltaMode,
      event.currentTarget.clientHeight,
    );
    if (next === dialogZoom) return;
    captureDialogAnchor(event.clientX, event.clientY);
    setDialogZoom(next);
  }, [captureDialogAnchor, dialogZoom]);
  const handleDialogCanvasLayout = React.useCallback(() => {
    window.requestAnimationFrame(() => restoreDialogAnchor(true));
  }, [restoreDialogAnchor]);

  return (
    <>
      <div
        className="app-library-ipod app-library-pdf-ipod"
        data-media-kind="pdf"
        data-loading={loadingDocument ? "true" : undefined}
      >
        <div className="app-library-ipod__screen">
          <div className="app-library-ipod__display app-library-pdf-ipod__display">
            <PdfPageStage
              document={document}
              labels={props.labels}
              loadError={loadError}
              loadingDocument={loadingDocument}
              pageNumber={pageNumber}
              presentation="ipod"
              title={props.title}
              zoom={1}
            />
          </div>
          <div className="app-library-ipod__range">
            <input
              aria-label={
                pageCount > 0
                  ? props.labels.pageOf(pageNumber, pageCount)
                  : props.labels.preview
              }
              aria-valuetext={
                pageCount > 0
                  ? props.labels.pageOf(pageNumber, pageCount)
                  : undefined
              }
              disabled={pageCount <= 1}
              max={Math.max(1, pageCount)}
              min={1}
              onChange={(event) => {
                setInlinePage(Number(event.currentTarget.value));
              }}
              step={1}
              style={{
                "--app-library-ipod-range-value": `${pageProgress}%`,
              } as React.CSSProperties}
              type="range"
              value={pageNumber}
            />
            <div>
              <span>
                {pageCount > 0
                  ? props.labels.pageOf(pageNumber, pageCount)
                  : "–"}
              </span>
              <span>PDF</span>
            </div>
          </div>
        </div>
        <IpodControlWheel
          bottomDisabled={pageNumber <= 1}
          bottomIcon={<RotateCcw aria-hidden="true" size={18} />}
          bottomLabel={props.labels.reset}
          leftDisabled={pageNumber <= 1}
          leftIcon={<ChevronLeft aria-hidden="true" size={18} />}
          leftLabel={props.labels.previousPage}
          onBottom={() => setInlinePage(1)}
          onLeft={() => setInlinePage(pageNumber - 1)}
          onRight={() => setInlinePage(pageNumber + 1)}
          onTop={openDialog}
          rightDisabled={pageCount === 0 || pageNumber >= pageCount}
          rightIcon={<ChevronRight aria-hidden="true" size={18} />}
          rightLabel={props.labels.nextPage}
          topDisabled={!document}
          topIcon={<Eye aria-hidden="true" size={18} />}
          topLabel={props.labels.preview}
        />
      </div>
      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent className="app-library-pdf-dialog app-media-preview-dialog min-w-0 max-w-none">
          <DialogHeader className="app-media-preview-dialog-header app-library-pdf-dialog__header">
            <DialogTitle
              className="app-library-pdf-dialog__title"
              title={props.title}
            >
              {props.title}
            </DialogTitle>
          </DialogHeader>
          <PdfPageStage
            document={document}
            externalStageRef={dialogStageRef}
            labels={props.labels}
            loadError={loadError}
            loadingDocument={loadingDocument}
            onCanvasLayout={handleDialogCanvasLayout}
            onWheel={handleDialogWheel}
            pageNumber={dialogPageNumber}
            presentation="dialog"
            title={props.title}
            zoom={dialogZoom}
          />
          <PdfToolbar
            labels={props.labels}
            pageCount={pageCount}
            pageNumber={dialogPageNumber}
            setPageNumber={setDialogPageNumber}
            setZoom={updateDialogZoom}
            zoom={dialogZoom}
          />
        </DialogContent>
      </Dialog>
    </>
  );
}
