import { describe, expect, test } from "bun:test";

describe("music collection detail layout", () => {
  test("gives playlists an Apple-style information header and column table", async () => {
    const source = await Bun.file(
      new URL("./MusicCollectionDetail.tsx", import.meta.url),
    ).text();

    expect(source).toContain('data-collection-kind={props.isAlbum ? "album" : "playlist"}');
    expect(source).toContain("listen-collection-detail__artwork");
    expect(source).toContain("listen-collection-detail__identity");
    expect(source).toContain("listen-collection-detail__actions");
    expect(source).toContain("props.text.listen.playlistColumnSong");
    expect(source).toContain("props.text.listen.playlistColumnArtist");
    expect(source).toContain("props.text.listen.playlistColumnTime");
    expect(source).toContain('data-collection-track-columns="playlist"');
    expect(source).toContain("ListenCollectionTrackArtwork");
    expect(source).toContain("listen-collection-track-row__album");
    expect(source).toContain("listen-collection-track-row__artist");
    expect(source).toContain("listen-collection-track-row__time");
    expect(source).toContain("<MoreVertical");
  });

  test("keeps album tracks headerless, numbered, and free of per-row artwork", async () => {
    const source = await Bun.file(
      new URL("./MusicCollectionDetail.tsx", import.meta.url),
    ).text();

    expect(source).toContain("{!props.isAlbum ? (");
    expect(source).toContain('data-track-layout={props.isAlbum ? "album" : "playlist"}');
    expect(source).toContain("listen-collection-track-row__number");
    expect(source).toContain("props.trackNumber");
    expect(source).toContain("props.isAlbum ? (");
    expect(source.indexOf("<ListenCollectionTrackArtwork")).toBeGreaterThan(
      source.indexOf(") : ("),
    );
  });

  test("keeps structured metadata in the header and description at the list end", async () => {
    const [source, css] = await Promise.all([
      Bun.file(new URL("./MusicCollectionDetail.tsx", import.meta.url)).text(),
      Bun.file(new URL("./listen.css", import.meta.url)).text(),
    ]);

    expect(source).toContain("listen-collection-detail__footer");
    expect(source).not.toContain("footerMetadata");
    expect(source).toContain("showFooter: boolean");
    expect(source).toContain("{props.showFooter && props.description ? (");
    expect(source.indexOf("<ListenCollectionDescription")).toBeGreaterThan(
      source.indexOf('<footer className="listen-collection-detail__footer">'),
    );
    expect(css).toContain(".listen-collection-detail__header");
    expect(css).toContain(
      "grid-template-columns: clamp(11rem, 24cqw, 15rem) minmax(0, 1fr)",
    );
    expect(css).not.toContain(
      "grid-template-columns: clamp(11rem, 24vw, 15rem) minmax(0, 1fr)",
    );
    expect(css).toContain(
      "@container workspace-page-content (max-width: 760px)",
    );
    expect(css).toContain(
      "@container workspace-page-content (max-width: 560px)",
    );
    expect(css).not.toContain("font-size: clamp(1.45rem, 5vw, 2rem)");
    expect(css).toContain(".listen-collection-tracks__columns");
    expect(css).toContain(
      '.listen-collection-track-row[data-track-layout="album"]',
    );
    expect(css).toContain("grid-template-columns: 2rem minmax(0, 1fr) 4rem 2.25rem");
    expect(source.match(/variant="glass"/g)).toHaveLength(2);
    expect(source.match(/shape="circle"/g)).toHaveLength(2);
    expect(source).toContain('shape="capsule"');
    expect(css).not.toMatch(/(?:-webkit-)?backdrop-filter\s*:/);
  });

  test("uses the shared Dream link button for a navigable album artist", async () => {
    const source = await Bun.file(
      new URL("./MusicCollectionDetail.tsx", import.meta.url),
    ).text();

    expect(source).toContain("onOpenAuthor?: () => void");
    expect(source).toContain('variant="link"');
    expect(source).toContain('size="compact"');
    expect(source).toContain('className="listen-collection-detail__author"');
    expect(source).not.toContain(
      'className="listen-collection-detail__author h-auto w-fit p-0"',
    );
    expect(source).toContain("onClick={props.onOpenAuthor}");
  });

  test("sizes collection action menus from their localized content", async () => {
    const [source, anatomy] = await Promise.all([
      Bun.file(new URL("./MusicCollectionDetail.tsx", import.meta.url)).text(),
      Bun.file(
        new URL("../../../shared/styles/dream/anatomy.css", import.meta.url),
      ).text(),
    ]);

    expect(source.match(/className="app-menu-content-fit"/g)).toHaveLength(2);
    expect(anatomy).toMatch(
      /\.app-menu-content-base\.app-menu-content-fit,[\s\S]*?\{[^}]*width:\s*max-content[^}]*min-width:\s*fit-content[^}]*max-width:\s*calc\(100vw - \(var\(--app-space-4\) \* 2\)\)/s,
    );
    expect(source).not.toMatch(
      /<DropdownMenuContent[^>]*className="[^"]*\bw-(?:\d+|auto|fit|max|min)\b/,
    );
  });

  test("top-aligns artwork and lets overflowing descriptions expand", async () => {
    const [source, css] = await Promise.all([
      Bun.file(new URL("./MusicCollectionDetail.tsx", import.meta.url)).text(),
      Bun.file(new URL("./listen.css", import.meta.url)).text(),
    ]);

    expect(source).toContain("ListenCollectionDescription");
    expect(source).toContain("description.scrollHeight > description.clientHeight + 1");
    expect(source).toContain('aria-expanded={expanded}');
    expect(source).toContain("props.text.listen.collectionDescriptionMore");
    expect(source).toContain("props.text.listen.collectionDescriptionLess");
    expect(css).toMatch(
      /\.listen-collection-detail__header\s*\{[\s\S]*?align-items: start;/,
    );
    expect(css).toContain(
      '.listen-collection-detail__description[data-expanded="true"]',
    );
    expect(css).toContain("-webkit-line-clamp: 3");
  });

  test("resets description disclosure when the collection identity changes", async () => {
    const source = await Bun.file(
      new URL("./MusicCollectionDetail.tsx", import.meta.url),
    ).text();

    expect(source).toContain("const collectionIdentity =");
    expect(source).toContain("props.collection?.playlistId.trim()");
    expect(source).toContain("`${props.typeLabel.trim()}:${props.title.trim()}`");
    expect(source).toContain("key={collectionIdentity}");
  });

  test("shows immediate busy feedback for a playlist play request", async () => {
    const source = await Bun.file(
      new URL("./MusicCollectionDetail.tsx", import.meta.url),
    ).text();

    expect(source).toContain("playbackBusy: boolean");
    expect(source).toContain("aria-busy={props.playbackBusy || undefined}");
    expect(source).toContain("props.playbackBusy ? (");
    expect(source).toContain('<Loader2 aria-hidden="true" className="listen-loading-spinner" />');
  });
});
