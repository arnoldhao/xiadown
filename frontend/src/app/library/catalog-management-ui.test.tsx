import { describe, expect, mock, test } from "bun:test";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderToStaticMarkup } from "react-dom/server";

import type {
  CatalogItemDetail,
} from "@/shared/contracts/catalog";
import { COMPLETED_DEFAULT_COVER_IMAGE_URLS } from "@/shared/assets/default-cover";
import type { LibraryDTO } from "@/shared/contracts/library";
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

const { catalogKeys } = await import("@/shared/query/catalog");
const { LIBRARY_LIST_QUERY_KEY } = await import("@/shared/query/library");
const { adaptCatalogItems } = await import("./catalog-adapter");
const {
  LibraryPreviewCompanion,
  resolveCatalogPreviewMedia,
} = await import("./LibraryPreviewCompanion");

const detail: CatalogItemDetail = {
  item: {
    id: "item-1",
    catalogId: "catalog-1",
    category: "video",
    status: "active",
    availability: "available",
    title: "Catalog Film",
    sortTitle: "Catalog Film",
    description: "Verified description",
    primaryFileId: "file-1",
    revision: 7,
    createdAt: "2026-01-01T10:00:00Z",
    updatedAt: "2026-02-01T10:00:00Z",
  },
  assets: [{
    id: "asset-1",
    itemId: "item-1",
    fileId: "file-1",
    role: "original",
    label: "Original master",
    position: 0,
    fileAvailable: true,
    availability: "available",
    file: {
      id: "file-1",
      libraryId: "legacy-1",
      kind: "video",
      name: "catalog-film.mov",
      displayName: "Catalog Film Master",
      fileName: "catalog-film.mov",
      storage: { mode: "local_path", localPath: "/Volumes/Media/catalog-film.mov" },
      origin: { kind: "download" },
      lineage: {},
      metadata: { title: "Catalog Film" },
      media: { format: "MOV", codec: "prores", durationMs: 9_000, sizeBytes: 1_024 },
      state: { status: "available", deleted: false, archived: false },
      createdAt: "2026-01-01T10:00:00Z",
      updatedAt: "2026-02-01T10:00:00Z",
    },
    createdAt: "2026-01-01T10:00:00Z",
    updatedAt: "2026-02-01T10:00:00Z",
  }],
  source: {
    originKind: "download",
    storageMode: "managed",
    storageRootId: "root-downloads",
    storageRootName: "XiaDown Downloads",
    storageRootPath: "/Users/arnold/Downloads",
    operationId: "operation-download-1",
  },
  representations: [{
    id: "representation-1",
    catalogId: "catalog-1",
    itemId: "item-1",
    assetId: "asset-1",
    kind: "optimized",
    purpose: "playback",
    mediaType: "video/mp4",
    container: "mp4",
    codec: "h264",
    width: 1920,
    height: 1080,
    durationMs: 9_000,
    sizeBytes: 512,
    availability: "available",
    revision: 2,
    createdAt: "2026-01-02T10:00:00Z",
    updatedAt: "2026-02-02T10:00:00Z",
  }],
  metadata: [{
    id: "metadata-1",
    catalogId: "catalog-1",
    itemId: "item-1",
    namespace: "media",
    key: "director",
    valueType: "string",
    valueJson: '"A. Director"',
    position: 0,
    source: "embedded",
    provenance: "ffprobe:format.tags",
    confidence: 0.94,
    locked: true,
    revision: 1,
    createdAt: "2026-01-01T10:00:00Z",
    updatedAt: "2026-01-01T10:00:00Z",
  }],
  tags: [],
};

function workspaceItem(): LibraryWorkspaceItem {
  return adaptCatalogItems([detail.item], {
    httpBaseURL: "http://127.0.0.1:1",
  })[0]!;
}

function renderCatalog(
  tab: "preview" | "info" | "versions" | "activity",
  seed?: (client: QueryClient) => void,
  labels?: ReturnType<typeof createLibraryWorkspaceLabels>,
) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  client.setQueryData(catalogKeys.item("item-1", ""), detail);
  seed?.(client);
  return renderToStaticMarkup(
    <QueryClientProvider client={client}>
      <LibraryPreviewCompanion item={workspaceItem()} initialTab={tab} httpBaseURL="http://127.0.0.1:1" labels={labels} />
    </QueryClientProvider>,
  );
}

function renderPendingCatalog(tab: "preview" | "info") {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return renderToStaticMarkup(
    <QueryClientProvider client={client}>
      <LibraryPreviewCompanion
        item={workspaceItem()}
        initialTab={tab}
        httpBaseURL="http://127.0.0.1:1"
      />
    </QueryClientProvider>,
  );
}

describe("catalog workspace adapter", () => {
  test("groups fine-grained other kinds without collapsing healthy items into unknown", () => {
    const base = { ...detail.item, category: "other" as const };
    const items = adaptCatalogItems([
      { ...base, id: "document", kind: "document" },
      { ...base, id: "font", kind: "font", format: "woff2", sizeBytes: 2048 },
      { ...base, id: "archive", kind: "archive" },
      { ...base, id: "review", kind: "document", status: "needs_review" },
    ]);

    expect(items.map((item) => item.otherGroup)).toEqual([
      "document",
      "font",
      "archive",
      "needs-review",
    ]);
    expect(items[1]?.format).toBe("woff2");
    expect(items[1]?.sizeBytes).toBe(2048);
    expect(items[1]?.coverURL).toBe(COMPLETED_DEFAULT_COVER_IMAGE_URLS.font);
  });
});

describe("catalog item companion", () => {
  test("keeps the simplified Preview anatomy while catalog details load", () => {
    const previewMarkup = renderPendingCatalog("preview");
    const infoMarkup = renderPendingCatalog("info");

    expect(previewMarkup).toContain("app-library-preview__overview--pending");
    expect(previewMarkup).toContain("app-library-preview__title-marquee");
    expect(previewMarkup).not.toContain("app-library-preview__identity");
    expect(previewMarkup).toContain("<dt>Category</dt>");
    expect(previewMarkup).not.toContain("<dt>Status</dt>");
    expect(previewMarkup).toContain("Loading");
    expect(infoMarkup).toContain("app-library-preview__identity");
    expect(infoMarkup).toContain('data-compact="true"');
  });

  test("previews the original catalog media through the trusted desktop asset URL", () => {
    const markup = renderCatalog("preview");
    expect(markup).toContain("app-library-preview__overview");
    expect(markup).toContain("app-library-preview__hero");
    expect(markup).toContain('class="app-library-preview__title-marquee"');
    expect(markup).toContain('title="Catalog Film" aria-label="Catalog Film"');
    expect(markup).toContain('aria-label="Rename: Catalog Film"');
    expect(markup).not.toContain("Verified description");
    expect(markup).toContain("<dt>Category</dt>");
    expect(markup).not.toContain("<dt>Status</dt>");
    expect(markup).not.toContain("<dt>Format</dt>");
    expect(markup).not.toContain('data-tone="success"');
    expect(markup).not.toContain("app-library-preview__metadata-summary");
    expect(markup).not.toContain("media.director");
    expect(markup).not.toContain("A. Director");
    expect(markup).toContain('data-preview-kind="video"');
    expect(markup).toContain('data-media-kind="video"');
    expect(markup).toContain("/api/library/video-thumbnail/item-1");
    expect(markup).toContain("app-library-preview__delete-button");
    const media = resolveCatalogPreviewMedia(
      detail,
      workspaceItem().fallbackCoverURL ?? workspaceItem().coverURL,
      "http://127.0.0.1:1",
    );
    expect(media.sourceAsset?.id).toBe("asset-1");
    expect(media.sourcePath).toBe("/Volumes/Media/catalog-film.mov");
    expect(media.sourceURL).toContain("/api/library/asset/catalog-film.mov?path=%2FVolumes%2FMedia%2Fcatalog-film.mov");

    const imageDetail: CatalogItemDetail = {
      ...detail,
      item: {
        ...detail.item,
        id: "image-1",
        category: "image",
        title: "Original Portrait",
      },
      assets: detail.assets.map((asset) => ({
        ...asset,
        id: "image-asset-1",
        itemId: "image-1",
        file: asset.file ? {
          ...asset.file,
          id: "image-file-1",
          kind: "image",
          name: "portrait.png",
          fileName: "portrait.png",
          storage: { ...asset.file.storage, localPath: "/Volumes/Photos/portrait.png" },
          media: { ...asset.file.media, format: "PNG" },
        } : undefined,
      })),
    };
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    client.setQueryData(catalogKeys.item("image-1", ""), imageDetail);
    const imageMarkup = renderToStaticMarkup(
      <QueryClientProvider client={client}>
        <LibraryPreviewCompanion
          item={adaptCatalogItems([imageDetail.item])[0]!}
          initialTab="preview"
          httpBaseURL="http://127.0.0.1:1"
        />
      </QueryClientProvider>,
    );
    expect(imageMarkup).toContain('data-preview-kind="image"');
    expect(imageMarkup).toContain("/api/library/asset/portrait.png?path=%2FVolumes%2FPhotos%2Fportrait.png");
  });

  test("explains an offline volume without offering a broken file preview", () => {
    const offlineDetail: CatalogItemDetail = {
      ...detail,
      item: { ...detail.item, availability: "offline" },
      assets: detail.assets.map((asset) => ({
        ...asset,
        fileAvailable: false,
        availability: "offline",
      })),
      representations: detail.representations.map((representation) => ({
        ...representation,
        availability: "offline",
      })),
    };
    const seed = (client: QueryClient) => {
      client.setQueryData(catalogKeys.item("item-1", ""), offlineDetail);
    };
    const previewMarkup = renderCatalog("preview", seed);
    const infoMarkup = renderCatalog("info", seed);

    expect(previewMarkup).toContain('data-availability="offline"');
    expect(previewMarkup).toContain("Offline");
    expect(previewMarkup).toContain("XiaDown Downloads");
    expect(previewMarkup).not.toContain("<dt>Status</dt>");
    expect(previewMarkup).not.toContain(
      "/api/library/asset/catalog-film.mov?path=",
    );
    expect(infoMarkup).toContain("<dt>Availability</dt>");
    expect(infoMarkup).toContain("<span>Offline</span>");
    expect(infoMarkup).not.toContain('aria-label="Open location"');
  });

  test("uses an available transcode after the downloaded original was replaced", () => {
    const replacementDetail: CatalogItemDetail = {
      ...detail,
      item: {
        ...detail.item,
        id: "item-replacement",
        title: "Replacement Episode",
      },
      assets: [
        {
          ...detail.assets[0]!,
          id: "asset-source-webm",
          itemId: "item-replacement",
          fileId: "file-source-webm",
          fileAvailable: false,
          file: detail.assets[0]!.file ? {
            ...detail.assets[0]!.file,
            id: "file-source-webm",
            name: "episode.webm",
            fileName: "episode.webm",
            storage: { mode: "local_path", localPath: "/Volumes/Media/episode.webm" },
            media: { format: "WEBM", videoCodec: "vp9", durationMs: 9_000, sizeBytes: 1_024 },
            state: { status: "deleted", deleted: true, archived: false },
          } : undefined,
        },
        {
          ...detail.assets[0]!,
          id: "asset-output-mp4",
          itemId: "item-replacement",
          fileId: "file-output-mp4",
          role: "representation",
          label: "MP4",
          fileAvailable: true,
          file: detail.assets[0]!.file ? {
            ...detail.assets[0]!.file,
            id: "file-output-mp4",
            kind: "transcode",
            name: "episode.mp4",
            fileName: "episode.mp4",
            storage: { mode: "local_path", localPath: "/Volumes/Media/episode.mp4" },
            lineage: { rootFileId: "file-source-webm" },
            media: { format: "MP4", videoCodec: "h264", durationMs: 9_000, sizeBytes: 512 },
            state: { status: "active", deleted: false, archived: false },
          } : undefined,
        },
      ],
      representations: [
        {
          ...detail.representations[0]!,
          id: "representation-source-webm",
          itemId: "item-replacement",
          assetId: "asset-source-webm",
          kind: "original",
          purpose: "primary",
          container: "webm",
          codec: "vp9",
          availability: "missing",
        },
        {
          ...detail.representations[0]!,
          id: "representation-output-mp4",
          itemId: "item-replacement",
          assetId: "asset-output-mp4",
          kind: "optimized",
          purpose: "playback",
          container: "mp4",
          codec: "h264",
          availability: "available",
        },
      ],
    };
    const renderReplacement = (tab: "preview" | "info") => {
      const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
      client.setQueryData(catalogKeys.item("item-replacement", ""), replacementDetail);
      return renderToStaticMarkup(
        <QueryClientProvider client={client}>
          <LibraryPreviewCompanion
            item={adaptCatalogItems([replacementDetail.item])[0]!}
            initialTab={tab}
            httpBaseURL="http://127.0.0.1:1"
          />
        </QueryClientProvider>,
      );
    };

    const previewMedia = resolveCatalogPreviewMedia(
      replacementDetail,
      COMPLETED_DEFAULT_COVER_IMAGE_URLS.video,
      "http://127.0.0.1:1",
    );
    expect(previewMedia.sourceAsset?.id).toBe("asset-output-mp4");
    expect(previewMedia.sourceURL).toContain("/api/library/asset/episode.mp4?path=%2FVolumes%2FMedia%2Fepisode.mp4");
    expect(previewMedia.sourceURL).not.toContain("episode.webm");

    const infoMarkup = renderReplacement("info");
    expect(infoMarkup).toContain("/Volumes/Media/episode.mp4");
    expect(infoMarkup).toContain("MP4");
    expect(infoMarkup).not.toContain("episode.webm");
  });

  test("does not reuse a cross-platform unsupported artwork file as the video poster", () => {
    const heicDetail: CatalogItemDetail = {
      ...detail,
      representations: [
        {
          ...detail.representations[0]!,
          id: "representation-heic-thumbnail",
          assetId: "asset-heic-cover",
          kind: "thumbnail",
          purpose: "preview",
          mediaType: "image/heic",
        },
        ...detail.representations,
      ],
      assets: [
        ...detail.assets,
        {
          ...detail.assets[0]!,
          id: "asset-heic-cover",
          fileId: "file-heic-cover",
          role: "artwork",
          label: "Camera cover",
          file: detail.assets[0]!.file ? {
            ...detail.assets[0]!.file,
            id: "file-heic-cover",
            kind: "thumbnail",
            name: "cover.heic",
            fileName: "cover.heic",
            storage: { mode: "local_path", localPath: "/Volumes/Media/cover.heic" },
          } : undefined,
        },
      ],
    };
    const media = resolveCatalogPreviewMedia(
      heicDetail,
      COMPLETED_DEFAULT_COVER_IMAGE_URLS.video,
      "http://127.0.0.1:1",
    );
    expect(media.posterURL).toBeUndefined();
    expect(media.sourcePath).toBe("/Volumes/Media/catalog-film.mov");
    expect(media.sourceURL).not.toContain("cover.heic");
  });

  test("shows file info as a direct card without preview-only actions", () => {
    const markup = renderCatalog("info");
    expect(markup).toContain("Catalog Film");
    expect(markup).toContain('aria-label="Rename: Catalog Film"');
    expect(markup).not.toContain("Verified description");
    expect(markup).toContain("/Volumes/Media/catalog-film.mov");
    expect(markup).toContain("<dt>Format</dt>");
    expect(markup).toContain("<dt>Size</dt>");
    expect(markup).toContain("<dt>Source</dt>");
    expect(markup).toContain("Downloaded");
    expect(markup).toContain("<dt>Storage mode</dt>");
    expect(markup).toContain("Managed by XiaDown");
    expect(markup).toContain("XiaDown Downloads");
    expect(markup).toContain("operation-download-1");
    expect(markup).toContain("app-library-preview__location-value");
    expect(markup).toContain('aria-label="Open Directory"');
    expect(markup).toContain("app-library-preview__copy-value");
    expect(markup).toContain(
      'data-library-preview-section="info"><dl class="app-library-preview__info app-dialog-list-card app-dialog-list-card-content"',
    );
    expect(markup).not.toContain("app-library-preview__info-panel");
    expect(markup).not.toContain("Revision");
    expect(markup).not.toContain("Save changes");
    expect(markup).not.toContain("app-library-preview__delete-button");
    expect(markup).not.toContain("Delete</button>");
  });

  test("describes referenced imports without inventing download information", () => {
    const markup = renderCatalog("info", (client) => {
      client.setQueryData(catalogKeys.item("item-1", ""), {
        ...detail,
        source: {
          originKind: "import",
          storageMode: "referenced",
          storageRootId: "root-reference",
          storageRootName: "Creator Media",
          storageRootPath: "/Volumes/Creator",
          importBatchId: "batch-reference-1",
          importPath: "/Volumes/Creator/catalog-film.mov",
          importedAt: "2026-01-03T10:00:00Z",
          keepSourceFile: true,
        },
      } satisfies CatalogItemDetail);
    });

    expect(markup).toContain("Referenced import");
    expect(markup).toContain("Reference existing files");
    expect(markup).toContain("Creator Media");
    expect(markup).toContain("/Volumes/Creator/catalog-film.mov");
    expect(markup).toContain("batch-reference-1");
    expect(markup).not.toContain("Related task");
  });

  test("reveals catalog files by persisted file id and legacy files by local path", async () => {
    const source = await Bun.file(
      new URL("./LibraryPreviewCompanion.tsx", import.meta.url),
    ).text();

    expect(source).toContain("useOpenLibraryFileLocation");
    expect(source).toContain("useOpenLibraryPath");
    expect(source).toContain('await openFileLocation.mutateAsync({ fileId: locationKey });');
    expect(source).toContain('await openPath.mutateAsync({ path: locationKey });');
    expect(source).toContain('{ kind: "catalog-file", fileId: file.id }');
    expect(source).toContain('{ kind: "legacy-path", path: props.item.path }');
  });

  test("renders persisted assets and representations as versions", () => {
    const markup = renderCatalog("versions");
    expect(markup).toContain("Original master");
    expect(markup).toContain("/Volumes/Media/catalog-film.mov");
    expect(markup).toContain("Optimized");
    expect(markup).toContain("video/mp4");
    expect(markup).toContain("1920 × 1080");
  });

  test("localizes catalog values and dates with the app locale", () => {
    const labels = createLibraryWorkspaceLabels(
      (key) => t(key, "zh-CN"),
      "zh-CN",
    );
    const zh = (key: string) => t(`xiadown.libraryCatalog.${key}`, "zh-CN");
    const versionsMarkup = renderCatalog("versions", undefined, labels);
    expect(versionsMarkup).toContain(zh("valueLabels.original"));
    expect(versionsMarkup).toContain(zh("valueLabels.optimized"));
    expect(versionsMarkup).toContain(
      `${zh("valueLabels.playback")} · ${zh("valueLabels.available")}`,
    );
    expect(versionsMarkup).not.toContain(">optimized<");

    expect(labels.catalogValueLabel("music.local-metadata-editor")).toBe(
      zh("valueLabels.localMusicMetadata"),
    );
    expect(labels.catalogValueLabel("video")).toBe(zh("video"));
    expect(labels.catalogValueLabel("audio")).toBe(zh("audio"));
    expect(labels.catalogValueLabel("book")).toBe(zh("books"));
    expect(labels.catalogValueLabel("image")).toBe(zh("images"));
    expect(labels.catalogValueLabel("other")).toBe(zh("others"));

    const infoMarkup = renderCatalog("info", undefined, labels);
    expect(infoMarkup).toContain(labels.dateTimeValue(detail.item.createdAt));
    expect(infoMarkup).not.toContain("Jan 1, 2026");
  });

  test("uses the persisted user activity instead of global migration noise", () => {
    const activityMarkup = renderCatalog("activity", (client) => client.setQueryData(
      catalogKeys.activity("item-1", 20),
      [{
        action: "catalog_item_restored",
        revision: 7,
        actor: "desktop-library",
        occurredAt: "2026-03-01T10:00:00Z",
      }],
    ));
    expect(activityMarkup).toContain("Restore item");
    expect(activityMarkup).toContain("Revision 7");
    expect(activityMarkup).toContain("Desktop library");
    expect(activityMarkup).toContain("Mar 1, 2026");
    expect(activityMarkup).not.toContain("Migration audit");
  });

  test("merges durable file events for the Catalog item's backing assets", () => {
    const backingLibrary: LibraryDTO = {
      version: "current",
      id: "legacy-1",
      name: "Library",
      createdAt: "2026-01-01T10:00:00Z",
      updatedAt: "2026-03-02T10:00:00Z",
      createdBy: { source: "test" },
      files: [detail.assets[0]!.file!],
      records: {
        history: [],
        workspaceStates: [],
        fileEvents: [{
          id: "file-event-1",
          libraryId: "legacy-1",
          fileId: "file-1",
          eventType: "file_relinked",
          detail: {
            cause: { category: "maintenance", actor: "desktop-library" },
            before: {
              fileId: "file-1",
              name: "Catalog Film Master",
              localPath: "/Volumes/Old/catalog-film.mov",
            },
            after: {
              fileId: "file-1",
              name: "Catalog Film Master",
              localPath: "/Volumes/Media/catalog-film.mov",
            },
            changes: [{
              field: "localPath",
              before: "/Volumes/Old/catalog-film.mov",
              after: "/Volumes/Media/catalog-film.mov",
            }],
          },
          occurredAt: "2026-03-02T10:00:00Z",
          createdAt: "2026-03-02T10:00:00Z",
        }, {
          id: "file-event-2",
          libraryId: "legacy-1",
          fileId: "file-1",
          eventType: "file_renamed",
          detail: {
            cause: { category: "file_metadata", actor: "desktop-library" },
            before: { fileId: "file-1", name: "Catalog Film Master" },
            after: { fileId: "file-1", name: "Archive Film Master" },
            changes: [{
              field: "displayName",
              before: "Catalog Film Master",
              after: "Archive Film Master",
            }],
          },
          occurredAt: "2026-03-03T10:00:00Z",
          createdAt: "2026-03-03T10:00:00Z",
        }],
      },
    };
    const activityMarkup = renderCatalog("activity", (client) => {
      client.setQueryData(catalogKeys.activity("item-1", 20), []);
      client.setQueryData(LIBRARY_LIST_QUERY_KEY, [backingLibrary]);
    });

    expect(activityMarkup).toContain("Updated · Location");
    expect(activityMarkup).toContain(
      "/Volumes/Old/catalog-film.mov → /Volumes/Media/catalog-film.mov",
    );
    expect(activityMarkup).toContain("Updated · Title");
    expect(activityMarkup).toContain(
      "Catalog Film Master → Archive Film Master",
    );
    expect(activityMarkup).toContain("Mar 2, 2026");
  });
});
