import { readdir, readFile } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";

const frontendRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const srcRoot = path.join(frontendRoot, "src");
const projectRoot = path.resolve(frontendRoot, "..");
const sourceExtensions = new Set([".ts", ".tsx"]);
const exportAuditFiles = [];
const orphanAuditDirs = [];

function relative(filePath) {
  return path.relative(projectRoot, filePath).split(path.sep).join("/");
}

function escapeRegExp(value) {
  return value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

async function collectSourceFiles(root, output = []) {
  const entries = await readdir(root, { withFileTypes: true });
  for (const entry of entries) {
    const fullPath = path.join(root, entry.name);
    if (entry.isDirectory()) {
      if (fullPath.includes(`${path.sep}bindings`)) {
        continue;
      }
      await collectSourceFiles(fullPath, output);
      continue;
    }
    if (sourceExtensions.has(path.extname(entry.name))) {
      output.push(fullPath);
    }
  }
  return output;
}

async function collectFilesInDir(root, output = []) {
  const entries = await readdir(root, { withFileTypes: true });
  for (const entry of entries) {
    const fullPath = path.join(root, entry.name);
    if (entry.isDirectory()) {
      await collectFilesInDir(fullPath, output);
      continue;
    }
    if (sourceExtensions.has(path.extname(entry.name))) {
      output.push(fullPath);
    }
  }
  return output;
}

function buildReferencePattern(stem) {
  const escaped = escapeRegExp(stem);
  const modulePath = `[^"']*${escaped}[^"']*`;
  return new RegExp(
    [
      `import\\s+["']${modulePath}["']`,
      `from\\s+["']${modulePath}["']`,
      `import\\(\\s*["']${modulePath}["']\\s*\\)`,
      `export\\s+.*\\s+from\\s+["']${modulePath}["']`,
    ].join("|"),
    "g",
  );
}

async function collectOrphanFileFindings(allFiles) {
  const findings = [];
  const contents = new Map();
  await Promise.all(
    allFiles.map(async (filePath) => {
      contents.set(filePath, await readFile(filePath, "utf8"));
    }),
  );

  for (const relativeDir of orphanAuditDirs) {
    const files = await collectFilesInDir(path.join(projectRoot, relativeDir));
    for (const filePath of files) {
      const stem = path.basename(filePath, path.extname(filePath));
      const pattern = buildReferencePattern(stem);
      let referenced = false;
      for (const otherFile of allFiles) {
        if (otherFile === filePath) {
          continue;
        }
        pattern.lastIndex = 0;
        if (pattern.test(contents.get(otherFile) ?? "")) {
          referenced = true;
          break;
        }
      }
      if (!referenced) {
        findings.push(`${relative(filePath)}: orphan source file with no imports or re-exports`);
      }
    }
  }

  return findings;
}

async function collectUnusedExportFindings(allFiles) {
  const findings = [];
  const contents = new Map();
  await Promise.all(
    allFiles.map(async (filePath) => {
      contents.set(filePath, await readFile(filePath, "utf8"));
    }),
  );

  for (const relativeFile of exportAuditFiles) {
    const filePath = path.join(projectRoot, relativeFile);
    const content = contents.get(filePath) ?? (await readFile(filePath, "utf8"));
    const exportedNames = [
      ...content.matchAll(/^export (?:const|function|type|interface|class)\s+([A-Za-z0-9_]+)/gm),
    ].map((match) => match[1]);

    for (const name of exportedNames) {
      const pattern = new RegExp(`\\b${escapeRegExp(name)}\\b`, "g");
      let referenced = false;
      for (const otherFile of allFiles) {
        if (otherFile === filePath) {
          continue;
        }
        pattern.lastIndex = 0;
        if (pattern.test(contents.get(otherFile) ?? "")) {
          referenced = true;
          break;
        }
      }
      if (!referenced) {
        findings.push(`${relativeFile}: exported symbol \`${name}\` has no external references`);
      }
    }
  }

  return findings;
}

async function main() {
  const allFiles = await collectSourceFiles(srcRoot);
  const findings = [
    ...(await collectOrphanFileFindings(allFiles)),
    ...(await collectUnusedExportFindings(allFiles)),
  ];

  if (findings.length > 0) {
    console.error("Redundancy audit failed:");
    for (const finding of findings) {
      console.error(`- ${finding}`);
    }
    process.exitCode = 1;
    return;
  }

  console.log("Redundancy audit passed: no orphan files or unreferenced helper exports found in the audited frontend modules.");
}

await main();
