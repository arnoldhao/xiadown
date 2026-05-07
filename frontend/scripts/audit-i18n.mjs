import fs from "node:fs";
import path from "node:path";
import ts from "typescript";

const rootDir = process.cwd();
const strict = process.argv.includes("--strict");
const jsonOnly = process.argv.includes("--json");
const pruneUnused = process.argv.includes("--prune-unused");
const fixEnglishStyle = process.argv.includes("--fix-english-style");

const localeDir = path.join(rootDir, "src", "shared", "i18n", "locales");
const sourceDir = path.join(rootDir, "src");
// Apple HIG alignment:
// - Keep user-visible text localizable, then test each supported language.
// - Prefer concise, direct labels; placeholders can hint input but must not be
//   the only localization surface for an interaction.
// - Use platform-consistent accessible labels for icon-only controls.
const englishStyleSkipKeys = new Set([
  "xiadown.welcome.readyHint",
  "xiadown.running.runningCountLine",
  "xiadown.running.queuedCountLine",
  "settings.language.option.zh-CN",
  "settings.connectors.loginTitle",
  "settings.connectors.openSite",
  "settings.connectors.viewCookies",
]);
const englishStylePreservePhrases = [
  "XiaDown",
  "DreamApp",
  "Pet Gallery",
  "Codex",
  "codex-pets.net",
  "codexpet.xyz",
  "pet.json",
  "spritesheet.webp",
  "hatch-pet",
  "Apple",
  "macOS",
  "iOS",
  "Chrome",
  "Chromium",
  "Edge",
  "Brave",
  "Bun",
  "FFmpeg",
  "GitHub",
  "YouTube",
  "Bilibili",
  "Markdown",
  "Vibe Coding",
  "WebSocket",
  "JSON",
  "API",
  "HTTP",
  "HTTPS",
  "URL",
  "PDF",
  "MIME",
  "ASS",
  "SSA",
  "SRT",
  "VTT",
  "TTML",
  "FCPXML",
  "CRF",
  "BOM",
  "UTC",
  "N/A",
  "OS",
  "Nix",
  "Wails",
  "yt-dlp",
  "User-Agent",
  "Accept-Language",
  "Content-Type",
];
const englishStylePreserveWords = new Set(englishStylePreservePhrases.filter((item) => !item.includes(" ")));
const englishStyleAlwaysLowerWords = new Set([
  "a",
  "an",
  "the",
  "and",
  "or",
  "but",
  "for",
  "nor",
  "at",
  "by",
  "to",
  "from",
  "of",
  "in",
  "on",
  "off",
  "up",
  "out",
  "as",
  "is",
  "are",
  "be",
  "been",
  "being",
  "into",
  "over",
  "with",
  "without",
  "per",
  "same",
  "after",
  "before",
  "when",
  "while",
  "that",
  "this",
  "these",
  "those",
  "all",
  "no",
  "not",
  "now",
  "than",
  "once",
  "only",
  "via",
  "max",
  "min",
]);
const englishTitleCaseLowerWords = new Set([
  "a",
  "an",
  "the",
  "and",
  "or",
  "but",
  "nor",
  "for",
  "so",
  "yet",
  "as",
  "at",
  "by",
  "en",
  "in",
  "of",
  "off",
  "on",
  "per",
  "to",
  "up",
  "via",
]);
const englishSentenceBoundaryChars = new Set([".", "!", "?"]);
// Apple HIG defines title-style capitalization for compact actionable controls like
// buttons and menu items, but uses sentence-style capitalization for labels and most prose.
const englishTitleStyleExplicitPatterns = [
  /^app\.settings\.title\./,
];
const englishTitleStyleHeuristicPattern = /(\.button$|\.action$|columns\.|Tabs\.|sections\.|Section$|dialogTitle$|pickTitle$)/;
const englishTitleStyleSentencePattern = /\?|\.{1,3}$|\b(is|are|was|were|am|can't|cannot|need|needs|shows?\s+up|please)\b/i;
const zhGlossaryReplacements = [
  ["Sidebar", "侧边栏"],
  ["Connector", "连接"],
  ["External Tools", "依赖"],
  ["Runtime", "运行时"],
  ["Mono", "单语"],
  ["Bilingual", "双语"],
  ["Builtin", "内置"],
  ["Topic", "主题"],
  ["Toast", "提示"],
  ["Cloud Dancer", "暖云白"],
];
const zhAllowedEnglishKeyPatterns = [
  /^settings\.language\.option\.en$/,
  /^xiadown\.running\.units\.bytesPerSecond$/,
  /^settings\.connectors\.item\./,
  /^xiadown\.welcome\.readyHint$/,
  /^xiadown\.settings\.pets\./,
  /^xiadown\.petGallery\./,
];
const zhAllowedEnglishTokens = new Set([
  "PNG",
  "ZIP",
  "DPI",
  "VPN",
  "px",
  "x",
  "yt-dlp",
  "ffmpeg",
  "bun",
  "GitHub",
  "Issues",
  "YouTube",
  "Hush",
  "Music",
  "lo-fi",
  "Lo-fi",
  "Lo-Fi",
  "FM",
  "Mix",
  "YT-DLP",
  "FFMPEG",
  "BUN",
  "AI",
  "API",
  "Markdown",
  "Vibe",
  "Coding",
  "AirPlay",
  "DreamApp",
  "Cookie",
  "Cookies",
  "cookies",
]);
const knownNativeLanguageLabels = {
  "settings.language.option.en": "English",
  "settings.language.option.zh-CN": "简体中文",
};
const hardcodedEnglishAllowedText = new Set([
  "HTTP",
  "HTTPS",
  "SOCKS5",
  "Dream.FM",
  "DreamApp",
]);
const hardcodedEnglishUserFacingPropertyNames = new Set([
  "label",
  "title",
  "description",
  "message",
  "placeholder",
]);
const hardcodedEnglishPropertyFileSkips = [
  /(?:^|\/)dreamfm\/api\.ts$/,
  /(?:^|\/)dreamfm\/catalog\.ts$/,
];
const hardcodedChineseFileSkips = [
  // Dream.FM catalog entries preserve external channel titles and descriptions.
  /(?:^|\/)dreamfm\/catalog\.ts$/,
];
const hardcodedEnglishSkipAttributes = new Set([
  "className",
  "contentClassName",
  "fallbackClassName",
  "petClassName",
  "style",
  "id",
  "key",
  "src",
  "href",
  "to",
  "type",
  "role",
  "target",
  "rel",
  "value",
  "defaultValue",
  "name",
  "method",
  "topic",
  "window",
  "variant",
  "size",
  "align",
  "side",
  "sideOffset",
  "width",
  "height",
  "viewBox",
  "fill",
  "stroke",
  "strokeWidth",
  "strokeLinecap",
  "strokeLinejoin",
  "d",
  "xmlns",
  "tabIndex",
  "aria-hidden",
]);

function flatten(input, prefix = "", output = {}) {
  for (const [key, value] of Object.entries(input)) {
    const nextKey = prefix ? `${prefix}.${key}` : key;
    if (value && typeof value === "object" && !Array.isArray(value)) {
      flatten(value, nextKey, output);
    } else {
      output[nextKey] = String(value);
    }
  }
  return output;
}

function walk(dir, output = []) {
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    const next = path.join(dir, entry.name);
    if (entry.isDirectory()) {
      if (next.includes(`${path.sep}bindings`) || next.includes(`${path.sep}shared${path.sep}i18n`)) {
        continue;
      }
      walk(next, output);
      continue;
    }
    if (!next.endsWith(".ts") && !next.endsWith(".tsx")) {
      continue;
    }
    output.push(next);
  }
  return output;
}

function relative(filePath) {
  return path.relative(rootDir, filePath).split(path.sep).join("/");
}

function isI18nCallExpression(node) {
  if (!ts.isCallExpression(node)) {
    return false;
  }
  if (ts.isIdentifier(node.expression)) {
    return node.expression.text === "t" || node.expression.text === "translate";
  }
  if (ts.isPropertyAccessExpression(node.expression)) {
    return node.expression.name.text === "t" || node.expression.name.text === "translate";
  }
  return false;
}

function isExplicitLanguageArg(node, sourceFile) {
  const text = node.getText(sourceFile).trim();
  return /(^language\b)|(\blanguage\b\s+as\s+)|(as\s+"en"\s*\|\s*"zh-CN")|(^"en"$)|(^"zh-CN"$)/.test(text);
}

function splitWord(word) {
  const parts = [];
  let buffer = "";
  for (const char of word) {
    if (char === "-" || char === "/") {
      if (buffer) {
        parts.push(buffer);
      }
      parts.push(char);
      buffer = "";
      continue;
    }
    buffer += char;
  }
  if (buffer) {
    parts.push(buffer);
  }
  return parts;
}

function maskEnglishPreservePhrases(value) {
  let text = value;
  englishStylePreservePhrases.forEach((phrase, index) => {
    const escaped = phrase.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
    text = text.replace(new RegExp(escaped, "g"), `@@${index}@@`);
  });
  return text;
}

function unmaskEnglishPreservePhrases(value) {
  return value.replace(/@@(\d+)@@/g, (_, index) => englishStylePreservePhrases[Number(index)] ?? "");
}

function isEnglishStylePreservedWord(segment) {
  return (
    englishStylePreserveWords.has(segment) ||
    /^[A-Z0-9]+(?:_[A-Z0-9]+)*$/.test(segment) ||
    /^[a-z0-9]+(?:_[a-z0-9]+)+$/.test(segment) ||
    /^[A-Za-z]+=[A-Za-z0-9_-]+$/.test(segment) ||
    (/^[A-Za-z0-9-]+$/.test(segment) && /[A-Z].*[A-Z]/.test(segment) && /[a-z]/.test(segment))
  );
}

function splitAffixes(token) {
  const leading = token.match(/^[^A-Za-z{]*/)?.[0] ?? "";
  const trailing = token.match(/[^A-Za-z}]*$/)?.[0] ?? "";
  return {
    leading,
    trailing,
    core: token.slice(leading.length, token.length - trailing.length),
  };
}

function normalizeEnglishTitleSegment(segment, isFirst, isLast) {
  if (segment === "-" || segment === "/") {
    return segment;
  }
  if (!segment || !/[A-Za-z]/.test(segment)) {
    return segment;
  }
  if (isEnglishStylePreservedWord(segment)) {
    return segment;
  }
  if (segment.includes("/")) {
    const parts = splitWord(segment);
    const wordIndexes = parts.map((part, index) => (part !== "/" ? index : null)).filter((index) => index !== null);
    return parts
      .map((part, index) => {
        const position = wordIndexes.indexOf(index);
        if (position === -1) {
          return part;
        }
        return normalizeEnglishTitleSegment(part, isFirst && position === 0, isLast && position === wordIndexes.length - 1);
      })
      .join("");
  }
  if (segment.includes("-")) {
    const parts = splitWord(segment);
    const wordIndexes = parts.map((part, index) => (part !== "-" ? index : null)).filter((index) => index !== null);
    return parts
      .map((part, index) => {
        const position = wordIndexes.indexOf(index);
        if (position === -1) {
          return part;
        }
        return normalizeEnglishTitleSegment(part, isFirst && position === 0, isLast && position === wordIndexes.length - 1);
      })
      .join("");
  }
  const lower = segment.toLowerCase();
  if (!isFirst && !isLast && englishTitleCaseLowerWords.has(lower)) {
    return lower;
  }
  return `${lower.charAt(0).toUpperCase()}${lower.slice(1)}`;
}

function shouldUseEnglishTitleCase(key, value) {
  if (englishStyleSkipKeys.has(key) || typeof value !== "string" || !/[A-Za-z]/.test(value)) {
    return false;
  }
  if (englishTitleStyleExplicitPatterns.some((pattern) => pattern.test(key))) {
    return true;
  }
  if (!englishTitleStyleHeuristicPattern.test(key)) {
    return false;
  }
  if (englishTitleStyleSentencePattern.test(value)) {
    return false;
  }
  return true;
}

function normalizeEnglishSentenceValue(key, value) {
  if (englishStyleSkipKeys.has(key) || typeof value !== "string" || !/[A-Za-z]/.test(value)) {
    return value;
  }
  const text = maskEnglishPreservePhrases(value);
  const tokens = text.match(/\{[^}]+\}|@@\d+@@|[A-Za-z][A-Za-z0-9'/-]*|[^A-Za-z{@]+|[@][^@]+[@]?/g) ?? [text];
  let sentenceStart = true;
  const output = tokens
    .map((token) => {
      if (token.startsWith("@@")) {
        sentenceStart = false;
        return token;
      }
      if (token.startsWith("{") && token.endsWith("}")) {
        sentenceStart = false;
        return token;
      }
      if (!/[A-Za-z]/.test(token)) {
        const trimmed = token.trimEnd();
        const lastChar = trimmed.charAt(trimmed.length - 1);
        if (englishSentenceBoundaryChars.has(lastChar)) {
          sentenceStart = true;
        }
        return token;
      }
      const leading = token.match(/^[^A-Za-z{]*/)?.[0] ?? "";
      const trailing = token.match(/[^A-Za-z}]*$/)?.[0] ?? "";
      const core = token.slice(leading.length, token.length - trailing.length);
      const segments = splitWord(core);
      let nextSentenceStart = sentenceStart;
      const normalized = segments
        .map((segment) => {
          if (segment === "-" || segment === "/") {
            return segment;
          }
          if (!segment || !/[A-Za-z]/.test(segment)) {
            return segment;
          }
          if (
            isEnglishStylePreservedWord(segment)
          ) {
            nextSentenceStart = false;
            return segment;
          }
          if (/^[A-Z][a-z0-9]+(?:'[A-Za-z]+)?$/.test(segment) || /^[a-z][a-z0-9]+(?:'[A-Za-z]+)?$/.test(segment)) {
            const lower = segment.toLowerCase();
            const nextValue = nextSentenceStart ? `${lower.charAt(0).toUpperCase()}${lower.slice(1)}` : (englishStyleAlwaysLowerWords.has(lower) ? lower : lower);
            nextSentenceStart = false;
            return nextValue;
          }
          if (nextSentenceStart && /^[a-z]/.test(segment)) {
            nextSentenceStart = false;
            return `${segment.charAt(0).toUpperCase()}${segment.slice(1)}`;
          }
          nextSentenceStart = false;
          return segment;
        })
        .join("");
      sentenceStart = false;
      return `${leading}${normalized}${trailing}`;
    })
    .join("");
  return unmaskEnglishPreservePhrases(output);
}

function normalizeEnglishTitleValue(key, value) {
  if (englishStyleSkipKeys.has(key) || typeof value !== "string" || !/[A-Za-z]/.test(value)) {
    return value;
  }
  const text = maskEnglishPreservePhrases(value);
  const parts = text.split(/(\s+)/);
  const wordIndexes = parts
    .map((part, index) => {
      if (!part || /^\s+$/.test(part) || /^@@\d+@@$/.test(part) || (part.startsWith("{") && part.endsWith("}"))) {
        return null;
      }
      const { core } = splitAffixes(part);
      return /[A-Za-z]/.test(core) ? index : null;
    })
    .filter((index) => index !== null);
  const output = parts
    .map((part, index) => {
      if (!part || /^\s+$/.test(part) || /^@@\d+@@$/.test(part) || (part.startsWith("{") && part.endsWith("}"))) {
        return part;
      }
      const position = wordIndexes.indexOf(index);
      if (position === -1) {
        return part;
      }
      const { leading, trailing, core } = splitAffixes(part);
      return `${leading}${normalizeEnglishTitleSegment(core, position === 0, position === wordIndexes.length - 1)}${trailing}`;
    })
    .join("");
  return unmaskEnglishPreservePhrases(output);
}

function normalizeEnglishLocaleValue(key, value) {
  if (shouldUseEnglishTitleCase(key, value)) {
    return normalizeEnglishTitleValue(key, value);
  }
  return normalizeEnglishSentenceValue(key, value);
}

function normalizeZhGlossaryValue(value) {
  if (typeof value !== "string") {
    return value;
  }
  let nextValue = value;
  for (const [from, to] of zhGlossaryReplacements) {
    nextValue = nextValue.split(from).join(to);
  }
  return nextValue;
}

function stripLocalePlaceholders(value) {
  return String(value)
    .replace(/\{[^}]+\}/g, "")
    .replace(/`[^`]*`/g, "")
    .replace(/#[0-9a-fA-F]{3,8}\b/g, "");
}

function isZhEnglishAllowedKey(key) {
  return zhAllowedEnglishKeyPatterns.some((pattern) => pattern.test(key));
}

function collectUnexpectedZhEnglishTokens(key, value) {
  if (isZhEnglishAllowedKey(key)) {
    return [];
  }
  const text = stripLocalePlaceholders(value);
  const tokens = (text.match(/[A-Za-z][A-Za-z0-9._+-]*/g) ?? [])
    .map((token) => token.replace(/[.。！？!?,，:：;；]+$/g, ""));
  return [...new Set(tokens.filter((token) => !zhAllowedEnglishTokens.has(token)))];
}

function visibleTextValue(value) {
  return String(value).replace(/\s+/g, " ").trim();
}

function looksLikeNonUserFacingVisibleText(value) {
  const text = visibleTextValue(value);
  return (
    !text ||
    !/[A-Za-z]/.test(text) ||
    hardcodedEnglishAllowedText.has(text) ||
    /^https?:/i.test(text) ||
    /^data:/i.test(text) ||
    /^#/.test(text) ||
    /^\/[A-Za-z0-9_./-]*$/.test(text) ||
    /^[a-z0-9_:/.[\]%-]+(?:\s+[a-z0-9_:/.[\]%-]+)*$/.test(text) ||
    /^[A-Z0-9_]+$/.test(text) ||
    /^[a-z]+(?:-[a-z]+)+$/.test(text) ||
    /^[a-z]+(?:\.[a-z0-9_.-]+)+$/.test(text) ||
    /^\d+(?:\.\d+)?(?:px|rem|%|s|ms)?$/.test(text)
  );
}

function hardcodedEnglishPropertyName(name) {
  if (ts.isIdentifier(name) || ts.isStringLiteral(name) || ts.isNumericLiteral(name)) {
    return name.text;
  }
  return "";
}

function shouldSkipHardcodedEnglishPropertyFile(filePath) {
  const rel = relative(filePath);
  return hardcodedEnglishPropertyFileSkips.some((pattern) => pattern.test(rel));
}

function shouldSkipHardcodedChineseFile(filePath) {
  const rel = relative(filePath);
  return hardcodedChineseFileSkips.some((pattern) => pattern.test(rel));
}

function filterLocaleTree(input, usedSet, prefix = "") {
  const output = {};
  for (const [key, value] of Object.entries(input)) {
    const nextKey = prefix ? `${prefix}.${key}` : key;
    if (value && typeof value === "object" && !Array.isArray(value)) {
      const nextValue = filterLocaleTree(value, usedSet, nextKey);
      if (Object.keys(nextValue).length > 0) {
        output[key] = nextValue;
      }
      continue;
    }
    if (usedSet.has(nextKey)) {
      output[key] = value;
    }
  }
  return output;
}

function mapLocaleTree(input, mapper, prefix = "") {
  const output = {};
  for (const [key, value] of Object.entries(input)) {
    const nextKey = prefix ? `${prefix}.${key}` : key;
    if (value && typeof value === "object" && !Array.isArray(value)) {
      output[key] = mapLocaleTree(value, mapper, nextKey);
      continue;
    }
    output[key] = mapper(nextKey, String(value));
  }
  return output;
}

let enSource = JSON.parse(fs.readFileSync(path.join(localeDir, "en.json"), "utf8"));
if (fixEnglishStyle) {
  enSource = mapLocaleTree(enSource, normalizeEnglishLocaleValue);
  fs.writeFileSync(path.join(localeDir, "en.json"), `${JSON.stringify(enSource, null, 2)}\n`);
}
const zhSource = JSON.parse(fs.readFileSync(path.join(localeDir, "zh-CN.json"), "utf8"));
const en = flatten(enSource);
const zh = flatten(zhSource);
const files = walk(sourceDir);
const localeKeys = new Set([...Object.keys(en), ...Object.keys(zh)]);
const sortedLocaleKeys = [...localeKeys];

const usedKeys = new Set();
const hardcodedChinese = [];
const hardcodedEnglish = [];
const i18nCopyNamingViolations = [];
const unresolvedDynamicKeys = [];
const i18nCallViolations = [];
const keyPattern = /\b(?:t|translate)\(\s*(["'`])([^"'`]+)\1/g;
const propertyKeyPattern = /\b(?:labelKey|descriptionKey|reasonKey)\s*:\s*(["'`])([^"'`]+)\1/g;
const stringLiteralPattern = /(["'`])([A-Za-z0-9._-]+)\1/g;

function resolveDynamicTemplate(raw) {
  const match = /^([A-Za-z0-9._-]*)\$\{[^}\n]+\}([A-Za-z0-9._-]*)$/.exec(raw);
  if (!match) {
    return false;
  }
  const [, prefix, suffix] = match;
  if (!`${prefix}${suffix}`.includes(".")) {
    return false;
  }
  const matchedKeys = sortedLocaleKeys.filter((key) => key.startsWith(prefix) && key.endsWith(suffix));
  if (matchedKeys.length === 0) {
    unresolvedDynamicKeys.push(raw);
    return false;
  }
  for (const key of matchedKeys) {
    usedKeys.add(key);
  }
  return true;
}

for (const file of files) {
  const text = fs.readFileSync(file, "utf8");
  const sourceFile = ts.createSourceFile(
    file,
    text,
    ts.ScriptTarget.Latest,
    true,
    file.endsWith(".tsx") ? ts.ScriptKind.TSX : ts.ScriptKind.TS
  );

  function visit(node) {
    if (ts.isJsxText(node)) {
      const content = visibleTextValue(node.getText(sourceFile));
      if (!looksLikeNonUserFacingVisibleText(content)) {
        const line = sourceFile.getLineAndCharacterOfPosition(node.getStart(sourceFile)).line + 1;
        hardcodedEnglish.push({
          file: relative(file),
          line,
          content,
        });
      }
    }
    if (
      ts.isJsxAttribute(node) &&
      node.initializer &&
      ts.isStringLiteral(node.initializer) &&
      !hardcodedEnglishSkipAttributes.has(node.name.getText(sourceFile))
    ) {
      const content = visibleTextValue(node.initializer.text);
      if (!looksLikeNonUserFacingVisibleText(content)) {
        const line = sourceFile.getLineAndCharacterOfPosition(node.getStart(sourceFile)).line + 1;
        hardcodedEnglish.push({
          file: relative(file),
          line,
          content: `${node.name.getText(sourceFile)}="${content}"`,
        });
      }
    }
    if (
      !shouldSkipHardcodedEnglishPropertyFile(file) &&
      ts.isPropertyAssignment(node) &&
      hardcodedEnglishUserFacingPropertyNames.has(hardcodedEnglishPropertyName(node.name)) &&
      (ts.isStringLiteral(node.initializer) || ts.isNoSubstitutionTemplateLiteral(node.initializer))
    ) {
      const content = visibleTextValue(node.initializer.text);
      if (!looksLikeNonUserFacingVisibleText(content)) {
        const line = sourceFile.getLineAndCharacterOfPosition(node.getStart(sourceFile)).line + 1;
        hardcodedEnglish.push({
          file: relative(file),
          line,
          content: `${hardcodedEnglishPropertyName(node.name)}="${content}"`,
        });
      }
    }
    if (isI18nCallExpression(node)) {
      const calleeName = ts.isPropertyAccessExpression(node.expression) ? node.expression.name.text : node.expression.text;
      const secondArg = node.arguments[1];
      const hasFallbackLikeArg =
        calleeName === "translate" ||
        (node.arguments.length >= 2 && secondArg && !isExplicitLanguageArg(secondArg, sourceFile));
      if (hasFallbackLikeArg) {
        const line = sourceFile.getLineAndCharacterOfPosition(node.getStart(sourceFile)).line + 1;
        i18nCallViolations.push({
          file: relative(file),
          line,
          content: node.getText(sourceFile),
        });
      }
    }
    ts.forEachChild(node, visit);
  }
  visit(sourceFile);

  for (const match of text.matchAll(keyPattern)) {
    const key = match[2];
    if (key.includes("${")) {
      resolveDynamicTemplate(key);
    } else {
      usedKeys.add(key);
    }
  }

  for (const match of text.matchAll(propertyKeyPattern)) {
    const key = match[2];
    if (key.includes("${")) {
      resolveDynamicTemplate(key);
    } else if (localeKeys.has(key)) {
      usedKeys.add(key);
    }
  }

  for (const match of text.matchAll(stringLiteralPattern)) {
    const candidate = match[2];
    if (localeKeys.has(candidate)) {
      usedKeys.add(candidate);
    }
  }

  for (const [index, line] of text.split(/\r?\n/).entries()) {
    const trimmed = line.trim();
    if (!trimmed) {
      continue;
    }
    if (/\bcopy\b/.test(line)) {
      i18nCopyNamingViolations.push({
        file: relative(file),
        line: index + 1,
        content: trimmed,
      });
    }
    if (/^\/\/|^\/\*|^\*|^\{\/\*/.test(trimmed)) {
      continue;
    }
    if (/\b(?:t|translate)\(/.test(line)) {
      continue;
    }
    if (!shouldSkipHardcodedChineseFile(file) && /[\u4e00-\u9fff]/.test(line)) {
      hardcodedChinese.push({
        file: relative(file),
        line: index + 1,
        content: trimmed,
      });
    }
  }
}

const enKeys = new Set(Object.keys(en));
const zhKeys = new Set(Object.keys(zh));

const missingInZh = [...enKeys].filter((key) => !zhKeys.has(key));
const extraInZh = [...zhKeys].filter((key) => !enKeys.has(key));
const unusedInEn = [...enKeys].filter((key) => !usedKeys.has(key));
const missingDefs = [...usedKeys].filter((key) => !enKeys.has(key));
const concreteMissingDefs = missingDefs.filter((key) => !key.includes("${"));
const dynamicMissingDefs = [...new Set(unresolvedDynamicKeys)];
const englishStyleViolations = Object.entries(en)
  .filter(([key, value]) => normalizeEnglishLocaleValue(key, value) !== value)
  .map(([key, value]) => ({ key, value, expected: normalizeEnglishLocaleValue(key, value) }));
const zhGlossaryViolations = Object.entries(zh)
  .filter(([, value]) => normalizeZhGlossaryValue(value) !== value)
  .map(([key, value]) => ({ key, value, expected: normalizeZhGlossaryValue(value) }));
const zhUnexpectedEnglishViolations = Object.entries(zh)
  .map(([key, value]) => ({
    key,
    value,
    tokens: collectUnexpectedZhEnglishTokens(key, value),
  }))
  .filter((item) => item.tokens.length > 0);
const zhSameAsEnglishViolations = Object.entries(zh)
  .filter(([key, value]) => en[key] === value && collectUnexpectedZhEnglishTokens(key, value).length > 0)
  .map(([key, value]) => ({ key, value }));
const nativeLanguageLabelViolations = [];
const nativeLanguageLabelKeys = new Set([
  ...Object.keys(knownNativeLanguageLabels),
  ...Object.keys(en).filter((item) => item.startsWith("settings.language.option.")),
]);
for (const key of nativeLanguageLabelKeys) {
  const expected = knownNativeLanguageLabels[key] ?? en[key];
  for (const [locale, values] of [["en", en], ["zh-CN", zh]]) {
    if (values[key] !== expected) {
      nativeLanguageLabelViolations.push({
        locale,
        key,
        value: values[key] ?? "",
        expected,
      });
    }
  }
}

const summary = {
  locale: {
    enCount: enKeys.size,
    zhCount: zhKeys.size,
    usedKeyCount: usedKeys.size,
    missingInZhCount: missingInZh.length,
    extraInZhCount: extraInZh.length,
    unusedInEnCount: unusedInEn.length,
    missingDefinitionCount: missingDefs.length,
    concreteMissingDefinitionCount: concreteMissingDefs.length,
    dynamicMissingDefinitionCount: dynamicMissingDefs.length,
    i18nCallViolationCount: i18nCallViolations.length,
    englishStyleViolationCount: englishStyleViolations.length,
    zhGlossaryViolationCount: zhGlossaryViolations.length,
    zhUnexpectedEnglishViolationCount: zhUnexpectedEnglishViolations.length,
    zhSameAsEnglishViolationCount: zhSameAsEnglishViolations.length,
    nativeLanguageLabelViolationCount: nativeLanguageLabelViolations.length,
    hardcodedEnglishCount: hardcodedEnglish.length,
    i18nCopyNamingViolationCount: i18nCopyNamingViolations.length,
  },
  samples: {
    concreteMissingDefinitions: concreteMissingDefs.slice(0, 40),
    dynamicMissingDefinitions: dynamicMissingDefs.slice(0, 20),
    unusedInEn: unusedInEn.slice(0, 40),
    hardcodedChinese: hardcodedChinese.slice(0, 40),
    i18nCallViolations: i18nCallViolations.slice(0, 40),
    englishStyleViolations: englishStyleViolations.slice(0, 40),
    zhGlossaryViolations: zhGlossaryViolations.slice(0, 40),
    zhUnexpectedEnglishViolations: zhUnexpectedEnglishViolations.slice(0, 40),
    zhSameAsEnglishViolations: zhSameAsEnglishViolations.slice(0, 40),
    nativeLanguageLabelViolations: nativeLanguageLabelViolations.slice(0, 40),
    hardcodedEnglish: hardcodedEnglish.slice(0, 40),
    i18nCopyNamingViolations: i18nCopyNamingViolations.slice(0, 40),
  },
};

if (jsonOnly) {
  console.log(JSON.stringify(summary, null, 2));
} else {
  console.log("i18n audit summary");
  console.log(`- locale keys: en=${summary.locale.enCount}, zh-CN=${summary.locale.zhCount}`);
  console.log(`- used keys in source: ${summary.locale.usedKeyCount}`);
  console.log(`- missing definitions: ${summary.locale.missingDefinitionCount} (concrete=${summary.locale.concreteMissingDefinitionCount}, dynamic=${summary.locale.dynamicMissingDefinitionCount})`);
  console.log(`- unused en keys: ${summary.locale.unusedInEnCount}`);
  console.log(`- missing zh-CN keys: ${summary.locale.missingInZhCount}`);
  console.log(`- extra zh-CN keys: ${summary.locale.extraInZhCount}`);
  console.log(`- hardcoded Chinese lines: ${hardcodedChinese.length}`);
  console.log(`- hardcoded English lines: ${hardcodedEnglish.length}`);
  console.log(`- invalid i18n calls: ${i18nCallViolations.length}`);
  console.log(`- english style violations: ${englishStyleViolations.length}`);
  console.log(`- zh glossary violations: ${zhGlossaryViolations.length}`);
  console.log(`- zh unexpected English violations: ${zhUnexpectedEnglishViolations.length}`);
  console.log(`- zh same-as-English violations: ${zhSameAsEnglishViolations.length}`);
  console.log(`- native language label violations: ${nativeLanguageLabelViolations.length}`);
  console.log(`- i18n copy naming violations: ${i18nCopyNamingViolations.length}`);

  if (concreteMissingDefs.length > 0) {
    console.log("\nmissing concrete keys:");
    for (const key of concreteMissingDefs.slice(0, 20)) {
      console.log(`- ${key}`);
    }
  }

  if (dynamicMissingDefs.length > 0) {
    console.log("\ndynamic keys that need manual review:");
    for (const key of dynamicMissingDefs.slice(0, 10)) {
      console.log(`- ${key}`);
    }
  }

  if (hardcodedChinese.length > 0) {
    console.log("\nhardcoded Chinese samples:");
    for (const item of hardcodedChinese.slice(0, 20)) {
      console.log(`- ${item.file}:${item.line} ${item.content}`);
    }
  }

  if (hardcodedEnglish.length > 0) {
    console.log("\nhardcoded English samples:");
    for (const item of hardcodedEnglish.slice(0, 20)) {
      console.log(`- ${item.file}:${item.line} ${item.content}`);
    }
  }

  if (i18nCallViolations.length > 0) {
    console.log("\ninvalid i18n call samples:");
    for (const item of i18nCallViolations.slice(0, 20)) {
      console.log(`- ${item.file}:${item.line} ${item.content}`);
    }
  }

  if (englishStyleViolations.length > 0) {
    console.log("\nenglish style samples:");
    for (const item of englishStyleViolations.slice(0, 20)) {
      console.log(`- ${item.key}: ${item.value} -> ${item.expected}`);
    }
  }

  if (zhGlossaryViolations.length > 0) {
    console.log("\nzh glossary samples:");
    for (const item of zhGlossaryViolations.slice(0, 20)) {
      console.log(`- ${item.key}: ${item.value} -> ${item.expected}`);
    }
  }

  if (zhUnexpectedEnglishViolations.length > 0) {
    console.log("\nzh unexpected English samples:");
    for (const item of zhUnexpectedEnglishViolations.slice(0, 20)) {
      console.log(`- ${item.key}: ${item.value} (${item.tokens.join(", ")})`);
    }
  }

  if (zhSameAsEnglishViolations.length > 0) {
    console.log("\nzh same-as-English samples:");
    for (const item of zhSameAsEnglishViolations.slice(0, 20)) {
      console.log(`- ${item.key}: ${item.value}`);
    }
  }

  if (nativeLanguageLabelViolations.length > 0) {
    console.log("\nnative language label samples:");
    for (const item of nativeLanguageLabelViolations.slice(0, 20)) {
      console.log(`- ${item.locale}:${item.key}: ${item.value} -> ${item.expected}`);
    }
  }

  if (i18nCopyNamingViolations.length > 0) {
    console.log("\ni18n copy naming samples:");
    for (const item of i18nCopyNamingViolations.slice(0, 20)) {
      console.log(`- ${item.file}:${item.line} ${item.content}`);
    }
  }
}

if (pruneUnused) {
  const nextEn = filterLocaleTree(enSource, usedKeys);
  const nextZh = filterLocaleTree(zhSource, usedKeys);
  fs.writeFileSync(path.join(localeDir, "en.json"), `${JSON.stringify(nextEn, null, 2)}\n`);
  fs.writeFileSync(path.join(localeDir, "zh-CN.json"), `${JSON.stringify(nextZh, null, 2)}\n`);
}

if (strict) {
  const hasBlockingIssues =
    missingInZh.length > 0 ||
    extraInZh.length > 0 ||
    concreteMissingDefs.length > 0 ||
    dynamicMissingDefs.length > 0 ||
    unusedInEn.length > 0 ||
    hardcodedChinese.length > 0 ||
    hardcodedEnglish.length > 0 ||
    i18nCallViolations.length > 0 ||
    englishStyleViolations.length > 0 ||
    zhGlossaryViolations.length > 0 ||
    zhUnexpectedEnglishViolations.length > 0 ||
    zhSameAsEnglishViolations.length > 0 ||
    nativeLanguageLabelViolations.length > 0 ||
    i18nCopyNamingViolations.length > 0;
  if (hasBlockingIssues) {
    process.exitCode = 1;
  }
}
