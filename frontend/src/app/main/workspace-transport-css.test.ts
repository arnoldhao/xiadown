import { describe, expect, test } from "bun:test";

describe("music workspace transport CSS", () => {
  test("centers the floating transport within safe side gutters", async () => {
    const css = await Bun.file(
      new URL("../../shared/styles/dream/transport.css", import.meta.url),
    ).text();

    expect(css).toContain("left: 50%");
    expect(css).toContain("right: auto");
    expect(css).toContain("width: min(620px, calc(100% - 40px))");
    expect(css).toContain("max-width: calc(100% - 40px)");
    expect(css).toContain("transform: translateX(-50%)");
    expect(css).not.toContain("left: 22px");
    expect(css).not.toContain("right: 22px");
  });

  test("reserves intrinsic side controls and caps the centered timeline", async () => {
    const css = await Bun.file(
      new URL("../../shared/styles/dream/transport.css", import.meta.url),
    ).text();

    expect(css).toContain(
      "--app-workspace-transport-center-max: 17rem",
    );
    expect(css).toContain(
      "minmax(0, var(--app-workspace-transport-center-max))",
    );
    const transportRule = css.match(
      /\.app-workspace-transport\s*\{([^}]*)\}/s,
    )?.[1];
    expect(transportRule?.match(/max-content/g)).toHaveLength(2);
    expect(css).toContain("justify-content: center");
    expect(css).toContain("column-gap: var(--app-space-3)");
    expect(css).toMatch(
      /@container workspace-primary \(max-width: 980px\)[^{]*\{[\s\S]*?--app-workspace-transport-center-max:\s*15rem;[\s\S]*?column-gap:\s*var\(--app-space-2-5\)/,
    );
    expect(css).not.toContain("minmax(135px, 0.72fr)");
    expect(css).not.toContain("minmax(82px, 0.5fr)");
    expect(css).not.toContain("@media (max-width: 980px)");
  });

  test("matches a compact mini-player with square artwork", async () => {
    const [source, css, buttonContract] = await Promise.all([
      Bun.file(new URL("./WorkspaceTransportBar.tsx", import.meta.url)).text(),
      Bun.file(
        new URL("../../shared/styles/dream/transport.css", import.meta.url),
      ).text(),
      Bun.file(
        new URL("../../shared/styles/dream/button-contract.css", import.meta.url),
      ).text(),
    ]);
    const artworkButtonRule = css.match(
      /\.app-workspace-transport__artwork-open\s*\{([^}]*)\}/s,
    )?.[1];

    expect(css).toContain("height: var(--app-control-bar-height)");
    expect(css).toContain("min-height: var(--app-control-bar-height)");
    expect(css).not.toContain("min-height: 72px");
    expect(css).toContain("grid-template-columns: 34px minmax(0, 1fr) 27px");
    expect(css).toMatch(
      /\.app-workspace-transport__artwork \{[^}]*--app-radius-control-compact:\s*var\(--app-radius-control-inner\);[^}]*width: 34px;[^}]*height: 34px;/s,
    );
    expect(css).toContain("aspect-ratio: 1 / 1");
    expect(css).toContain("object-fit: cover");
    expect(css).toContain("grid-template-rows: 36px 10px");
    expect(css).toContain("grid-row: 2");
    expect(source).toMatch(
      /shape="square"\s+className="app-workspace-transport__artwork-open"/,
    );
    expect(source).toMatch(
      /shape="square"\s+className="app-workspace-transport__fullscreen"/,
    );
    expect(artworkButtonRule).toContain("--app-button-inline-size: 100%");
    expect(artworkButtonRule).toContain("--app-button-block-size: 100%");
    expect(artworkButtonRule).toContain("--app-button-padding: 0");
    expect(artworkButtonRule).toContain("--app-button-border: 0");
    expect(buttonContract).toContain(
      "border: var(--app-button-border, 1px solid transparent)",
    );
  });

  test("preserves the transport control hierarchy through shared Button variables", async () => {
    const [source, css] = await Promise.all([
      Bun.file(new URL("./WorkspaceTransportBar.tsx", import.meta.url)).text(),
      Bun.file(
        new URL("../../shared/styles/dream/transport.css", import.meta.url),
      ).text(),
    ]);
    const standardRule = css.match(
      /\.app-workspace-transport__icon-button\s*\{([^}]*)\}/s,
    )?.[1];
    const stepRule = css.match(
      /\[data-transport-emphasis="step"\]\s*\{([^}]*)\}/s,
    )?.[1];
    const primaryRule = css.match(
      /\.app-workspace-transport__button--primary\s*\{([^}]*)\}/s,
    )?.[1];
    const artistRule = css.match(
      /\.app-workspace-transport__track-artist-open\s*\{([^}]*)\}/s,
    )?.[1];
    const favoriteRule = css.match(
      /\.app-workspace-transport__favorite\s*\{([^}]*)\}/s,
    )?.[1];

    expect(source.match(/emphasis="step"/g)).toHaveLength(2);
    expect(source).toContain('emphasis="primary"');
    expect(source).toContain("data-transport-emphasis={emphasis}");
    expect(standardRule).toContain("--app-button-inline-size: 27px");
    expect(standardRule).toContain("--app-button-block-size: 30px");
    expect(standardRule).toContain("--app-button-icon-size: 14px");
    expect(stepRule).toContain("--app-button-inline-size: 30px");
    expect(stepRule).toContain("--app-button-block-size: 34px");
    expect(stepRule).toContain("--app-button-icon-size: 18px");
    expect(primaryRule).toContain("--app-button-inline-size: 34px");
    expect(primaryRule).toContain("--app-button-block-size: 38px");
    expect(primaryRule).toContain("--app-button-icon-size: 20px");
    expect(artistRule).toContain("--app-button-block-size: auto");
    expect(artistRule).toContain("--app-button-padding: 0");
    expect(favoriteRule).toContain("--app-button-inline-size: 18px");
    expect(favoriteRule).toContain("--app-button-icon-size: 12px");
  });

  test("keeps idle artwork passive while the empty transport remains visible", async () => {
    const css = await Bun.file(
      new URL("../../shared/styles/dream/transport.css", import.meta.url),
    ).text();

    expect(css).toContain('.app-workspace-transport[data-state="idle"]');
    expect(css).toMatch(
      /\[data-state="idle"\][^{]*\.app-workspace-transport__fullscreen\s*\{[^}]*display:\s*none/s,
    );
    expect(css).toMatch(
      /\[data-state="idle"\][^{]*\.app-workspace-transport__artwork-open\s*\{[^}]*cursor:\s*default/s,
    );
  });

  test("delegates the floating capsule material to the shared glass contract", async () => {
    const [source, css] = await Promise.all([
      Bun.file(new URL("./WorkspaceTransportBar.tsx", import.meta.url)).text(),
      Bun.file(new URL("../../shared/styles/dream/transport.css", import.meta.url)).text(),
    ]);
    const surfaceRule = css.match(
      /\.app-workspace-transport \{([^}]*)\}/,
    )?.[1];

    expect(source).toContain("<GlassGroup");
    expect(source).toContain('import { Button } from "@/shared/ui/button"');
    expect(source).not.toContain("<button");
    expect(source).toContain('surfaceRole="control"');
    expect(source).not.toContain('material="regular"');
    expect(source).toContain('elevation="floating"');
    expect(source).toContain('shape="capsule"');
    expect(surfaceRule).toContain(
      "z-index: var(--app-layer-floating-controls)",
    );
    expect(surfaceRule).not.toContain("--app-glass-");
    expect(surfaceRule).not.toContain("border-radius");
    expect(surfaceRule).not.toContain("background:");
    expect(surfaceRule).not.toContain("box-shadow:");
    expect(surfaceRule).not.toContain("backdrop-filter:");
    expect(css).not.toContain("backdrop-filter:");
    expect(css).toContain("transform: scale(1.07)");
    expect(css).toContain(
      ".app-workspace-transport__track-details:focus-within .app-workspace-transport__favorite",
    );
  });

  test("keeps live playback on the standard timeline and supports inline volume", async () => {
    const css = await Bun.file(
      new URL("../../shared/styles/dream/transport.css", import.meta.url),
    ).text();

    expect(css).not.toContain(".app-workspace-transport__timeline--live");
    expect(css).not.toContain(".app-workspace-transport__track--live");
    expect(css).toContain(".app-workspace-transport__timeline:hover");
    expect(css).toContain(".app-workspace-transport__timeline-input");
    expect(css).toContain(
      ".app-workspace-transport__timeline-input:focus-visible",
    );
    expect(css).toContain(
      '.app-workspace-transport__right[data-volume-expanded="true"]',
    );
    expect(css).toContain(".app-workspace-transport__volume-editor");
    expect(css).toContain("opacity 160ms ease");
  });

  test("reveals times above a thickened timeline while frosting metadata", async () => {
    const css = await Bun.file(
      new URL("../../shared/styles/dream/transport.css", import.meta.url),
    ).text();

    expect(css).toContain(".app-workspace-transport__timeline::before");
    expect(css).toContain("inset: 0");
    expect(css).toMatch(
      /\.app-workspace-transport__times \{[^}]*bottom: 7px;/s,
    );
    expect(css).toMatch(
      /\.app-workspace-transport__timeline:hover \.app-workspace-transport__track,[^{]*\{[^}]*height: 5px;/s,
    );
    expect(css).toContain("opacity: 0.18");
    expect(css).toContain("filter: blur(3px) saturate(0.58)");
    expect(css).toContain("transform: translateY(-1px) scale(0.985)");
    expect(css).not.toMatch(
      /:has\([\s\S]*?\.app-workspace-transport__timeline:hover[\s\S]*?\)\s*>\s*:not\([^}]*pointer-events:\s*none/,
    );
  });

  test("centers the More menu above its trigger", async () => {
    const [source, css] = await Promise.all([
      Bun.file(new URL("./WorkspaceTransportBar.tsx", import.meta.url)).text(),
      Bun.file(new URL("../../shared/styles/dream/transport.css", import.meta.url)).text(),
    ]);

    expect(source).toContain(
      'side="top"\n            align="center"',
    );
    expect(source).toContain('className="app-workspace-transport__more-menu"');
    expect(css).toContain("width: max-content");
    expect(css).toContain("min-width: 0");
  });

  test("keeps icon controls unadorned until interaction", async () => {
    const css = await Bun.file(
      new URL("../../shared/styles/dream/transport.css", import.meta.url),
    ).text();
    const iconRule = css.match(
      /\.app-workspace-transport__icon-button \{([^}]*)\}/,
    )?.[1];
    const primaryRule = css.match(
      /\.app-workspace-transport__button--primary \{([^}]*)\}/,
    )?.[1];

    expect(iconRule).toContain("background: transparent");
    expect(iconRule).toContain("box-shadow: none");
    expect(primaryRule).toContain("background: transparent");
    expect(primaryRule).toContain("box-shadow: none");
    expect(primaryRule).not.toContain("hsl(var(--primary))");
  });

  test("keeps pointer focus clean and keyboard focus inside controls", async () => {
    const css = await Bun.file(
      new URL("../../shared/styles/dream/transport.css", import.meta.url),
    ).text();

    expect(css).toMatch(
      /:root:not\(\[data-input-modality="pointer"\]\)[^{]*\.app-workspace-transport__icon-button:focus-visible\s*\{[^}]*outline-offset:\s*-2px/s,
    );
    expect(css).toMatch(
      /:root:not\(\[data-input-modality="pointer"\]\)[^{]*\.app-workspace-transport__track-details:has\(:focus-visible\)\s*\{[^}]*outline-offset:\s*-2px/s,
    );
    expect(css).toMatch(
      /:root:not\(\[data-input-modality="pointer"\]\)[^{]*\.app-workspace-transport__timeline:has\([\s\S]*?focus-visible[\s\S]*?\)\s*\{[^}]*outline-offset:\s*-2px/s,
    );
  });
});
