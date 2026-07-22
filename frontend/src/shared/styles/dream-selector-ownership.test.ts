import { describe, expect, test } from "bun:test";
import postcss from "postcss";

const DOMAIN_DREAM_MODULES = {
  "activity.css": /\.app-(?:activity|workspace)-/,
  "completed.css": /\.app-(?:completed|flv|media)-/,
  "library.css": /\.(?:app-library-|library-data-management)/,
  "listen.css": /(?:\.|\[data-)listen-/,
  "pets.css": /\.app-pets-/,
  "rss-documents.css": /\.rss-/,
  "rss.css": /\.rss-/,
  "settings.css": /\.app-settings-/,
  "welcome.css": /\.app-welcome-/,
  "workspace.css": /\.app-(?:station|workspace)-/,
  "youtube.css": /\.youtube-/,
} as const;

const SHARED_PRIMITIVE_CLASS =
  /\.(?:app-dream-[\w-]+|app-menu-item)(?![\w-])/;

function splitSelectorList(selector: string) {
  const branches: string[] = [];
  let depth = 0;
  let quote = "";
  let start = 0;

  for (let index = 0; index < selector.length; index += 1) {
    const character = selector[index];

    if (quote) {
      if (character === "\\") {
        index += 1;
      } else if (character === quote) {
        quote = "";
      }
      continue;
    }

    if (character === '"' || character === "'") {
      quote = character;
    } else if (character === "(" || character === "[") {
      depth += 1;
    } else if (character === ")" || character === "]") {
      depth -= 1;
    } else if (character === "," && depth === 0) {
      branches.push(selector.slice(start, index).trim());
      start = index + 1;
    }
  }

  branches.push(selector.slice(start).trim());
  return branches;
}

describe("Dream selector ownership", () => {
  test("keeps shared primitive base recipes out of domain modules", async () => {
    const violations: string[] = [];

    for (const [file, domainScope] of Object.entries(DOMAIN_DREAM_MODULES)) {
      const source = await Bun.file(
        new URL(`./dream/${file}`, import.meta.url),
      ).text();

      postcss.parse(source, { from: file }).walkRules((rule) => {
        for (const selector of splitSelectorList(rule.selector)) {
          if (
            SHARED_PRIMITIVE_CLASS.test(selector) &&
            !domainScope.test(selector)
          ) {
            violations.push(
              `${file}:${rule.source?.start?.line ?? "?"}: ${selector.replace(/\s+/g, " ")}`,
            );
          }
        }
      });
    }

    expect(violations).toEqual([]);
  });

  test("keeps the reusable compact and menu recipes in controls.css", async () => {
    const controls = await Bun.file(
      new URL("./dream/controls.css", import.meta.url),
    ).text();

    for (const selector of [
      ".app-dream-select.app-control-compact",
      ".app-dream-input.app-control-compact",
      ".app-dream-badge[class]",
      ".app-dream-button-group",
      ".app-dream-segment-group",
      ".app-dream-menu-item",
    ]) {
      expect(controls).toContain(selector);
    }
  });
});
