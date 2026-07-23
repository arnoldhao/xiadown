import { describe, expect, test } from "bun:test";
import { join } from "node:path";
import { fileURLToPath } from "node:url";
import postcss, { type Declaration } from "postcss";
import * as ts from "typescript";

const frontendRoot = fileURLToPath(new URL("../../../", import.meta.url));

const FORBIDDEN_APPEARANCE_PROPERTY = /^(?:color|color-scheme|background(?:-.+)?|border(?:-.+)?|border-radius|outline(?:-.+)?|box-shadow|text-shadow|filter|-webkit-backdrop-filter|backdrop-filter|font|font-.+|line-height|letter-spacing|text-align|text-transform|text-decoration(?:-.+)?|accent-color|caret-color|fill|stroke(?:-.+)?|mix-blend-mode|animation(?:-.+)?|transition(?:-.+)?|opacity|cursor|resize|forced-color-adjust|appearance|clip-path|mask(?:-.+)?|-webkit-mask(?:-.+)?)$/;
const VISUAL_CUSTOM_PROPERTY = /^--.*(?:accent|background|blur|color|fill|filter|font|foreground|line|material|motion|opacity|radius|shadow|surface|tone|typography|wash)/;
const ALLOWED_DYNAMIC_INTERFACE_PROPERTIES = new Set([
  "--listen-lyrics-focus-line-width",
  "--listen-native-video-hole-h",
  "--listen-native-video-hole-r",
  "--listen-native-video-hole-w",
  "--listen-native-video-hole-x",
  "--listen-native-video-hole-y",
  "--listen-native-video-outside-mask",
  "--listen-workspace-fullscreen-now-playing-width",
]);

function belongsToKeyframes(declaration: Declaration): boolean {
  for (let parent = declaration.parent; parent; parent = parent.parent) {
    if (parent.type === "atrule" && /keyframes$/i.test(parent.name)) {
      return true;
    }
  }
  return false;
}

function isLyricsOrVisualizerMotion(selector: string): boolean {
  return /\.listen-(?:lyrics-scroll|artwork-visualizer|player-inline-visualizer)/.test(
    selector,
  );
}

const APPEARANCE_UTILITY_TOKEN = new RegExp(
  String.raw`^(?:[^:\s]+:)*!?(?:` +
    [
      String.raw`bg-.+`,
      String.raw`text-.+`,
      String.raw`font-.+`,
      String.raw`leading-.+`,
      String.raw`tracking-.+`,
      String.raw`rounded(?:-.+)?`,
      String.raw`border(?:-.+)?`,
      String.raw`shadow(?:-.+)?`,
      String.raw`ring(?:-.+)?`,
      String.raw`outline-.+`,
      String.raw`opacity-.+`,
      String.raw`cursor-.+`,
      String.raw`transition(?:-.+)?`,
      String.raw`duration-.+`,
      String.raw`ease-.+`,
      String.raw`animate-.+`,
      String.raw`fade-.+`,
      String.raw`slide-(?:in|out)-.+`,
      String.raw`zoom-(?:in|out)-.+`,
      String.raw`filter(?:-.+)?`,
      String.raw`blur-.+`,
      String.raw`backdrop-.+`,
      String.raw`drop-shadow-.+`,
      String.raw`brightness-.+`,
      String.raw`contrast-.+`,
      String.raw`saturate-.+`,
      String.raw`grayscale(?:-.+)?`,
      String.raw`invert(?:-.+)?`,
      String.raw`sepia(?:-.+)?`,
      String.raw`hue-rotate-.+`,
      String.raw`mix-blend-.+`,
      String.raw`fill-.+`,
      String.raw`stroke-.+`,
      String.raw`decoration-.+`,
      String.raw`underline-offset-.+`,
      String.raw`uppercase|lowercase|capitalize|normal-case`,
      String.raw`italic|not-italic|underline|no-underline`,
      String.raw`tabular-nums|lining-nums|oldstyle-nums|proportional-nums`,
      String.raw`antialiased|subpixel-antialiased`,
    ].join("|") +
    String.raw`)$`,
);

function isClassContext(node: ts.Node): boolean {
  if (
    ts.isJsxAttribute(node) &&
    ts.isIdentifier(node.name) &&
    /class/i.test(node.name.text)
  ) {
    return true;
  }

  if (ts.isCallExpression(node) && ts.isIdentifier(node.expression)) {
    return ["classNames", "clsx", "cn", "cva"].includes(node.expression.text);
  }

  if (
    (ts.isVariableDeclaration(node) || ts.isPropertyAssignment(node)) &&
    ts.isIdentifier(node.name)
  ) {
    return /class/i.test(node.name.text);
  }

  return false;
}

function collectAppearanceUtilityTokens(
  source: string,
  fileName: string,
): string[] {
  const sourceFile = ts.createSourceFile(
    fileName,
    source,
    ts.ScriptTarget.Latest,
    true,
    fileName.endsWith(".tsx") ? ts.ScriptKind.TSX : ts.ScriptKind.TS,
  );
  const failures = new Set<string>();

  const inspect = (node: ts.Node) => {
    if (ts.isStringLiteralLike(node)) {
      for (const token of node.text.split(/\s+/).filter(Boolean)) {
        if (APPEARANCE_UTILITY_TOKEN.test(token)) {
          const { line } = sourceFile.getLineAndCharacterOfPosition(
            node.getStart(sourceFile),
          );
          failures.add(`${fileName}:${line + 1} ${token}`);
        }
      }
    }
    ts.forEachChild(node, inspect);
  };

  const visit = (node: ts.Node) => {
    if (isClassContext(node)) {
      inspect(node);
      return;
    }
    ts.forEachChild(node, visit);
  };
  visit(sourceFile);
  return [...failures];
}

describe("Listen style ownership", () => {
  test("keeps feature CSS limited to composition, responsive layout, and domain animation", async () => {
    const css = await Bun.file(
      new URL("../../app/main/listen/listen.css", import.meta.url),
    ).text();
    const root = postcss.parse(css);
    const violations: string[] = [];

    root.walkDecls((declaration) => {
      if (belongsToKeyframes(declaration)) {
        return;
      }

      if (FORBIDDEN_APPEARANCE_PROPERTY.test(declaration.prop)) {
        violations.push(
          `${declaration.source?.start?.line ?? 0} ${declaration.prop}`,
        );
      }

      if (
        VISUAL_CUSTOM_PROPERTY.test(declaration.prop) &&
        !ALLOWED_DYNAMIC_INTERFACE_PROPERTIES.has(declaration.prop)
      ) {
        violations.push(
          `${declaration.source?.start?.line ?? 0} visual recipe ${declaration.prop}`,
        );
      }

      const selector = declaration.parent?.selector ?? "";
      if (
        declaration.prop === "transform" &&
        /:(?:hover|focus|focus-visible|focus-within|active)\b|\[data-(?:active|selected|visible|state)/.test(
          selector,
        ) &&
        !isLyricsOrVisualizerMotion(selector)
      ) {
        violations.push(
          `${declaration.source?.start?.line ?? 0} interactive transform`,
        );
      }

      if (
        /#[0-9a-f]{3,8}\b|\b(?:rgba?|hsla?)\(/i.test(declaration.value) &&
        declaration.prop !== "--listen-native-video-outside-mask"
      ) {
        violations.push(
          `${declaration.source?.start?.line ?? 0} literal palette ${declaration.prop}`,
        );
      }
    });

    expect(violations).toEqual([]);
    expect(css).not.toContain(".app-glass-surface.listen-lyrics-focus");
    expect(css).not.toContain("--app-glass-");
    expect(css).not.toMatch(/(?:-webkit-)?backdrop-filter\s*:/);
    expect(css).not.toContain("data-platform");
    expect(css).not.toContain("data-window-material");
    expect(css).toContain("grid-template-columns:");
    expect(css).toContain("container: listen-lyrics-scroll / inline-size;");
    expect(css).toContain("@keyframes listen-lyrics-focus-prism-in");
    expect(css).toContain("--listen-native-video-outside-mask:");
  });

  test("imports and defines Listen palette, type, controls, status, focus, and motion in Dream", async () => {
    const [entry, appearance] = await Promise.all([
      Bun.file(new URL("./dream.css", import.meta.url)).text(),
      Bun.file(new URL("./dream/listen.css", import.meta.url)).text(),
    ]);
    const root = postcss.parse(appearance);
    const literalPaletteViolations: string[] = [];

    root.walkDecls((declaration) => {
      const isAlphaMask =
        /(?:^|-)mask(?:-|$)/.test(declaration.prop) ||
        declaration.prop === "--listen-native-video-outside-mask";
      if (
        !isAlphaMask &&
        /#(?:000|fff)(?:[0-9a-f]{3,5})?\b|rgb\((?:0 0 0|255 255 255)(?:\s*\/|\))/i.test(
          declaration.value,
        )
      ) {
        literalPaletteViolations.push(
          `${declaration.source?.start?.line ?? 0} ${declaration.prop}`,
        );
      }
    });

    expect(entry).toContain('@import "./dream/listen.css";');
    expect(appearance).toContain(".listen-list-item-button");
    expect(appearance).toContain(".listen-control-icon-button");
    expect(appearance).toContain(".listen-player-icon-button");
    expect(appearance).toContain(".listen-primary-play-button");
    expect(appearance).toContain(".listen-live-status-badge");
    expect(appearance).toContain("var(--app-status-tone-error)");
    expect(appearance).toContain("var(--app-status-surface-orphan)");
    expect(appearance).toContain("var(--app-media-chrome-canvas)");
    expect(appearance).toContain("var(--app-media-chrome-foreground)");
    expect(appearance).toContain(":focus-visible");
    for (const selector of [
      ".listen-scrolling-text__artist:focus-visible",
      ".listen-workspace-queue-row__identity:focus-visible",
      ".listen-track-list-row:focus-visible",
      ".listen-queue-move-button:focus-visible",
      ".listen-queue-popup-row__button:focus-visible",
    ]) {
      expect(appearance).toContain(selector);
    }
    expect(appearance).toContain("@media (forced-colors: active)");
    expect(appearance).toContain("@media (prefers-reduced-motion: reduce)");
    expect(literalPaletteViolations).toEqual([]);
  });

  test("keeps the fullscreen artwork wash recipe in Dream", async () => {
    const workflows = await Bun.file(
      new URL("./dream/workflows.css", import.meta.url),
    ).text();
    const washRule = workflows.match(
      /\.listen-workspace-fullscreen-backdrop__wash\s*\{([^}]*)\}/s,
    )?.[1];

    expect(washRule).toContain("hsl(var(--background) / 0.56)");
    expect(washRule).toContain("backdrop-filter: saturate(0.92)");
  });

  test("keeps Hush column-card paint under the Listen Dream owner", async () => {
    const [listen, workflows] = await Promise.all([
      Bun.file(new URL("./dream/listen.css", import.meta.url)).text(),
      Bun.file(new URL("./dream/workflows.css", import.meta.url)).text(),
    ]);

    expect(listen).toMatch(
      /\.listen-hush-column-card\s*\{[^}]*background:\s*var\(--dream-settings-card-surface\);[^}]*box-shadow:\s*var\(--dream-settings-card-shadow\);/s,
    );
    expect(workflows).not.toContain(".listen-hush-column-card");
  });

  test("keeps category tint and strip recipes in Dream while React publishes only data color", async () => {
    const [source, listen] = await Promise.all([
      Bun.file(new URL("../../app/main/listen/ui.tsx", import.meta.url)).text(),
      Bun.file(new URL("./dream/listen.css", import.meta.url)).text(),
    ]);

    expect(source).toContain('"--listen-category-color": swatch');
    expect(source).toContain("listen-category-color-strip");
    expect(source).not.toContain("backgroundColor");
    expect(source).not.toContain("${swatch}20");
    expect(listen).toContain('data-category-color="true"');
    expect(listen).toContain("var(--listen-category-color) 12.5%");
    expect(listen).toMatch(
      /\.listen-category-color-strip\s*\{[^}]*background:\s*var\(--listen-category-color\);/s,
    );
  });

  test("keeps song thumbnails rectangular while Artist artwork owns the circle shape", async () => {
    const appearance = await Bun.file(
      new URL("./dream/listen.css", import.meta.url),
    ).text();
    const trackArtworkSelectors = [
      ".listen-local-track-row__artwork",
      ".listen-local-add-track-row__artwork",
      ".listen-queue-popup-artwork",
      ".listen-local-artwork",
      ".listen-collection-track-row__artwork",
      ".listen-muse-list-artwork",
    ];

    for (const selector of trackArtworkSelectors) {
      const escapedSelector = selector.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
      const rules = [
        ...appearance.matchAll(
          new RegExp(`${escapedSelector}\\s*\\{([^}]*)\\}`, "gs"),
        ),
      ].map((match) => match[1]);
      expect(
        rules.some((rule) =>
          rule.includes("border-radius: var(--app-radius-control-inner)"),
        ),
        selector,
      ).toBe(true);
      expect(rules.join("\n"), selector).not.toContain("--app-radius-capsule");
      expect(rules.join("\n"), selector).not.toContain("--app-radius-xl");
    }

    expect(appearance).toMatch(
      /\.listen-muse-card-artwork\[data-shape="circle"\],[\s\S]*?border-radius:\s*var\(--app-radius-capsule\)/,
    );
    expect(appearance).toMatch(
      /\.listen-avatar\[data-shape="circle"\]\s*\{[^}]*border-radius:\s*var\(--app-radius-capsule\)/s,
    );
  });

  test("keeps shared class constants structural and delegates controls and statuses to primitives", async () => {
    const [source, playback, queue, companion, controls, ui, hush] =
      await Promise.all([
        Bun.file(new URL("./listen.ts", import.meta.url)).text(),
        Bun.file(
          new URL("../../app/main/listen/playback-ui.tsx", import.meta.url),
        ).text(),
        Bun.file(
          new URL("../../app/main/listen/queue-popups.tsx", import.meta.url),
        ).text(),
        Bun.file(
          new URL(
            "../../app/main/listen/workspace-companion.tsx",
            import.meta.url,
          ),
        ).text(),
        Bun.file(
          new URL(
            "../../app/main/listen/playback-controls.tsx",
            import.meta.url,
          ),
        ).text(),
        Bun.file(
          new URL("../../app/main/listen/ui.tsx", import.meta.url),
        ).text(),
        Bun.file(
          new URL("../../app/main/listen/HushLiveList.tsx", import.meta.url),
        ).text(),
      ]);

    expect(source).not.toContain("React.CSSProperties");
    expect(source).not.toMatch(/_STYLE\s*(?::[^=]+)?=\s*\{/);
    expect(source).not.toMatch(
      /(?:^|[\s"])(?:dark:|hover:|active:|focus:|focus-visible:|data-\[[^\]]+\]:)*(?:bg-|text-|border-|shadow-|ring-|outline-|rounded-|opacity-|transition|duration-|ease-|font-|leading-|tracking-)/m,
    );
    expect(source).toContain("listen-list-item-button");
    expect(source).toContain("listen-player-icon-button");
    expect(source).toContain("listen-primary-play-button");
    for (const primitiveConsumer of [playback, queue, companion, controls]) {
      expect(primitiveConsumer).toContain("@/shared/ui/button");
    }
    expect(playback).toContain("<Button");
    expect(queue).toContain("<Button");
    expect(companion).toContain("<Button");
    expect(controls).toContain("<Button");
    expect(ui).toContain("<StatusBadge");
    expect(hush).toContain("<StatusBadge");
  });

  test("keeps every appearance utility out of Listen sources", async () => {
    const glob = new Bun.Glob("src/app/main/listen/**/*.{ts,tsx}");
    const violations: string[] = [];
    const scannedFiles = new Set<string>();

    for await (const file of glob.scan({ cwd: frontendRoot, onlyFiles: true })) {
      if (/\.(?:test|spec)\.[^/]+$/.test(file)) {
        continue;
      }
      scannedFiles.add(file);
      const source = await Bun.file(join(frontendRoot, file)).text();
      violations.push(
        ...collectAppearanceUtilityTokens(source, file),
      );
    }

    expect(violations).toEqual([]);
    expect([...scannedFiles]).toEqual(expect.arrayContaining([
      "src/app/main/listen/LocalLibraryWorkspace.tsx",
      "src/app/main/listen/api.ts",
      "src/app/main/listen/playback-controls.tsx",
    ]));
  });

  test("keeps fullscreen stage geometry in Dream instead of a static React style branch", async () => {
    const [nativeVideoSurfaces, components] = await Promise.all([
      Bun.file(
        new URL(
          "../../app/main/listen/native-video-surfaces.tsx",
          import.meta.url,
        ),
      ).text(),
      Bun.file(new URL("./dream/components.css", import.meta.url)).text(),
    ]);

    expect(nativeVideoSurfaces).toContain(
      'if (props.variant === "fullscreen") {\n      return undefined;',
    );
    expect(nativeVideoSurfaces).not.toMatch(
      /props\.variant === "fullscreen"[\s\S]{0,120}width:\s*"100%"/,
    );
    expect(components).toMatch(
      /\.listen-inline-video-frame-fullscreen \.listen-inline-video-stage\s*\{[^}]*width:\s*100%;[^}]*height:\s*100%;/s,
    );
  });
});
