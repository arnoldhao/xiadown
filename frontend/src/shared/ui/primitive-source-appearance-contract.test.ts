import { describe, expect, test } from "bun:test";
import { readdir } from "node:fs/promises";
import { basename, join } from "node:path";
import { fileURLToPath } from "node:url";
import * as ts from "typescript";

const SOURCE_DIRECTORIES = [
  fileURLToPath(new URL("../../components/ui", import.meta.url)),
  fileURLToPath(new URL(".", import.meta.url)),
] as const;
const FRONTEND_SOURCE_DIRECTORY = fileURLToPath(
  new URL("../..", import.meta.url),
);

const tailwindVariant = String.raw`(?:(?:[a-z][a-z0-9-]*|data-\[[^\]]+\]|\[[^\]]+\]):)*`;
const tailwindUtility = new RegExp(
  String.raw`^${tailwindVariant}-?(?:` +
    [
      String.raw`flex|grid|block|inline|inline-flex|inline-block|hidden|relative|absolute|fixed|sticky|truncate|sr-only`,
      String.raw`(?:inset|top|right|bottom|left|z|order|basis|grow|shrink)(?:-.+)?`,
      String.raw`(?:w|h|min-w|min-h|max-w|max-h|size)-.+`,
      String.raw`(?:p|px|py|pt|pr|pb|pl|m|mx|my|mt|mr|mb|ml)-.+`,
      String.raw`(?:gap|space-x|space-y|divide-x|divide-y)-.+`,
      String.raw`(?:items|justify|content|self|place-items|place-content|place-self)-.+`,
      String.raw`(?:overflow|overflow-x|overflow-y|overscroll|overscroll-x|overscroll-y)-.+`,
      String.raw`(?:select|whitespace|break|object|aspect|columns)-.+`,
      String.raw`(?:rounded|border|bg|text|font|leading|tracking|shadow|ring|outline|opacity|cursor|transition|duration|delay|ease|animate|decoration|underline-offset|placeholder|caret|accent|fill|stroke)(?:-.+)?`,
      String.raw`\[[^\]]+\]`,
    ].join("|") +
    String.raw`)$`,
  "i",
);

async function collectSources(directory: string): Promise<string[]> {
  const entries = await readdir(directory, { withFileTypes: true });
  const nested = await Promise.all(
    entries.map(async (entry) => {
      const path = join(directory, entry.name);
      if (entry.isDirectory()) return collectSources(path);
      if (!entry.isFile()) return [];
      if (!/\.(?:ts|tsx)$/.test(entry.name) || /\.test\./.test(entry.name)) {
        return [];
      }
      return [path];
    }),
  );
  return nested.flat();
}

function isClassContext(node: ts.Node): boolean {
  if (
    ts.isJsxAttribute(node) &&
    ts.isIdentifier(node.name) &&
    node.name.text === "className"
  ) {
    return true;
  }

  if (ts.isCallExpression(node) && ts.isIdentifier(node.expression)) {
    return ["classNames", "clsx", "cn", "cva"].includes(
      node.expression.text,
    );
  }

  return (
    ts.isVariableDeclaration(node) &&
    ts.isIdentifier(node.name) &&
    /class/i.test(node.name.text)
  );
}

function collectUtilityTokens(source: string, fileName: string): string[] {
  const sourceFile = ts.createSourceFile(
    fileName,
    source,
    ts.ScriptTarget.Latest,
    true,
    fileName.endsWith(".tsx") ? ts.ScriptKind.TSX : ts.ScriptKind.TS,
  );
  const failures = new Set<string>();

  const inspect = (node: ts.Node) => {
    if (
      ts.isStringLiteralLike(node) ||
      ts.isNoSubstitutionTemplateLiteral(node)
    ) {
      for (const token of node.text.split(/\s+/).filter(Boolean)) {
        if (tailwindUtility.test(token)) {
          const { line } = sourceFile.getLineAndCharacterOfPosition(
            node.getStart(sourceFile),
          );
          failures.add(`${basename(fileName)}:${line + 1} ${token}`);
        }
      }
    }
    ts.forEachChild(node, inspect);
  };

  const visit = (node: ts.Node) => {
    if (isClassContext(node)) inspect(node);
    ts.forEachChild(node, visit);
  };
  visit(sourceFile);
  return [...failures];
}

describe("shared primitive source appearance boundary", () => {
  test("keeps every primitive source free of Tailwind appearance and anatomy recipes", async () => {
    const files = (
      await Promise.all(SOURCE_DIRECTORIES.map(collectSources))
    ).flat();
    const failures = (
      await Promise.all(
        files.map(async (file) =>
          collectUtilityTokens(await Bun.file(file).text(), file),
        ),
      )
    ).flat();

    expect(failures).toEqual([]);
  });

  test("keeps base component imports behind shared UI wrappers", async () => {
    const files = await collectSources(FRONTEND_SOURCE_DIRECTORY);
    const bypasses = (
      await Promise.all(
        files.map(async (file) => {
          if (file.includes("/shared/ui/")) return [];
          const source = await Bun.file(file).text();
          return source.includes('from "@/components/ui/') ||
            source.includes("from '@/components/ui/")
            ? [file]
            : [];
        }),
      )
    ).flat();

    expect(bypasses).toEqual([]);
  });

  test("keeps the stable primitive selectors in the Dream entry", async () => {
    const [entry, anatomy, components, motion] = await Promise.all([
      Bun.file(new URL("../styles/dream.css", import.meta.url)).text(),
      Bun.file(new URL("../styles/dream/anatomy.css", import.meta.url)).text(),
      Bun.file(new URL("../styles/dream/components.css", import.meta.url)).text(),
      Bun.file(new URL("../styles/dream/motion.css", import.meta.url)).text(),
    ]);

    expect(entry).toContain('@import "./dream/anatomy.css"');
    for (const selector of [
      ".app-base-button",
      ".app-base-badge",
      ".app-base-card__header",
      ".app-base-input",
      ".app-progress__indicator",
      ".app-separator[data-orientation=\"horizontal\"]",
      ".app-sidebar-menu__button",
      ".app-dialog-content-base",
      ".app-menu-indicator",
      ".app-pet-player--fallback",
    ]) {
      expect(anatomy).toContain(selector);
    }
    expect(components).toContain(".app-base-button--outline");
    expect(components).toContain(".app-base-badge--destructive");
    expect(motion).toContain('.app-menu-content-base[data-state="open"]');
  });
});
