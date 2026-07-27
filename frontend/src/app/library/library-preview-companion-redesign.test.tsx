import { describe, expect, mock, test } from "bun:test";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderToStaticMarkup } from "react-dom/server";

import { COMPLETED_DEFAULT_COVER_IMAGE_URLS } from "@/shared/assets/default-cover";
import type { LibraryWorkspaceItem } from "./types";
import { createLibraryWorkspaceLabels } from "./types";
import { t } from "@/shared/i18n";

mock.module("@wailsio/runtime", () => ({
  Call: { ByID: () => Promise.resolve(undefined), ByName: () => Promise.resolve(undefined) },
  Create: {
    Any: (value: unknown) => value,
    Array: (create: (value: unknown) => unknown) => (values: unknown[]) => values.map(create),
    Nullable: (create: (value: unknown) => unknown) => (value: unknown) => value == null ? value : create(value),
  },
  Events: {
    On: () => () => {},
    Types: { Common: { WindowFullscreen: "window-fullscreen", WindowUnFullscreen: "window-unfullscreen" } },
  },
  Window: { Fullscreen: () => Promise.resolve(), UnFullscreen: () => Promise.resolve() },
}));

const { LibraryPreviewCompanion } = await import("./LibraryPreviewCompanion");

function previewItem(
  overrides: Partial<LibraryWorkspaceItem> = {},
): LibraryWorkspaceItem {
  return {
    id: "preview-item",
    source: "file",
    libraryId: "library-one",
    libraryName: "Home Library",
    title: "Field Recording",
    subtitle: "A quiet morning by the sea",
    category: "audio",
    status: "available",
    format: "FLAC",
    sizeBytes: 8_192,
    durationMs: 91_000,
    createdAt: "2026-01-01T10:00:00Z",
    updatedAt: "2026-02-01T10:00:00Z",
    path: "/Library/field-recording.flac",
    coverURL: COMPLETED_DEFAULT_COVER_IMAGE_URLS.audio,
    fallbackCoverURL: COMPLETED_DEFAULT_COVER_IMAGE_URLS.audio,
    rootId: "preview-item",
    searchText: "field recording audio",
    ...overrides,
  };
}

describe("library preview companion presentation", () => {
  test("ships every catalog value label in each supported locale", async () => {
    const locales = [
      "en",
      "zh-CN",
      "zh-TW",
      "ja-JP",
      "ko-KR",
      "es-419",
      "pt-BR",
      "id-ID",
      "vi-VN",
    ];
    const expectedValues = [
      "original", "representation", "attachment", "artwork", "optimized",
      "thumbnail", "transcript", "subtitle", "preview", "primary",
      "playback", "accessibility", "indexing", "available", "processing",
      "offline", "missing", "corrupt", "manual", "smart", "playlist",
      "album", "shelf", "series", "string", "integer", "number",
      "boolean", "date", "datetime", "durationMs", "object", "array",
      "json", "user", "embedded", "sidecar", "remote", "derived",
      "migration", "system", "desktopLibrary", "localMusicMetadata",
      "embeddedFileMetadata", "mediaAnalysis", "remoteProvider",
      "checksumScanner",
    ];

    for (const locale of locales) {
      const messages = await Bun.file(
        new URL(`../../shared/i18n/locales/${locale}.json`, import.meta.url),
      ).json();
      expect(Object.keys(messages.xiadown.libraryCatalog.valueLabels).sort()).toEqual(
        [...expectedValues].sort(),
      );
      expect(
        Object.values(messages.xiadown.libraryCatalog.valueLabels).every(
          (value) => typeof value === "string" && value.trim().length > 0,
        ),
      ).toBe(true);
    }
  });

  test("puts a centered title above classification-first preview facts", () => {
    const markup = renderToStaticMarkup(
      <LibraryPreviewCompanion item={previewItem()} />,
    );

    expect(markup).toContain("app-library-preview__overview");
    expect(markup).toContain("app-library-preview__hero");
    expect(markup).not.toContain("app-library-preview__identity");
    expect(markup).toContain('class="app-library-preview__title-marquee"');
    expect(markup).toContain('data-overflow="false"');
    expect(markup).toContain('title="Field Recording" aria-label="Field Recording"');
    expect(markup).toContain('<span aria-hidden="true">Field Recording</span>');
    expect(markup).not.toContain("A quiet morning by the sea");
    expect(markup).toContain("Audio");
    expect(markup).not.toContain('data-tone="success"');
    expect(markup).toContain("app-library-preview__facts");
    expect(markup).toContain("app-dialog-list-card app-dialog-list-card-content");
    expect(markup).toContain("app-library-preview__fact app-dialog-row");
    expect(markup).not.toContain("Home Library");
    expect(markup).toContain("<dt>Category</dt>");
    expect(markup).not.toContain("<dt>Status</dt>");
    expect(markup).not.toContain("<dt>Format</dt>");
  });

  test("exposes an explicit rename entry for a persisted standalone file", () => {
    const client = new QueryClient();
    const markup = renderToStaticMarkup(
      <QueryClientProvider client={client}>
        <LibraryPreviewCompanion
          item={previewItem({
            title: "Field Recording.flac",
            file: {
              id: "file-one",
              libraryId: "library-one",
              kind: "audio",
              name: "field-recording.flac",
              displayName: "Field Recording.flac",
              storage: { mode: "local_path", localPath: "/Library/field-recording.flac" },
              origin: { kind: "download" },
              lineage: {},
              metadata: {},
              media: { format: "flac", sizeBytes: 8192 },
              state: { status: "available", deleted: false, archived: false },
              createdAt: "2026-01-01T10:00:00Z",
              updatedAt: "2026-02-01T10:00:00Z",
            },
          })}
        />
      </QueryClientProvider>,
    );

    expect(markup).toContain('data-state="display"');
    expect(markup).toContain('data-placement="hero"');
    expect(markup).toContain('aria-label="Rename: Field Recording.flac"');
    expect(markup).toContain("lucide-pencil-line");
  });

  test("wires each rename model to its matching mutation and keyboard contract", async () => {
    const source = await Bun.file(
      new URL("./LibraryPreviewCompanion.tsx", import.meta.url),
    ).text();

    expect(source).toContain("const renameOperation = useRenameOperation()");
    expect(source).toContain("const renameFile = useRenameFile()");
    expect(source).toContain("const updateCatalogItem = useUpdateCatalogItem()");
    expect(source).toContain("expectedRevision: props.detail.item.revision");
    expect(source).toContain("actorId: LIBRARY_CATALOG_ACTOR_ID");
    expect(source).toContain("key={`task:${operationId}`}");
    expect(source).toContain("key={`file:${props.fileId}`}");
    expect(source).toContain("key={`catalog:${props.detail.item.id}`}");
    expect(source).toContain('event.key === "Enter"');
    expect(source).toContain('event.key === "Escape"');
    expect(source).toContain("event.nativeEvent.isComposing");
    expect(source).toContain("aria-describedby={error ? errorId : undefined}");
    expect(source).toContain("triggerRef.current?.focus()");
    expect(source).toContain("composeProtectedFileDisplayName");
    expect(source).toContain("app-library-preview__inline-rename-error");
  });

  test("keeps a compact selected-item context above non-preview management tabs", () => {
    const client = new QueryClient();
    const markup = renderToStaticMarkup(
      <QueryClientProvider client={client}>
        <LibraryPreviewCompanion item={previewItem()} initialTab="info" />
      </QueryClientProvider>,
    );

    expect(markup).toContain("app-library-preview__context");
    expect(markup).toContain('data-compact="true"');
    expect(markup).toContain('data-library-preview-section="info"');
    expect(markup).toContain("Field Recording");
    expect(markup).toContain("app-library-preview__footer");
  });

  test("uses localized facts and the four-way iPod audio controls", () => {
    const labels = createLibraryWorkspaceLabels(
      (key) => t(key, "zh-CN"),
      "zh-CN",
    );
    const zh = (key: string) => t(key, "zh-CN");
    const markup = renderToStaticMarkup(
      <LibraryPreviewCompanion
        httpBaseURL="http://127.0.0.1:43127"
        item={previewItem({ libraryName: "" })}
        labels={labels}
      />,
    );

    expect(markup).not.toContain(zh("xiadown.libraryCatalog.library"));
    expect(markup).toContain(`<dt>${zh("xiadown.libraryCatalog.category")}</dt>`);
    expect(markup).not.toContain(`<dt>${zh("xiadown.libraryCatalog.status")}</dt>`);
    expect(markup).not.toContain("Library · FLAC");
    expect(markup).not.toContain(labels.dateTimeValue(previewItem().updatedAt));
    expect(markup).not.toContain("Feb 1, 2026");
    expect(markup).toContain('class="app-library-ipod"');
    expect(markup).toContain('data-media-kind="audio"');
    expect(markup).toContain(`aria-label="${zh("xiadown.listen.play")}"`);
    expect(markup).toContain(`aria-label="${zh("xiadown.listen.seek")}"`);
    expect(markup).toContain(`aria-label="${zh("xiadown.listen.volume")}"`);
    expect(markup).toContain('data-position="top"');
    expect(markup).toContain('data-position="left"');
    expect(markup).toContain('data-position="right"');
    expect(markup).toContain('data-position="bottom"');
    expect(markup).toContain("<audio");
    expect(markup).not.toContain("<audio controls");
    expect(markup).not.toContain("lucide-heart");
    expect(markup).not.toContain("lucide-download");
    expect(markup).not.toContain("lucide-repeat");
    expect(markup).not.toContain("lucide-more-horizontal");
    expect(labels.catalogValueLabel("desktop-library")).toBe(
      zh("xiadown.libraryCatalog.valueLabels.desktopLibrary"),
    );
    expect(labels.catalogValueLabel("local-music-metadata")).toBe(
      zh("xiadown.libraryCatalog.valueLabels.localMusicMetadata"),
    );
  });

  test("uses the iPod screen and four-way wheel for video previews", () => {
    const labels = createLibraryWorkspaceLabels(
      (key) => t(key, "en"),
      "en",
    );
    const markup = renderToStaticMarkup(
      <LibraryPreviewCompanion
        httpBaseURL="http://127.0.0.1:43127"
        item={previewItem({
          category: "video",
          format: "MP4",
          path: "/Library/field-recording.mp4",
        })}
        labels={labels}
      />,
    );

    expect(markup).toContain('data-media-kind="video"');
    expect(markup).toContain("app-library-ipod__screen");
    expect(markup).toContain("app-library-ipod__wheel");
    expect(markup).toContain(`aria-label="${labels.play}"`);
    expect(markup).toContain(`aria-label="${labels.seek}"`);
    expect(markup).toContain(`aria-label="${labels.volume}"`);
    expect(markup).toContain("app-library-ipod__range-mode");
    expect(markup).toContain(`aria-label="${labels.preview}"`);
    expect(markup).toContain("lucide-eye");
    expect(markup).not.toContain("lucide-maximize");
    expect(markup).not.toContain("lucide-heart");
    expect(markup).not.toContain("lucide-download");
    expect(markup).not.toContain("lucide-repeat");
    expect(markup).not.toContain("lucide-more-horizontal");
  });

  test("uses the localized Preview label for the companion shell title", async () => {
    const source = await Bun.file(
      new URL("../main/MainApp.tsx", import.meta.url),
    ).text();

    expect(source).toContain('t("xiadown.libraryCatalog.preview")');
    expect(source).not.toContain('selectedLibraryItem?.title || "Library Preview"');
  });

  test("styles Preview as an artwork-first companion with grouped facts and context", async () => {
    const [css, appearanceCss, source, controls, statusContract] = await Promise.all([
      Bun.file(new URL("./library.css", import.meta.url)).text(),
      Bun.file(
        new URL("../../shared/styles/dream/library.css", import.meta.url),
      ).text(),
      Bun.file(new URL("./LibraryPreviewCompanion.tsx", import.meta.url)).text(),
      Bun.file(
        new URL("../../shared/styles/dream/controls.css", import.meta.url),
      ).text(),
      Bun.file(
        new URL("../../shared/styles/dream/status-contract.css", import.meta.url),
      ).text(),
    ]);

    expect(css).toMatch(
      /\.app-library-preview__overview\s*\{[^}]*display:\s*grid[^}]*max-width:\s*22rem/s,
    );
    expect(css).toMatch(
      /\.app-library-preview__body\s*\{[^}]*min-width:\s*0[^}]*overflow-x:\s*hidden[^}]*overflow-y:\s*auto/s,
    );
    expect(css).toMatch(
      /\.app-library-preview__overview\s*\{[^}]*min-width:\s*0/s,
    );
    expect(css).toMatch(
      /\.app-library-preview__facts > dl\s*\{[^}]*display:\s*block/s,
    );
    expect(css).toMatch(
      /\.app-library-preview__title-marquee\s*\{[^}]*white-space:\s*nowrap/s,
    );
    expect(appearanceCss).toMatch(
      /\.app-library-preview__title-marquee\s*\{[^}]*text-align:\s*center/s,
    );
    expect(css).toMatch(
      /\.app-library-preview__title-marquee\[data-overflow="true"\] > span\s*\{[^}]*animation:[^}]*infinite alternate/s,
    );
    expect(css).toContain("@keyframes app-library-preview-title-bounce");
    expect(css).toMatch(
      /@media \(prefers-reduced-motion: reduce\)[\s\S]*?\.app-library-preview__title-marquee\[data-overflow="true"\] > span\s*\{[^}]*text-overflow:\s*ellipsis[^}]*animation:\s*none/s,
    );
    expect(source).toContain("content.scrollWidth - viewport.clientWidth");
    expect(source).toContain("const observer = new ResizeObserver(measure)");
    expect(source).toContain("overflow <= 1");
    expect(source).toContain('shift: `-${distance}px`');
    expect(source).toContain("measurement.title === title ? measurement.overflow : 0");
    expect(css).toMatch(
      /\.app-library-preview__fact\s*\{[^}]*min-height:\s*var\(--app-settings-row-compact-height/s,
    );
    expect(css).toMatch(
      /\.app-library-preview__context\s*\{[^}]*grid-template-columns:\s*3\.5rem minmax\(0, 1fr\)/s,
    );
    expect(appearanceCss).toContain('.app-library-preview__status[data-app-status-badge="true"]');
    expect(statusContract).toContain('[data-tone="success"]');
    expect(css).toMatch(
      /\.app-library-preview__media > \.app-library-artwork--placeholder\s*\{[^}]*display:\s*grid[^}]*place-items:\s*center/s,
    );
    expect(css).not.toContain(".app-library-preview__metadata-summary");
    expect(css).toContain(".app-library-preview__tab-content");
    expect(css).toMatch(
      /\.app-library-preview\s*\{[^}]*container: library-preview \/ inline-size;/s,
    );
    expect(css).toContain(".app-library-preview__tabs .app-dream-segment-switch-tab");
    expect(css).toContain(".app-library-ipod__wheel");
    expect(css).toContain(".app-library-ipod__screen");
    expect(css).toMatch(
      /\.app-library-preview__task-file > header\s*\{[^}]*display:\s*grid[^}]*grid-template-columns:\s*minmax\(0, 1fr\) auto/s,
    );
    expect(css).toMatch(
      /\.app-library-preview__inline-rename\[data-placement="row"\]\s+\.app-library-preview__inline-rename-display\s*\{[^}]*display:\s*grid[^}]*grid-template-columns:\s*minmax\(0, 1fr\) auto/s,
    );
    expect(css).toMatch(
      /\.app-library-preview__task-file > header strong\s*\{[^}]*display:\s*block[^}]*max-width:\s*100%[^}]*overflow:\s*hidden[^}]*text-overflow:\s*ellipsis[^}]*white-space:\s*nowrap/s,
    );
    expect(css).toMatch(
      /\.app-library-preview__media\[data-preview-kind="video"\]\s*\{[^}]*overflow:\s*hidden/s,
    );
    expect(appearanceCss).toMatch(
      /\.app-library-preview__media\s*\{[^}]*border-radius:\s*var\(--app-radius-media\)/s,
    );
    expect(appearanceCss).toMatch(
      /\.app-library-ipod\s*\{[^}]*border-radius:\s*1\.45rem[^}]*background:/s,
    );
    expect(appearanceCss).toMatch(
      /\.app-library-preview__media:has\(> \.app-library-ipod\)\s*\{[^}]*overflow:\s*visible[^}]*border-radius:\s*1\.45rem[^}]*background:\s*transparent[^}]*box-shadow:\s*none/s,
    );
    expect(appearanceCss).toMatch(
      /\.app-library-ipod__screen\s*\{[^}]*overflow:\s*hidden[^}]*border:\s*1px solid hsl\(0 0% 0% \/ 0\.72\)[^}]*background:\s*hsl\(220 14% 5%\)/s,
    );
    expect(appearanceCss).toMatch(
      /\.app-library-ipod__display\s*\{[^}]*border-radius:\s*0\.42rem[^}]*clip-path:\s*inset\(0 round 0\.42rem\)/s,
    );
    expect(appearanceCss).toMatch(
      /\.app-library-ipod__display > :is\(video, \.app-library-artwork\),[\s\S]*?\{[^}]*border-radius:\s*inherit/s,
    );
    expect(appearanceCss).toMatch(
      /\.app-library-ipod__wheel\s*\{[^}]*border-radius:\s*50%[^}]*background:/s,
    );
    expect(appearanceCss).toMatch(
      /\.app-library-ipod__wheel-button:not\(:disabled\):hover,\s*\.app-library-ipod__wheel-button\[aria-pressed="true"\]\s*\{[^}]*background:\s*transparent[^}]*color:\s*hsl\(var\(--app-accent-text,/s,
    );
    expect(appearanceCss).toMatch(
      /\.app-library-ipod__wheel-button\[aria-pressed="true"\]\s*>\s*svg\s*\{[^}]*filter:\s*drop-shadow/s,
    );
    expect(appearanceCss).toMatch(
      /\.app-library-ipod__wheel-button:focus-visible\s*\{[^}]*color:\s*hsl\(var\(--app-accent-text,[^}]*outline:\s*none/s,
    );
    expect(appearanceCss).toMatch(
      /\.app-library-ipod__wheel-button:focus-visible\s*>\s*svg\s*\{[^}]*filter:\s*drop-shadow/s,
    );
    expect(appearanceCss).not.toMatch(
      /\.app-library-preview__media\[data-preview-kind="video"\]\s*\{[^}]*(?:background:\s*transparent|box-shadow:\s*none)/s,
    );
    expect(css).toMatch(
      /\.app-library-preview__task-folder-artwork\s*\{[^}]*width:\s*min\(62%, 10\.5rem\)[^}]*aspect-ratio:\s*5 \/ 6[^}]*justify-self:\s*center/s,
    );
    expect(css).toMatch(
      /\.app-library-preview__task-folder-artwork\[data-presentation="companion-open"\]\s*\{[^}]*aspect-ratio:\s*16 \/ 9/s,
    );
    expect(css).toMatch(
      /\.app-library-preview__body\s*\{[^}]*padding-inline:\s*var\(--app-workspace-companion-gutter, 1\.25rem\)/s,
    );
    expect(css).toMatch(
      /\.app-library-preview__tabs-frame\s*\{[^}]*margin:\s*0 var\(--app-workspace-companion-gutter, 1\.25rem\) 0\.85rem/s,
    );
    expect(appearanceCss).toMatch(
      /\.app-library-preview__tabs\.app-dream-segment-switch\s*\{[^}]*width:\s*100%[^}]*min-width:\s*0[^}]*max-width:\s*none/s,
    );
    expect(css).not.toMatch(
      /\.app-library-preview__tabs \.app-dream-segment-switch-indicator\s*\{[^}]*display:\s*none/s,
    );
    expect(controls).toMatch(
      /\.app-dream-segment-switch\[data-count="4"\]:is\(\[data-index="1"\], \[data-index="2"\]\)[\s\S]*?border-radius:\s*0;/s,
    );
    expect(controls).toMatch(
      /\.app-dream-segment-switch\[data-count="4"\]\[data-index="3"\][\s\S]*?border-radius:\s*0 var\(--app-radius-lg\) var\(--app-radius-lg\) 0;/s,
    );
    expect(controls).toMatch(
      /\.app-dream-segment-switch-tab:focus-visible[\s\S]*?box-shadow:\s*inset 0 0 0 1px var\(--dream-line-focus\)/s,
    );
    expect(css).not.toContain("app-library-preview__metadata-editor");
    expect(source).not.toContain("function CatalogInfoEditor");
    expect(source).toContain("function CatalogInfoPanel");
  });
});
