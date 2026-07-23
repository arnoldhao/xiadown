import { describe, expect, mock, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";

import { COMPLETED_DEFAULT_COVER_IMAGE_URLS } from "@/shared/assets/default-cover";
import { t } from "@/shared/i18n";
import {
  createLibraryWorkspaceLabels,
  formatLibraryRelativeTime,
  type LibraryWorkspaceItem,
} from "./types";

mock.module("@wailsio/runtime", () => ({
  Call: {
    ByID: () => Promise.resolve(undefined),
    ByName: () => Promise.resolve(undefined),
  },
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

mock.module("@/shared/ui/tooltip", () => ({
  TOOLTIP_ALIGNS: ["start", "center", "end"],
  TOOLTIP_SIDES: ["top", "bottom", "left", "right"],
  Tooltip: ({ children }: { children?: unknown }) => children,
  TooltipContent: () => null,
  TooltipProvider: ({ children }: { children?: unknown }) => children,
  TooltipTrigger: ({ children }: { children?: unknown }) => children,
}));

const { LibraryWorkspacePage } = await import("./LibraryWorkspacePage");

const missingItem: LibraryWorkspaceItem = {
  id: "missing-video",
  source: "file",
  libraryId: "library-one",
  libraryName: "Home Library",
  title: "Missing video",
  subtitle: "MP4",
  category: "video",
  otherGroup: "missing",
  status: "missing",
  format: "MP4",
  sizeBytes: 8_192,
  durationMs: 91_000,
  createdAt: "2026-01-01T10:00:00Z",
  updatedAt: "2026-02-01T10:00:00Z",
  path: "/Library/missing-video.mp4",
  coverURL: COMPLETED_DEFAULT_COVER_IMAGE_URLS.video,
  fallbackCoverURL: COMPLETED_DEFAULT_COVER_IMAGE_URLS.video,
  rootId: "missing-video",
  searchText: "missing video mp4",
};

describe("Library item presentation", () => {
  test("keeps status semantics and tone without painting a badge surface", async () => {
    const appearanceCss = await Bun.file(
      new URL("../../shared/styles/dream/library.css", import.meta.url),
    ).text();

    expect(appearanceCss).toMatch(
      /:root \.app-library-items\[data-view\] \.app-library-item__status\[data-app-status-badge="true"\]\s*\{[^}]*border-color:\s*transparent;[^}]*background:\s*transparent;[^}]*box-shadow:\s*none;/s,
    );
  });

  test("keeps duration out of imagery and renders exceptional state as a semantic label", () => {
    const markup = renderToStaticMarkup(
      <LibraryWorkspacePage route="all" items={[missingItem]} initialView="grid" />,
    );

    const artworkStart = markup.indexOf("app-library-item__artwork");
    const artworkEnd = markup.indexOf("</span>", artworkStart);
    expect(markup.slice(artworkStart, artworkEnd)).not.toContain("1:31");
    expect(markup).toContain('aria-label="Duration: 1:31"');
    expect(markup).toContain('data-app-status-badge="true"');
    expect(markup).toContain("app-library-artwork");
    expect(markup).not.toContain("app-library-task-folder");
    expect(markup).toContain('data-tone="danger"');
    expect(markup).toContain('aria-label="Status: Missing"');
    expect(markup).not.toContain(">missing</span>");
    const classification = markup.indexOf("app-library-item__classification");
    const status = markup.indexOf("app-library-item__status", classification);
    const type = markup.indexOf("app-library-item__type", classification);
    const time = markup.indexOf("app-library-item__time", type);
    const size = markup.indexOf("8.0 KB", time);
    const duration = markup.indexOf("1:31", size);
    expect(status).toBeGreaterThan(classification);
    expect(type).toBeGreaterThan(status);
    expect(time).toBeGreaterThan(type);
    expect(markup).toContain('dateTime="2026-02-01T10:00:00Z"');
    expect(markup).toContain('aria-label="Updated:');
    expect(markup.slice(type, markup.indexOf("</span>", type))).toContain("MP4");
    expect(size).toBeGreaterThan(type);
    expect(duration).toBeGreaterThan(size);
  });

  test("uses a static multi-file folder while keeping the task count outside artwork", () => {
    const task: LibraryWorkspaceItem = {
      ...missingItem,
      id: "task-one",
      source: "task",
      category: "task",
      status: "running",
      format: "video_download",
      taskPreviewItems: [
        {
          id: "task-cover",
          kind: "thumbnail",
          previewURL: "http://127.0.0.1:43127/assets/task-cover.jpg",
          label: "JPG",
        },
        { id: "task-video", kind: "video", label: "MP4" },
        { id: "task-subtitle", kind: "subtitle", label: "VTT" },
      ],
      taskPreviewTotalCount: 4,
      operation: {
        operationId: "task-one",
        libraryId: "library-one",
        name: "Task one",
        kind: "video_download",
        status: "running",
        correlation: {},
        metrics: { fileCount: 4, totalSizeBytes: 8_192 },
        createdAt: "2026-01-01T10:00:00Z",
      },
    };
    const markup = renderToStaticMarkup(
      <LibraryWorkspacePage route="tasks" items={[task]} initialView="grid" />,
    );
    const taskCardStart = markup.indexOf('data-item-id="task-one"');
    const taskCardEnd = markup.indexOf("</button>", taskCardStart);
    const taskCard = markup.slice(taskCardStart, taskCardEnd);
    const folderStart = taskCard.indexOf('data-task-folder-artwork="true"');
    const copyStart = taskCard.indexOf("app-library-item__copy", folderStart);

    expect(markup).toContain("Download");
    expect(markup).not.toContain("Video download");
    expect(markup).toContain("4 items");
    expect(markup).toContain('class="app-library-task-folder ');
    expect(markup).toContain('data-task-folder-artwork="true"');
    expect(markup).toContain(
      'app-library-task-folder app-library-item__artwork app-library-item__artwork--task-folder',
    );
    expect(taskCard).not.toContain('<span class="app-library-item__artwork"');
    expect(markup).toContain('data-view="grid"');
    expect(markup.match(/class="app-library-task-folder__page"/g)).toHaveLength(2);
    expect(markup).not.toContain("app-library-task-folder__overflow");
    expect(markup).not.toContain("app-library-task-folder__tab");
    expect(markup).not.toContain('aria-label="Duration: 1:31"');
    expect(taskCardStart).toBeGreaterThan(0);
    expect(taskCardEnd).toBeGreaterThan(taskCardStart);
    expect(folderStart).toBeGreaterThan(0);
    expect(copyStart).toBeGreaterThan(folderStart);
    expect(taskCard.slice(folderStart, copyStart)).toContain('aria-hidden="true"');
    expect(taskCard.slice(folderStart, copyStart)).toContain('focusable="false"');
    expect(taskCard.slice(folderStart, copyStart)).not.toContain("<title");
    expect(taskCard.slice(folderStart, copyStart)).not.toContain("4 items");
    expect(taskCard).not.toContain("<button");
    expect(taskCard).not.toContain("<a");
    expect(taskCard).not.toContain("tabindex=");
  });

  test("localizes task sources and formats update time in the active locale", () => {
    const fixedNow = Date.parse("2026-07-19T12:00:00Z");
    expect(
      formatLibraryRelativeTime("2026-07-19T10:00:00Z", "en", fixedNow),
    ).toBe("2 hr. ago");
    expect(
      formatLibraryRelativeTime("2026-07-19T10:00:00Z", "zh-CN", fixedNow),
    ).toBe(
      new Intl.RelativeTimeFormat("zh-CN", { numeric: "auto", style: "short" })
        .format(-2, "hour"),
    );
    expect(formatLibraryRelativeTime("not-a-date", "zh-CN", fixedNow)).toBe("");

    const labels = createLibraryWorkspaceLabels(
      (key) => t(key, "zh-CN"),
      "zh-CN",
    );
    expect(labels.operationKindLabel("video_download")).toBe(
      t("xiadown.running.downloadBadge", "zh-CN"),
    );
    expect(labels.operationKindLabel("audio-transcode")).toBe(
      t("xiadown.running.transcodeBadge", "zh-CN"),
    );
    expect(labels.operationKindLabel("library_import")).toBe(
      t("xiadown.libraryData.importTab", "zh-CN"),
    );
    expect(labels.operationStageLabel("i18n:library.status.succeeded")).toBe(
      t("library.status.succeeded", "zh-CN"),
    );
    expect(
      labels.operationStageLabel("i18n:library.status.succeeded?source=task"),
    ).toBe(t("library.status.succeeded", "zh-CN"));
  });

  test("uses the same portrait artwork and text width for every Grid category", async () => {
    const css = await Bun.file(new URL("./library.css", import.meta.url)).text();

    expect(css).toMatch(
      /\.app-library-item__artwork\s*\{[^}]*aspect-ratio:\s*5 \/ 6;/s,
    );
    expect(css).toMatch(
      /\.app-library-items\[data-view="grid"\] \.app-library-item__artwork,[\s\S]*?\.app-library-items\[data-view="grid"\] \.app-library-item__copy\s*\{[^}]*width:\s*min\(84%, 10\.75rem\);[^}]*margin-inline:\s*auto;/,
    );
    expect(css).toMatch(
      /\.app-library-items\[data-view="list"\] \.app-library-item__artwork\s*\{[^}]*width:\s*2\.7rem;/s,
    );
  });
});
