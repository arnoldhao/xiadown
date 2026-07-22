import { auditThirdPartyNotices } from "./third-party-notices.mjs";

// This is deliberately a deny-list rather than a narrow allow-list: the
// shipped app may consume permissive, notice, attribution, or public-domain
// licenses, but must not silently acquire reciprocal source-distribution or
// source-available obligations. Every installed package must still declare a
// license so a new or unusual identifier receives human review.
const forbiddenLicensePattern = /(?:^|[^A-Z])(?:AGPL|GPL|LGPL|SSPL|BUSL|CPAL|EUPL|OSL|RPL)(?:-|\b)/i;
const missingLicenseValues = new Set(["", "UNLICENSED", "UNKNOWN", "NONE"]);

async function main() {
  const findings = [];
  let inventory;
  try {
    inventory = await auditThirdPartyNotices({ checkDist: process.argv.includes("--dist") });
  } catch (error) {
    findings.push(error.message);
  }
  const components = inventory
    ? [...inventory.frontendComponents, ...inventory.goComponents]
    : [];
  for (const component of components) {
    if (missingLicenseValues.has(component.license.toUpperCase())) {
      findings.push(`${component.identity}: missing or non-distributable license declaration`);
    } else if (forbiddenLicensePattern.test(component.license)) {
      findings.push(`${component.identity}: forbidden production dependency license ${JSON.stringify(component.license)}`);
    }
  }
  for (const icon of inventory?.simpleIconAttributions ?? []) {
    if (forbiddenLicensePattern.test(icon.license)) {
      findings.push(`Simple Icons ${icon.title}: forbidden per-icon license ${JSON.stringify(icon.license)}`);
    }
  }

  if (findings.length > 0) {
    console.error("Production dependency license and notices audit failed:");
    for (const finding of findings.sort()) console.error(`- ${finding}`);
    process.exitCode = 1;
    return;
  }

  const licenseCounts = new Map();
  for (const item of components) {
    licenseCounts.set(item.license, (licenseCounts.get(item.license) ?? 0) + 1);
  }
  const summary = [...licenseCounts]
    .sort(([left], [right]) => left.localeCompare(right))
    .map(([license, count]) => `${license}: ${count}`)
    .join(", ");
  console.log(
    `Production dependency license and notices audit passed for ${inventory.frontendComponents.length} frontend runtime components, ${inventory.goComponents.length} Go production components, and ${inventory.simpleIconAttributions.length} attributed Simple Icons marks (${summary}).`,
  );
}

await main();
