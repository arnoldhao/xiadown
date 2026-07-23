import { describe, expect, test } from "bun:test";

describe("music workspace primary page layout", () => {
  test("uses the shared fixed page shell for Search and its assistive heading", async () => {
    const [source, layoutContract] = await Promise.all([
      Bun.file(new URL("./PageView.tsx", import.meta.url)).text(),
      Bun.file(
        new URL(
          "../../../shared/styles/dream/layout-contract.css",
          import.meta.url,
        ),
      ).text(),
    ]);

    expect(source).toContain("{!props.workspaceLayout ? (");
    expect(source).toContain("<WorkspacePage");
    expect(source).toContain("<WorkspacePageTopBar");
    expect(source).toContain("<WorkspacePageContent");
    expect(source.indexOf("<WorkspacePageTopBar")).toBeLessThan(
      source.indexOf("<WorkspacePageContent"),
    );
    expect(source).toContain('recipe: "search"');
    expect(source).toContain('topBar: "search"');
    expect(source).toContain('heading: "assistive"');
    expect(source).not.toContain('<h1 className="sr-only">');
    expect(source).toContain("<WorkspaceSearchControl");
    expect(source).toContain('from "@/shared/ui/workspace-page"');
    expect(source).toContain('from "@/shared/ui/workspace-search-control"');
    expect(source).toContain("value={workspaceSearchDraft}");
    expect(source).toContain("onValueChange={setWorkspaceSearchDraft}");
    expect(source).toContain("onSubmit={submitWorkspaceSearch}");
    expect(source).toContain("setQuery(value);");
    expect(layoutContract).toContain(".app-workspace-page__topbar {");
    expect(layoutContract).toContain(".app-workspace-page__content {");
    expect(layoutContract).toContain(
      "height: var(--app-workspace-page-topbar-height);",
    );
    expect(layoutContract).toContain("width: min(720px, calc(100% - 24px));");
  });

  test("keeps the empty Local Music search landing free of tracks and actions", async () => {
    const source = await Bun.file(
      new URL("./LocalLibraryWorkspace.tsx", import.meta.url),
    ).text();

    expect(source).toContain(
      'route?.kind === "search" && props.query.trim().length === 0',
    );
    expect(source).toContain("if (emptySearchLanding) {");
    expect(source).toContain("return [];");
    expect(source).toContain("const headerActions = !emptySearchLanding ? (");
    expect(source).toContain("{emptySearchLanding ? null : contentLoading ? (");
  });

  test("lets Artist and collection details own the sole visible heading", async () => {
    const [source, artistSource, collectionSource] = await Promise.all([
      Bun.file(new URL("./PageView.tsx", import.meta.url)).text(),
      Bun.file(new URL("./ArtistDetailHero.tsx", import.meta.url)).text(),
      Bun.file(new URL("./MusicCollectionDetail.tsx", import.meta.url)).text(),
    ]);

    expect(source).toContain("const workspaceDetailActive =");
    expect(source).toContain('recipe: "detail"');
    expect(source).toContain('topBar: "host-owned"');
    expect(source).toContain('heading: "host-owned"');
    expect(source).toContain("<ListenMuseArtistHero");
    expect(source).toContain("<ListenMusicCollectionDetail");
    expect(source).toContain("selectedArtistTrackListShelf");
    expect(source).not.toContain("<h1");
    expect(artistSource.match(/<h1/g)).toHaveLength(1);
    expect(collectionSource.match(/<h1/g)).toHaveLength(1);
    expect(artistSource).toContain(
      'className="listen-muse-artist-hero__body wails-drag"',
    );
    expect(collectionSource).toContain(
      'className="listen-collection-detail__toolbar wails-drag"',
    );
    expect(artistSource).toContain(
      'className="listen-muse-artist-hero__back wails-no-drag"',
    );
    expect(collectionSource).toContain(
      'className="listen-collection-detail__back wails-no-drag"',
    );
    expect(source).toContain(
      "{isWindows && !props.workspaceLayout ? (",
    );
    expect(source).not.toContain(
      'workspacePageContract?.topBar === "host-owned") ? (',
    );
  });

  test("gives Browse a display heading and Local/Radio an action rail", async () => {
    const [pageSource, localSource] = await Promise.all([
      Bun.file(new URL("./PageView.tsx", import.meta.url)).text(),
      Bun.file(new URL("./LocalLibraryWorkspace.tsx", import.meta.url)).text(),
    ]);

    expect(pageSource).toContain('recipe: "browse"');
    expect(pageSource).toContain('heading: "display"');
    expect(pageSource).toContain('recipe: "collection"');
    expect(pageSource).toContain('topBar: "actions"');
    expect(pageSource).toContain('heading: "assistive"');
    expect(pageSource).toContain(
      "headerActionsTarget={workspaceTopBarActionsTarget}",
    );
    expect(localSource).toContain(
      "createPortal(headerActions, props.headerActionsTarget)",
    );
    expect(localSource).toContain("<WorkspacePrimaryHeaderAction");
    expect(localSource).toContain("<WorkspacePrimaryHeaderMenuContent");
  });

  test("keeps collection details independent from the parent search query", async () => {
    const source = await Bun.file(
      new URL("./PageView.tsx", import.meta.url),
    ).text();
    const detailStart = source.indexOf("<ListenMusicCollectionDetail");
    const detailEnd = source.indexOf("/>", detailStart);
    const detailSource = source.slice(detailStart, detailEnd);

    expect(detailSource).toContain(
      "items={playlistLoading ? [] : playlistTracks}",
    );
    expect(detailSource).not.toContain("filteredPlaylistTracks");
    expect(source).toContain("playlistTracks.length === 0");
  });

  test("waits for collection pagination before rendering the footer description", async () => {
    const source = await Bun.file(
      new URL("./PageView.tsx", import.meta.url),
    ).text();
    const detailStart = source.indexOf("<ListenMusicCollectionDetail");
    const detailEnd = source.indexOf("/>", detailStart);
    const detailSource = source.slice(detailStart, detailEnd);

    expect(detailSource).toContain("showFooter={");
    expect(detailSource).toContain("!playlistLoading &&");
    expect(detailSource).toContain("!playlistAppending &&");
    expect(detailSource).toContain("!playlistContinuation");
  });

  test("starts playlist playback from visible tracks before background pagination", async () => {
    const [listenSource, pageSource, playbackSource] = await Promise.all([
      Bun.file(new URL("../Listen.tsx", import.meta.url)).text(),
      Bun.file(new URL("./PageView.tsx", import.meta.url)).text(),
      Bun.file(new URL("./playlist-playback.ts", import.meta.url)).text(),
    ]);

    expect(listenSource).toContain(
      "const loadCompleteSelectedPlaylistTracks = React.useCallback",
    );
    expect(listenSource).toContain("await fetchCompleteListenPlaylistQueue(");
    expect(listenSource).toContain("startListenPlaylistPlaybackFromIndex({");
    expect(listenSource).toContain(
      "initialItems: applyListenPlaylistPlaybackFallback(",
    );
    expect(playbackSource).toContain("startListenPlaylistPlayback({");
    expect(playbackSource).toContain(
      "appendRemaining: (items, expectedQueueIdentity)",
    );
    expect(playbackSource).toContain(
      "callListenPlaybackAppendToQueue(items, { expectedQueueIdentity })",
    );
    expect(pageSource).toContain("isListenPlaylistPlaybackDisabled({");
  });

  test("starts every playlist queue action from visible rows and preserves explicit duplicates", async () => {
    const [listenSource, playbackSource] = await Promise.all([
      Bun.file(new URL("../Listen.tsx", import.meta.url)).text(),
      Bun.file(new URL("./playlist-playback.ts", import.meta.url)).text(),
    ]);

    expect(listenSource).toContain("startListenPlaylistQueueAction({");
    expect(listenSource).toContain("resolveListenPlaylistQueueAction({");
    expect(listenSource).not.toContain("const existingVideoIDs = new Set(");
    expect(playbackSource).toContain(
      "Explicit queue actions are allowed to add the same song more than once",
    );
  });

  test("wires the online search result actions that Listen already provides", async () => {
    const [pageSource, groupSource] = await Promise.all([
      Bun.file(new URL("./PageView.tsx", import.meta.url)).text(),
      Bun.file(new URL("./ui.tsx", import.meta.url)).text(),
    ]);

    expect(pageSource).toContain("playOnlineSearchResults");
    expect(pageSource).toContain("shuffleOnlineSearchResults");
    expect(pageSource).toContain("onPlayAll={playOnlineSearchResults}");
    expect(groupSource).toContain("onPlayAll?: () => void;");
    expect(groupSource).toContain("onShuffle?: () => void;");
  });

  test("prefers explicit Single and EP metadata over release browse IDs", async () => {
    const source = await Bun.file(
      new URL("./PageView.tsx", import.meta.url),
    ).text();
    const resolverStart = source.indexOf(
      "function resolveListenPlaylistTypeLabel",
    );
    const resolverEnd = source.indexOf(
      "function resolveUsefulListenPlaylistAuthor",
      resolverStart,
    );
    const resolverSource = source.slice(resolverStart, resolverEnd);
    const albumFallback = resolverSource.indexOf(
      'playlistId.startsWith("MPRE")',
    );

    expect(resolverSource.indexOf('normalized === "single"')).toBeLessThan(
      albumFallback,
    );
    expect(resolverSource.indexOf('normalized === "ep"')).toBeLessThan(
      albumFallback,
    );
    expect(albumFallback).toBeGreaterThan(0);
  });

  test("reserves scroll space above the floating transport", async () => {
    const [source, css] = await Promise.all([
      Bun.file(new URL("./PageView.tsx", import.meta.url)).text(),
      Bun.file(new URL("./listen.css", import.meta.url)).text(),
    ]);

    expect(source).toContain("listen-workspace-scroll");
    expect(css).toContain(".listen-workspace-page > .listen-workspace-scroll");
    expect(css).toContain(
      "5.5rem + var(--app-workspace-page-content-padding-block)",
    );
    expect(css).toContain("scroll-padding-block-end:");
  });

  test("automatically paginates every Online primary view without Load More buttons", async () => {
    const source = await Bun.file(
      new URL("./PageView.tsx", import.meta.url),
    ).text();

    expect(source).toContain('data-listen-primary-scroll="true"');
    expect(source.match(/<ListenInfiniteScrollSentinel/g)).toHaveLength(5);
    expect(source).not.toContain("props.text.listen.loadMore");
    expect(source).not.toMatch(
      /onClick=\{loadMore(?:Artist|Playlist|Search|Library)\}/,
    );
  });
});
