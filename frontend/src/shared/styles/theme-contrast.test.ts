import { describe, expect, test } from "bun:test";
import postcss, { type Rule } from "postcss";

import {
  contrastRatioForHslColors,
  parseHslToken,
  pickAccessibleForeground,
  WCAG_CONTRAST,
  type HslColor,
} from "@/lib/color";
import { XIA_THEME_PACKS } from "./xiadown-theme";

const PALETTE_TOKENS = [
  "background",
  "foreground",
  "card",
  "card-foreground",
  "popover",
  "popover-foreground",
  "secondary",
  "secondary-foreground",
  "muted",
  "muted-foreground",
  "accent",
  "accent-foreground",
  "border",
  "input",
  "sidebar-background",
  "sidebar-foreground",
  "sidebar-accent",
  "sidebar-accent-foreground",
  "sidebar-border",
  "chart-1",
  "chart-2",
  "chart-3",
  "chart-4",
  "chart-5",
] as const;

const themePackCssPromise = Bun.file(
  new URL("./dream/theme-packs.css", import.meta.url),
).text();

function declarations(rule: Rule) {
  const values = new Map<string, string>();
  rule.walkDecls((declaration) => {
    values.set(declaration.prop, declaration.value);
  });
  return values;
}

async function themeRule(id: string, appearance: "light" | "dark") {
  const root = postcss.parse(await themePackCssPromise);
  const selector =
    appearance === "dark"
      ? `:root.dark[data-xiadown-theme-pack="${id}"]`
      : `:root[data-xiadown-theme-pack="${id}"]`;
  let match: Rule | undefined;
  root.walkRules((rule) => {
    if (rule.selector === selector) match = rule;
  });
  if (!match) throw new Error(`Missing Dream theme rule: ${selector}`);
  return declarations(match);
}

async function previewRule(id: string) {
  const root = postcss.parse(await themePackCssPromise);
  let match: Rule | undefined;
  root.walkRules((rule) => {
    if (rule.selectors.includes(`[data-theme-pack-preview="${id}"]`)) {
      match = rule;
    }
  });
  if (!match) throw new Error(`Missing Dream theme preview rule: ${id}`);
  return declarations(match);
}

function hsl(token: string | undefined, label: string): HslColor {
  const color = parseHslToken(token);
  if (!color) throw new Error(`${label} is not a valid HSL token: ${token}`);
  return color;
}

function expectContrast(
  foreground: string,
  background: string,
  minimum: number,
  label: string,
) {
  const ratio = contrastRatioForHslColors(
    hsl(foreground, `${label} foreground`),
    hsl(background, `${label} background`),
  );
  expect(ratio, `${label}: ${ratio.toFixed(2)}:1`).toBeGreaterThanOrEqual(
    minimum,
  );
}

describe("Dream theme pack contrast", () => {
  test("uses WCAG relative luminance instead of the legacy YIQ threshold", () => {
    expect(pickAccessibleForeground("#EA580C")).toBe("#111111");
    expect(pickAccessibleForeground("not-a-color")).toBeNull();
  });

  test("owns every palette, preview and functional accent in Dream CSS", async () => {
    expect(XIA_THEME_PACKS).toHaveLength(12);
    for (const pack of XIA_THEME_PACKS) {
      const preview = await previewRule(pack.id);
      for (const token of [
        "--app-theme-preview-shell",
        "--app-theme-preview-sidebar",
        "--app-theme-preview-accent",
        "--app-theme-preview-accent-hsl",
        "--app-theme-preview-dark-accent",
      ]) {
        expect(preview.get(token), `${pack.id} ${token}`).toBeTruthy();
      }
      for (const appearance of ["light", "dark"] as const) {
        const variant = await themeRule(pack.id, appearance);
        for (const token of PALETTE_TOKENS) {
          expect(variant.get(`--${token}`), `${pack.id}/${appearance} --${token}`).toBeTruthy();
        }
        expect(variant.get("--app-theme-pack-functional-accent")).toMatch(
          /^#[\dA-F]{6}$/i,
        );
        expect(variant.get("--app-accent-surface")).toBe(
          variant.get("--accent"),
        );
      }
    }
  });

  test("keeps visual values out of the semantic TypeScript registry", async () => {
    const source = await Bun.file(
      new URL("./xiadown-theme.ts", import.meta.url),
    ).text();
    expect(source).not.toMatch(/#[\dA-F]{6}/i);
    expect(source).not.toMatch(/\d+\s+\d+%\s+\d+%/);
    expect(source).not.toContain("functionalAccent");
    expect(source).not.toContain("preview:");
    expect(source).not.toContain("light:");
    expect(source).not.toContain("dark:");
  });

  for (const pack of XIA_THEME_PACKS) {
    for (const appearance of ["light", "dark"] as const) {
      test(`${pack.id} ${appearance} meets text, control, and focus contrast`, async () => {
        const variant = await themeRule(pack.id, appearance);
        const token = (name: string) => {
          const value = variant.get(`--${name}`);
          if (!value) throw new Error(`Missing ${pack.id}/${appearance} --${name}`);
          return value;
        };
        const commonSurfaces = [
          ["background", token("background")],
          ["card", token("card")],
          ["popover", token("popover")],
          ["sidebar", token("sidebar-background")],
        ] as const;

        expectContrast(
          token("foreground"),
          token("background"),
          WCAG_CONTRAST.smallText,
          `${pack.id}/${appearance} foreground`,
        );
        expectContrast(
          token("muted-foreground"),
          token("background"),
          WCAG_CONTRAST.smallText,
          `${pack.id}/${appearance} muted foreground`,
        );
        expectContrast(
          token("card-foreground"),
          token("card"),
          WCAG_CONTRAST.smallText,
          `${pack.id}/${appearance} card foreground`,
        );
        expectContrast(
          token("popover-foreground"),
          token("popover"),
          WCAG_CONTRAST.smallText,
          `${pack.id}/${appearance} popover foreground`,
        );
        for (const role of ["secondary", "accent", "sidebar-accent"] as const) {
          expectContrast(
            token(`${role}-foreground`),
            token(role),
            WCAG_CONTRAST.smallText,
            `${pack.id}/${appearance} ${role} foreground`,
          );
        }

        for (const [surfaceName, surface] of commonSurfaces) {
          expectContrast(
            token("app-accent-text"),
            surface,
            WCAG_CONTRAST.smallText,
            `${pack.id}/${appearance} accent text on ${surfaceName}`,
          );
          expectContrast(
            token("app-accent-ring"),
            surface,
            WCAG_CONTRAST.nonText,
            `${pack.id}/${appearance} focus ring on ${surfaceName}`,
          );
          expectContrast(
            token("app-accent-solid"),
            surface,
            WCAG_CONTRAST.nonText,
            `${pack.id}/${appearance} solid control boundary on ${surfaceName}`,
          );
        }

        expectContrast(
          token("app-accent-on-solid"),
          token("app-accent-solid"),
          WCAG_CONTRAST.smallText,
          `${pack.id}/${appearance} solid button label`,
        );
      });
    }
  }

  test("lets foundation CSS own the native control color scheme", async () => {
    const [css, runtime] = await Promise.all([
      Bun.file(new URL("./dream/foundation.css", import.meta.url)).text(),
      Bun.file(new URL("./theme-runtime.ts", import.meta.url)).text(),
    ]);
    expect(css).toMatch(/:root\s*\{[\s\S]*?color-scheme:\s*light;/);
    expect(css).toMatch(/\.dark\s*\{[\s\S]*?color-scheme:\s*dark;/);
    expect(runtime).not.toContain("style.colorScheme");
  });
});
