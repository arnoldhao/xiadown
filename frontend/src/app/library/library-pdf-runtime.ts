let runtimePromise: Promise<
  typeof import("pdfjs-dist/legacy/build/pdf.mjs")
> | undefined;

/**
 * Shares the PDF.js module and worker URL setup across the Companion reader
 * and the bounded Library-card thumbnail queue.
 */
export function loadLibraryPdfRuntime() {
  runtimePromise ??= Promise.all([
    import("pdfjs-dist/legacy/build/pdf.mjs"),
    import("pdfjs-dist/legacy/build/pdf.worker.min.mjs?url"),
  ]).then(([pdfjs, worker]) => {
    pdfjs.GlobalWorkerOptions.workerSrc = worker.default;
    return pdfjs;
  });
  return runtimePromise;
}
