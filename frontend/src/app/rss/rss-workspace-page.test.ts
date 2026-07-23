import { describe, expect, test } from "bun:test";
import rssDocumentStyles from "../../shared/styles/dream/rss-documents.css?raw";

import {
  applyRSSStateToEntry,
  boundedRSSEntryImages,
  buildRSSArticlePrintDocument,
  buildRSSReaderDocument,
  DEFAULT_RSS_READER_PREFERENCES,
  filterAndSortRSSSubscriptions,
  mergeRSSEntryPages,
  normalizeRSSReaderPreferences,
  readRSSIdentityScopedBoolean,
  readRSSReaderImageContextMessage,
  readRSSReaderLayoutMessage,
  readRSSReaderLinkMessage,
  readRSSReaderOutlineMessage,
  readRSSReaderProgressMessage,
  readRSSReaderSelectionMessage,
  readRSSReaderWheelMessage,
  resolveRSSReaderVideoEmbed,
  resolveRSSCollectionPresentation,
  rssEntryImageCandidates,
  RSS_READER_MAX_DOCUMENT_HEIGHT,
  RSS_READER_MAX_OUTLINE_ITEMS,
  RSS_READER_OUTLINE_ID_PREFIX,
  setRSSIdentityScopedBoolean,
  resolveRSSReaderDocumentSnapshot,
  resolveRSSAudioPresentation,
  resolveRSSReaderOutlineMarkerMetrics,
  resolveRSSReaderOutlineMarkers,
  resolveRSSReaderOutlineProgress,
  rssReaderScrollFraction,
  rssReaderSpeechText,
  rssReaderVideoDownloadURL,
  rssReaderVideoEmbedURL,
  rssReaderWheelPixels,
  resolveRSSWorkspaceShortcut,
  shouldAcceptRSSResumedPlaybackProgress,
  toggleRSSIdentityScopedBoolean,
  updateEntryStateInCache,
} from "./workspace-utils";
import { buildRSSReadStateRequest } from "./state-utils";
import type { RSSEntry, RSSEntryPage, RSSSubscription } from "./types";

function entry(id: string, overrides: Partial<RSSEntry> = {}): RSSEntry {
  return {
    id,
    subscriptionId: "subscription-1",
    externalId: id,
    title: `Entry ${id}`,
    contentHtml: '<p><a href="next">Next</a><img src="images/cover.jpg"></p>',
    kind: "article",
    imageUrls: [],
    media: [],
    stateRevision: 1,
    revision: 1,
    createdAt: "2026-07-13T00:00:00Z",
    modifiedAt: "2026-07-13T00:00:00Z",
    ...overrides,
  };
}

function subscription(id: string, overrides: Partial<RSSSubscription> = {}): RSSSubscription {
  return {
    id,
    workspaceId: "rss-default",
    feedUrl: `https://example.com/${id}.xml`,
    title: id,
    viewType: "auto",
    enabled: true,
    unreadCount: 0,
    createdAt: "2026-07-13T00:00:00Z",
    updatedAt: "2026-07-13T00:00:00Z",
    revision: 1,
    ...overrides,
  };
}

describe("RSS workspace reading interactions", () => {
  test("reads the visible article body without serialized summary markup", () => {
    expect(rssReaderSpeechText(entry("article", {
      title: "Article title",
      summary: "&lt;p&gt;Duplicated summary&lt;/p&gt;",
      contentHtml: "<p>First paragraph.</p><p>Second paragraph.</p>",
    }))).toBe("First paragraph. Second paragraph.");
    expect(rssReaderSpeechText(entry("article", {
      title: "Fallback title",
      summary: "<p>Summary text</p>",
      contentHtml: "",
    }))).toBe("Summary text");
    expect(rssReaderSpeechText(entry("article", {
      contentHtml: '<p>Example: &lt;Component prop="x"&gt;Body&lt;/Component&gt;</p>',
    }))).toBe('Example: <Component prop="x">Body</Component>');
  });

  test("requires an explicit video layout instead of promoting an auto article subscription", () => {
    expect(resolveRSSCollectionPresentation("all", subscription("auto-video", {
      viewType: "auto",
      resolvedViewType: "video",
    }))).toBe("article");
    expect(resolveRSSCollectionPresentation("all", subscription("explicit-video", {
      viewType: "video",
      resolvedViewType: "video",
    }))).toBe("video");
    expect(resolveRSSCollectionPresentation("video")).toBe("video");
  });

  test("normalizes persisted reader preferences without allowing a single-pane open mode", () => {
    expect(normalizeRSSReaderPreferences({
      autoMarkRead: false,
      openMode: "original",
      fontSize: "large",
      density: "relaxed",
    })).toEqual({
      autoMarkRead: false,
      openMode: "original",
      fontSize: "large",
      density: "relaxed",
    });
    expect(normalizeRSSReaderPreferences({ openMode: "focused" })).toEqual(
      DEFAULT_RSS_READER_PREFERENCES,
    );
  });

  test("maps RSS shortcuts while ignoring modified, repeated, or composing keys", () => {
    expect(resolveRSSWorkspaceShortcut({ key: "j" })).toBe("next-entry");
    expect(resolveRSSWorkspaceShortcut({ key: "K" })).toBe("previous-entry");
    expect(resolveRSSWorkspaceShortcut({ key: "Enter" })).toBe("open-entry");
    expect(resolveRSSWorkspaceShortcut({ key: "Escape" })).toBe("close-entry");
    expect(resolveRSSWorkspaceShortcut({ key: "m" })).toBe("toggle-read");
    expect(resolveRSSWorkspaceShortcut({ key: "s" })).toBe("toggle-starred");
    expect(resolveRSSWorkspaceShortcut({ key: "u" })).toBe("toggle-unread-filter");
    expect(resolveRSSWorkspaceShortcut({ key: "r" })).toBe("refresh");
    expect(resolveRSSWorkspaceShortcut({ key: "j", metaKey: true })).toBeNull();
    expect(resolveRSSWorkspaceShortcut({ key: "j", repeat: true })).toBeNull();
    expect(resolveRSSWorkspaceShortcut({ key: "j", isComposing: true })).toBeNull();
  });

  test("detects only projected audio enclosures for the podcast layout", () => {
    const token = "a".repeat(64);
    const audioURL = `http://127.0.0.1:43127/_xiadown/${token}/api/rss/entries/podcast/resources/media-0`;
    expect(resolveRSSAudioPresentation(entry("podcast", {
      media: [{
        kind: "audio",
        mimeType: "audio/mpeg",
        durationMs: 62_000,
        url: audioURL,
      }],
    }))).toEqual({
      url: audioURL,
      mimeType: "audio/mpeg",
      durationMs: 62_000,
    });
    expect(resolveRSSAudioPresentation(entry("remote", {
      media: [{ kind: "audio", url: "https://cdn.example.com/episode.mp3" }],
    }))).toBeNull();
  });

  test("does not carry route filters or reader modes across identities", () => {
    const initial = { identity: "", enabled: false };
    const allUnread = toggleRSSIdentityScopedBoolean(initial, "all");
    expect(readRSSIdentityScopedBoolean(allUnread, "all")).toBeTrue();
    expect(readRSSIdentityScopedBoolean(allUnread, "articles")).toBeFalse();

    const originalA = setRSSIdentityScopedBoolean("entry-a", true);
    expect(readRSSIdentityScopedBoolean(originalA, "entry-a")).toBeTrue();
    expect(readRSSIdentityScopedBoolean(originalA, "entry-b")).toBeFalse();
  });

  test("merges every fetched entry page and keeps the latest duplicate entry", () => {
    const pages: RSSEntryPage[] = [
      { items: [entry("one"), entry("two")], total: 3, nextOffset: 2 },
      { items: [entry("two", { title: "Updated" }), entry("three")], total: 3 },
    ];
    expect(mergeRSSEntryPages(pages).map((item) => item.id)).toEqual(["one", "two", "three"]);
    expect(mergeRSSEntryPages(pages)[1]?.title).toBe("Updated");
  });

  test("bounds focused image decoding while preserving unique slot order", () => {
    const token = "a".repeat(64);
    const resource = (index: number) =>
      `http://127.0.0.1:43127/_xiadown/${token}/api/rss/entries/gallery/resources/image-${index}`;
    const images = Array.from({ length: 20 }, (_, index) => resource(index));
    const bounded = boundedRSSEntryImages(entry("gallery", {
      thumbnailUrl: images[0],
      imageUrls: [images[0], "", ...images.slice(1)],
      media: [{ kind: "image", url: resource(20) }],
    }));
    expect(bounded).toHaveLength(12);
    expect(bounded).toEqual(images.slice(0, 12));
    expect(boundedRSSEntryImages(entry("gallery", { imageUrls: images }), 4)).toEqual(images.slice(0, 4));
  });

  test("prefers content-image slots and collapses canonical duplicate projections", () => {
    const token = "a".repeat(64);
    const base = `http://127.0.0.1:43127/_xiadown/${token}/api/rss/entries/gallery/resources`;
    const contentImage = `${base}/image-0`;
    const thumbnail = `${base}/thumbnail`;
    expect(boundedRSSEntryImages(entry("gallery", {
      thumbnailUrl: thumbnail,
      imageUrls: [contentImage],
    }), 1)).toEqual([contentImage]);
    expect(boundedRSSEntryImages(entry("gallery", {
      thumbnailUrl: contentImage,
      imageUrls: [contentImage],
      media: [{ kind: "image", url: contentImage, thumbnail: contentImage }],
    }))).toEqual([contentImage]);
  });

  test("keeps preview and poster candidates thumbnail-first", () => {
    const token = "a".repeat(64);
    const base = `http://127.0.0.1:43127/_xiadown/${token}/api/rss/entries/gallery/resources`;
    const thumbnail = `${base}/thumbnail`;
    const mediaThumbnail = `${base}/media-0-thumbnail`;
    const contentImage = `${base}/image-0`;
    const mediaImage = `${base}/media-0`;
    expect(rssEntryImageCandidates(entry("gallery", {
      thumbnailUrl: thumbnail,
      imageUrls: [contentImage],
      media: [{ kind: "image", url: mediaImage, thumbnail: mediaThumbnail }],
    }))).toEqual([thumbnail, mediaThumbnail, contentImage, mediaImage]);
  });

  test("creates an independent read mutation for each rapid entry click", () => {
    const first = buildRSSReadStateRequest(entry("one", { fieldRevisions: { read: 3, starred: 0, articleProgress: 0, videoProgressSeconds: 0 } }), true);
    const second = buildRSSReadStateRequest(entry("two", { fieldRevisions: { read: 7, starred: 0, articleProgress: 0, videoProgressSeconds: 0 } }), true);
    expect(first).toMatchObject({ id: "one", field: "read", read: true, expectedRevision: 3 });
    expect(second).toMatchObject({ id: "two", field: "read", read: true, expectedRevision: 7 });
    expect(first.mutationId).not.toBe(second.mutationId);
  });

  test("builds a themed sandbox document whose CSP blocks unprojected feed media", () => {
    const document = buildRSSReaderDocument(
      entry("article", { url: "https://example.com/posts/one?x=1&y=2" }),
      "dark",
    );
    expect(document).toContain('<html class="app-rss-reader-document" data-theme="dark"');
    expect(document).toContain(`<style>${rssDocumentStyles}</style>`);
    expect(rssDocumentStyles).toContain('html.app-rss-reader-document[data-theme="dark"]');
    expect(rssDocumentStyles).toContain("color-scheme: dark");
    expect(document).toContain('<base href="https://example.com/posts/one?x=1&amp;y=2" target="_blank">');
    expect(document).toContain('src="images/cover.jpg"');
    expect(document).toContain('<meta name="referrer" content="no-referrer">');
    expect(document).toContain("img-src data:; media-src data:");
    expect(document).not.toContain("img-src https:");
    expect(document).not.toContain("media-src https:");
    expect(document).toContain("script-src 'nonce-");
    expect(document).toContain("frame-src 'none'");
    expect(document).toContain("xiadown-rss-reader-v1");
    expect(document).toContain('type:"layout"');
    expect(document).toContain('type:"wheel"');
    expect(document).toContain('type:"selection"');
    expect(document).toContain('type:"link"');
    expect(document).toContain('type:"image-context"');
    expect(document).toContain('event.isTrusted');
    expect(document).toContain('addEventListener("pointermove"');
    expect(rssDocumentStyles).toContain("html.app-rss-reader-document body");
    expect(rssDocumentStyles).not.toContain("max-width: 65ch");
    expect(rssDocumentStyles).toContain("padding: 24px 0 64px");
    expect(rssDocumentStyles).toContain("font-family: system-ui, -apple-system, BlinkMacSystemFont");
    expect(document).toContain("--app-rss-reader-font-size:16px");
    expect(document).toContain("--app-rss-reader-line-height:1.75");
    expect(rssDocumentStyles).toContain("overflow-wrap: anywhere");
    expect(rssDocumentStyles).toContain("text-wrap: wrap");
    expect(rssDocumentStyles).toContain("-webkit-user-select: text");
    expect(rssDocumentStyles).toContain("user-select: text");
    expect(rssDocumentStyles).toContain("html.app-rss-reader-document ::selection");
    expect(rssDocumentStyles).toContain("margin: 0 0 var(--app-rss-reader-paragraph-spacing)");
    expect(rssDocumentStyles).toMatch(/html\.app-rss-reader-document :is\(img, video\) \{[^}]*-webkit-user-drag: none;[^}]*-webkit-user-select: none;/s);
    expect(rssDocumentStyles).toContain("html.app-rss-reader-document a:focus-visible");
    expect(rssDocumentStyles).toContain("html.app-rss-reader-document :not(pre) > code");
    expect(rssDocumentStyles).toContain("html.app-rss-reader-document table");
    expect(rssDocumentStyles).not.toMatch(/html\.app-rss-reader-document blockquote \{[^}]*opacity:/s);
    expect(rssDocumentStyles).toContain("html.app-rss-reader-document :is(h1, h2, h3, h4, h5, h6)");
    expect(document).toContain("Math.hypot(event.clientX-selectionStartX,event.clientY-selectionStartY)<3");
    expect(document).toContain("const selection=document.getSelection()");
    expect(document).toContain("selection.isCollapsed");
    expect(document).toContain('addEventListener("dragstart",stopSelection');
    expect(document).toContain('.replace(/\\s+/g," ")');
    expect(rssDocumentStyles).toMatch(
      /html\.app-rss-reader-document body \{[^}]*font-family: system-ui/s,
    );
  });

  test("projects font size and density preferences into the sandboxed reader", () => {
    const document = buildRSSReaderDocument(
      entry("article"),
      "light",
      undefined,
      {
        autoMarkRead: false,
        openMode: "reader",
        fontSize: "large",
        density: "relaxed",
      },
    );
    expect(document).toContain("--app-rss-reader-font-size:18px");
    expect(document).toContain("--app-rss-reader-line-height:1.9");
    expect(document).toContain("--app-rss-reader-paragraph-spacing:1.5em");
    expect(rssDocumentStyles).toContain(
      "margin: 0 0 var(--app-rss-reader-paragraph-spacing)",
    );
  });

  test("accepts only credential-free HTTP links from the active reader", () => {
    expect(readRSSReaderLinkMessage({
      channel: "xiadown-rss-reader-v1",
      type: "link",
      entryId: "article",
      url: "https://example.com/read?q=one",
    }, "article")).toMatchObject({
      type: "link",
      url: "https://example.com/read?q=one",
    });
    expect(readRSSReaderLinkMessage({
      channel: "xiadown-rss-reader-v1",
      type: "link",
      entryId: "other",
      url: "https://example.com/read",
    }, "article")).toBeNull();
    for (const url of [
      "javascript:alert(1)",
      "data:text/html,escape",
      "https://user:secret@example.com/read",
    ]) {
      expect(readRSSReaderLinkMessage({
        channel: "xiadown-rss-reader-v1",
        type: "link",
        entryId: "article",
        url,
      }, "article")).toBeNull();
    }
  });

  test("reduces an active reader image context message to its persisted slot", () => {
    const token = "a".repeat(64);
    const message = {
      channel: "xiadown-rss-reader-v1",
      type: "image-context",
      entryId: "article",
      documentId: "document-1",
      src: `http://127.0.0.1:43127/_xiadown/${token}/api/rss/entries/article/resources/image-2?v=3`,
      alt: "Cover",
      clientX: 25,
      clientY: 40,
    };
    expect(readRSSReaderImageContextMessage(
      message,
      "article",
      "document-1",
    )).toMatchObject({
      entryId: "article",
      documentId: "document-1",
      slot: "image-2",
      alt: "Cover",
    });
    expect(readRSSReaderImageContextMessage(
      { ...message, entryId: "another" },
      "article",
      "document-1",
    )).toBeNull();
    expect(readRSSReaderImageContextMessage(
      { ...message, documentId: "stale-document" },
      "article",
      "document-1",
    )).toBeNull();
  });

  test("keeps arbitrary RSS documents and trusted embeds out of popup-capable sandboxes", async () => {
    const [pageSource, videoSource] = await Promise.all([
      Bun.file(new URL("./RSSWorkspacePage.tsx", import.meta.url)).text(),
      Bun.file(new URL("./RSSWebVideoPlayback.tsx", import.meta.url)).text(),
    ]);
    expect(pageSource).toContain('sandbox=""');
    expect(pageSource).toContain('sandbox="allow-scripts"');
    expect(pageSource).not.toContain("allow-popups");
    expect(pageSource).not.toContain("allow-same-origin allow-scripts");
    expect(videoSource).toContain(
      'sandbox="allow-scripts allow-same-origin allow-presentation"',
    );
    expect(videoSource).not.toContain("allow-popups");
  });

  test("allows only the projected loopback origin inside reader CSP", () => {
    const token = "a".repeat(64);
    const image = `http://127.0.0.1:43127/_xiadown/${token}/api/rss/entries/article/resources/image-0`;
    const document = buildRSSReaderDocument(entry("article", {
      contentHtml: `<p><img src="${image}"></p>`,
      imageUrls: [image],
    }), "light");
    expect(document).toContain("img-src http://127.0.0.1:43127 data:");
    expect(document).toContain("media-src http://127.0.0.1:43127 data:");
  });

  test("projects known image dimensions into the reader before lazy media lays out", () => {
    const token = "b".repeat(64);
    const image = `http://127.0.0.1:43127/_xiadown/${token}/api/rss/entries/article/resources/image-0`;
    const document = buildRSSReaderDocument(entry("article", {
      contentHtml: `<p>Before</p><img src="${image}"><p>After</p>`,
      imageUrls: [image],
      media: [{
        kind: "image",
        url: image,
        width: 1600,
        height: 900,
      }],
    }), "light");

    expect(document).toContain("const hintedImageDimensions=new Map(");
    expect(document).toContain(`[\"${image}\",1600,900]`);
    expect(document).toContain('image.setAttribute("width",String(hint.width))');
    expect(document).toContain("const imageResizeObserver=new ResizeObserver");
    expect(rssDocumentStyles).toContain(
      'img[data-rss-reader-image-state="loading"]',
    );
  });

  test("keeps trusted webpage video embeds inside the general article reader", () => {
    const videoEntry = entry("video", {
      kind: "video",
      platform: "youtube",
      platformVideoId: "AbCdEfGhI12",
      playbackUrl: "https://www.youtube-nocookie.com/embed/AbCdEfGhI12",
      contentHtml: "<p>Article introduction</p>",
    });
    const embed = resolveRSSReaderVideoEmbed(videoEntry);
    const document = buildRSSReaderDocument(videoEntry, "light");

    expect(embed).toEqual({ provider: "youtube", videoId: "AbCdEfGhI12" });
    expect(rssReaderVideoEmbedURL(embed!)).toBe(
      "https://www.youtube-nocookie.com/embed/AbCdEfGhI12",
    );
    expect(rssReaderVideoDownloadURL(embed!)).toBe(
      "https://www.youtube.com/watch?v=AbCdEfGhI12",
    );
    expect(rssReaderVideoDownloadURL({
      provider: "vimeo",
      videoId: "123456789",
    })).toBe("https://vimeo.com/123456789");
    expect(rssReaderVideoDownloadURL({
      provider: "bilibili",
      videoId: "BV1AbCdEfGh2",
    })).toBe("https://www.bilibili.com/video/BV1AbCdEfGh2/");
    expect(rssReaderVideoDownloadURL({
      provider: "youtube",
      videoId: "invalid",
    })).toBe("");
    expect(document).toContain(
      'data-xiadown-rss-video-provider="youtube" data-xiadown-rss-video-id="AbCdEfGhI12"',
    );
    expect(document).toContain("frame-src 'none'");
    expect(document).toContain("const collectVideoEmbeds=");
    expect(document).toContain('rss-reader-video-embed-action-slot');
    expect(document).toContain('if(!valid){node.remove();return null}');
    expect(document).toContain('node.getClientRects().length===0');
    expect(document).toContain(
      'node.insertAdjacentElement("afterend",action)',
    );
    expect(rssDocumentStyles).toContain(
      '.rss-reader-video-embed-action-slot',
    );
  });

  test("builds a CSP-limited article-only print document", () => {
    const printDocument = buildRSSArticlePrintDocument(
      entry("article", {
        title: "A <safe> title",
        url: "https://example.com/posts/one",
      }),
      "light",
      "https://example.com/",
      {
        source: "Example & Co",
        author: "A. Writer",
        published: "Jul 14, 2026",
      },
    );
    expect(printDocument).toContain("<article><header>");
    expect(printDocument).toContain('<html class="app-rss-print-document" data-theme="light">');
    expect(printDocument).toContain(`<style>${rssDocumentStyles}</style>`);
    expect(printDocument).toContain("A &lt;safe&gt; title");
    expect(printDocument).toContain("Example &amp; Co · A. Writer · Jul 14, 2026");
    expect(printDocument).toContain("script-src 'none'");
    expect(printDocument).toContain("frame-src 'none'");
    expect(printDocument).not.toContain("xiadown-rss-reader-v1");
    expect(printDocument).not.toContain("rss-workspace-page");
  });

  test("keeps the reader srcDoc stable across state-only rerenders", () => {
    const initialEntry = entry("article", {
      articleProgress: { fraction: 0.25, contentRevision: 1 },
    });
    const first = resolveRSSReaderDocumentSnapshot(
      null,
      initialEntry,
      "light",
      "https://example.com/",
    );
    const afterProgress = resolveRSSReaderDocumentSnapshot(
      first,
      {
        ...initialEntry,
        articleProgress: { fraction: 0.75, contentRevision: 1 },
        fieldRevisions: {
          read: 0,
          starred: 0,
          articleProgress: 2,
          videoProgressSeconds: 0,
        },
        stateRevision: 2,
      },
      "light",
      "https://example.com/",
    );
    expect(afterProgress).toBe(first);
    expect(
      resolveRSSReaderDocumentSnapshot(
        first,
        { ...initialEntry, revision: 2 },
        "light",
        "https://example.com/",
      ).document,
    ).not.toBe(first.document);
  });

  test("accepts only bounded progress messages for the active reader entry", () => {
    expect(readRSSReaderProgressMessage({
      channel: "xiadown-rss-reader-v1",
      type: "progress",
      entryId: "article",
      fraction: 0.5,
      anchor: "section-two",
    }, "article")).toMatchObject({ fraction: 0.5, anchor: "section-two" });
    expect(readRSSReaderProgressMessage({
      channel: "xiadown-rss-reader-v1",
      type: "progress",
      entryId: "other",
      fraction: 0.5,
      anchor: "",
    }, "article")).toBeNull();
  });

  test("accepts bounded reader layout messages and computes parent-pane progress", () => {
    expect(readRSSReaderLayoutMessage({
      channel: "xiadown-rss-reader-v1",
      type: "layout",
      entryId: "article",
      height: 1240.2,
    }, "article")).toMatchObject({ height: 1241 });
    expect(readRSSReaderLayoutMessage({
      channel: "xiadown-rss-reader-v1",
      type: "layout",
      entryId: "article",
      documentId: "reader-current",
      sequence: 2,
      height: 1600,
      shifts: [{ top: 240, bottom: 420, delta: 320 }],
      embeds: [{
        provider: "youtube",
        videoId: "AbCdEfGhI12",
        top: 520,
        left: 0,
        width: 720,
        height: 405,
        action: {
          top: 933,
          left: 0,
          width: 720,
          height: 32,
        },
      }],
    }, "article", "reader-current")).toMatchObject({
      height: 1600,
      documentId: "reader-current",
      sequence: 2,
      shifts: [{ top: 240, bottom: 420, delta: 320 }],
      embeds: [{
        provider: "youtube",
        videoId: "AbCdEfGhI12",
        top: 520,
        left: 0,
        width: 720,
        height: 405,
        action: {
          top: 933,
          left: 0,
          width: 720,
          height: 32,
        },
      }],
    });
    expect(readRSSReaderLayoutMessage({
      channel: "xiadown-rss-reader-v1",
      type: "layout",
      entryId: "article",
      height: 1600,
      embeds: [{
        provider: "youtube",
        videoId: "AbCdEfGhI12",
        top: 520,
        left: 0,
        width: 720,
        height: 405,
      }],
    }, "article")).toBeNull();
    expect(readRSSReaderLayoutMessage({
      channel: "xiadown-rss-reader-v1",
      type: "layout",
      entryId: "article",
      height: 1600,
      embeds: [{
        provider: "youtube",
        videoId: "AbCdEfGhI12",
        top: 520,
        left: 0,
        width: 720,
        height: 405,
        action: {
          top: 900,
          left: 0,
          width: 720,
          height: 32,
        },
      }],
    }, "article")).toBeNull();
    expect(readRSSReaderLayoutMessage({
      channel: "xiadown-rss-reader-v1",
      type: "layout",
      entryId: "article",
      documentId: "reader-current",
      height: 1600,
      shifts: [{ top: 420, bottom: 240, delta: 320 }],
    }, "article", "reader-current")).toBeNull();
    expect(readRSSReaderLayoutMessage({
      channel: "xiadown-rss-reader-v1",
      type: "layout",
      entryId: "article",
      documentId: "reader-stale",
      height: 1600,
      shifts: [],
    }, "article", "reader-current")).toBeNull();
    expect(readRSSReaderLayoutMessage({
      channel: "xiadown-rss-reader-v1",
      type: "layout",
      entryId: "article",
      height: 2_000_001,
    }, "article")).toBeNull();
    expect(rssReaderScrollFraction(300, 1000, 400)).toBe(0.5);
    expect(rssReaderScrollFraction(900, 1000, 400)).toBe(1);
    expect(rssReaderScrollFraction(599.25, 1000, 400)).toBe(1);
    expect(rssReaderScrollFraction(598.75, 1000, 400)).toBeCloseTo(0.9979, 4);
    expect(rssReaderScrollFraction(0, 300, 400)).toBe(1);
  });

  test("validates a bounded article outline and computes section progress", () => {
    const outline = readRSSReaderOutlineMessage({
      channel: "xiadown-rss-reader-v1",
      type: "outline",
      entryId: "article",
      items: [
        { id: "intro", title: "Intro", depth: 1, top: 120 },
        { id: "details", title: "Details", depth: 2, top: 520 },
      ],
    }, "article");
    expect(outline?.items).toHaveLength(2);
    expect(resolveRSSReaderOutlineProgress(outline?.items ?? [], 320, 1_000)).toEqual({
      activeIndex: 0,
      sectionFraction: 0.5,
    });
    expect(resolveRSSReaderOutlineProgress(outline?.items ?? [], 760, 1_000)).toEqual({
      activeIndex: 1,
      sectionFraction: 0.5,
    });
    expect(resolveRSSReaderOutlineProgress(
      outline?.items ?? [],
      760,
      1_000,
      { atDocumentEnd: true },
    )).toEqual({
      activeIndex: 1,
      sectionFraction: 1,
    });
    expect(readRSSReaderOutlineMessage({
      channel: "xiadown-rss-reader-v1",
      type: "outline",
      entryId: "article",
      items: [{ id: "bad", title: "Bad", depth: 7, top: 0 }],
    }, "article")).toBeNull();
  });

  test("scales right-aligned outline tracks by section text length", () => {
    expect(resolveRSSReaderOutlineMarkerMetrics([
      { top: 0, textLength: 1_000 },
      { top: 500, textLength: 500 },
      { top: 1_000, textLength: 40 },
    ], 1_500)).toEqual([
      { index: 0, contentLength: 1_000, widthFraction: 1 },
      { index: 1, contentLength: 500, widthFraction: 0.5 },
      { index: 2, contentLength: 40, widthFraction: 0.32 },
    ]);

    // Legacy/fallback outlines do not carry text metrics. Their visual span is
    // still a stable proxy and keeps the longest section at full width.
    expect(resolveRSSReaderOutlineMarkerMetrics([
      { top: 0 },
      { top: 300 },
      { top: 900 },
    ], 1_200)).toEqual([
      { index: 0, contentLength: 300, widthFraction: 0.5 },
      { index: 1, contentLength: 600, widthFraction: 1 },
      { index: 2, contentLength: 300, widthFraction: 0.5 },
    ]);
  });

  test("keeps capped outline markers stable while preserving omitted-chapter progress", () => {
    const outline = Array.from({ length: 25 }, (_, index) => ({
      id: `chapter-${index}`,
      title: `Chapter ${index}`,
      depth: 2 as const,
      top: index * 100,
      textLength: (index + 1) * 10,
    }));
    const markers = resolveRSSReaderOutlineMarkers(
      outline,
      2_500,
      { activeIndex: 13, sectionFraction: 0.5 },
    );

    expect(markers).toHaveLength(12);
    expect(markers[0]?.startIndex).toBe(0);
    expect(markers[markers.length - 1]?.endIndex).toBe(25);
    expect(markers.every((marker, index) => (
      index === 0 || markers[index - 1]?.endIndex === marker.startIndex
    ))).toBe(true);
    const active = markers.find((marker) => marker.active);
    expect(active?.startIndex).toBeLessThanOrEqual(13);
    expect(active?.endIndex).toBeGreaterThan(13);
    expect(active?.fillFraction).toBeGreaterThan(0);
    expect(active?.fillFraction).toBeLessThan(1);

    const completed = resolveRSSReaderOutlineMarkers(
      outline,
      2_500,
      { activeIndex: 24, sectionFraction: 1 },
    );
    expect(completed.every((marker) => marker.fillFraction === 1)).toBe(true);
    expect(completed[completed.length - 1]?.active).toBe(true);
  });

  test("validates optional outline text metrics without breaking legacy messages", () => {
    expect(readRSSReaderOutlineMessage({
      channel: "xiadown-rss-reader-v1",
      type: "outline",
      entryId: "article",
      items: [{ id: "measured", title: "Measured", depth: 2, top: 20, textLength: 640 }],
    }, "article")?.items[0]?.textLength).toBe(640);
    expect(readRSSReaderOutlineMessage({
      channel: "xiadown-rss-reader-v1",
      type: "outline",
      entryId: "article",
      items: [{ id: "legacy", title: "Legacy", depth: 2, top: 20 }],
    }, "article")?.items[0]).not.toHaveProperty("textLength");
    expect(readRSSReaderOutlineMessage({
      channel: "xiadown-rss-reader-v1",
      type: "outline",
      entryId: "article",
      items: [{ id: "bad", title: "Bad", depth: 2, top: 20, textLength: -1 }],
    }, "article")).toBeNull();
  });

  test("sorts, deduplicates, and clamps untrusted reader outline geometry", () => {
    const outline = readRSSReaderOutlineMessage({
      channel: "xiadown-rss-reader-v1",
      type: "outline",
      entryId: "article",
      items: [
        { id: "details", title: "Details", depth: 2, top: 900.4 },
        { id: "intro", title: "Intro", depth: 1, top: -48 },
        { id: "details", title: "Duplicate details", depth: 3, top: 24 },
        { id: "appendix", title: "Appendix", depth: 2, top: RSS_READER_MAX_DOCUMENT_HEIGHT + 500 },
      ],
    }, "article");

    expect(outline?.items).toEqual([
      { id: "intro", title: "Intro", depth: 1, top: 0 },
      { id: "details", title: "Details", depth: 2, top: 900 },
      { id: "appendix", title: "Appendix", depth: 2, top: RSS_READER_MAX_DOCUMENT_HEIGHT },
    ]);
    expect(resolveRSSReaderOutlineProgress(outline?.items ?? [], 450, 1_200)).toEqual({
      activeIndex: 0,
      sectionFraction: 0.5,
    });
  });

  test("accepts exactly the outline item budget and rejects one item beyond it", () => {
    const items = Array.from({ length: RSS_READER_MAX_OUTLINE_ITEMS }, (_, index) => ({
      id: `heading-${index}`,
      title: `Heading ${index}`,
      depth: 2,
      top: index * 10,
    }));
    expect(readRSSReaderOutlineMessage({
      channel: "xiadown-rss-reader-v1",
      type: "outline",
      entryId: "article",
      items,
    }, "article")?.items).toHaveLength(RSS_READER_MAX_OUTLINE_ITEMS);
    expect(readRSSReaderOutlineMessage({
      channel: "xiadown-rss-reader-v1",
      type: "outline",
      entryId: "article",
      items: [...items, { id: "overflow", title: "Overflow", depth: 2, top: 2_000 }],
    }, "article")).toBeNull();
  });

  test("replaces authored duplicate heading ids with collision-free reader namespace ids", () => {
    const readerDocument = buildRSSReaderDocument(entry("article"), "light");
    const script = readerDocument.match(/<script nonce="[^"]+">([\s\S]+)<\/script><\/body>/)?.[1];
    expect(script).toBeTruthy();

    const headings = [
      { id: "duplicate", nodeType: 1, textContent: "First", tagName: "H1", getBoundingClientRect: () => ({ top: 400 }) },
      { id: "duplicate", nodeType: 1, textContent: "Second", tagName: "H2", getBoundingClientRect: () => ({ top: 120 }) },
      { id: "", nodeType: 1, textContent: "Third", tagName: "H3", getBoundingClientRect: () => ({ top: 720 }) },
    ];
    const reserved = { id: `${RSS_READER_OUTLINE_ID_PREFIX}1` };
    const messages: unknown[] = [];
    const fakeDocument = {
      body: { scrollHeight: 1_000 },
      documentElement: { scrollHeight: 1_000 },
      querySelectorAll: (selector: string) => {
        if (selector === "h1,h2,h3,h4,h5,h6") return headings;
        if (selector === "[id]") return [reserved, ...headings];
        return [];
      },
      createTreeWalker: () => {
        const visibleParent = { closest: () => null };
        const nodes = [
          headings[0],
          { nodeType: 3, textContent: "First chapter body", parentElement: visibleParent },
          headings[1],
          { nodeType: 3, textContent: "Short", parentElement: visibleParent },
          headings[2],
          { nodeType: 3, textContent: "Third chapter body is longer", parentElement: visibleParent },
        ];
        let index = 0;
        return { nextNode: () => nodes[index++] ?? null };
      },
    };
    class FakeResizeObserver {
      constructor(_callback: () => void) {}
      observe(_target: unknown) {}
    }
    const run = new Function(
      "document",
      "parent",
      "ResizeObserver",
      "addEventListener",
      "requestAnimationFrame",
      script!,
    );
    run(
      fakeDocument,
      { postMessage: (message: unknown) => messages.push(message) },
      FakeResizeObserver,
      () => undefined,
      (callback: () => void) => {
        callback();
        return 1;
      },
    );

    const ids = headings.map((heading) => heading.id);
    expect(new Set([reserved.id, ...ids]).size).toBe(4);
    expect(ids).toEqual([
      `${RSS_READER_OUTLINE_ID_PREFIX}2`,
      `${RSS_READER_OUTLINE_ID_PREFIX}3`,
      `${RSS_READER_OUTLINE_ID_PREFIX}4`,
    ]);
    const outlineMessage = messages.find((message) => (
      typeof message === "object" && message !== null && "type" in message && message.type === "outline"
    )) as { items?: Array<{ id: string; textLength: number }> } | undefined;
    expect(outlineMessage?.items?.map((item) => item.id)).toEqual(ids);
    expect(outlineMessage?.items?.map((item) => item.textLength)).toEqual([18, 5, 28]);
  });

  test("forwards only bounded iframe wheel gestures to the parent reader", () => {
    const wheel = readRSSReaderWheelMessage({
      channel: "xiadown-rss-reader-v1",
      type: "wheel",
      entryId: "article",
      deltaY: 3,
      deltaMode: 1,
    }, "article");
    expect(wheel).toMatchObject({ deltaY: 3, deltaMode: 1 });
    expect(rssReaderWheelPixels(wheel!, 900)).toBe(48);
    expect(rssReaderWheelPixels({ deltaY: 10, deltaMode: 2 }, 900)).toBe(3600);
    expect(readRSSReaderWheelMessage({
      channel: "xiadown-rss-reader-v1",
      type: "wheel",
      entryId: "other",
      deltaY: 1,
      deltaMode: 0,
    }, "article")).toBeNull();
    expect(readRSSReaderWheelMessage({
      channel: "xiadown-rss-reader-v1",
      type: "wheel",
      entryId: "article",
      deltaY: 10_001,
      deltaMode: 0,
    }, "article")).toBeNull();
  });

  test("accepts only active-reader selection coordinates used for edge auto-scroll", () => {
    expect(readRSSReaderSelectionMessage({
      channel: "xiadown-rss-reader-v1",
      type: "selection",
      entryId: "article",
      active: true,
      clientY: 412.5,
      screenY: 612.5,
    }, "article")).toEqual({
      channel: "xiadown-rss-reader-v1",
      type: "selection",
      entryId: "article",
      active: true,
      clientY: 412.5,
      screenY: 612.5,
    });
    expect(readRSSReaderSelectionMessage({
      channel: "xiadown-rss-reader-v1",
      type: "selection",
      entryId: "other",
      active: true,
      clientY: 412.5,
      screenY: 612.5,
    }, "article")).toBeNull();
    expect(readRSSReaderSelectionMessage({
      channel: "xiadown-rss-reader-v1",
      type: "selection",
      entryId: "article",
      active: true,
      clientY: RSS_READER_MAX_DOCUMENT_HEIGHT + 1,
      screenY: 612.5,
    }, "article")).toBeNull();
  });

  test("updates read state inside every cached infinite-query page", () => {
    const current = {
      pages: [
        { items: [entry("one")], total: 2, nextOffset: 1 },
        { items: [entry("two")], total: 2 },
      ],
      pageParams: [0, 1],
    };
    const next = updateEntryStateInCache(current, {
      entryId: "two",
      subjectId: "subject-two",
      read: true,
      readAt: "2026-07-13T01:00:00Z",
      starred: false,
      videoCompleted: false,
      fieldRevisions: { read: 2, starred: 0, articleProgress: 0, videoProgressSeconds: 0 },
      revision: 2,
      updatedAt: "2026-07-13T01:00:00Z",
    });
    expect(next && "pages" in next ? next.pages[1]?.items[0]?.readAt : undefined).toBe("2026-07-13T01:00:00Z");
  });

  test("does not let a late full-state response roll back a newer field", () => {
    const current = entry("one", {
      starredAt: "2026-07-13T02:00:00Z",
      stateRevision: 4,
      fieldRevisions: {
        read: 1,
        starred: 2,
        articleProgress: 0,
        videoProgressSeconds: 0,
      },
    });
    const merged = applyRSSStateToEntry(current, {
      entryId: "one",
      subjectId: "rss-owner",
      read: true,
      readAt: "2026-07-13T01:00:00Z",
      starred: false,
      videoCompleted: false,
      fieldRevisions: {
        read: 1,
        starred: 1,
        articleProgress: 0,
        videoProgressSeconds: 0,
      },
      revision: 3,
      updatedAt: "2026-07-13T01:00:00Z",
    });
    expect(merged.readAt).toBe("2026-07-13T01:00:00Z");
    expect(merged.starredAt).toBe("2026-07-13T02:00:00Z");
    expect(merged.fieldRevisions?.starred).toBe(2);
    expect(merged.stateRevision).toBe(4);
  });

  test("protects direct-player resume state from an initial zero status", () => {
    expect(
      shouldAcceptRSSResumedPlaybackProgress(0, 180, 20_000, 10_000),
    ).toBeFalse();
    expect(
      shouldAcceptRSSResumedPlaybackProgress(179, 180, 20_000, 10_000),
    ).toBeTrue();
    expect(
      shouldAcceptRSSResumedPlaybackProgress(12, 180, 20_000, 20_000),
    ).toBeTrue();
  });
});

describe("RSS subscription management", () => {
  test("searches metadata and supports title, update, and unread sorting", () => {
    const subscriptions = [
      subscription("alpha", { title: "Alpha", unreadCount: 2, updatedAt: "2026-07-11T00:00:00Z" }),
      subscription("beta", { title: "Beta", description: "Design notes", unreadCount: 9, updatedAt: "2026-07-13T00:00:00Z" }),
    ];
    expect(filterAndSortRSSSubscriptions(subscriptions, "design", "title").map((item) => item.id)).toEqual(["beta"]);
    expect(filterAndSortRSSSubscriptions(subscriptions, "", "updated").map((item) => item.id)).toEqual(["beta", "alpha"]);
    expect(filterAndSortRSSSubscriptions(subscriptions, "", "unread").map((item) => item.id)).toEqual(["beta", "alpha"]);
  });

  test("uses infinite pagination and shared Dream controls without global mutation guards", async () => {
    const [pageSource, remoteImageSource, bilibiliSurfaceSource, bilibiliPlaybackSource, webVideoSource, apiSource, dialogSource, mainSource, css, rssAppearance, dreamControls, dreamTokens] = await Promise.all([
      Bun.file(new URL("./RSSWorkspacePage.tsx", import.meta.url)).text(),
      Bun.file(new URL("./RSSRemoteImage.tsx", import.meta.url)).text(),
      Bun.file(new URL("./RSSBilibiliVideoSurface.tsx", import.meta.url)).text(),
      Bun.file(new URL("./RSSBilibiliPlayback.tsx", import.meta.url)).text(),
      Bun.file(new URL("./RSSWebVideoPlayback.tsx", import.meta.url)).text(),
      Bun.file(new URL("./api.ts", import.meta.url)).text(),
      Bun.file(new URL("./RSSSubscriptionDialog.tsx", import.meta.url)).text(),
      Bun.file(new URL("../main/MainApp.tsx", import.meta.url)).text(),
      Bun.file(new URL("./rss-workspace.css", import.meta.url)).text(),
      Bun.file(new URL("../../shared/styles/dream/rss.css", import.meta.url)).text(),
      Bun.file(new URL("../../shared/styles/dream/controls.css", import.meta.url)).text(),
      Bun.file(new URL("../../shared/styles/dream/tokens.css", import.meta.url)).text(),
    ]);
    expect(pageSource).toContain("useRSSEntriesInfinite");
    expect(pageSource).toContain("useRSSEntry(");
    expect(pageSource).toContain("detailContentRevision");
    expect(pageSource).toContain("buildRSSVideoBatchDownloadTarget(entries)");
    expect(pageSource).toContain("loadedVideoBatchTargets,");
    expect(pageSource).toContain("<RSSDetailHydrationSurface");
    expect(pageSource).toContain("query={detailQuery}");
    expect(pageSource).toContain("renderEntryDetailSurface(selectedListEntry)");
    expect(pageSource).toContain("RSSDetailPlaceholderToolbar");
    expect(pageSource).toContain("<RSSHistorySentinel pagination={historyPagination}");
    expect(pageSource).toContain("!subscriptionId && !collectionId && !categoryId");
    expect(pageSource).toContain("collectionEnabled && subscriptionHistoryReady");
    expect(pageSource).toContain("limit: RSS_ENTRY_PAGE_SIZE");
    expect(pageSource).toContain("isMalformedRSSDynamicRouteID(routeId)");
    expect(pageSource).toContain("resolveRSSCollectionPresentation(collectionRoute, subscription)");
    expect(pageSource).toContain("videoPresentation={shouldUseRSSVideoLayoutPresentation(");
    expect(pageSource).toContain("const focusedVideoPresentation = shouldUseRSSVideoLayoutPresentation(");
    expect(pageSource).not.toContain("dominant[1] / entries.length >= 0.6");
    expect(pageSource).toContain("if (progressTimerRef.current !== null) flushProgress()");
    expect(pageSource).toContain("query.isFetchNextPageError");
    expect(pageSource).toContain("new IntersectionObserver");
    expect(pageSource).toContain('rootMargin: "640px 0px"');
    expect(pageSource).toContain("useRSSBackfillHistory");
    expect(pageSource).toContain("rssBackfillRequestForEntries");
    expect(pageSource).not.toContain("RSSLoadMoreButton");
    expect(pageSource).not.toContain("xiadown.rss.endOfList");
    expect(pageSource).toContain("xiadown.rss.loadMoreFailed");
    expect(pageSource).not.toContain("limit: 300");
    expect(pageSource).not.toContain("setRead.isPending");
    expect(pageSource).not.toContain("window.confirm");
    expect(pageSource).toContain('from "@/shared/ui/select"');
    expect(pageSource).toContain("<Dialog open={Boolean(pendingDelete)}");
    expect(pageSource).toContain("parseRSSSubscriptionsFromOPML");
    expect(pageSource).toContain("exportRSSSubscriptionsToOPML");
    expect(pageSource).toMatch(
      /<RSSHeaderAction[\s\S]{0,160}label=\{t\("xiadown\.rss\.importOPML"\)\}/,
    );
    expect(pageSource).toMatch(
      /<RSSHeaderAction[\s\S]{0,200}label=\{t\("xiadown\.rss\.exportOPML"\)\}/,
    );
    expect(css).not.toContain(".rss-page-heading__actions {");
    expect(pageSource).toContain("for (const item of items)");
    expect(pageSource).toContain("await add.mutateAsync(item)");
    expect(pageSource).toContain("key={entry.id}");
    expect(pageSource).toContain("pending.entryId !== entryIdRef.current");
    expect(pageSource).toContain("boundedRSSEntryImages(props.entry)");
    expect(pageSource).toContain("useRSSMarkAllRead");
    expect(pageSource).toContain("<RSSOrganizationManager");
    expect(pageSource).toContain("selectedSubscriptionIDs={selectedSubscriptionIDs}");
    expect(pageSource).toContain("refresh.mutate(subscriptionId ? { id: subscriptionId } : {})");
    expect(pageSource).toContain("onRefresh={refreshCurrentCollection}");
    expect(pageSource).toContain("ref={managementSurfaceRef}");
    expect(pageSource).toContain("managementSurfaceRef.current");
    expect(pageSource).toContain("RSSDetailToolbar");
    expect(pageSource).toContain("readRSSReaderOutlineMessage");
    expect(pageSource).toContain("rss-reader-progress__segment");
    expect(pageSource).toContain("readRSSReaderSelectionMessage");
    expect(pageSource).toContain("scrollSelectionAtReaderEdge");
    expect(pageSource).toContain("frameBounds.top + selection.clientY");
    expect(pageSource).toContain('window.addEventListener("pointerup", endReaderSelectionInteraction, true)');
    expect(pageSource).toContain('window.addEventListener("pointercancel", endReaderSelectionInteraction, true)');
    expect(pageSource).toContain("resolveRSSReaderOutlineMarkers");
    expect(pageSource).toContain("atDocumentEnd: rssReaderScrollFraction(");
    expect(pageSource).toContain("<SecondaryReveal");
    expect(pageSource).toContain('className="rss-reader-progress__sheet"');
    expect(pageSource).toContain("openDelay={0}");
    expect(pageSource).toContain("pinOnClick={false}");
    expect(pageSource).not.toContain("const revealOnClick = triggerProps.onClick");
    expect(pageSource).toMatch(/onClick=\{\(\) => \{\s*scrollToOutlineItem\(item\);\s*close\(\);/);
    expect(pageSource).toContain("focusRSSReaderProgressSheet(event.currentTarget");
    expect(pageSource).toContain("outlineMarkers.map((marker)");
    expect(pageSource).toContain('aria-current={index === outlineProgress.activeIndex');
    expect(pageSource).toContain("preferredRSSReaderScrollBehavior()");
    expect(pageSource).toContain("rss-reader-progress__value");
    expect(pageSource).toContain("rss-reader-progress__back-to-top");
    expect(pageSource).toContain("printRSSArticle(entry, source, language)");
    expect(pageSource).toContain("buildRSSArticlePrintDocument");
    expect(pageSource).not.toContain("window.print()");
    expect(pageSource).not.toContain("rss-entry-header__actions");
    expect(pageSource).toContain("<RSSBilibiliPlayback");
    expect(pageSource).toContain("<RSSSiteVideoPlayback");
    expect(pageSource).toContain("geometrySuspended={siteMenuOpen}");
    expect(pageSource).toContain('onOpenChange={experience.mode === "site" ? setSiteMenuOpen : undefined}');
    expect(pageSource).toContain("<RSSWebVideoPlayback");
    expect(pageSource).not.toContain("<RSSBilibiliVideoSurface");
    expect(pageSource).not.toContain("<video");
    expect(pageSource).not.toContain("OpenAppSessionSite");
    expect(pageSource).toContain('import { YouTubeVideoCard } from "@/app/youtube/YouTubeWorkspacePage"');
    expect(pageSource).toContain('className="youtube-workspace-scroll rss-video-browse-scroll"');
    expect(pageSource).toContain('className="youtube-workspace-grid"');
    expect(pageSource).toContain("<YouTubeVideoCard");
    expect(pageSource).toContain('className="rss-image-scroll"');
    expect(pageSource).toMatch(/<WorkspacePageContent[\s\S]{0,180}className="rss-image-scroll"[\s\S]{0,220}<div className="rss-image-masonry">/);
    expect(pageSource).toContain("<RSSImageLightbox");
    expect(pageSource).toContain('loading="eager"');
    expect(pageSource).toContain('event.key === "ArrowLeft"');
    expect(pageSource).toContain('event.key === "ArrowRight"');
    expect(pageSource).not.toMatch(/className="rss-image-lightbox"\s+unstyled/);
    expect(pageSource).toContain("thumbnail={(");
    expect(pageSource).toContain("metadataPrefix={<UnreadDot");
    expect(pageSource).not.toContain('className="rss-video-card"');
    expect(pageSource).not.toContain("rss-video-watch-footer");
    expect(remoteImageSource).toContain('loading = "lazy"');
    expect(remoteImageSource).toContain('decoding = "async"');
    expect(remoteImageSource).toContain("probe?.parentElement");
    expect(remoteImageSource).toContain('rootMargin: "320px 0px"');
    expect(remoteImageSource).toContain('loading="eager"');
    expect(remoteImageSource).toContain("src={requested ? controlled : undefined}");
    expect(remoteImageSource).toContain('rss-remote-image--probe');
    expect(remoteImageSource).toContain('rss-image-skeleton');
    expect(remoteImageSource).toContain('data-rss-image-state={state}');
    expect(bilibiliSurfaceSource).toContain("showRSSBilibiliVideo({");
    expect(bilibiliSurfaceSource).toContain("interactive: false");
    expect(bilibiliSurfaceSource).toContain(
      'document.documentElement.dataset.rssBilibiliVideoActive = "true"',
    );
    expect(bilibiliSurfaceSource).toContain(
      "delete document.documentElement.dataset.rssBilibiliVideoActive",
    );
    expect(bilibiliSurfaceSource).not.toContain("prepareRSSBilibiliVideo");
    expect(bilibiliPlaybackSource).toContain("prepareRSSBilibiliVideo({");
    expect(bilibiliPlaybackSource).toContain("closeRSSBilibiliVideo(sessionID)");
    expect(bilibiliPlaybackSource).toContain("<YouTubeWorkspaceTransportBar");
    expect(bilibiliPlaybackSource).toContain(
      "visibleControls={RSS_BILIBILI_TRANSPORT_CONTROLS}",
    );
    expect(bilibiliPlaybackSource).toContain("toggleRSSBilibiliVideoDanmaku(sessionID)");
    expect(bilibiliPlaybackSource).not.toContain("{playback ? (");
    expect(bilibiliPlaybackSource).toContain("data-native-ready={nativeReady");
    expect(bilibiliPlaybackSource).toContain("subscribeRSSBilibiliVideoStatus");
    expect(webVideoSource).toContain("<YouTubeWorkspaceTransportBar");
    expect(webVideoSource).toContain("<video");
    expect(webVideoSource).not.toContain("controls\n");
    expect(webVideoSource).toContain("createRSSWebVideoTransportPlayback({");
    expect(apiSource).toContain('Events.On("rss:bilibili-video-player"');
    expect(apiSource).toContain("isRSSBilibiliVideoStatusForSession");
    expect(apiSource).toContain("entriesInfinite");
    expect(apiSource).toContain("lastPage.nextOffset");
    expect(apiSource).toContain('"BackfillHistory"');
    expect(apiSource).toContain("export function useRSSBackfillHistory");
    expect(apiSource).toContain("resetRSSBackfillCaches(queryClient, result, request)");
    expect(apiSource).toContain("export function useRSSEntry");
    expect(apiSource).toContain("queryFn: () => getRSSEntry(normalizedID)");
    expect(apiSource).toContain('return callRSS("MarkAllRead", request)');
    expect(apiSource).toContain("useRSSCollectionUnreadCounts");
    expect(mainSource).toMatch(/useRSSCollectionUnreadCounts\(\s*activeWorkspaceId === APP_WORKSPACE_IDS\.rss,?\s*\)/);
    expect(mainSource).toMatch(/useRSSSubscriptions\(\s*activeWorkspaceId === APP_WORKSPACE_IDS\.rss,?\s*\)/);
    expect(pageSource).not.toContain("useRSSCompactLayout");
    expect(pageSource).not.toContain("shouldFocusRSSEntry");
    expect(pageSource).toContain("onClick={() => openEntry(entry)}");
    expect(pageSource).not.toContain('readerPreferences.openMode === "focused"');
    expect(pageSource).not.toContain("openEntry(current, true)");
    expect(pageSource).toContain('readerPreferences.openMode === "original"');
    expect(pageSource).toContain("if (focus) {");
    expect(pageSource).toContain('className="rss-entry-detail-pane app-workspace-primary-subpane"');
    expect(pageSource).toContain("tabIndex={-1}");
    expect(pageSource).toContain("resolveRSSWorkspaceShortcut(event)");
    expect(pageSource).toContain("<RSSReaderPreferencesDialog");
    expect(pageSource).toContain("<RSSKeyboardShortcutsDialog");
    expect(pageSource).toContain("<RSSAudioPlayer entry={entry} audio={audio}");
    expect(pageSource).toContain('key={`${audio.url}:${audio.mimeType}`}');
    expect(dialogSource).toContain('from "@/shared/ui/select"');
    expect(dialogSource).toContain('className="rss-discovery-dialog"');
    expect(dialogSource).not.toContain("overlayClassName=");
    expect(dialogSource).not.toContain("unstyled");
    expect(css).not.toContain("rss-discovery-dialog-backdrop");
    expect(css).not.toMatch(/(?:-webkit-)?backdrop-filter\s*:/);
    expect(bilibiliSurfaceSource).toContain('surfaceRole="status"');
    expect(bilibiliPlaybackSource).toContain("app-dream-status-message");
    expect(webVideoSource).toContain('surfaceRole="status"');
    expect(css).toContain(".rss-dropdown-menu [role=\"menuitem\"] > svg");
    expect(css).toContain(".rss-remote-image--probe");
    expect(css).toContain('.rss-image-skeleton[data-rss-image-state="loading"]::after');
    expect(rssAppearance).toContain('[data-rss-bilibili-video-active="true"]');
    expect(rssAppearance).toContain('[data-rss-site-video-active="true"]');
    expect(rssAppearance).toContain(".app-workspace-primary-pane");
    expect(pageSource).toContain('from "@/shared/ui/workspace-page"');
    expect(pageSource).toContain('recipe: "feed"');
    expect(pageSource).toContain('recipe: "detail"');
    expect(pageSource).toContain('recipe: "collection"');
    expect(pageSource).toContain('heading: "assistive"');
    expect(pageSource).toContain('heading: "host-owned"');
    expect(pageSource).toContain('footer: "none"');
    expect(pageSource).toContain('headingLevel={focused ? 1 : 2}');
    expect(pageSource).toContain('const Heading = headingLevel === 1 ? "h1" : "h2"');
    expect(pageSource).not.toContain('<h1 className="sr-only">');
    expect(css).not.toMatch(/\.rss-page-heading \{[^}]*(?:height|min-height):\s*52px;/);
    expect(css).toMatch(/\.rss-split-view \{[^}]*min-width: var\(--app-workspace-primary-min-width, 800px\);[^}]*grid-template-columns: clamp\(300px, 34%, 420px\) minmax\(460px, 1fr\);/);
    expect(css).toMatch(/\.rss-entry-list \{[^}]*overflow-x: hidden;[^}]*overflow-y: auto;[^}]*overscroll-behavior: contain;/);
    expect(css).toMatch(/\.rss-entry-detail-pane \{[^}]*overflow: hidden;[^}]*scrollbar-gutter: auto;/);
    expect(css).toMatch(/\.rss-reader \{[^}]*--rss-reader-page-min-width: 460px;[^}]*--rss-reader-article-max-width: 800px;[^}]*--rss-reader-progress-width: 44px;[^}]*--rss-reader-progress-inner-gap-min: 12px;[^}]*--rss-reader-side-min: calc\(\s*var\(--rss-reader-progress-width\) \+\s*var\(--rss-reader-progress-inner-gap-min\) \+\s*var\(--rss-reader-progress-inner-gap-min\)\s*\);[^}]*min-width: var\(--rss-reader-page-min-width\);[^}]*grid-template-columns:\s*minmax\(var\(--rss-reader-side-min\), 1fr\)\s*min\(\s*var\(--rss-reader-article-max-width\),\s*calc\(\s*100% - var\(--rss-reader-side-min\) - var\(--rss-reader-side-min\)\s*\)\s*\)\s*minmax\(var\(--rss-reader-side-min\), 1fr\);[^}]*overflow-x: hidden;[^}]*overflow-y: auto;[^}]*scrollbar-gutter: stable;/s);
    expect(css).toMatch(/\.rss-reader__main \{[^}]*grid-column: 2;/s);
    expect(css).toMatch(/\.rss-reader \.rss-entry-header \{[^}]*padding-right: 0;[^}]*padding-left: 0;/s);
    expect(css).toMatch(/\.rss-reader \.rss-entry-header \{[^}]*width: 100%;[^}]*margin: 0;/s);
    expect(mainSource).toContain(
      "const workspacePrimaryMinWidth = PRIMARY_PANE_DEFAULT_MIN_WIDTH;",
    );
    expect(mainSource).not.toContain(
      "const workspacePrimaryMinWidth = activeWorkspaceId",
    );
    expect(css).toMatch(/\.rss-image-scroll \{[^}]*width: 100%;[^}]*height: 100%;[^}]*min-width: 0;[^}]*overflow-x: hidden;[^}]*overflow-y: auto;[^}]*overscroll-behavior: contain;/);
    expect(css).toMatch(/\.rss-image-masonry \{[^}]*width: 100%;[^}]*min-width: 0;[^}]*min-height: 100%;[^}]*column-width: 220px;/);
    expect(css).not.toMatch(/\.rss-image-masonry \{[^}]*height: 100%;[^}]*overflow:/);
    expect(rssAppearance).toMatch(/\.rss-entry-row\[aria-current="true"\] \{[^}]*background: var\(--dream-primary-surface\);[^}]*box-shadow: inset 0 0 0 1px var\(--dream-primary-line\);/);
    expect(css).toMatch(/\.rss-entry-list--articles \{[^}]*display: grid;[^}]*align-content: start;[^}]*gap: var\(--app-space-1\);[^}]*padding-block: var\(--app-space-1\);[^}]*scroll-padding-block: var\(--app-space-1\);/s);
    expect(rssAppearance).toMatch(/\.rss-entry-row--article \{[^}]*border-bottom: 0;[^}]*border-radius: var\(--dream-control-radius\);/s);
    expect(rssAppearance).toMatch(/\.rss-entry-row--article\[aria-current="true"\] \{[^}]*box-shadow: none;/s);
    expect(pageSource).toContain("rss-entry-list app-dream-selection-list");
    expect(pageSource).toContain("rss-entry-row app-dream-selection-item");
    expect(dreamTokens).toContain("--app-selection-list-inset: var(--app-space-1);");
    expect(dreamControls).toMatch(/\.app-dream-selection-list \{[^}]*padding-inline: var\(--app-selection-list-inset\);[^}]*scroll-padding-inline: var\(--app-selection-list-inset\);[^}]*scrollbar-gutter: stable;/s);
    expect(dreamControls).toMatch(/\.app-dream-selection-list > \.app-dream-selection-item \{[^}]*width: 100%;[^}]*max-width: 100%;/s);
    expect(css).toMatch(/\.rss-discovery-category-grid \{[^}]*grid-auto-rows: 88px;/);
    expect(css).toMatch(/\.rss-discovery-route-grid \{[^}]*grid-auto-rows: 136px;/);
    expect(css).toMatch(/\.rss-discovery-route-card \{[^}]*min-height: 0;[^}]*align-self: stretch;/);
    expect(css).toMatch(/\.rss-discovery-route-card__copy \{[^}]*height: 100%;[^}]*grid-template-rows: auto minmax\(0, 1fr\) auto;/);
    expect(css).toContain("top: clamp(128px, 32vh, 220px)");
    expect(css).toMatch(
      /\.rss-reader-progress \{[^}]*grid-column: 3;[^}]*grid-template-columns: var\(--rss-reader-progress-width\);[^}]*justify-self: center;[^}]*justify-items: center;[^}]*padding: 0 0 18px;/s,
    );
    expect(css).not.toMatch(/\.rss-reader-progress \{[^}]*transform:/s);
    expect(css).not.toMatch(/\.rss-reader-progress \{[^}]*margin-inline-end:/s);
    expect(css).toMatch(
      /\.rss-reader-progress__outline \{[^}]*width: var\(--rss-reader-progress-width\);[^}]*justify-items: stretch;/s,
    );
    expect(css).toMatch(/\.rss-reader-progress__segment \{[^}]*width: var\(--rss-reader-progress-width\);[^}]*height: 16px;[^}]*justify-content: flex-end;/s);
    expect(css).toMatch(/\.rss-reader-progress__track \{[^}]*width: var\(--rss-toc-width, 100%\);[^}]*height: 5px;/s);
    expect(css).toMatch(/\.rss-reader-progress__track > span \{[^}]*width: var\(--rss-toc-progress, 0%\);/s);
    expect(css).toMatch(
      /\.rss-reader-progress__value \{[^}]*width: var\(--rss-reader-progress-width\);[^}]*min-height: 24px;[^}]*place-items: center;/s,
    );
    expect(rssAppearance).toMatch(
      /\.rss-reader-progress__value \{[^}]*text-align: center;/s,
    );
    expect(css).toMatch(/\.rss-reader-progress\[data-selection-active="true"\][\s\S]*\.rss-reader-progress__disclosure \{\s*pointer-events: none;/);
    expect(css).toMatch(/\.rss-reader-progress__sheet \{[^}]*width: min\(264px, calc\(100vw - 20px\)\);[^}]*max-height: min\(440px, calc\(100vh - 20px\)\);/s);
    expect(rssAppearance).toMatch(/\.rss-reader-progress__sheet-list button\[aria-current="location"\] \{[^}]*background: var\(--dream-primary-surface\);/s);
    expect(css).toMatch(
      /\.rss-reader-progress__back-to-top \{[^}]*width: 30px;[^}]*justify-self: center;/s,
    );
    expect(css).not.toContain(".rss-youtube-playback > .youtube-workspace-transport::before");
    expect(css).not.toContain(".rss-video-grid");
    expect(css).not.toContain(".rss-video-card__media");
    expect(rssAppearance).toMatch(/\.rss-video-player \{[^}]*border-radius: inherit;/);
    expect(rssAppearance).toMatch(/\.rss-bilibili-video-surface \{[^}]*border-radius: inherit;/);
    expect(css).not.toContain(".rss-video-watch-footer");
    expect(css).toMatch(/\.rss-youtube-playback,\s*\.rss-bilibili-playback,\s*\.rss-site-video-playback,\s*\.rss-web-video-playback/);
    expect(css).toMatch(/\.rss-image-tile > img \{[^}]*height: auto;[^}]*object-fit: contain;/);
    expect(css).toMatch(/\.app-dialog-content\.rss-image-lightbox \{[^}]*top: 50%;[^}]*left: 50%;[^}]*width: calc\([^;]*100vw[^;]*\);[^}]*height: calc\([^;]*100vh[^;]*\);[^}]*transform: translate\(-50%, -50%\);/s);
    expect(css).toMatch(/\.rss-image-lightbox__media > img \{[^}]*width: auto;[^}]*max-width: 100%;[^}]*height: auto;[^}]*max-height: 100%;[^}]*object-fit: contain;/s);
    expect(css).not.toContain("control-placeholder");
    expect(css).not.toContain("@container rss-page (max-width: 840px)");
    expect(css).not.toMatch(/\.rss-entry-detail-pane \{[^}]*display:\s*none;/s);
    expect(css).not.toMatch(/@media \(max-width: (?:1040|840)px\)/);
    expect(mainSource).toMatch(/<PrimaryPane\s+minimumWidth=\{workspacePrimaryMinWidth\}/);
  });
});
