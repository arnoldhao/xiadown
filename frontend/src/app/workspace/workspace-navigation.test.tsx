import { describe, expect, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";

import {
  MusicWorkspaceSidebar,
  type MusicWorkspaceSidebarCatalog,
} from "./MusicWorkspaceSidebar";
import {
  SniffWorkspaceSidebar,
  type SniffWorkspaceSidebarCatalog,
} from "./SniffWorkspaceSidebar";
import {
  YouTubeWorkspaceSidebar,
  type YouTubeWorkspaceSidebarCatalog,
} from "./YouTubeWorkspaceSidebar";
import { resolveWorkspaceSwitchStations } from "./station-navigation";

async function readWorkspaceAppearance() {
  return Bun.file(
    new URL("../../shared/styles/dream/workspace.css", import.meta.url),
  ).text();
}

const musicCatalog: MusicWorkspaceSidebarCatalog = {
  sidebarAriaLabel: "nav.music",
  sections: {
    explore: { label: "section.explore" },
    library: { label: "section.library" },
    playlists: { label: "section.playlists" },
  },
  routes: {
    search: {
      label: "route.search",
      icon: <span data-icon="search" />,
    },
    home: { label: "route.home", icon: <span data-icon="home" /> },
    radio: { label: "route.radio", icon: <span data-icon="radio" /> },
    newReleases: {
      label: "route.new.releases",
      icon: <span data-icon="new-releases" />,
    },
    charts: { label: "route.charts", icon: <span data-icon="charts" /> },
    moods: { label: "route.moods", icon: <span data-icon="moods" /> },
    podcasts: {
      label: "route.podcasts",
      icon: <span data-icon="podcasts" />,
    },
    recent: { label: "route.recent", icon: <span data-icon="recent" /> },
    history: {
      label: "route.history",
      icon: <span data-icon="history" />,
    },
    onlinePlaylists: {
      label: "route.online.playlists",
      icon: <span data-icon="online-playlists" />,
    },
    localSearch: {
      label: "route.local.search",
      icon: <span data-icon="local-search" />,
    },
    localHome: {
      label: "route.local.home",
      icon: <span data-icon="local-home" />,
    },
    recentlyAdded: {
      label: "route.recently.added",
      icon: <span data-icon="recently-added" />,
    },
    artists: { label: "route.artists", icon: <span data-icon="artists" /> },
    albums: { label: "route.albums", icon: <span data-icon="albums" /> },
    songs: { label: "route.songs", icon: <span data-icon="songs" /> },
  },
};

function expectTextOrder(markup: string, values: readonly string[]) {
  let previousIndex = -1;
  values.forEach((value) => {
    const index = markup.indexOf(value);
    expect(index).toBeGreaterThan(previousIndex);
    previousIndex = index;
  });
}

const sniffCatalog: SniffWorkspaceSidebarCatalog = {
  sidebarAriaLabel: "nav.sniff",
  sections: {
    types: { label: "section.types" },
    sources: { label: "section.sources" },
    resources: { label: "section.resources" },
  },
};

const youtubeCatalog: YouTubeWorkspaceSidebarCatalog = {
  sidebarAriaLabel: "nav.youtube",
  sections: {
    discover: { label: "section.discover" },
    collections: { label: "section.collections" },
  },
  routes: {
    search: { icon: <span data-icon="search" />, label: "route.search" },
    home: { icon: <span data-icon="home" />, label: "route.home" },
    subscriptions: {
      icon: <span data-icon="subscriptions" />,
      label: "route.subscriptions",
    },
    explore: { icon: <span data-icon="explore" />, label: "route.explore" },
    shorts: { icon: <span data-icon="shorts" />, label: "route.shorts" },
    likedVideos: {
      icon: <span data-icon="liked-videos" />,
      label: "route.liked.videos",
    },
    watchLater: {
      icon: <span data-icon="watch-later" />,
      label: "route.watch.later",
    },
    playlists: {
      icon: <span data-icon="playlists" />,
      label: "route.playlists",
    },
    history: { icon: <span data-icon="history" />, label: "route.history" },
  },
};

describe("wide workspace sidebars", () => {
  test("merges missing built-in stations into old persisted navigation", () => {
    const resolved = resolveWorkspaceSwitchStations([
      {
        id: "music",
        workspaceId: "music",
        label: "Music",
        order: 0,
        enabled: true,
      },
      {
        id: "disabled",
        workspaceId: "disabled",
        label: "Disabled",
        order: 1,
        enabled: false,
      },
    ]);

    expect(resolved.map((station) => station.workspaceId)).toEqual([
      "library",
      "sniff",
      "music",
      "youtube",
      "rss",
    ]);
    expect(resolved.at(-2)?.id).toBe("youtube");
    expect(resolved.at(-2)?.pinned).toBe(false);
    expect(resolved.at(-1)?.id).toBe("rss");
    expect(resolved.at(-1)?.pinned).toBe(true);
  });

  test("renders the online music catalog in source order with icons and chrome", () => {
    const markup = renderToStaticMarkup(
      <MusicWorkspaceSidebar
        account={<span>slot.account slot.switcher slot.new</span>}
        activeRouteId="moods"
        activity={<span>slot.activity</span>}
        catalog={musicCatalog}
        onNavigate={() => undefined}
        scope="online"
        controlPanel={<span>slot.control.panel</span>}
      />,
    );

    expect(markup).toContain('aria-label="nav.music"');
    expectTextOrder(markup, [
      "route.search",
      "route.home",
      "route.radio",
      "section.explore",
      "route.new.releases",
      "route.charts",
      "route.moods",
      "route.podcasts",
      "section.library",
      "route.recent",
      "route.history",
      "route.online.playlists",
    ]);
    expect(markup).not.toContain("section.playlists");
    expect(markup).toContain("app-music-workspace-sidebar");
    expect(markup).not.toContain("route.local.search");
    expect(markup).toContain('data-icon="search"');
    expect(markup).toContain('data-icon="podcasts"');
    expect(markup).toContain('data-icon="online-playlists"');
    expect(markup).toContain('aria-label="route.moods"');
    expect(markup).toContain('data-active="true"');
    expectTextOrder(markup, [
      "slot.activity",
      "slot.control.panel",
      "slot.account",
    ]);
    expect(markup).toContain(
      "app-workspace-wide-sidebar__control-panel",
    );
    expect(markup).not.toContain(
      "app-workspace-wide-sidebar__information-docks",
    );
    expect(markup).toContain("slot.switcher");
    expect(markup).toContain("slot.new");
    expect(markup).not.toContain("app-workspace-sidebar__footer");
  });

  test("keeps wide workspace navigation scrollbars against the sidebar edge", async () => {
    const css = await Bun.file(
      new URL("./workspace-navigation.css", import.meta.url),
    ).text();

    expect(css).toMatch(
      /:is\(\s*\.app-library-workspace-sidebar,\s*\.app-music-workspace-sidebar,\s*\.app-rss-workspace-sidebar,\s*\.app-sniff-workspace-sidebar,\s*\.app-youtube-workspace-sidebar\s*\)\s*\{[^}]*padding-right:\s*0/s,
    );
    expect(css).toMatch(
      /:is\(\s*\.app-library-workspace-sidebar,\s*\.app-music-workspace-sidebar,\s*\.app-rss-workspace-sidebar,\s*\.app-sniff-workspace-sidebar,\s*\.app-youtube-workspace-sidebar\s*\)\s*>\s*\.app-workspace-sidebar__navigation\s*\{[^}]*padding-right:\s*12px/s,
    );
  });

  test("uses one semantic selected row and keeps focus inside the control", async () => {
    const css = await readWorkspaceAppearance();

    expect(css).toMatch(
      /\.app-workspace-nav-button__icon\s*\{[^}]*color:\s*hsl\(var\(--app-accent-text, var\(--sidebar-primary\)\)\)[^}]*background:\s*transparent/s,
    );
    expect(css).toMatch(
      /\.app-workspace-nav-button\[data-active="true"\]\s*\.app-workspace-nav-button__icon\s*\{[^}]*color:\s*inherit/s,
    );
    const activeIconRule = css.match(
      /\.app-workspace-nav-button\[data-active="true"\]\s*\.app-workspace-nav-button__icon\s*\{([^}]*)\}/s,
    );
    expect(activeIconRule?.[1]).not.toContain("background");
    expect(css).toMatch(
      /\.app-workspace-nav-button\[data-active="true"\]\s*\.app-rss-workspace-sidebar__favicon\s*\{[^}]*background:\s*transparent[^}]*color:\s*inherit/s,
    );
    expect(css).toMatch(
      /\.app-workspace-nav-button\[data-active="true"\]:hover:not\(:disabled\)[^{]*,\s*:root:not\(\[data-input-modality="pointer"\]\)[^{]*\.app-workspace-nav-button\[data-active="true"\]:focus-visible\s*\{[^}]*color:\s*hsl\(\s*var\(--app-accent-on-solid, var\(--sidebar-primary-foreground\)\)\s*\)[^}]*background:\s*hsl\(var\(--app-accent-solid, var\(--sidebar-primary\)\)\)/s,
    );
    expect(css).not.toMatch(
      /\.app-workspace-nav-button(?:\[data-active="true"\])?[\s\S]{0,240}(?:color|background):\s*(?:white|#fff(?:fff)?|rgb\(255\s+255\s+255\))/i,
    );
    expect(css).toMatch(
      /:root:not\(\[data-input-modality="pointer"\]\)[^{]*\.app-workspace-nav-button:focus-visible\s*\{[^}]*outline:\s*2px solid[^}]*outline-offset:\s*-2px/s,
    );
    expect(css).not.toMatch(
      /\.app-workspace-nav-button:focus-visible\s*\{[^}]*box-shadow:/s,
    );
    expect(css).toMatch(
      /@media \(forced-colors:\s*active\)[\s\S]*?\.app-workspace-nav-button__icon\s*\{[^}]*color:\s*LinkText[^}]*\}[\s\S]*?\.app-workspace-nav-button\[data-active="true"\]:focus-visible\s*\{[^}]*color:\s*HighlightText[^}]*background:\s*Highlight/s,
    );
    expect(css).toContain("border-radius: var(--app-radius-control)");
    expect(css).toContain(
      "border-radius: var(--app-radius-control-inner)",
    );
    expect(css).not.toMatch(/border-radius:\s*(?:\d|\.\d)/);
  });

  test("keeps activity first and divides the account only below upper content", async () => {
    const markup = renderToStaticMarkup(
      <MusicWorkspaceSidebar
        account={<span>slot.account</span>}
        activity={<span>slot.activity</span>}
        catalog={musicCatalog}
        onNavigate={() => undefined}
        scope="online"
        controlPanel={<span>slot.control.panel</span>}
      />,
    );
    const [css, appearance] = await Promise.all([
      Bun.file(new URL("./workspace-navigation.css", import.meta.url)).text(),
      readWorkspaceAppearance(),
    ]);

    expectTextOrder(markup, [
      "app-workspace-wide-sidebar__activity",
      "app-workspace-wide-sidebar__control-panel",
      "app-workspace-wide-sidebar__account",
    ]);
    expect(css).toMatch(
      /\.app-workspace-wide-sidebar__activity\s*\+\s*\.app-workspace-wide-sidebar__control-panel\s*,\s*\.app-workspace-wide-sidebar__control-panel\s*\+\s*\.app-workspace-wide-sidebar__activity\s*\{[^}]*margin-top:\s*12px/s,
    );
    expect(markup).toContain('data-has-upper-content="true"');
    expect(css).toMatch(
      /\.app-workspace-wide-sidebar__bottom\[data-has-upper-content="true"\]\s*> \.app-workspace-wide-sidebar__account\s*\{[^}]*margin-top:\s*var\(--app-workspace-sidebar-divider-gap\)[^}]*padding-top:\s*var\(--app-workspace-sidebar-divider-gap\)/s,
    );
    expect(appearance).toMatch(
      /\.app-workspace-wide-sidebar__bottom\[data-has-upper-content="true"\]\s*> \.app-workspace-wide-sidebar__account\s*\{[^}]*border-top:\s*1px/s,
    );
    expect(css).not.toMatch(
      /\.app-workspace-wide-sidebar__account::before/s,
    );
    const bottomRule = css.match(
      /\.app-workspace-wide-sidebar__bottom\s*\{([^}]*)\}/s,
    )?.[1];
    expect(bottomRule).not.toMatch(/\n\s*gap:/);
  });

  test("renders every optional bottom region without empty wrappers", () => {
    const cases = [
      {
        props: { activity: <span>slot.activity</span> },
        expected: ["app-workspace-wide-sidebar__activity"],
        hasUpperContent: true,
      },
      {
        props: { controlPanel: <span>slot.control.panel</span> },
        expected: ["app-workspace-wide-sidebar__control-panel"],
        hasUpperContent: true,
      },
      {
        props: { account: <span>slot.account</span> },
        expected: ["app-workspace-wide-sidebar__account"],
        hasUpperContent: false,
      },
      {
        props: {
          activity: <span>slot.activity</span>,
          account: <span>slot.account</span>,
        },
        expected: [
          "app-workspace-wide-sidebar__activity",
          "app-workspace-wide-sidebar__account",
        ],
        hasUpperContent: true,
      },
      {
        props: {
          controlPanel: <span>slot.control.panel</span>,
          account: <span>slot.account</span>,
        },
        expected: [
          "app-workspace-wide-sidebar__control-panel",
          "app-workspace-wide-sidebar__account",
        ],
        hasUpperContent: true,
      },
    ] as const;

    cases.forEach(({ props, expected, hasUpperContent }) => {
      const markup = renderToStaticMarkup(
        <MusicWorkspaceSidebar
          {...props}
          catalog={musicCatalog}
          onNavigate={() => undefined}
          scope="online"
        />,
      );

      expect(markup).toContain("app-workspace-wide-sidebar__bottom");
      expect(markup).toContain(
        `data-has-upper-content="${hasUpperContent ? "true" : "false"}"`,
      );
      expectTextOrder(markup, expected);
      expect(markup).not.toContain(
        "app-workspace-wide-sidebar__information-docks",
      );
    });

    const markupWithoutBottom = renderToStaticMarkup(
      <MusicWorkspaceSidebar
        catalog={musicCatalog}
        onNavigate={() => undefined}
        scope="online"
      />,
    );
    expect(markupWithoutBottom).not.toContain(
      "app-workspace-wide-sidebar__bottom",
    );
  });

  test("routes every Station through the shared conditional bottom contract", async () => {
    const stationSidebars = [
      "LibraryWorkspaceSidebar.tsx",
      "MusicWorkspaceSidebar.tsx",
      "RSSWorkspaceSidebar.tsx",
      "SniffWorkspaceSidebar.tsx",
      "YouTubeWorkspaceSidebar.tsx",
    ];

    for (const filename of stationSidebars) {
      const source = await Bun.file(new URL(filename, import.meta.url)).text();
      expect(source).toContain("<WorkspaceSidebarNavigation");
    }
  });

  test("aligns Music source and station selectors without nested spacing", async () => {
    const source = await Bun.file(
      new URL("../main/MainApp.tsx", import.meta.url),
    ).text();
    const accountMenuStart = source.indexOf("const workspaceAccountMenu = (");
    const accountMenuEnd = source.indexOf(
      "const workspaceNewAction = (",
      accountMenuStart,
    );
    const accountMenuSource = source.slice(accountMenuStart, accountMenuEnd);
    const accountIdentityStart = source.indexOf("const accountIdentityPanel = (");
    const accountIdentityEnd = source.indexOf(
      "const libraryAccessControlRow = (",
      accountIdentityStart,
    );
    const accountIdentitySource = source.slice(
      accountIdentityStart,
      accountIdentityEnd,
    );
    const accessControlEnd = source.indexOf(
      "const resolveStationDisplayLabel",
      accountIdentityEnd,
    );
    const accessControlSource = source.slice(accountIdentityEnd, accessControlEnd);

    expect(source).toContain(
      'className="app-music-workspace-source-switcher w-full"',
    );
    expect(source).not.toContain('className="px-2 pb-1"');
    expect(source).toContain(
      'icon: <Globe2 className="h-3.5 w-3.5 shrink-0" />',
    );
    expect(source).toContain(
      'icon: <HardDrive className="h-3.5 w-3.5 shrink-0" />',
    );
    expect(source).toContain("<ChevronsUpDown />");
    expect(source).toContain('align="start"');
    expect(source).toContain('className="app-workspace-account-menu"');
    expect(source).toContain("app-workspace-account-menu__quick-actions");
    expect(source).toContain("<Settings2 />");
    expect(source).toContain("<FileText />");
    expect(source).toContain("<LibraryBig");
    expect(source).toContain("? text.workspace.libraryStation");
    expect(source).toMatch(
      /<Check\s+className="h-3\.5 w-3\.5 shrink-0"\s+data-menu-indicator="true"\s*\/>/,
    );
    expect(accountMenuSource).toContain("{accountIdentityPanel}");
    expect(source).toContain("<UserAvatar");
    expect(source).toContain("profile={profile}");
    expect(source).toContain("resolveUserDisplayName(profile)");
    expect(source).toContain("app-workspace-account-menu__access-switch");
    expect(accessControlSource).toContain(
      'className="app-workspace-account-menu__access-name"',
    );
    expect(accessControlSource).toMatch(
      />\s*\{libraryAccessCopy\.remote\}\s*</,
    );
    expect(accessControlSource).not.toMatch(
      />\s*\{libraryAccessCopy\.local\}\s*</,
    );
    expect(accessControlSource).toContain("<DropdownMenuCheckboxItem");
    expect(accessControlSource).toContain("<DreamInlineSwitchVisual");
    expect(accessControlSource).toContain(
      "checked={libraryAccessDisplayRemote}",
    );
    expect(accessControlSource).toContain(
      "onSelect={(event) => event.preventDefault()}",
    );
    expect(accessControlSource).toContain(
      'aria-describedby="app-workspace-account-menu-access-status"',
    );
    expect(accessControlSource).toContain("aria-busy={");
    expect(accessControlSource).toContain(
      "aria-invalid={libraryAccessError ? true : undefined}",
    );
    expect(accessControlSource).toContain("role={libraryAccessError ? \"alert\" : \"status\"}");
    expect(accessControlSource).toContain('className="sr-only"');
    expect(accessControlSource).not.toContain("app-dream-inline-switch-knob");
    expect(source).not.toContain("app-workspace-account-menu__access-indicator");
    expect(source).toContain("app-workspace-account-menu__identity-id");
    expect(source).toContain("app-workspace-account-menu__mobile-action");
    expect(accountIdentitySource).toContain("<DropdownMenuItem");
    expect(accountIdentitySource).toContain("onSelect={() => {");
    expect(accountIdentitySource).not.toContain("onPointerDown");
    expect(accessControlSource).not.toContain("<DropdownMenuRadioGroup");
    expect(accessControlSource).not.toContain("<DropdownMenuRadioItem");
    expect(accessControlSource).not.toContain("<DreamSegmentSwitch");
    expect(source).toContain("<LibraryPairingSheet");
    expect(source).toContain("app-workspace-account-profile__update-dot");
    expect(source).toContain("userMenuUpdateItems.map");
    expect(source).not.toContain("app-workspace-account-menu__access-status");
    expect(source).not.toContain("resolveUserSubtitle(profile)");
    expect(accountMenuSource.indexOf("{accountIdentityPanel}")).toBeLessThan(
      accountMenuSource.indexOf('className="grid"'),
    );
    expect(accountMenuSource.indexOf('className="grid"')).toBeLessThan(
      accountMenuSource.indexOf("{libraryAccessControlRow}"),
    );
    expect(accountMenuSource.indexOf("{libraryAccessControlRow}")).toBeLessThan(
      accountMenuSource.indexOf("app-workspace-account-menu__quick-actions"),
    );
    expect(
      accountMenuSource.match(/<DropdownMenuSeparator \/>/g)?.length ?? 0,
    ).toBeGreaterThanOrEqual(2);
    expect(source).not.toMatch(
      /<(?:Radar|Youtube|Music2|Check)[^>]*className="[^"]*(?:mr-|ml-)/,
    );
  });

  test("keeps Music source tabs in the sidebar and local controls on Local Home", async () => {
    const [mainSource, pageSource, localWorkspaceSource, css] = await Promise.all([
      Bun.file(new URL("../main/MainApp.tsx", import.meta.url)).text(),
      Bun.file(
        new URL("../main/listen/PageView.tsx", import.meta.url),
      ).text(),
      Bun.file(
        new URL(
          "../main/listen/LocalLibraryWorkspace.tsx",
          import.meta.url,
        ),
      ).text(),
      Bun.file(new URL("./workspace-navigation.css", import.meta.url)).text(),
    ]);

    expect(mainSource).toContain("controlPanel:");
    expect(mainSource).toContain("? musicSourceSwitcher");
    expect(mainSource).not.toContain("listenControlPanelPortalTarget");
    expect(pageSource).toContain('mode === "linger"');
    expect(pageSource).toContain("localTracksRefreshing");
    expect(pageSource).toContain("refreshLocalTracks");
    expect(pageSource).toContain(
      "tracksRefreshing={localTracksRefreshing}",
    );
    expect(pageSource).toContain(
      "onRepairMissingTracks={openRepairMissingLocalTracks}",
    );
    expect(localWorkspaceSource).toContain('route?.kind === "home"');
    expect(localWorkspaceSource).toContain("props.onRefreshTracks");
    expect(localWorkspaceSource).toContain("props.onRepairMissingTracks");
    expect(localWorkspaceSource).toContain("<ListenLocalPlaylistDirectory");
    expect(localWorkspaceSource).toContain(
      "playlists={localPlaylists.playlists}",
    );
    expect(localWorkspaceSource).toContain(
      "title={props.text.workspace.playlists}",
    );
    expect(localWorkspaceSource).toContain(
      '{ routeId: `playlist:${playlistId}` }',
    );
    expect(localWorkspaceSource.indexOf("props.text.listen.localRefresh"))
      .toBeLessThan(
        localWorkspaceSource.lastIndexOf("props.text.listen.localNewPlaylist"),
      );
    expect(css).not.toContain(
      ".app-music-workspace-control-panel__actions",
    );
  });

  test("renders only local music routes while retaining the playlist props API", () => {
    const markup = renderToStaticMarkup(
      <MusicWorkspaceSidebar
        activeRouteId="artists"
        catalog={musicCatalog}
        onNavigate={() => undefined}
        playlistItems={[{ routeId: "playlist:1", label: "playlist.morning" }]}
        playlistsSlot={<span>playlist.loading</span>}
        scope="local"
      />,
    );

    expectTextOrder(markup, [
      "route.local.search",
      "route.local.home",
      "section.library",
      "route.recently.added",
      "route.artists",
      "route.albums",
      "route.songs",
    ]);
    expect(markup).not.toContain("route.search");
    expect(markup).not.toContain("section.explore");
    expect(markup).not.toContain("section.playlists");
    expect(markup).not.toContain("playlist.morning");
    expect(markup).not.toContain("playlist.loading");
    expect(markup).toContain('data-icon="local-search"');
    expect(markup).toContain('data-icon="artists"');
    expect(markup).toContain('aria-current="page"');
  });

  test("keeps disabled sniff filters behind the waiting pet without a managed session", () => {
    const markup = renderToStaticMarkup(
      <SniffWorkspaceSidebar
        account={<span>slot.account</span>}
        catalog={sniffCatalog}
        filtersVisible={false}
        onNavigate={() => undefined}
        pet={null}
        petImageURL="pet.png"
        resourcesFilter={<div>filter.resources</div>}
        searchControl={<div>filter.search</div>}
        sourcesFilter={<div>filter.sources</div>}
        typesFilter={<div>filter.types</div>}
        waitingLabel="waiting.sniff"
      />,
    );

    expect(markup).toContain("app-sniff-workspace-sidebar");
    expect(markup).toContain("slot.account");
    expect(markup).toContain('data-filters-available="false"');
    expect(markup).not.toContain("route.search.current");
    expect(markup).toContain('data-section="search"');
    expect(markup).toContain("section.types");
    expect(markup).toContain("section.sources");
    expect(markup).toContain("section.resources");
    expect(markup).toContain("filter.search");
    expect(markup).toContain("filter.types");
    expect(markup).toContain("filter.sources");
    expect(markup).toContain("filter.resources");
    expect(markup.match(/<fieldset[^>]*disabled=""/g)).toHaveLength(4);
    expect(markup).toContain('data-section="waiting"');
    expect(markup).toContain("app-sniff-workspace-sidebar__waiting-pet");
    expect(markup).toContain("waiting.sniff");
  });

  test("renders persistent sniff filters without a search navigation item", () => {
    const markup = renderToStaticMarkup(
      <SniffWorkspaceSidebar
        activeRouteId="resources"
        catalog={sniffCatalog}
        filtersVisible
        onNavigate={() => undefined}
        pet={null}
        petImageURL="pet.png"
        resourcesFilter={<div>filter.resources</div>}
        searchControl={<div>filter.search</div>}
        sourcesFilter={<div>filter.sources</div>}
        typesFilter={<div>filter.types</div>}
        waitingLabel="waiting.sniff"
      />,
    );

    expect(markup).toContain('data-filters-available="true"');
    expect(markup).not.toContain("route.search.current");
    expect(markup).not.toContain('data-icon="sniff-search"');
    expect(markup).toContain('data-section="search"');
    expect(markup).toContain('data-section="types"');
    expect(markup).toContain("filter.types");
    expect(markup).toContain("filter.sources");
    expect(markup).toContain("filter.resources");
    expect(markup).not.toContain('data-section="waiting"');
    expect(markup).not.toContain("<fieldset disabled");
    expectTextOrder(markup, [
      "filter.search",
      "section.types",
      "filter.types",
      "section.sources",
      "filter.sources",
      "section.resources",
      "filter.resources",
    ]);
  });

  test("disables filters only while no sniff session is available", async () => {
    const source = await Bun.file(
      new URL("./SniffWorkspaceSidebar.tsx", import.meta.url),
    ).text();

    expect(source).toContain("const filtersDisabled = !filtersVisible");
    expect(source).not.toContain("structuredModeActive");
    expect(source).toContain("...(!filtersVisible");
  });

  test("puts the sniff control panel above activity and account", () => {
    const markup = renderToStaticMarkup(
      <SniffWorkspaceSidebar
        account={<span>slot.account</span>}
        activity={<span>slot.activity</span>}
        catalog={sniffCatalog}
        controlPanel={<span>slot.control.panel</span>}
        filtersVisible
        onNavigate={() => undefined}
        pet={null}
        petImageURL="pet.png"
        waitingLabel="waiting.sniff"
      />,
    );

    expectTextOrder(markup, [
      "slot.control.panel",
      "slot.activity",
      "slot.account",
    ]);
  });

  test("keeps sniff workspace actions inside the shared account footer", () => {
    const markup = renderToStaticMarkup(
      <SniffWorkspaceSidebar
        account={<span>slot.account slot.switcher slot.new</span>}
        catalog={sniffCatalog}
        filtersVisible={false}
        onNavigate={() => undefined}
        pet={null}
        petImageURL="pet.png"
        waitingLabel="waiting.sniff"
      />,
    );

    expect(markup).not.toContain("app-workspace-sidebar__footer");
    expect(markup).toContain("slot.account");
    expect(markup).toContain("slot.switcher");
    expect(markup).toContain("slot.new");
  });

  test("styles the sniff waiting overlay and compact segmented filters", async () => {
    const [css, appearance] = await Promise.all([
      Bun.file(new URL("./workspace-navigation.css", import.meta.url)).text(),
      readWorkspaceAppearance(),
    ]);

    expect(css).toMatch(
      /\.app-sniff-workspace-sidebar\[data-filters-available="false"\][^{]*\.app-workspace-nav-section:not\(\[data-section="waiting"\]\)\s*\{[^}]*pointer-events:\s*none/s,
    );
    expect(appearance).toMatch(
      /\.app-sniff-workspace-sidebar\[data-filters-available="false"\][^{]*\.app-workspace-nav-section:not\(\[data-section="waiting"\]\)\s*\{[^}]*opacity:\s*0\.3/s,
    );
    expect(css).toMatch(
      /\.app-workspace-nav-section\[data-section="waiting"\]\s*\{[^}]*position:\s*absolute[^}]*place-items:\s*center/s,
    );
    expect(css).toMatch(
      /\.app-sniff-workspace-segmented\s*\{[^}]*grid-template-columns:\s*repeat\(2,\s*minmax\(0,\s*1fr\)\)/s,
    );
  });

  test("renders every YouTube navigation route from the catalog", () => {
    const markup = renderToStaticMarkup(
      <YouTubeWorkspaceSidebar
        activeRouteId="watch-later"
        catalog={youtubeCatalog}
        onNavigate={() => undefined}
      />,
    );

    expectTextOrder(markup, [
      "route.search",
      "route.home",
      "route.subscriptions",
      "section.discover",
      "route.explore",
      "route.shorts",
      "section.collections",
      "route.liked.videos",
      "route.watch.later",
      "route.playlists",
      "route.history",
    ]);
    expect(markup).toContain('aria-label="route.watch.later"');
    expect(markup).toContain('aria-current="page"');
    expect(markup).toContain("app-youtube-workspace-sidebar");
    expect(markup).toContain('data-icon="search"');
    expect(markup).toContain('data-icon="home"');
    expect(markup).toContain('data-icon="subscriptions"');
    expect(markup).toContain('data-icon="explore"');
    expect(markup).toContain('data-icon="shorts"');
    expect(markup).toContain('data-icon="liked-videos"');
    expect(markup).toContain('data-icon="watch-later"');
    expect(markup).toContain('data-icon="playlists"');
    expect(markup).toContain('data-icon="history"');
  });

  test("keeps the shared traffic-light inset without workspace sidebar titles", async () => {
    const source = await Bun.file(
      new URL("../main/MainApp.tsx", import.meta.url),
    ).text();

    expect(source).toContain('cn("min-h-[28px]", isWindows && "wails-drag")');
    expect(source).not.toContain("activeWorkspaceLabel");
    expect(source).not.toContain('<h1 className="truncate px-2 text-sm font-semibold">');
  });
});
