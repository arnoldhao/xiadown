import { describe, expect, test } from "bun:test";
import { readdir } from "node:fs/promises";

import {
  createDreamStyleRegistry,
  filterDreamStyleRegistry,
  parseDreamCssModule,
  splitCssSelectorList,
} from "./dream-style-registry";

const SAMPLE_CSS = `/* comment with .not-a-selector { } */
:root {
  --app-surface: hsl(0 0% 100% / 0.8);
}

.dream-card,
:is(.dream-button, .dream-link)[data-state="open"] {
  --app-surface: hsl(0 0% 0% / 0.2);
  color: var(--app-surface);
}

@layer dream-foundation;

@property --dream-progress {
  syntax: "<number>";
  inherits: false;
  initial-value: 0;
}

@supports (backdrop-filter: blur(1px)) {
  .dream-glass { backdrop-filter: blur(1px); }
}

@container card (min-width: 20rem) {
  .dream-card__body { display: grid; }
}

@media (prefers-reduced-motion: no-preference) {
  .dream-card:hover { animation: dream-in 120ms ease; }
}

@keyframes dream-in {
  from { opacity: 0; }
  50%, to { opacity: 1; }
}`;

describe("Dream CSS inventory parser", () => {
  test("lets Vite transform the raw Dream glob without a false runtime guard", async () => {
    const source = await Bun.file(
      new URL("./dream-style-registry.ts", import.meta.url),
    ).text();

    expect(source).toContain("return import.meta.glob(");
    expect(source).not.toContain('typeof import.meta.glob !== "function"');
  });

  test("splits selector lists only at top-level commas", () => {
    expect(
      splitCssSelectorList(
        '.one, :is(.two, .three), [data-label="a,b"], .four:not(.five, .six)',
      ),
    ).toEqual([
      ".one",
      ":is(.two, .three)",
      '[data-label="a,b"]',
      ".four:not(.five, .six)",
    ]);
  });

  test("inventories tokens, selectors, at-rules and keyframes", () => {
    const module = parseDreamCssModule("/dream/primitives.css", SAMPLE_CSS);

    expect(module.id).toBe("primitives");
    expect(module.tokens.map(({ name, value }) => [name, value])).toEqual([
      ["--app-surface", "hsl(0 0% 100% / 0.8)"],
      ["--app-surface", "hsl(0 0% 0% / 0.2)"],
    ]);
    expect(module.selectors.map(({ name }) => name)).toEqual([
      ":root",
      ".dream-card",
      ':is(.dream-button, .dream-link)[data-state="open"]',
      ".dream-glass",
      ".dream-card__body",
      ".dream-card:hover",
    ]);
    expect(module.atRules.map(({ name, value }) => [name, value])).toEqual([
      ["@layer", "dream-foundation"],
      ["@property", "--dream-progress"],
      ["@supports", "(backdrop-filter: blur(1px))"],
      ["@container", "card (min-width: 20rem)"],
      ["@media", "(prefers-reduced-motion: no-preference)"],
    ]);
    expect(module.keyframes.map(({ name }) => name)).toEqual(["dream-in"]);
    expect(module.selectors.map(({ name }) => name)).not.toContain("from");
    expect(module.selectors.map(({ name }) => name)).not.toContain("50%");
    expect(module.tokens[0]?.line).toBe(3);
  });

  test("keeps the Dream entrypoint import order as statement at-rules", () => {
    const module = parseDreamCssModule(
      "/styles/dream.css",
      '@import "./dream/tokens.css";\n@import "./dream/glass.css";\n@import "./dream/motion.css";',
    );

    expect(module.id).toBe("dream");
    expect(module.atRules.map(({ name }) => name)).toEqual([
      "@import",
      "@import",
      "@import",
    ]);
    expect(module.atRules.map(({ value }) => value)).toEqual([
      '"./dream/tokens.css"',
      '"./dream/glass.css"',
      '"./dream/motion.css"',
    ]);
    expect(module.atRules.map(({ line }) => line)).toEqual([1, 2, 3]);
  });

  test("the real entrypoint imports every Dream module exactly once", async () => {
    const entryUrl = new URL("../../shared/styles/dream.css", import.meta.url);
    const moduleDirectoryUrl = new URL(
      "../../shared/styles/dream/",
      import.meta.url,
    );
    const filenames = (await readdir(moduleDirectoryUrl))
      .filter((filename) => filename.endsWith(".css"))
      .sort();
    const entry = parseDreamCssModule(
      entryUrl.pathname,
      await Bun.file(entryUrl).text(),
    );
    const imports = entry.atRules
      .filter(({ name }) => name === "@import")
      .map(({ value }) => value?.replaceAll('"', ""));

    expect(imports).toHaveLength(filenames.length);
    expect(new Set(imports).size).toBe(imports.length);
    expect([...imports].sort()).toEqual(
      filenames.map((filename) => `./dream/${filename}`),
    );
  });

  test("catalogs migrated shared and Sniff semantic selectors automatically", async () => {
    const modulePaths = [
      "../../shared/styles/dream/controls.css",
      "../../shared/styles/dream/components.css",
      "../../shared/styles/dream/workflows.css",
    ] as const;
    const sources = Object.fromEntries(
      await Promise.all(
        modulePaths.map(async (path) => {
          const url = new URL(path, import.meta.url);
          return [url.pathname, await Bun.file(url).text()] as const;
        }),
      ),
    );
    const registry = createDreamStyleRegistry(sources);
    const selectors = registry.modules.flatMap((module) =>
      module.selectors.map((entry) => entry.name),
    );

    for (const selector of [
      ".app-dream-tooltip",
      ".app-whats-new-eyebrow",
      ".app-dialog-markdown",
      ".app-managed-profile-avatar",
      ".app-sniff-desk-stat-label",
    ]) {
      expect(selectors).toContain(selector);
    }
  });

  test("sorts modules, reports totals and filters at entry granularity", () => {
    const registry = createDreamStyleRegistry({
      "/dream/z-motion.css": "@keyframes drift { to { opacity: 1; } }",
      "/dream/a-token.css": ":root { --app-accent: blue; } .accent { color: blue; }",
    });

    expect(registry.modules.map(({ id }) => id)).toEqual(["a-token", "z-motion"]);
    expect(registry.totals).toEqual({
      modules: 2,
      "at-rule": 0,
      keyframe: 1,
      selector: 2,
      token: 1,
    });

    const filtered = filterDreamStyleRegistry(registry, "accent");
    expect(filtered.modules).toHaveLength(1);
    expect(filtered.modules[0]?.tokens).toHaveLength(1);
    expect(filtered.modules[0]?.selectors).toHaveLength(1);
    expect(filtered.totals).toEqual({
      modules: 1,
      "at-rule": 0,
      keyframe: 0,
      selector: 1,
      token: 1,
    });
  });
});
