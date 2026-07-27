import { describe, expect, test } from "bun:test";

describe("Library PDF preview", () => {
  test("renders PDF pages in-app with a worker, navigation, zoom, and cleanup", async () => {
    const [
      source,
      companion,
      layout,
      appearance,
      workflows,
      packageJSON,
      runtime,
    ] = await Promise.all([
      Bun.file(new URL("./LibraryPdfPreview.tsx", import.meta.url)).text(),
      Bun.file(new URL("./LibraryPreviewCompanion.tsx", import.meta.url)).text(),
      Bun.file(new URL("./library.css", import.meta.url)).text(),
      Bun.file(
        new URL("../../shared/styles/dream/library.css", import.meta.url),
      ).text(),
      Bun.file(
        new URL("../../shared/styles/dream/workflows.css", import.meta.url),
      ).text(),
      Bun.file(new URL("../../../package.json", import.meta.url)).json(),
      Bun.file(new URL("./library-pdf-runtime.ts", import.meta.url)).text(),
    ]);

    expect(packageJSON.dependencies["pdfjs-dist"]).toBeTruthy();
    expect(companion).toContain('import { LibraryPdfPreview } from "./LibraryPdfPreview"');
    expect(companion).toContain("<LibraryPdfPreview");
    expect(companion).not.toContain("<iframe");
    expect(source).toContain("loadLibraryPdfRuntime()");
    expect(runtime).toContain('import("pdfjs-dist/legacy/build/pdf.mjs")');
    expect(runtime).toContain('import("pdfjs-dist/legacy/build/pdf.worker.min.mjs?url")');
    expect(runtime).toContain("pdfjs.GlobalWorkerOptions.workerSrc = worker.default");
    expect(source).toContain("pdfjs.getDocument({ url: sourceURL })");
    expect(source).toContain("page.render({");
    expect(source).toContain("renderTask?.cancel()");
    expect(source).toContain("loadingTask.destroy()");
    expect(source).toContain("props.labels.previousPage");
    expect(source).toContain("props.labels.nextPage");
    expect(source).toContain('className="app-library-ipod app-library-pdf-ipod"');
    expect(source).toContain('data-media-kind="pdf"');
    expect(source).toContain("<IpodControlWheel");
    expect(source).toContain('presentation="ipod"');
    expect(source).toContain('"--app-library-ipod-range-value": `${pageProgress}%`');
    expect(source).toContain("leftDisabled={pageNumber <= 1}");
    expect(source).toContain(
      "rightDisabled={pageCount === 0 || pageNumber >= pageCount}",
    );
    expect(source).toContain("onTop={openDialog}");
    expect(source).toContain("const [dialogOpen, setDialogOpen]");
    expect(source).toContain("<Dialog open={dialogOpen} onOpenChange={setDialogOpen}>");
    expect(source).toContain(
      'className="app-library-pdf-dialog app-media-preview-dialog min-w-0 max-w-none"',
    );
    expect(source).toContain('className="app-media-preview-dialog-header app-library-pdf-dialog__header"');
    expect(source).toContain('className="app-library-pdf-dialog__title"');
    expect(source).toContain("setDialogPageNumber(pageNumber)");
    expect(source).toContain("setDialogZoom(1)");
    expect(source).toContain("pdfZoomAfterWheel(");
    expect(source).toContain("event.deltaMode");
    expect(source).toContain("capturePointerZoomAnchor(");
    expect(source).toContain("restorePointerZoomAnchor(stage, anchor)");
    expect(source).toContain("onWheel={handleDialogWheel}");
    expect(source).toContain("onCanvasLayout={handleDialogCanvasLayout}");
    expect(layout).toContain(".app-library-pdf__stage");
    expect(layout).toContain("overscroll-behavior: contain");
    expect(layout).toContain(
      ".app-library-pdf-ipod__display > .app-library-pdf__stage--ipod",
    );
    expect(layout).toContain(".app-library-pdf-dialog__stage.app-media-preview-dialog-stage");
    expect(appearance).toContain(".app-library-pdf__stage");
    expect(appearance).toContain(
      ".app-library-pdf-ipod__display > .app-library-pdf__stage--ipod",
    );
    expect(workflows).toContain(".app-library-pdf-dialog.app-media-preview-dialog");
    expect(workflows).toContain(".app-library-pdf-dialog__title");
  });
});
