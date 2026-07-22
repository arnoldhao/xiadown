export type DreamStyleEntryKind =
  | "token"
  | "selector"
  | "keyframe"
  | "at-rule";

export interface DreamStyleEntry {
  kind: DreamStyleEntryKind;
  line: number;
  name: string;
  value?: string;
}

export interface DreamStyleModuleInventory {
  id: string;
  path: string;
  atRules: DreamStyleEntry[];
  keyframes: DreamStyleEntry[];
  selectors: DreamStyleEntry[];
  tokens: DreamStyleEntry[];
}

export interface DreamStyleRegistry {
  modules: DreamStyleModuleInventory[];
  totals: Record<DreamStyleEntryKind | "modules", number>;
}

type CssBlockKind = "at-rule" | "keyframe" | "keyframe-step" | "rule";

function createLineNumberResolver(source: string) {
  const lineStarts = [0];
  for (let index = 0; index < source.length; index += 1) {
    if (source.charCodeAt(index) === 10) lineStarts.push(index + 1);
  }

  return (offset: number) => {
    let lower = 0;
    let upper = lineStarts.length - 1;
    while (lower <= upper) {
      const middle = Math.floor((lower + upper) / 2);
      if ((lineStarts[middle] ?? 0) <= offset) lower = middle + 1;
      else upper = middle - 1;
    }
    return upper + 1;
  };
}

function stripCommentsPreservingLines(source: string) {
  return source.replace(/\/\*[\s\S]*?\*\//g, (comment) =>
    comment.replace(/[^\n]/g, " "),
  );
}

/** Splits selector lists without splitting commas inside :is(), :not(), etc. */
export function splitCssSelectorList(selectorList: string): string[] {
  const selectors: string[] = [];
  let start = 0;
  let parentheses = 0;
  let brackets = 0;
  let quote = "";

  for (let index = 0; index < selectorList.length; index += 1) {
    const character = selectorList[index] ?? "";
    if (quote) {
      if (character === "\\") index += 1;
      else if (character === quote) quote = "";
      continue;
    }
    if (character === '"' || character === "'") {
      quote = character;
      continue;
    }
    if (character === "(") parentheses += 1;
    else if (character === ")") parentheses = Math.max(0, parentheses - 1);
    else if (character === "[") brackets += 1;
    else if (character === "]") brackets = Math.max(0, brackets - 1);
    else if (character === "," && parentheses === 0 && brackets === 0) {
      const selector = selectorList.slice(start, index).replace(/\s+/g, " ").trim();
      if (selector) selectors.push(selector);
      start = index + 1;
    }
  }

  const tail = selectorList.slice(start).replace(/\s+/g, " ").trim();
  if (tail) selectors.push(tail);
  return selectors;
}

function moduleId(path: string) {
  const filename = path.split("/").pop() ?? path;
  return filename.replace(/\.css$/i, "");
}

function parseAtRule(header: string) {
  const match = header.match(/^(@[a-zA-Z-]+)\s*(.*)$/s);
  if (!match) return null;
  return {
    name: match[1] ?? "",
    value: (match[2] ?? "").replace(/\s+/g, " ").trim() || undefined,
  };
}

/** Dependency-free inventory parser; this is not intended to mutate CSS. */
export function parseDreamCssModule(
  path: string,
  source: string,
): DreamStyleModuleInventory {
  const searchableSource = stripCommentsPreservingLines(source);
  const lineNumber = createLineNumberResolver(searchableSource);
  const atRules: DreamStyleEntry[] = [];
  const selectors: DreamStyleEntry[] = [];
  const keyframes: DreamStyleEntry[] = [];
  const tokens: DreamStyleEntry[] = [];
  const stack: CssBlockKind[] = [];
  let segmentStart = 0;
  let brackets = 0;
  let parentheses = 0;
  let quote = "";

  for (let index = 0; index < searchableSource.length; index += 1) {
    const character = searchableSource[index] ?? "";
    if (quote) {
      if (character === "\\") index += 1;
      else if (character === quote) quote = "";
      continue;
    }
    if (character === '"' || character === "'") {
      quote = character;
      continue;
    }
    if (character === "(") {
      parentheses += 1;
      continue;
    }
    if (character === ")") {
      parentheses = Math.max(0, parentheses - 1);
      continue;
    }
    if (character === "[") {
      brackets += 1;
      continue;
    }
    if (character === "]") {
      brackets = Math.max(0, brackets - 1);
      continue;
    }
    if (parentheses > 0 || brackets > 0) continue;
    if (character === ";") {
      const rawStatement = searchableSource.slice(segmentStart, index);
      const leadingWhitespace = rawStatement.search(/\S/);
      const statementOffset =
        leadingWhitespace < 0 ? segmentStart : segmentStart + leadingWhitespace;
      const statement = rawStatement.replace(/\s+/g, " ").trim();
      const atRule = parseAtRule(statement);
      if (atRule && atRule.name.toLocaleLowerCase() !== "@keyframes") {
        atRules.push({
          kind: "at-rule",
          line: lineNumber(statementOffset),
          ...atRule,
        });
      }
      segmentStart = index + 1;
      continue;
    }
    if (character === "}") {
      stack.pop();
      segmentStart = index + 1;
      continue;
    }
    if (character !== "{") continue;

    const rawHeader = searchableSource.slice(segmentStart, index);
    const leadingWhitespace = rawHeader.search(/\S/);
    const headerOffset =
      leadingWhitespace < 0 ? segmentStart : segmentStart + leadingWhitespace;
    const header = rawHeader.replace(/\s+/g, " ").trim();
    const insideKeyframes = stack.includes("keyframe");
    let blockKind: CssBlockKind = "at-rule";

    const keyframeMatch = header.match(/^@(?:-webkit-)?keyframes\s+([^\s{]+)/i);
    if (keyframeMatch) {
      blockKind = "keyframe";
      keyframes.push({
        kind: "keyframe",
        line: lineNumber(headerOffset),
        name: keyframeMatch[1] ?? "",
      });
    } else if (insideKeyframes) {
      blockKind = "keyframe-step";
    } else if (header.startsWith("@")) {
      const atRule = parseAtRule(header);
      if (atRule) {
        atRules.push({
          kind: "at-rule",
          line: lineNumber(headerOffset),
          ...atRule,
        });
      }
    } else if (header && !header.startsWith("@")) {
      blockKind = "rule";
      for (const selector of splitCssSelectorList(header)) {
        selectors.push({
          kind: "selector",
          line: lineNumber(headerOffset),
          name: selector,
        });
      }
    }

    stack.push(blockKind);
    segmentStart = index + 1;
  }

  for (const match of searchableSource.matchAll(/(--[a-zA-Z0-9_-]+)\s*:\s*([^;{}]+);/g)) {
    tokens.push({
      kind: "token",
      line: lineNumber(match.index ?? 0),
      name: match[1] ?? "",
      value: (match[2] ?? "").replace(/\s+/g, " ").trim(),
    });
  }

  return { id: moduleId(path), path, atRules, keyframes, selectors, tokens };
}

function totals(modules: DreamStyleModuleInventory[]) {
  return {
    modules: modules.length,
    "at-rule": modules.reduce((total, module) => total + module.atRules.length, 0),
    keyframe: modules.reduce((total, module) => total + module.keyframes.length, 0),
    selector: modules.reduce((total, module) => total + module.selectors.length, 0),
    token: modules.reduce((total, module) => total + module.tokens.length, 0),
  };
}

export function createDreamStyleRegistry(
  sources: Record<string, string>,
): DreamStyleRegistry {
  const modules = Object.entries(sources)
    .map(([path, source]) => parseDreamCssModule(path, source))
    .sort((left, right) => left.id.localeCompare(right.id));
  return { modules, totals: totals(modules) };
}

function matchesEntry(entry: DreamStyleEntry, query: string) {
  return `${entry.name} ${entry.value ?? ""}`.toLocaleLowerCase().includes(query);
}

export function filterDreamStyleRegistry(
  registry: DreamStyleRegistry,
  query: string,
): DreamStyleRegistry {
  const normalizedQuery = query.trim().toLocaleLowerCase();
  if (!normalizedQuery) return registry;

  const modules = registry.modules
    .map((module) => {
      if (`${module.id} ${module.path}`.toLocaleLowerCase().includes(normalizedQuery)) {
        return module;
      }
      return {
        ...module,
        atRules: module.atRules.filter((entry) => matchesEntry(entry, normalizedQuery)),
        keyframes: module.keyframes.filter((entry) => matchesEntry(entry, normalizedQuery)),
        selectors: module.selectors.filter((entry) => matchesEntry(entry, normalizedQuery)),
        tokens: module.tokens.filter((entry) => matchesEntry(entry, normalizedQuery)),
      };
    })
    .filter(
      (module) =>
        module.atRules.length > 0 ||
        module.keyframes.length > 0 ||
        module.selectors.length > 0 ||
        module.tokens.length > 0,
    );
  return { modules, totals: totals(modules) };
}

function loadViteDreamCssSources(): Record<string, string> {
  try {
    /* Vite replaces this exact call with eager raw imports. Do not guard on
       `typeof import.meta.glob`: the transform does not leave a runtime glob
       function behind, so that check makes the real browser registry empty. */
    return import.meta.glob(
      ["../../shared/styles/dream.css", "../../shared/styles/dream/*.css"],
      {
        eager: true,
        import: "default",
        query: "?raw",
      },
    ) as Record<string, string>;
  } catch {
    // Bun does not provide Vite's compile-time glob transform.
    return {};
  }
}

/** Empty under Bun; populated automatically by Vite in the actual Lab. */
export const DREAM_STYLE_REGISTRY = createDreamStyleRegistry(
  loadViteDreamCssSources(),
);
