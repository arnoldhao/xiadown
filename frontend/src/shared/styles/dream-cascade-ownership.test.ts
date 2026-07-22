import { describe, expect, test } from "bun:test";
import { readdir } from "node:fs/promises";
import { fileURLToPath } from "node:url";
import postcss, { type Node, type Rule } from "postcss";

const dreamDirectory = fileURLToPath(new URL("./dream/", import.meta.url));

async function read(relativePath: string) {
  return Bun.file(new URL(relativePath, import.meta.url)).text();
}

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

function hasAtRuleAncestor(node: Node, name: string, params?: string) {
  let parent = node.parent;
  while (parent) {
    if (
      parent.type === "atrule" &&
      parent.name === name &&
      (params === undefined || parent.params === params)
    ) {
      return true;
    }
    parent = parent.parent;
  }
  return false;
}

function declarationValue(rule: Rule, property: string) {
  let value: string | undefined;
  rule.walkDecls(property, (declaration) => {
    value = declaration.value;
  });
  return value;
}

async function parseDreamModules() {
  const modules = new Map<string, postcss.Root>();
  for (const file of await readdir(dreamDirectory)) {
    if (!file.endsWith(".css")) continue;
    const source = await Bun.file(`${dreamDirectory}/${file}`).text();
    modules.set(file, postcss.parse(source, { from: file }));
  }
  return modules;
}

describe("Dream cascade ownership", () => {
  test("keeps cross-domain reduced-motion overrides unlayered and after their animation owners", async () => {
    const [entry, motion, pets, workflows] = await Promise.all([
      read("./dream.css"),
      read("./dream/motion.css"),
      read("./dream/pets.css"),
      read("./dream/workflows.css"),
    ]);

    const petsImport = entry.indexOf('@import "./dream/pets.css";');
    const workflowsImport = entry.indexOf('@import "./dream/workflows.css";');
    const motionImport = entry.indexOf('@import "./dream/motion.css";');
    expect(petsImport).toBeGreaterThan(-1);
    expect(workflowsImport).toBeGreaterThan(-1);
    expect(motionImport).toBeGreaterThan(petsImport);
    expect(motionImport).toBeGreaterThan(workflowsImport);

    expect(pets).toMatch(
      /\.app-pets-gallery-card\s*\{[^}]*animation:\s*app-pets-card-enter/s,
    );
    expect(workflows).toMatch(
      /\.app-settings-window \.app-dream-card\s*\{[^}]*animation:\s*app-settings-card-enter/s,
    );

    const expectedScope = [
      ".listen-online-group",
      ".listen-playlist-group",
      ".listen-category-group",
      ".listen-artist-group",
      ".listen-muse-group-frame",
      ".listen-muse-track-list-group",
      ".listen-hush-live-group",
      ".listen-muse-card",
      ".listen-hush-card",
      ".listen-track-list-row",
      ".app-settings-window .app-settings-tab-content",
      ".app-settings-window .app-dream-card",
      ".app-pets-gallery-card",
      ".app-pets-detail-card",
      ".app-pets-guide-step",
      ".app-pets-guide-tips",
      ".app-pets-import-section",
      ".app-pets-status-message",
      ".listen-muse-card:hover .listen-muse-card-artwork::after",
      ".listen-muse-card:focus-within .listen-muse-card-artwork::after",
      ".listen-hush-card:hover .listen-hush-card-artwork::after",
      ".listen-hush-card:focus-within .listen-hush-card-artwork::after",
      ".listen-cover-change-sweep",
      ".app-pets-gallery-card:hover::after",
      ".app-pets-gallery-card:focus-visible::after",
      ".app-dialog-overlay.app-dialog-overlay[data-state]",
      ".app-dialog-content.app-dialog-content[data-state]",
      ".app-dialog-list-card",
      ".listen-mini-live-dot",
      ".listen-mini-live-dot::after",
      ".listen-mini-live-line",
    ];

    let override: Rule | undefined;
    postcss.parse(motion, { from: "motion.css" }).walkRules((rule) => {
      const selectors = splitSelectorList(rule.selector);
      if (
        selectors.includes(".app-pets-gallery-card") &&
        selectors.includes(".app-settings-window .app-dream-card") &&
        declarationValue(rule, "animation") === "none"
      ) {
        override = rule;
      }
    });

    expect(override).toBeDefined();
    expect(hasAtRuleAncestor(override!, "media", "(prefers-reduced-motion: reduce)"))
      .toBe(true);
    expect(hasAtRuleAncestor(override!, "layer")).toBe(false);
    expect(splitSelectorList(override!.selector)).toEqual(expectedScope);
  });

  test("gives Settings rows one unlayered anatomy owner, including mobile layout", async () => {
    const [components, anatomy] = await Promise.all([
      read("./dream/components.css"),
      read("./dream/anatomy.css"),
    ]);
    const componentRoot = postcss.parse(components, { from: "components.css" });
    const anatomyRoot = postcss.parse(anatomy, { from: "anatomy.css" });

    const componentRowRules: Rule[] = [];
    componentRoot.walkRules((rule) => {
      if (splitSelectorList(rule.selector).includes(".app-settings-row")) {
        componentRowRules.push(rule);
      }
    });
    expect(componentRowRules).toEqual([]);

    const anatomyRows: Rule[] = [];
    anatomyRoot.walkRules((rule) => {
      if (splitSelectorList(rule.selector).includes(".app-settings-row")) {
        anatomyRows.push(rule);
      }
    });

    const base = anatomyRows.find((rule) => !hasAtRuleAncestor(rule, "media"));
    const mobile = anatomyRows.find((rule) =>
      hasAtRuleAncestor(rule, "media", "(max-width: 420px)"),
    );
    expect(declarationValue(base!, "display")).toBe("flex");
    expect(declarationValue(base!, "align-items")).toBe("center");
    expect(declarationValue(mobile!, "display")).toBe("grid");
    expect(declarationValue(mobile!, "align-items")).toBe("stretch");
    expect(hasAtRuleAncestor(mobile!, "layer")).toBe(false);
    expect(mobile!.source?.start?.line).toBeGreaterThan(
      base!.source?.start?.line ?? 0,
    );
  });

  test("keeps list-card appearance and menu-item spacing with one shared owner", async () => {
    const modules = await parseDreamModules();
    const listCardOwners = new Set<string>();
    const menuGapOwners = new Set<string>();
    const menuGapRules: Rule[] = [];

    for (const [file, root] of modules) {
      root.walkRules((rule) => {
        const selectors = splitSelectorList(rule.selector);
        if (selectors.includes(".app-settings-list-card")) {
          listCardOwners.add(file);
        }
        if (
          /\.(?:app-dream-menu-item|app-menu-item)(?![\w-])/.test(rule.selector) &&
          declarationValue(rule, "gap") !== undefined
        ) {
          menuGapOwners.add(file);
          menuGapRules.push(rule);
        }
      });
    }

    expect([...listCardOwners]).toEqual(["anatomy.css"]);
    const listCardRule = modules
      .get("anatomy.css")!
      .nodes.filter((node): node is Rule => node.type === "rule")
      .find((rule) =>
        splitSelectorList(rule.selector).includes(".app-settings-list-card"),
      );
    expect(declarationValue(listCardRule!, "border")).toBe("0");
    expect(declarationValue(listCardRule!, "background")).toBe(
      "var(--dream-surface-card)",
    );
    expect(declarationValue(listCardRule!, "box-shadow")).toBe(
      "var(--dream-card-shadow)",
    );

    expect([...menuGapOwners]).toEqual(["anatomy.css"]);
    expect(menuGapRules).toHaveLength(1);
    expect(menuGapRules[0].selector).toBe(
      ":is(.app-dream-menu-item, .app-menu-item)",
    );
    expect(declarationValue(menuGapRules[0], "gap")).toBe(
      "var(--app-space-2)",
    );
  });
});
