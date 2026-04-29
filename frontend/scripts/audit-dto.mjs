import { readdir, readFile } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";

const frontendRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const projectRoot = path.resolve(frontendRoot, "..");
const scanRoots = [
  path.join(frontendRoot, "src"),
  path.join(projectRoot, "internal"),
];
const frontendContractDir = path.join(frontendRoot, "src", "shared", "contracts");
const backendDTORoot = path.join(projectRoot, "internal", "application");
const generatedBindingDTORoot = path.join(frontendRoot, "bindings", "xiadown", "internal", "application");
const allowedExtensions = new Set([".ts", ".tsx", ".go"]);
const transportContractSuffixPattern = /(?:Request|Response|Result)$/;
const generatedContractPairs = [
  ["connectors", "connectors.ts"],
  ["dependencies", "dependencies.ts"],
  ["library", "library.ts"],
  ["settings", "settings.ts"],
  ["sprites", "sprites.ts"],
];

async function collectFiles(root) {
  const results = [];
  const entries = await readdir(root, { withFileTypes: true });
  for (const entry of entries) {
    const fullPath = path.join(root, entry.name);
    if (entry.isDirectory()) {
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
  return path.relative(projectRoot, filePath) || filePath;
}

function collectTypeVersionFindings(content, filePath) {
  const findings = [];
  const pattern = /\b(?:type|interface|class|struct)\s+([A-Za-z0-9_]*(?:V1|V2|v1|v2))\b/g;
  for (const match of content.matchAll(pattern)) {
    findings.push(`${relative(filePath)}: version suffix in DTO/type name \`${match[1]}\``);
  }
  return findings;
}

function collectLegacyFieldFindings(content, filePath) {
  const findings = [];
  const pattern = /\b(?:ManifestJSON|CompatibilityJSON|manifestJson|compatibilityJson)\b/g;
  for (const match of content.matchAll(pattern)) {
    findings.push(`${relative(filePath)}: legacy field \`${match[0]}\``);
  }
  return findings;
}

function collectStoreImportFindings(content, filePath) {
  const findings = [];
  const storePattern = /from\s+["']@\/shared\/store\/(library|connectors|dependencies)["']/g;
  for (const match of content.matchAll(storePattern)) {
    findings.push(`${relative(filePath)}: transport contract imported from store path \`${match[1]}\``);
  }

  const settingsPattern = /import\s+(?:type\s+)?\{([^}]*)\}\s+from\s+["']@\/shared\/store\/settings["']/gs;
  for (const match of content.matchAll(settingsPattern)) {
    const specifiers = match[1]
      .split(",")
      .map((value) => value.trim())
      .filter(Boolean);
    const invalid = specifiers.filter((value) => value !== "useSettingsStore");
    if (invalid.length > 0) {
      findings.push(
        `${relative(filePath)}: settings transport contract imported from store path \`${invalid.join(", ")}\``,
      );
    }
  }
  return findings;
}

function parseGeneratedBindingClasses(content) {
  const result = new Map();
  const classPattern = /\bexport\s+class\s+([A-Za-z0-9_]+)\s+\{([\s\S]*?)(?=\nexport\s+class\s+|\n\/\/ Private type creation functions|$)/g;
  for (const classMatch of content.matchAll(classPattern)) {
    const fields = [];
    const fieldPattern = /^\s+"([^"]+)"\??:/gm;
    for (const fieldMatch of classMatch[2].matchAll(fieldPattern)) {
      fields.push(fieldMatch[1]);
    }
    result.set(classMatch[1], fields);
  }
  return result;
}

function parseFrontendContractInterfaces(content) {
  const result = new Map();
  const interfacePattern = /\bexport\s+interface\s+([A-Za-z0-9_]+)\s+\{([\s\S]*?)\n\}/g;
  for (const interfaceMatch of content.matchAll(interfacePattern)) {
    const fields = [];
    const fieldPattern = /^\s*([A-Za-z_$][A-Za-z0-9_$]*)\??:/gm;
    for (const fieldMatch of interfaceMatch[2].matchAll(fieldPattern)) {
      fields.push(fieldMatch[1]);
    }
    result.set(interfaceMatch[1], fields);
  }
  return result;
}

function diffFields(expectedFields, actualFields) {
  const expected = new Set(expectedFields);
  const actual = new Set(actualFields);
  return {
    missing: expectedFields.filter((field) => !actual.has(field)),
    extra: actualFields.filter((field) => !expected.has(field)),
  };
}

async function collectGeneratedBindingContractFindings() {
  const findings = [];

  for (const [domainName, contractFileName] of generatedContractPairs) {
    const bindingPath = path.join(generatedBindingDTORoot, domainName, "dto", "models.ts");
    const contractPath = path.join(frontendContractDir, contractFileName);
    const bindingClasses = parseGeneratedBindingClasses(await readFile(bindingPath, "utf8"));
    const contractInterfaces = parseFrontendContractInterfaces(await readFile(contractPath, "utf8"));

    for (const [typeName, bindingFields] of bindingClasses) {
      const contractFields = contractInterfaces.get(typeName);
      if (!contractFields) {
        findings.push(`${relative(contractPath)}: missing frontend contract interface \`${typeName}\``);
        continue;
      }
      const { missing, extra } = diffFields(bindingFields, contractFields);
      if (missing.length > 0) {
        findings.push(`${relative(contractPath)}: contract \`${typeName}\` is missing fields ${missing.map((field) => `\`${field}\``).join(", ")}`);
      }
      if (extra.length > 0) {
        findings.push(`${relative(contractPath)}: contract \`${typeName}\` has fields not present in generated Go binding ${extra.map((field) => `\`${field}\``).join(", ")}`);
      }
    }

    for (const typeName of contractInterfaces.keys()) {
      if (!bindingClasses.has(typeName)) {
        findings.push(`${relative(contractPath)}: frontend transport contract \`${typeName}\` has no generated Go binding`);
      }
    }
  }

  return findings;
}

async function collectGoDTOTypeNames(root) {
  const files = await collectFiles(root);
  const names = new Set();
  const typePattern = /\btype\s+([A-Za-z0-9_]+)\s+struct\b/g;
  for (const filePath of files) {
    if (!filePath.includes(`${path.sep}dto${path.sep}`) || path.extname(filePath) !== ".go") {
      continue;
    }
    const content = await readFile(filePath, "utf8");
    for (const match of content.matchAll(typePattern)) {
      names.add(match[1].toLowerCase());
    }
  }
  return names;
}

async function collectFrontendContractFindings(goDTOTypeNames) {
  const findings = [];
  const files = await collectFiles(frontendContractDir);
  const interfacePattern = /\bexport\s+interface\s+([A-Za-z0-9_]+)\b/g;

  for (const filePath of files) {
    if (path.extname(filePath) !== ".ts") {
      continue;
    }
    const content = await readFile(filePath, "utf8");
    for (const match of content.matchAll(interfacePattern)) {
      const name = match[1];
      if (!transportContractSuffixPattern.test(name)) {
        continue;
      }
      if (goDTOTypeNames.has(name.toLowerCase())) {
        continue;
      }
      findings.push(`${relative(filePath)}: frontend transport contract \`${name}\` has no matching Go DTO`);
    }
  }

  return findings;
}

async function main() {
  const files = (await Promise.all(scanRoots.map((root) => collectFiles(root)))).flat();
  const findings = [];

  for (const filePath of files) {
    const content = await readFile(filePath, "utf8");
    findings.push(...collectTypeVersionFindings(content, filePath));
    findings.push(...collectLegacyFieldFindings(content, filePath));
    findings.push(...collectStoreImportFindings(content, filePath));
  }
  findings.push(...(await collectGeneratedBindingContractFindings()));
  findings.push(...(await collectFrontendContractFindings(await collectGoDTOTypeNames(backendDTORoot))));

  if (findings.length > 0) {
    console.error("DTO audit failed:");
    for (const finding of findings) {
      console.error(`- ${finding}`);
    }
    process.exitCode = 1;
    return;
  }

  console.log("DTO audit passed: no V1/V2 DTO names, legacy patch fields, store-path transport imports, or frontend-only transport contracts found.");
}

await main();
