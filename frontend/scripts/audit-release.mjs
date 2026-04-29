import { readdir, readFile } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";

const frontendRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const projectRoot = path.resolve(frontendRoot, "..");
const packageJsonPath = path.join(frontendRoot, "package.json");
const taskfilePaths = [
  path.join(projectRoot, "Taskfile.yml"),
  path.join(projectRoot, "build", "Taskfile.yml"),
  path.join(projectRoot, "build", "darwin", "Taskfile.yml"),
  path.join(projectRoot, "build", "linux", "Taskfile.yml"),
  path.join(projectRoot, "build", "windows", "Taskfile.yml"),
];
const workflowDir = path.join(projectRoot, ".github", "workflows");
const sourceRoots = [
  path.join(projectRoot, "internal"),
  path.join(frontendRoot, "src"),
  path.join(projectRoot, "Taskfile.yml"),
  path.join(projectRoot, "build"),
];
const sourceExtensions = new Set([".go", ".ts", ".tsx", ".json", ".yml", ".yaml"]);
const knownRealFixtureTokens = [
  "dQw4w9WgXcQ",
  "KMXZF-K2mus",
  "jfKfPfyJRdk",
  "LTphVIore3A",
  "ZyxwvutsrQ1",
  "L9OtFm1ltr8",
  "M1dIElsId2k",
  "dI0AauuJF_k",
  "gmv54pfxk0Q",
  "onA4NTUIAek",
  "Joanna Go",
  "云南大理",
  "泰国清迈",
  "潮州Vlog",
];

function relative(filePath) {
  return path.relative(projectRoot, filePath).split(path.sep).join("/");
}

async function collectFiles(root, output = []) {
  const statPath = path.basename(root);
  if (statPath === "node_modules" || statPath === "dist" || statPath === ".git") {
    return output;
  }
  const entries = await readdir(root, { withFileTypes: true });
  for (const entry of entries) {
    const fullPath = path.join(root, entry.name);
    if (entry.isDirectory()) {
      if (entry.name === "node_modules" || entry.name === "dist" || entry.name === ".git") {
        continue;
      }
      await collectFiles(fullPath, output);
      continue;
    }
    if (sourceExtensions.has(path.extname(entry.name))) {
      output.push(fullPath);
    }
  }
  return output;
}

async function collectWorkflowFiles(output = []) {
  let entries = [];
  try {
    entries = await readdir(workflowDir, { withFileTypes: true });
  } catch {
    return output;
  }
  for (const entry of entries) {
    const fullPath = path.join(workflowDir, entry.name);
    if (entry.isDirectory()) {
      continue;
    }
    if (entry.name.endsWith(".yml") || entry.name.endsWith(".yaml")) {
      output.push(fullPath);
    }
  }
  return output;
}

async function collectSourceFiles() {
  const files = [];
  for (const root of sourceRoots) {
    if (path.extname(root)) {
      files.push(root);
      continue;
    }
    await collectFiles(root, files);
  }
  return files;
}

function isTestFile(filePath) {
  return /(?:_test\.go|\.test\.[tj]sx?|\.spec\.[tj]sx?)$/.test(filePath);
}

async function collectScriptReferenceFindings() {
  const findings = [];
  const packageJson = JSON.parse(await readFile(packageJsonPath, "utf8"));
  const scriptNames = new Set(Object.keys(packageJson.scripts ?? {}));
  const scriptReferencePattern = /\b(?:bun|npm|pnpm|yarn)\s+run\s+([A-Za-z0-9:_-]+)/g;

  for (const filePath of [...taskfilePaths, ...(await collectWorkflowFiles())]) {
    const content = await readFile(filePath, "utf8");
    for (const match of content.matchAll(scriptReferencePattern)) {
      const scriptName = match[1];
      if (scriptNames.has(scriptName)) {
        continue;
      }
      findings.push(`${relative(filePath)}: references missing frontend script \`${scriptName}\``);
    }
  }

  return findings;
}

async function collectStaleNameFindings(sourceFiles) {
  const findings = [];
  for (const filePath of sourceFiles) {
    const content = await readFile(filePath, "utf8");
    if (content.includes("DREAMCREATOR_")) {
      findings.push(`${relative(filePath)}: stale DreamCreator environment variable prefix`);
    }
  }
  return findings;
}

async function collectTestFixtureFindings(sourceFiles) {
  const findings = [];
  for (const filePath of sourceFiles.filter(isTestFile)) {
    const content = await readFile(filePath, "utf8");
    for (const token of knownRealFixtureTokens) {
      if (content.includes(token)) {
        findings.push(`${relative(filePath)}: real fixture token \`${token}\``);
      }
    }
  }
  return findings;
}

async function main() {
  const sourceFiles = await collectSourceFiles();
  const findings = [
    ...(await collectScriptReferenceFindings()),
    ...(await collectStaleNameFindings(sourceFiles)),
    ...(await collectTestFixtureFindings(sourceFiles)),
  ];

  if (findings.length > 0) {
    console.error("Release audit failed:");
    for (const finding of findings) {
      console.error(`- ${finding}`);
    }
    process.exitCode = 1;
    return;
  }

  console.log("Release audit passed: referenced frontend scripts exist in Taskfile/workflow gates, stale project env prefixes are absent, and known real fixture tokens are not used in tests.");
}

await main();
