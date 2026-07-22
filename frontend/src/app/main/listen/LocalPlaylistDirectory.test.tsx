import { describe, expect, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";

import { ListenLocalPlaylistDirectory } from "./LocalPlaylistDirectory";

const playlists = [
  {
    id: "morning",
    name: "Morning Mix",
    itemCount: 3,
    createdAt: "2026-07-12T01:00:00Z",
    updatedAt: "2026-07-12T01:00:00Z",
  },
  {
    id: "evening",
    name: "Evening Mix",
    itemCount: 8,
    createdAt: "2026-07-12T02:00:00Z",
    updatedAt: "2026-07-12T02:00:00Z",
  },
];

describe("ListenLocalPlaylistDirectory", () => {
  test("renders every persisted playlist as a navigable entry", () => {
    const markup = renderToStaticMarkup(
      <ListenLocalPlaylistDirectory
        emptyLabel="No playlists"
        itemCountTemplate="{count} songs"
        loading={false}
        loadingLabel="Loading playlists"
        onSelect={() => undefined}
        playlists={playlists}
        title="Playlists"
      />,
    );

    expect(markup).toContain('aria-label="Playlists"');
    expect(markup).toContain('data-playlist-id="morning"');
    expect(markup).toContain('aria-label="Morning Mix, 3 songs"');
    expect(markup).toContain('data-playlist-id="evening"');
    expect(markup).toContain('aria-label="Evening Mix, 8 songs"');
    expect(markup.indexOf("Morning Mix")).toBeLessThan(
      markup.indexOf("Evening Mix"),
    );
  });

  test("shows initial loading and the loaded empty state distinctly", () => {
    const loadingMarkup = renderToStaticMarkup(
      <ListenLocalPlaylistDirectory
        emptyLabel="No playlists"
        itemCountTemplate="{count} songs"
        loading
        loadingLabel="Loading playlists"
        onSelect={() => undefined}
        playlists={[]}
        title="Playlists"
      />,
    );
    const emptyMarkup = renderToStaticMarkup(
      <ListenLocalPlaylistDirectory
        emptyLabel="No playlists"
        itemCountTemplate="{count} songs"
        loading={false}
        loadingLabel="Loading playlists"
        onSelect={() => undefined}
        playlists={[]}
        title="Playlists"
      />,
    );

    expect(loadingMarkup).toContain('role="status"');
    expect(loadingMarkup).toContain("Loading playlists");
    expect(loadingMarkup).not.toContain("No playlists");
    expect(emptyMarkup).toContain("No playlists");
    expect(emptyMarkup).not.toContain('role="status"');
  });

  test("keeps existing entries available during background refresh", () => {
    const markup = renderToStaticMarkup(
      <ListenLocalPlaylistDirectory
        emptyLabel="No playlists"
        itemCountTemplate="songs: {count} / {count}"
        loading
        loadingLabel="Refreshing playlists"
        onSelect={() => undefined}
        playlists={[playlists[0]]}
        title="Playlists"
      />,
    );

    expect(markup).toContain('data-playlist-id="morning"');
    expect(markup).toContain("songs: 3 / 3");
    expect(markup).toContain('aria-label="Refreshing playlists"');
    expect(markup).not.toContain('role="status"');
  });
});
