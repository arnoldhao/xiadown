import { readdir, readFile, stat } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";

const frontendRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const distRoot = path.join(frontendRoot, "dist");
const assetsRoot = path.join(distRoot, "assets");
const maxJavaScriptBytes = 1_250_000;
const maxCSSBytes = 850_000;

function staticImports(source) {
  const imports = [];
  for (const match of source.matchAll(/(?:from|import)\s*["']\.\/([^"']+)["']/g)) {
    imports.push(match[1]);
  }
  return imports;
}

async function main() {
  const findings = [];
  const assets = await readdir(assetsRoot);
  for (const asset of assets) {
    const size = (await stat(path.join(assetsRoot, asset))).size;
    if (asset.endsWith(".js") && size > maxJavaScriptBytes) {
      findings.push(`${asset}: ${size} bytes exceeds the ${maxJavaScriptBytes}-byte JavaScript budget`);
    }
    if (asset.endsWith(".css") && size > maxCSSBytes) {
      findings.push(`${asset}: ${size} bytes exceeds the ${maxCSSBytes}-byte CSS budget`);
    }
    if (asset.includes("AppearanceLab")) {
      findings.push(`${asset}: development-only Appearance Lab leaked into the production bundle`);
    }
  }

  const html = await readFile(path.join(distRoot, "index.html"), "utf8");
  const entryMatch = html.match(/<script[^>]+src="\/assets\/([^"]+\.js)"/);
  if (!entryMatch) findings.push("index.html: production JavaScript entry was not found");
  if (/vendor-(?:media|opencc)[^"']*\.js/.test(html)) {
    findings.push("index.html: a media player or OpenCC dictionary is eagerly preloaded");
  }

  const entryName = entryMatch?.[1];
  if (entryName) {
    const entrySource = await readFile(path.join(assetsRoot, entryName), "utf8");
    if (/vendor-(?:media|opencc)[^"']*\.js/.test(entrySource)) {
      findings.push(`${entryName}: startup dependency map includes a media player or OpenCC dictionary`);
    }
  }

  let mainRoot = "";
  for (const asset of assets.filter((name) => name.endsWith(".js"))) {
    const source = await readFile(path.join(assetsRoot, asset), "utf8");
    if (source.includes("RunningPage-") && source.includes("RSSWorkspacePage-")) {
      mainRoot = asset;
      const eager = staticImports(source);
      if (eager.some((name) => name.startsWith("vendor-media-") || name.startsWith("vendor-opencc-"))) {
        findings.push(`${asset}: main window statically imports a media player or OpenCC dictionary`);
      }
      break;
    }
  }
  if (!mainRoot) findings.push("production bundle: main window root chunk was not identified");

  if (findings.length > 0) {
    console.error("Production bundle audit failed:");
    for (const finding of findings) console.error(`- ${finding}`);
    process.exitCode = 1;
    return;
  }
  console.log(`Production bundle audit passed (${assets.length} assets; main root ${mainRoot}).`);
}

await main();
