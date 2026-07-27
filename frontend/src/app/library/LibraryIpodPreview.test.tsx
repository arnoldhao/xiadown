import { describe, expect, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";

import { t } from "@/shared/i18n";

import {
  clampImageZoom,
  clampPlaybackTime,
  imageZoomAfterWheel,
  LibraryIpodPreview,
} from "./LibraryIpodPreview";
import { createLibraryWorkspaceLabels } from "./types";

describe("LibraryIpodPreview", () => {
  test("clamps seek and image zoom controls to safe media bounds", () => {
    expect(clampPlaybackTime(-10, 90)).toBe(0);
    expect(clampPlaybackTime(45, 90)).toBe(45);
    expect(clampPlaybackTime(100, 90)).toBe(90);
    expect(clampPlaybackTime(Number.NaN, 90)).toBe(0);
    expect(clampImageZoom(0.1)).toBe(0.5);
    expect(clampImageZoom(1.75)).toBe(1.75);
    expect(clampImageZoom(8)).toBe(3);
    expect(clampImageZoom(Number.NaN)).toBe(1);
    expect(imageZoomAfterWheel(1, -100)).toBeGreaterThan(1);
    expect(imageZoomAfterWheel(1, 100)).toBeLessThan(1);
    expect(imageZoomAfterWheel(3, -100)).toBe(3);
    expect(imageZoomAfterWheel(0.5, 100)).toBe(0.5);
  });

  test("renders image zoom, dialog, reset and four-way wheel controls", () => {
    const labels = createLibraryWorkspaceLabels((key) => t(key, "en"), "en");
    const markup = renderToStaticMarkup(
      <LibraryIpodPreview
        category="image"
        coverURL="http://127.0.0.1:43127/cover.png"
        labels={labels}
        sourceURL="http://127.0.0.1:43127/photo.png"
        title="Photo"
      />,
    );

    expect(markup).toContain('data-media-kind="image"');
    expect(markup).toContain('min="0.5"');
    expect(markup).toContain('max="3"');
    expect(markup).toContain('aria-label="Preview"');
    expect(markup).toContain('aria-label="Reset"');
    expect(markup.match(/app-library-ipod__wheel-button/g)).toHaveLength(4);
    expect(markup).toContain("lucide-zoom-in");
    expect(markup).toContain("lucide-zoom-out");
    expect(markup).toContain("lucide-rotate-ccw");
  });

  test("keeps a fallback-aware artwork layer over video until playback presents a frame", () => {
    const labels = createLibraryWorkspaceLabels((key) => t(key, "en"), "en");
    const markup = renderToStaticMarkup(
      <LibraryIpodPreview
        category="video"
        coverURL="http://127.0.0.1:43127/video-thumbnail/item-1"
        fallbackCoverURL="/default-video-cover.svg"
        labels={labels}
        sourceURL="http://127.0.0.1:43127/movie.mp4"
        title="Movie"
      />,
    );

    expect(markup).toContain("app-library-ipod__video-poster");
    expect(markup).toContain('src="http://127.0.0.1:43127/video-thumbnail/item-1"');
    expect(markup).not.toContain(" poster=");
    expect(markup).toContain('class="app-library-ipod__range-mode"');
    expect(markup).toContain('data-position="top"');
    expect(markup).toContain('aria-label="Preview"');
    expect(markup).toContain("lucide-eye");
  });

  test("keeps unsupported file artwork inside the same dialog-first iPod shell", () => {
    const labels = createLibraryWorkspaceLabels((key) => t(key, "en"), "en");
    const markup = renderToStaticMarkup(
      <LibraryIpodPreview
        category="other"
        coverURL="/default-file-cover.svg"
        labels={labels}
        title="Archive"
      />,
    );

    expect(markup).toContain('data-media-kind="other"');
    expect(markup).toContain("app-library-ipod__screen");
    expect(markup).toContain("app-library-ipod__wheel");
    expect(markup).toContain('data-position="top"');
    expect(markup).toContain('aria-label="Preview"');
  });

  test("keeps long-title landscape artwork inside a stable media dialog", async () => {
    const [source, appearance] = await Promise.all([
      Bun.file(new URL("./LibraryIpodPreview.tsx", import.meta.url)).text(),
      Bun.file(
        new URL("../../shared/styles/dream/workflows.css", import.meta.url),
      ).text(),
    ]);

    expect(source).toContain(
      'className="app-library-ipod-dialog app-media-preview-dialog min-w-0 max-w-none"',
    );
    expect(source).toContain(
      'className="app-library-ipod-dialog__stage app-media-preview-dialog-stage"',
    );
    expect(source).toContain(
      'className="app-library-ipod-dialog__image app-media-preview-dialog-image"',
    );
    expect(source).toContain(
      'className="app-media-preview-dialog-header app-library-ipod-dialog__header"',
    );
    expect(source).toContain('className="app-library-ipod-dialog__title"');
    expect(source).toContain("event.deltaMode");
    expect(source).toContain("capturePointerZoomAnchor(");
    expect(source).toContain("restorePointerZoomAnchor(stage, anchor)");
    expect(source).toContain('className="app-library-ipod__range-mode"');
    expect(source).toContain(
      'className="app-library-playable-dialog app-media-preview-dialog min-w-0 max-w-none"',
    );
    expect(source).toContain("topIcon={<Eye");
    expect(source).toContain("onTop={() => updateDialogOpen(true)}");
    expect(source).toContain('className="app-library-ipod-dialog__zoom-content"');
    expect(source).toContain('className="app-library-ipod-dialog__zoom-indicator"');
    expect(source).toContain("title={props.title}");
    expect(appearance).toMatch(
      /\.app-library-ipod-dialog\.app-media-preview-dialog\s*\{[^}]*width:\s*min\([^}]*52rem,[^}]*calc\(100vw - 2rem\),[^}]*calc\(177\.7778vh - 8\.75rem\)[^}]*\);[^}]*min-width:\s*0;[^}]*max-width:\s*none;[^}]*max-height:\s*calc\(100vh - 2rem\);[^}]*grid-template-rows:\s*auto minmax\(0, 1fr\);/s,
    );
    expect(appearance).toMatch(
      /\.app-library-ipod-dialog__title\s*\{[^}]*display:\s*block;[^}]*overflow:\s*hidden;[^}]*padding-inline-end:\s*2rem;[^}]*text-overflow:\s*ellipsis;[^}]*white-space:\s*nowrap;/s,
    );
    expect(appearance).toMatch(
      /\.app-library-ipod-dialog__stage\.app-media-preview-dialog-stage\s*\{[^}]*width:\s*100%;[^}]*min-width:\s*0;[^}]*height:\s*auto;[^}]*min-height:\s*0;[^}]*max-height:\s*none;[^}]*aspect-ratio:\s*16 \/ 9;[^}]*overflow:\s*auto;/s,
    );
    expect(appearance).toMatch(
      /\.app-media-preview-dialog,[\s\S]*?\.app-sniff-desk-preview-dialog\s*\{[^}]*max-height:\s*min\(86vh, 44rem\);[^}]*overflow:\s*hidden;/s,
    );
    expect(appearance).toMatch(
      /\.app-media-preview-dialog-image,[\s\S]*?\.app-sniff-desk-preview-dialog-image\s*\{[^}]*width:\s*100%;[^}]*height:\s*100%;[^}]*max-width:\s*100%;[^}]*max-height:\s*100%;[^}]*object-fit:\s*contain;/s,
    );
  });
});
