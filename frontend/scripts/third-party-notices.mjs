import { execFile } from "node:child_process";
import { access, mkdir, readdir, readFile, rm, rmdir, writeFile } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";
import { promisify } from "node:util";

const execFileAsync = promisify(execFile);
const scriptDirectory = path.dirname(fileURLToPath(import.meta.url));
export const frontendRoot = path.resolve(scriptDirectory, "..");
export const workspaceRoot = path.resolve(frontendRoot, "..");
export const noticesPath = path.join(frontendRoot, "public", "THIRD_PARTY_NOTICES.txt");
export const distNoticesPath = path.join(frontendRoot, "dist", "THIRD_PARTY_NOTICES.txt");

const licenseDocumentPattern = /^(?:licen[sc]e|copying|notice|authors?)(?:[._-].*)?$/i;
const licenseTermsPattern = /^(?:licen[sc]e|copying)(?:[._-].*)?$/i;
const sourceExtensions = new Set([".js", ".jsx", ".mjs", ".ts", ".tsx"]);
const mitStandardTerms = `Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.`;

// These two npm archives declare MIT and preserve their author/source metadata,
// but do not contain a license document. Keep the identities exact so any
// version change requires a fresh human review instead of silently inheriting
// this exception.
const reviewedMissingLicenseDocuments = new Map([
  [
    "react-remove-scroll-bar@2.3.8",
    "The npm archive contains no license document; package.json declares MIT and identifies Anton Korzunov as author.",
  ],
  [
    "webworkify-webpack@2.1.5",
    "The npm archive and upstream repository contain no license document; package.json declares MIT and identifies Boris Sirota as author.",
  ],
]);

async function pathExists(candidate) {
  try {
    await access(candidate);
    return true;
  } catch {
    return false;
  }
}

let goEmbedPlaceholderSequence = 0;

async function withGoProductionEmbedPlaceholder(callback) {
  const distDirectory = path.join(frontendRoot, "dist");
  const directoryExisted = await pathExists(distDirectory);
  await mkdir(distDirectory, { recursive: true });
  const placeholderPath = path.join(
    distDirectory,
    `.license-audit-${process.pid}-${goEmbedPlaceholderSequence++}`,
  );
  try {
    // The production package embeds all:frontend/dist. A clean checkout has no
    // generated dist directory yet, but go list still evaluates embed patterns
    // while the license audit discovers the production dependency graph.
    await writeFile(placeholderPath, "", { flag: "wx" });
    return await callback();
  } finally {
    await rm(placeholderPath, { force: true });
    if (!directoryExisted) {
      try {
        await rmdir(distDirectory);
      } catch (error) {
        if (error?.code !== "ENOENT" && error?.code !== "ENOTEMPTY") throw error;
      }
    }
  }
}

async function readJSON(filename) {
  return JSON.parse(await readFile(filename, "utf8"));
}

export function normalizeLicense(value) {
  if (Array.isArray(value)) {
    return value
      .map((item) => typeof item === "string" ? item : item?.type)
      .filter(Boolean)
      .join(" OR ");
  }
  return typeof value === "string" ? value.trim() : "";
}

export function requiredRuntimeDependencyNames(manifest) {
  // Optional packages vary with the install host. Browser code that deliberately
  // ships one must promote it to a required app dependency so the committed
  // notices remain complete and identical on every release platform.
  return Object.keys(manifest.dependencies ?? {}).sort();
}

function normalizeRepository(repository, fallback) {
  let value = typeof repository === "string" ? repository : repository?.url;
  if (!value) return fallback;
  value = value
    .replace(/^git\+/, "")
    .replace(/^git@github\.com:/, "https://github.com/")
    .replace(/^ssh:\/\/git@github\.com\//, "https://github.com/")
    .replace(/^git:\/\/github\.com\//, "https://github.com/")
    .replace(/\.git(?:#.*)?$/, "")
    .replace(/#.*$/, "");
  return value;
}

function goRepository(modulePath) {
  const segments = modulePath.split("/");
  if (segments[0] === "github.com" && segments.length >= 3) {
    return `https://github.com/${segments[1]}/${segments[2]}`;
  }
  if (segments[0] === "git.sr.ht" && segments.length >= 3) {
    return `https://git.sr.ht/${segments[1]}/${segments[2]}`;
  }
  return "";
}

function repositoryKey(source) {
  return source
    .toLowerCase()
    .replace(/^git\+/, "")
    .replace(/\.git$/, "")
    .replace(/\/$/, "");
}

async function resolvePackageDirectory(packageName, fromDirectory) {
  let current = fromDirectory;
  while (current.startsWith(frontendRoot)) {
    const candidate = path.join(current, "node_modules", ...packageName.split("/"));
    if (await pathExists(path.join(candidate, "package.json"))) return candidate;
    if (current === frontendRoot) break;
    current = path.dirname(current);
  }
  return "";
}

async function collectLicenseDocuments(directory) {
  const names = (await readdir(directory))
    .filter((name) => licenseDocumentPattern.test(name))
    .sort((left, right) => left.localeCompare(right));
  const documents = [];
  for (const name of names) {
    const content = (await readFile(path.join(directory, name), "utf8"))
      .replace(/^\uFEFF/, "")
      .replace(/\r\n?/g, "\n")
      .replace(/[ \t]+$/gm, "")
      .trimEnd();
    if (!content || content.includes("\0")) {
      throw new Error(`${path.join(directory, name)} is empty or not a text license document`);
    }
    documents.push({ name, content, origin: "component archive" });
  }
  return documents;
}

export async function collectFrontendRuntimeComponents() {
  const rootManifest = await readJSON(path.join(frontendRoot, "package.json"));
  const queue = requiredRuntimeDependencyNames(rootManifest)
    .map((name) => ({ name, fromDirectory: frontendRoot }));
  const components = new Map();

  while (queue.length > 0) {
    const request = queue.shift();
    // Type declaration packages are build inputs, not JavaScript shipped in
    // the desktop bundle, even when an upstream package lists them under
    // dependencies instead of devDependencies.
    if (request.name.startsWith("@types/")) continue;
    const directory = await resolvePackageDirectory(request.name, request.fromDirectory);
    if (!directory) {
      throw new Error(`cannot resolve runtime dependency ${request.name} from ${request.fromDirectory}`);
    }

    const manifest = await readJSON(path.join(directory, "package.json"));
    const identity = `${manifest.name ?? request.name}@${manifest.version ?? "unknown"}`;
    if (components.has(identity)) continue;
    const license = normalizeLicense(manifest.license ?? manifest.licenses);
    if (!license) throw new Error(`${identity} has no license declaration`);
    const source = normalizeRepository(
      manifest.repository,
      manifest.homepage || `https://www.npmjs.com/package/${manifest.name ?? request.name}`,
    );
    components.set(identity, {
      kind: "frontend",
      identity,
      name: manifest.name ?? request.name,
      version: manifest.version ?? "unknown",
      license,
      source,
      author: typeof manifest.author === "string" ? manifest.author : manifest.author?.name ?? "",
      documents: await collectLicenseDocuments(directory),
    });

    for (const name of requiredRuntimeDependencyNames(manifest)) {
      queue.push({ name, fromDirectory: directory });
    }
  }

  return [...components.values()].sort((left, right) => left.identity.localeCompare(right.identity));
}

function detectGoLicense(documents, identity) {
  const text = documents
    .filter((document) => licenseTermsPattern.test(document.name))
    .map((document) => document.content)
    .join("\n");
  if (/Apache License\s+Version 2\.0/i.test(text)) return "Apache-2.0";
  if (/Permission is hereby granted, free of charge/i.test(text)) return "MIT";
  if (/Redistribution and use in source and binary forms/i.test(text)) {
    if (/Neither the name|name of the copyright holder nor the names of its contributors/i.test(text)) {
      return "BSD-3-Clause";
    }
    return "BSD-2-Clause";
  }
  if (/Permission to use, copy, modify, and\/or distribute/i.test(text)) return "ISC";
  throw new Error(`${identity} has an unrecognized production module license; review it explicitly`);
}

const goTargets = [
  { goos: "darwin", goarch: "amd64", cgo: "1" },
  { goos: "darwin", goarch: "arm64", cgo: "1" },
  { goos: "windows", goarch: "amd64", cgo: "0" },
  { goos: "linux", goarch: "amd64", cgo: "1" },
];

export async function collectGoProductionComponents() {
  const modules = new Map();
  const format = "{{with .Module}}{{if not .Main}}{{if .Replace}}{{printf \"%s\\t%s\\t%s\" .Path .Version .Replace.Dir}}{{else}}{{printf \"%s\\t%s\\t%s\" .Path .Version .Dir}}{{end}}{{end}}{{end}}";
  await withGoProductionEmbedPlaceholder(async () => {
    for (const target of goTargets) {
      const { stdout } = await execFileAsync(
        "go",
        ["list", "-deps", "-tags", "production", "-f", format, "."],
        {
          cwd: workspaceRoot,
          env: {
            ...process.env,
            GOOS: target.goos,
            GOARCH: target.goarch,
            CGO_ENABLED: target.cgo,
          },
          maxBuffer: 16 * 1024 * 1024,
        },
      );
      for (const line of stdout.split("\n")) {
        if (!line.trim()) continue;
        const [modulePath, version, directory] = line.split("\t");
        if (!modulePath || !version || !directory) {
          throw new Error(`unexpected go list module record: ${JSON.stringify(line)}`);
        }
        modules.set(`${modulePath}@${version}`, { modulePath, version, directory });
      }
    }
  });

  const components = [];
  for (const module of [...modules.values()].sort((left, right) => left.modulePath.localeCompare(right.modulePath))) {
    const identity = `${module.modulePath}@${module.version}`;
    const documents = await collectLicenseDocuments(module.directory);
    if (!documents.some((document) => licenseTermsPattern.test(document.name))) {
      throw new Error(`${identity} contains no LICENSE, LICENCE, or COPYING document`);
    }
    const repository = goRepository(module.modulePath);
    components.push({
      kind: "go",
      identity,
      name: module.modulePath,
      version: module.version,
      license: detectGoLicense(documents, identity),
      source: repository || `https://pkg.go.dev/${module.modulePath}@${module.version}`,
      modulePage: `https://pkg.go.dev/${module.modulePath}@${module.version}`,
      author: "",
      documents,
    });
  }

  const { stdout: goEnvironment } = await execFileAsync(
    "go",
    ["env", "GOVERSION", "GOROOT"],
    { cwd: workspaceRoot, maxBuffer: 1024 * 1024 },
  );
  const [goVersion, goRoot] = goEnvironment.trim().split("\n");
  if (!goVersion || !goRoot) throw new Error("go env did not return GOVERSION and GOROOT");
  const goDirective = goVersion.replace(/^go/, "");
  const goMod = await readFile(path.join(workspaceRoot, "go.mod"), "utf8");
  if (!new RegExp(`^go\\s+${goDirective.replaceAll(".", "\\.")}\\s*$`, "m").test(goMod)) {
    throw new Error(`Go runtime ${goVersion} does not match the go.mod go directive`);
  }
  const standardLibraryDocuments = [];
  for (const name of ["LICENSE", "PATENTS"]) {
    const content = (await readFile(path.join(goRoot, name), "utf8"))
      .replace(/^\uFEFF/, "")
      .replace(/\r\n?/g, "\n")
      .replace(/[ \t]+$/gm, "")
      .trimEnd();
    if (!content) throw new Error(`Go runtime ${name} is empty`);
    standardLibraryDocuments.push({ name, content, origin: "Go toolchain distribution" });
  }
  components.push({
    kind: "go",
    identity: `Go standard library and runtime@${goVersion}`,
    name: "Go standard library and runtime",
    version: goVersion,
    license: detectGoLicense(standardLibraryDocuments, `Go standard library and runtime@${goVersion}`),
    source: "https://go.dev/",
    author: "",
    documents: standardLibraryDocuments,
  });
  return components.sort((left, right) => left.identity.localeCompare(right.identity));
}

async function walkRuntimeSource(directory, result = []) {
  for (const entry of await readdir(directory, { withFileTypes: true })) {
    const entryPath = path.join(directory, entry.name);
    if (entry.isDirectory()) {
      if (entry.name !== "__snapshots__") await walkRuntimeSource(entryPath, result);
      continue;
    }
    if (!sourceExtensions.has(path.extname(entry.name))) continue;
    if (/\.(?:test|spec)\.[^.]+$/.test(entry.name)) continue;
    result.push(entryPath);
  }
  return result;
}

export async function collectSimpleIconAttributions(frontendComponents) {
  const imports = new Set();
  for (const filename of await walkRuntimeSource(path.join(frontendRoot, "src"))) {
    const source = await readFile(filename, "utf8");
    const pattern = /import\s*\{([^}]*)\}\s*from\s*["']simple-icons["']/g;
    for (const match of source.matchAll(pattern)) {
      for (const rawSpecifier of match[1].split(",")) {
        const specifier = rawSpecifier
          .replace(/\/\*[\s\S]*?\*\//g, "")
          .replace(/\/\/.*$/g, "")
          .trim()
          .replace(/^type\s+/, "")
          .split(/\s+as\s+/)[0];
        if (specifier) imports.add(specifier);
      }
    }
  }
  if (imports.size === 0) return [];
  const simpleIconsComponent = frontendComponents.find((component) => component.name === "simple-icons");
  if (!simpleIconsComponent) throw new Error("runtime source imports simple-icons but it is absent from the runtime dependency inventory");
  const icons = await import(pathToFileURL(path.join(frontendRoot, "node_modules", "simple-icons", "index.mjs")));
  return [...imports]
    .sort()
    .map((exportName) => {
      const icon = icons[exportName];
      if (!icon?.title || !icon?.slug || !icon?.source) {
        throw new Error(`cannot resolve Simple Icons metadata for ${exportName}`);
      }
      const license = icon.license?.type || simpleIconsComponent.license;
      return {
        exportName,
        title: icon.title,
        slug: icon.slug,
        source: icon.source,
        guidelines: icon.guidelines || "",
        license,
        licenseURL: icon.license?.url || `https://spdx.org/licenses/${license}.html`,
      };
    });
}

function attachLicenseFallbacks(components) {
  const repositoryLicenses = new Map();
  for (const component of components) {
    const terms = component.documents.filter((document) => licenseTermsPattern.test(document.name));
    if (terms.length === 0 || !component.source) continue;
    const key = `${repositoryKey(component.source)}\0${component.license}`;
    if (!repositoryLicenses.has(key)) repositoryLicenses.set(key, { component, terms });
  }

  for (const component of components) {
    if (component.documents.some((document) => licenseTermsPattern.test(document.name))) continue;
    const fallback = repositoryLicenses.get(`${repositoryKey(component.source)}\0${component.license}`);
    if (fallback) {
      component.documents.push(...fallback.terms.map((document) => ({
        ...document,
        name: `${document.name} (same upstream repository: ${fallback.component.identity})`,
        origin: "same upstream repository",
      })));
      continue;
    }
    const reviewedReason = reviewedMissingLicenseDocuments.get(component.identity);
    if (!reviewedReason) {
      throw new Error(`${component.identity} contains no license document and has no reviewed same-repository fallback`);
    }
    component.missingLicenseDocumentReason = reviewedReason;
    component.documents.push({
      name: "MIT standard permission terms (upstream archive omitted LICENSE)",
      content: mitStandardTerms,
      origin: "standard MIT terms; copyright/author metadata is not synthesized",
    });
  }
}

export async function createThirdPartyInventory() {
  const [frontendComponents, goComponents] = await Promise.all([
    collectFrontendRuntimeComponents(),
    collectGoProductionComponents(),
  ]);
  attachLicenseFallbacks([...frontendComponents, ...goComponents]);
  const simpleIconAttributions = await collectSimpleIconAttributions(frontendComponents);
  return { frontendComponents, goComponents, simpleIconAttributions };
}

function renderComponent(component) {
  const lines = [
    `Component: ${component.identity}`,
    `License: ${component.license}`,
    `Source: ${component.source}`,
  ];
  if (component.modulePage && component.modulePage !== component.source) {
    lines.push(`Module page: ${component.modulePage}`);
  }
  if (component.author) lines.push(`Author metadata: ${component.author}`);
  if (component.missingLicenseDocumentReason) {
    lines.push(
      `License document status: ${component.missingLicenseDocumentReason}`,
      `License terms: https://spdx.org/licenses/${component.license}.html`,
    );
  }
  for (const document of component.documents) {
    lines.push("");
    if (document.origin && document.origin !== "component archive") {
      lines.push(`Document origin: ${document.origin}`);
    }
    lines.push(`----- ${document.name} -----`, document.content, `----- end ${document.name} -----`);
  }
  return lines.join("\n");
}

export function renderThirdPartyNotices(inventory) {
  const lines = [
    "XiaDown Third-Party Notices",
    "============================",
    "",
    "This file is generated by frontend/scripts/third-party-notices.mjs. Do not edit it by hand.",
    "Run `bun run notices:update` from frontend/ after changing production dependencies.",
    "",
    "Scope and method",
    "----------------",
    "Frontend entries are the transitive closure of required package.json dependencies. devDependencies, host-dependent optional dependencies, and type-only @types packages are excluded from this browser inventory. Any optional package deliberately shipped by browser code must be promoted to a required app dependency. Go entries are the union of modules linked by production-tag builds for macOS (amd64/arm64), Windows (amd64), and Linux (amd64). License, NOTICE, COPYING, and AUTHORS files below preserve their text except for newline and trailing-whitespace normalization.",
    "",
    "Simple Icons brand-mark attribution",
    "-----------------------------------",
    `XiaDown imports the following marks from simple-icons@${inventory.frontendComponents.find((component) => component.name === "simple-icons")?.version ?? "unknown"}. Trademark rights are not granted by these notices. Source and per-icon license data are the metadata shipped by Simple Icons. XiaDown keeps the upstream path data unchanged and applies presentation color/styling at runtime.`,
    "",
  ];
  for (const icon of inventory.simpleIconAttributions) {
    lines.push(
      `- ${icon.title} (${icon.exportName}, slug: ${icon.slug})`,
      `  Source: ${icon.source}`,
      ...(icon.guidelines ? [`  Brand guidelines: ${icon.guidelines}`] : []),
      `  License: ${icon.license} (${icon.licenseURL})`,
      "  XiaDown modification: no change to upstream SVG path data; presentation styling is applied at runtime.",
    );
  }
  lines.push(
    "",
    `Frontend runtime components (${inventory.frontendComponents.length})`,
    "---------------------------------",
    "",
    ...inventory.frontendComponents.flatMap((component, index) => [
      renderComponent(component),
      ...(index === inventory.frontendComponents.length - 1 ? [] : ["", "========================================", ""]),
    ]),
    "",
    `Go production components (${inventory.goComponents.length})`,
    "-----------------------------",
    "",
    ...inventory.goComponents.flatMap((component, index) => [
      renderComponent(component),
      ...(index === inventory.goComponents.length - 1 ? [] : ["", "========================================", ""]),
    ]),
  );
  return `${lines.join("\n")}\n`;
}

async function assertFileMatches(filename, expected, label) {
  let actual;
  try {
    actual = await readFile(filename, "utf8");
  } catch (error) {
    throw new Error(`${label} is missing (${filename}): ${error.message}`);
  }
  if (!thirdPartyNoticesTextMatches(actual, expected)) {
    throw new Error(`${label} is stale; run \`bun run notices:update\` from frontend/ and commit the result`);
  }
}

export function normalizeThirdPartyNoticesText(value) {
  return value.replace(/^\uFEFF/, "").replace(/\r\n?/g, "\n");
}

export function thirdPartyNoticesTextMatches(actual, expected) {
  return normalizeThirdPartyNoticesText(actual) === normalizeThirdPartyNoticesText(expected);
}

export async function auditThirdPartyNotices({ checkDist = false } = {}) {
  const inventory = await createThirdPartyInventory();
  const expected = renderThirdPartyNotices(inventory);
  await assertFileMatches(noticesPath, expected, "committed third-party notices");
  if (checkDist) await assertFileMatches(distNoticesPath, expected, "built third-party notices asset");
  return inventory;
}

async function main() {
  const write = process.argv.includes("--write");
  const checkDist = process.argv.includes("--dist");
  const inventory = await createThirdPartyInventory();
  const expected = renderThirdPartyNotices(inventory);
  if (write) {
    await writeFile(noticesPath, expected, "utf8");
    console.log(`Wrote ${path.relative(workspaceRoot, noticesPath)} (${inventory.frontendComponents.length} frontend components, ${inventory.goComponents.length} Go production components).`);
    return;
  }
  await assertFileMatches(noticesPath, expected, "committed third-party notices");
  if (checkDist) await assertFileMatches(distNoticesPath, expected, "built third-party notices asset");
  console.log(`Third-party notices are current (${inventory.frontendComponents.length} frontend components, ${inventory.goComponents.length} Go production components, ${inventory.simpleIconAttributions.length} Simple Icons marks).`);
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  await main();
}
