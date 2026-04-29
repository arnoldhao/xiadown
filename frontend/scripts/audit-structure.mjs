import { readdir, readFile } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";

const frontendRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const projectRoot = path.resolve(frontendRoot, "..");
const localeDir = path.join(frontendRoot, "src", "shared", "i18n", "locales");
const frontendSrcRoot = path.join(frontendRoot, "src");
const scanRoots = [
  {
    root: path.join(frontendRoot, "src"),
    thresholds: {
      ".ts": 5000,
      ".tsx": 5000,
    },
  },
  {
    root: path.join(projectRoot, "internal"),
    thresholds: {
      ".go": 5000,
    },
  },
];
const allowedExtensions = new Set([".ts", ".tsx", ".go"]);
const localeKeyPattern = /^[a-z0-9][A-Za-z0-9_-]*(?:\.[a-z0-9][A-Za-z0-9_-]*)*$/;

async function collectFiles(root) {
  const results = [];
  const entries = await readdir(root, { withFileTypes: true });
  for (const entry of entries) {
    const fullPath = path.join(root, entry.name);
    if (entry.isDirectory()) {
      if (fullPath.includes(`${path.sep}bindings`)) {
        continue;
      }
      results.push(...(await collectFiles(fullPath)));
      continue;
    }
    if (allowedExtensions.has(path.extname(entry.name))) {
      results.push(fullPath);
    }
  }
  return results;
}

function relative(filePath) {
  return path.relative(projectRoot, filePath).split(path.sep).join("/");
}

function countLines(content) {
  return content.split(/\r?\n/).length;
}

async function collectOversizeFindings() {
  const findings = [];

  for (const scanRoot of scanRoots) {
    const files = await collectFiles(scanRoot.root);
    for (const filePath of files) {
      const extension = path.extname(filePath);
      const threshold = scanRoot.thresholds[extension];
      if (!threshold) {
        continue;
      }
      const content = await readFile(filePath, "utf8");
      const lineCount = countLines(content);
      if (lineCount <= threshold) {
        continue;
      }
      const relativePath = relative(filePath);
      findings.push(`${relativePath}: ${lineCount} lines exceeds ${threshold}-line threshold`);
    }
  }

  return findings;
}

function flattenLocale(input, prefix = "", output = []) {
  for (const [key, value] of Object.entries(input)) {
    const nextKey = prefix ? `${prefix}.${key}` : key;
    if (value && typeof value === "object" && !Array.isArray(value)) {
      flattenLocale(value, nextKey, output);
      continue;
    }
    output.push(nextKey);
  }
  return output;
}

async function collectLocaleKeyFindings() {
  const findings = [];
  const localeFiles = await readdir(localeDir);
  for (const fileName of localeFiles) {
    if (!fileName.endsWith(".json")) {
      continue;
    }
    const filePath = path.join(localeDir, fileName);
    const content = JSON.parse(await readFile(filePath, "utf8"));
    const keys = flattenLocale(content);
    for (const key of keys) {
      if (localeKeyPattern.test(key)) {
        continue;
      }
      findings.push(`frontend/src/shared/i18n/locales/${fileName}: invalid locale key \`${key}\``);
    }
  }
  return findings;
}

async function collectFrontendBoundaryFindings() {
  const findings = [];
  const files = await collectFiles(frontendSrcRoot);

  for (const filePath of files) {
    const content = await readFile(filePath, "utf8");
    const relativePath = relative(filePath);
    const normalized = relativePath.split(path.sep).join("/");

    if (normalized.startsWith("frontend/src/shared/")) {
      if (content.includes("@/app/")) {
        findings.push(`${relativePath}: shared module imports from app layer`);
      }
      if (content.includes("@/features/")) {
        findings.push(`${relativePath}: shared module imports from feature layer`);
      }
    }

    if (!normalized.startsWith("frontend/src/shared/ui/") && content.includes("@/components/ui/")) {
      findings.push(`${relativePath}: app code must import UI primitives through shared/ui wrappers`);
    }

    if (normalized.startsWith("frontend/src/shared/contracts/") && content.includes("@/shared/store/")) {
      findings.push(`${relativePath}: transport contract imports from store layer`);
    }
  }

  return findings;
}

async function main() {
  const findings = [
    ...(await collectOversizeFindings()),
    ...(await collectLocaleKeyFindings()),
    ...(await collectFrontendBoundaryFindings()),
  ];

  if (findings.length > 0) {
    console.error("Structure audit failed:");
    for (const finding of findings) {
      console.error(`- ${finding}`);
    }
    process.exitCode = 1;
    return;
  }

  console.log("Structure audit passed: file-size thresholds, locale key naming rules, and frontend layer boundaries are within limits.");
}

await main();
