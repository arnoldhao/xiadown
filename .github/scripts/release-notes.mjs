export const changelogExcludeLabel = "skip-changelog";

export function normalizeStableVersion(value) {
  const normalized = String(value || "").trim().replace(/^[vV]/, "");
  return /^\d+\.\d+\.\d+$/.test(normalized) ? normalized : "";
}

function stableVersionParts(value) {
  const normalized = normalizeStableVersion(value);
  return normalized ? normalized.split(".").map((part) => Number.parseInt(part, 10)) : null;
}

export function compareStableVersions(left, right) {
  const leftParts = stableVersionParts(left);
  const rightParts = stableVersionParts(right);
  if (!leftParts || !rightParts) {
    return 0;
  }
  for (let index = 0; index < 3; index += 1) {
    if (leftParts[index] !== rightParts[index]) {
      return leftParts[index] - rightParts[index];
    }
  }
  return 0;
}

export function incrementStableVersion(version, level) {
  const parts = stableVersionParts(version) || [0, 0, 0];
  if (level === "major") {
    return `${parts[0] + 1}.0.0`;
  }
  if (level === "minor") {
    return `${parts[0]}.${parts[1] + 1}.0`;
  }
  return `${parts[0]}.${parts[1]}.${parts[2] + 1}`;
}

export async function collectComparedCommits({ baseTag, headSha, fetchPage, perPage = 100 }) {
  const commits = [];
  let expectedTotal = null;
  for (let page = 1; ; page += 1) {
    const comparison = await fetchPage(page, perPage);
    if (!comparison) {
      throw new Error(`GitHub could not compare ${baseTag} with ${headSha}; refusing to generate incomplete release notes`);
    }
    if (page === 1 && comparison.status !== "ahead") {
      throw new Error(
        `release head ${headSha} must be ahead of ${baseTag}; GitHub reported comparison status '${comparison.status || "unknown"}'`,
      );
    }
    const reportedTotal = Number(comparison.total_commits);
    if (!Number.isSafeInteger(reportedTotal) || reportedTotal < 1) {
      throw new Error(`GitHub returned an invalid commit count while comparing ${baseTag} with ${headSha}`);
    }
    if (expectedTotal === null) {
      expectedTotal = reportedTotal;
    } else if (expectedTotal !== reportedTotal) {
      throw new Error(`GitHub changed the comparison commit count while paginating ${baseTag}...${headSha}`);
    }
    if (!Array.isArray(comparison.commits)) {
      throw new Error(`GitHub returned no commit list while comparing ${baseTag} with ${headSha}`);
    }
    commits.push(...comparison.commits);
    if (commits.length === expectedTotal) {
      return commits;
    }
    if (comparison.commits.length === 0 || commits.length > expectedTotal) {
      throw new Error(
        `GitHub returned ${commits.length} of ${expectedTotal} commits while comparing ${baseTag} with ${headSha}`,
      );
    }
  }
}

export function filterChangelogPulls(pulls) {
  return pulls.filter((pull) => {
    const labels = (pull.labels || []).map((label) => String(label).trim().toLowerCase());
    return !labels.includes(changelogExcludeLabel);
  });
}
