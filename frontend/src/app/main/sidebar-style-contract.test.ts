function cssRule(css: string, selector: string) {
  const escaped = selector.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
  const matches = [
    ...css.matchAll(
      new RegExp(`(?:^|\\n)\\s*${escaped}\\s*\\{([^}]*)\\}`, "gs"),
    ),
  ];
  expect(matches.length).toBeGreaterThan(0);
  return matches.map((match) => match[1] ?? "").join("\n");
}

describe("main sidebar style ownership", () => {
  test("keeps static sidebar and artwork recipes in Dream CSS", async () => {
    const [source, artworkSource, listenStyles, dream] = await Promise.all([
      Bun.file(new URL("./sidebar.tsx", import.meta.url)).text(),
      Bun.file(
        new URL("../../shared/assets/listen-cover-artwork.tsx", import.meta.url),
      ).text(),
      Bun.file(
        new URL("../../shared/styles/listen.ts", import.meta.url),
      ).text(),
      Bun.file(
        new URL("../../shared/styles/dream/components.css", import.meta.url),
      ).text(),
    ]);

    for (const productSource of [source, artworkSource]) {
      expect(productSource).not.toMatch(
        /(?:backgroundImage|backgroundSize|maskImage|WebkitMaskImage|boxShadow)\s*:/,
      );
      expect(productSource).not.toMatch(/\[(?:-webkit-)?mask-image:/);
      expect(productSource).not.toMatch(/\bbg-(?:gradient|\[)/);
      expect(productSource).not.toMatch(/\bshadow-(?:sm|md|lg|xl|2xl|\[)/);
      expect(productSource).not.toMatch(/\b(?:backdrop-blur|blur-\[)/);
    }

    for (const deadExport of [
      "SidebarIconButton",
      "CDPBrowserStatusMiniButton",
      "ListenSidebarSourceBadge",
      "ListenNowPlayingMiniPlayer",
    ]) {
      expect(source).not.toContain(deadExport);
    }
    expect(listenStyles).not.toContain("LISTEN_NOW_PLAYING_PANEL_CLASS");
    expect(listenStyles).not.toContain("LISTEN_MINI_SIDE_CONTROL_CLASS");
    expect(listenStyles).not.toContain("LISTEN_MINI_PRIMARY_CONTROL_CLASS");

    const glow = cssRule(dream, ".listen-panel-artwork-glow");
    expect(glow).toContain("mask-image: linear-gradient(");
    expect(glow).toContain("filter: var(--listen-panel-artwork-glow-filter)");

    const artwork = cssRule(dream, ".listen-panel-artwork-main");
    expect(artwork).toContain("mask-image: linear-gradient(");
    expect(artwork).toContain("filter: var(--listen-panel-artwork-main-filter)");

    const vignette = cssRule(dream, ".listen-panel-bottom-vignette");
    expect(vignette).toContain("background: var(--listen-panel-bottom-vignette)");
    expect(vignette).toContain("mask-image: linear-gradient(");

    const grain = cssRule(dream, ".listen-panel-grain");
    expect(grain).toContain("repeating-radial-gradient(");
    expect(grain).toContain("background-size: 7px 7px, 11px 11px");
    expect(grain).toContain("mix-blend-mode: overlay");
    expect(grain).toContain("mask-image: linear-gradient(");

    const sideControl = cssRule(dream, ".listen-mini-side-control");
    expect(sideControl).toContain(
      "color: hsl(var(--tray-control-foreground) / 0.74)",
    );

    const primaryControl = cssRule(dream, ".listen-mini-primary-control");
    expect(primaryControl).toContain("background: hsl(var(--sidebar-primary))");
    expect(primaryControl).toContain("box-shadow:");

    const progressTrack = cssRule(dream, ".listen-mini-progress-track");
    expect(progressTrack).toContain(
      "background: hsl(var(--tray-control-foreground) / 0.16)",
    );

    const fallback = cssRule(dream, ".listen-default-cover-artwork");
    expect(fallback).toContain("background: linear-gradient(");

    const fallbackOrb = cssRule(
      dream,
      ".listen-default-cover-artwork__orb",
    );
    expect(fallbackOrb).toContain("box-shadow:");
    expect(fallbackOrb).toContain(
      "backdrop-filter: var(--app-glass-clear-filter)",
    );
  });

  test("retains only data-driven inline style values", async () => {
    const source = await Bun.file(
      new URL("./sidebar.tsx", import.meta.url),
    ).text();

    expect(source).toContain('"--listen-marquee-shift"');
    expect(source).toContain('"--listen-marquee-duration"');
    expect(source).toContain(
      'style={{ width: `${progress.bufferedPercent}%` }}',
    );
    expect(source).toContain(
      'style={{ width: `${progress.progressPercent}%` }}',
    );
    expect(source.match(/\bstyle=/g)).toHaveLength(3);
  });
});
