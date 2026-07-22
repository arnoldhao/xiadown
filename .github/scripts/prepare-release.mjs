import { appendFileSync } from "node:fs";
import { randomUUID } from "node:crypto";

import {
  collectComparedCommits,
  compareStableVersions,
  filterChangelogPulls,
  incrementStableVersion,
  normalizeStableVersion,
} from "./release-notes.mjs";

const token = process.env.GITHUB_TOKEN;
const repository = process.env.GITHUB_REPOSITORY || "";
const headSha = (process.env.GITHUB_SHA || "").trim();
const rawRequestedVersion = (process.env.RELEASE_VERSION || "").trim();
const requestedVersion = normalizeStableVersion(rawRequestedVersion);

if (!token) {
  throw new Error("GITHUB_TOKEN is required");
}
if (!repository.includes("/")) {
  throw new Error("GITHUB_REPOSITORY is required");
}
if (!headSha) {
  throw new Error("GITHUB_SHA is required");
}
if (rawRequestedVersion && !requestedVersion) {
  throw new Error("RELEASE_VERSION must be a stable x.y.z version; prerelease and build suffixes are not supported");
}

const [owner, repo] = repository.split("/");
const headers = {
  Accept: "application/vnd.github+json",
  Authorization: `Bearer ${token}`,
  "User-Agent": "xiadown-release-preparer",
  "X-GitHub-Api-Version": "2022-11-28",
};

async function request(path, init = {}) {
  const url = path.startsWith("http") ? path : `https://api.github.com${path}`;
  const response = await fetch(url, {
    ...init,
    headers: {
      ...headers,
      ...(init.headers || {}),
    },
  });
  if (response.status === 404) {
    return null;
  }
  if (!response.ok) {
    const text = await response.text();
    throw new Error(`${init.method || "GET"} ${url} failed: ${response.status} ${text}`);
  }
  if (response.status === 204) {
    return null;
  }
  return response.json();
}

async function paginate(path, params = {}) {
  const result = [];
  let url = new URL(`https://api.github.com${path}`);
  for (const [key, value] of Object.entries({ per_page: 100, ...params })) {
    if (value !== undefined && value !== null && value !== "") {
      url.searchParams.set(key, String(value));
    }
  }
  while (url) {
    const response = await fetch(url, { headers });
    if (!response.ok) {
      const text = await response.text();
      throw new Error(`GET ${url} failed: ${response.status} ${text}`);
    }
    result.push(...(await response.json()));
    const link = response.headers.get("link") || "";
    const next = link.split(",").find((part) => part.includes('rel="next"'));
    const match = next?.match(/<([^>]+)>/);
    url = match ? new URL(match[1]) : null;
  }
  return result;
}

function parseStableVersion(value) {
  const normalized = normalizeStableVersion(value);
  if (!normalized) {
    return null;
  }
  return normalized.split(".").map((part) => Number.parseInt(part, 10));
}

async function latestSemverReleaseOrTag() {
  const releases = await paginate(`/repos/${owner}/${repo}/releases`);
  const release = releases
    .filter((item) => !item.draft && parseStableVersion(item.tag_name))
    .sort((left, right) => {
      const versionCompare = compareStableVersions(right.tag_name, left.tag_name);
      if (versionCompare !== 0) {
        return versionCompare;
      }
      return Date.parse(right.published_at || right.created_at || 0) - Date.parse(left.published_at || left.created_at || 0);
    })[0];
  if (release) {
    return {
      tag: release.tag_name,
      version: normalizeStableVersion(release.tag_name),
    };
  }

  const tags = await paginate(`/repos/${owner}/${repo}/tags`);
  const tag = tags
    .filter((item) => parseStableVersion(item.name))
    .sort((left, right) => compareStableVersions(right.name, left.name))[0];
  if (!tag) {
    return { tag: "", version: "0.0.0" };
  }
  return { tag: tag.name, version: normalizeStableVersion(tag.name) };
}

async function compareCommits(baseTag) {
  return collectComparedCommits({
    baseTag,
    headSha,
    fetchPage: (page, perPage) =>
      request(
        `/repos/${owner}/${repo}/compare/${encodeURIComponent(baseTag)}...${encodeURIComponent(headSha)}` +
          `?per_page=${perPage}&page=${page}`,
      ),
  });
}

async function collectPullRequestNumbers(base) {
  const numbers = new Set();
  const unmatchedCommits = [];
  if (base.tag) {
    for (const commit of await compareCommits(base.tag)) {
      const pulls = await request(`/repos/${owner}/${repo}/commits/${commit.sha}/pulls`, {
        headers: { Accept: "application/vnd.github+json" },
      });
      const mergedPulls = (pulls || []).filter((pull) => pull.merged_at);
      if (mergedPulls.length === 0) {
        unmatchedCommits.push({
          sha: commit.sha,
          title: String(commit.commit?.message || "").split("\n", 1)[0],
        });
      }
      for (const pull of mergedPulls) {
        numbers.add(pull.number);
      }
    }
  }

  return {
    numbers: [...numbers].sort((left, right) => left - right),
    unmatchedCommits,
  };
}

async function collectPullRequests(numbers) {
  const pulls = [];
  for (const number of numbers) {
    const pull = await request(`/repos/${owner}/${repo}/pulls/${number}`);
    if (!pull || !pull.merged_at) {
      throw new Error(`could not load merged pull request #${number}; refusing to generate incomplete release notes`);
    }
    pulls.push({
      number,
      title: pull.title || `Pull request #${number}`,
      author: pull.user?.login || "unknown",
      labels: (pull.labels || []).map((label) => String(label.name || "").toLowerCase()),
      body: pull.body || "",
    });
  }
  return pulls.sort((left, right) => left.number - right.number);
}

function bumpLevelForPulls(pulls) {
  const labels = pulls.flatMap((pull) => pull.labels);
  if (labels.some((label) => label === "breaking" || label === "type: breaking")) {
    return "major";
  }
  if (labels.some((label) => label === "type: feature" || label === "feat")) {
    return "minor";
  }
  return "patch";
}

function categoryForPull(pull) {
  const labels = new Set(pull.labels);
  if (labels.has("breaking") || labels.has("type: breaking")) {
    return "Breaking Changes";
  }
  if (labels.has("type: feature") || labels.has("feat")) {
    return "Features";
  }
  if (labels.has("type: bug") || labels.has("fix") || labels.has("bug")) {
    return "Fixes";
  }
  if (labels.has("refactor") || labels.has("perf") || labels.has("optimize")) {
    return "Refactor/Perf/Optimize";
  }
  if (labels.has("chore") || labels.has("build") || labels.has("ci") || labels.has("deps")) {
    return "Chore/Build/CI";
  }
  return "Other";
}

function normalizeDetails(body) {
  const cleaned = String(body || "")
    .replace(/\r\n?/g, "\n")
    .replace(/<!--[\s\S]*?-->/g, "")
    .trim();
  if (!cleaned) {
    return [];
  }

  const lines = [];
  let paragraph = [];
  let inCodeBlock = false;
  const flushParagraph = () => {
    if (paragraph.length > 0) {
      lines.push(paragraph.join(" "));
      paragraph = [];
    }
  };
  const cleanItem = (line) =>
    line
      .replace(/^- \[[ xX]\]\s+/, "")
      .replace(/^\[[ xX]\]\s+/, "")
      .replace(/^[-*+]\s+/, "")
      .replace(/^\d+\.\s+/, "")
      .replace(/^#+\s+/, "")
      .trim();

  for (const rawLine of cleaned.split("\n")) {
    const line = rawLine.trim();
    if (!line) {
      flushParagraph();
      continue;
    }
    if (/^```/.test(line)) {
      flushParagraph();
      inCodeBlock = !inCodeBlock;
      continue;
    }
    if (inCodeBlock) {
      continue;
    }
    const listLike =
      /^- \[[ xX]\]\s+/.test(line) ||
      /^\[[ xX]\]\s+/.test(line) ||
      /^[-*+]\s+/.test(line) ||
      /^\d+\.\s+/.test(line) ||
      /^#+\s+/.test(line);
    if (listLike) {
      flushParagraph();
      const item = cleanItem(line);
      if (item) {
        lines.push(item);
      }
      continue;
    }
    paragraph.push(line);
  }
  flushParagraph();
  return lines.filter(Boolean).slice(0, 6);
}

function releaseBody(version, pulls) {
  const tag = `v${version}`;
  const downloadBase = `https://github.com/${owner}/${repo}/releases/download/${tag}`;
  const sections = [
    "<!-- dreamapp-release-header:start -->",
    "## Current Version",
    `\`${tag}\``,
    "",
    "## Downloads",
    "| Platform | Official | Mirror |",
    "| --- | --- | --- |",
    `| macOS Apple Silicon | [Download](${downloadBase}/xiadown-macos-arm64-${version}.dmg) | [Mirror](https://gh-proxy.com/${downloadBase}/xiadown-macos-arm64-${version}.dmg) |`,
    `| macOS Intel | [Download](${downloadBase}/xiadown-macos-x64-${version}.dmg) | [Mirror](https://gh-proxy.com/${downloadBase}/xiadown-macos-x64-${version}.dmg) |`,
    `| Windows Installer | [Download](${downloadBase}/xiadown-windows-x64-${version}-installer.exe) | [Mirror](https://gh-proxy.com/${downloadBase}/xiadown-windows-x64-${version}-installer.exe) |`,
    `| Windows Portable | [Download](${downloadBase}/xiadown-windows-x64-${version}.zip) | [Mirror](https://gh-proxy.com/${downloadBase}/xiadown-windows-x64-${version}.zip) |`,
    "",
    "## macOS",
    "Requires macOS 14 (Sonoma) or later.",
    "",
    "Open the `.dmg`, drag `XiaDown.app` to Applications, and open it.",
    "",
    "---",
    "<!-- dreamapp-release-header:end -->",
    "",
    "## Changelog",
  ];

  if (pulls.length === 0) {
    sections.push("", "- Maintenance release.");
    return sections.join("\n");
  }

  const categories = ["Breaking Changes", "Features", "Fixes", "Refactor/Perf/Optimize", "Chore/Build/CI", "Other"];
  for (const category of categories) {
    const items = pulls.filter((pull) => categoryForPull(pull) === category);
    if (items.length === 0) {
      continue;
    }
    sections.push("", `### ${category}`);
    for (const pull of items) {
      sections.push(`- ${pull.title} (#${pull.number}) by @${pull.author}`);
      for (const detail of normalizeDetails(pull.body)) {
        sections.push(`  - ${detail}`);
      }
    }
  }
  return sections.join("\n");
}

async function assertReleaseDoesNotExist(tagName) {
  const tagRef = await request(`/repos/${owner}/${repo}/git/ref/tags/${encodeURIComponent(tagName)}`);
  if (tagRef) {
    throw new Error(`tag ${tagName} already exists`);
  }
  const release = await request(`/repos/${owner}/${repo}/releases/tags/${encodeURIComponent(tagName)}`);
  if (release) {
    throw new Error(`release ${tagName} already exists`);
  }
}

function setOutputs(outputs) {
  const delimiter = `release_body_${randomUUID()}`;
  const simpleOutputs = Object.entries(outputs).filter(([key]) => key !== "release_body");
  appendFileSync(
    process.env.GITHUB_OUTPUT,
    simpleOutputs.map(([key, value]) => `${key}=${value}`).join("\n") + "\n",
  );
  appendFileSync(process.env.GITHUB_OUTPUT, `release_body<<${delimiter}\n${outputs.release_body}\n${delimiter}\n`);
}

const base = await latestSemverReleaseOrTag();
if (requestedVersion && compareStableVersions(requestedVersion, base.version) <= 0) {
  throw new Error(`RELEASE_VERSION ${requestedVersion} must be newer than ${base.version}`);
}
const collected = await collectPullRequestNumbers(base);
const collectedPulls = await collectPullRequests(collected.numbers);
const pulls = filterChangelogPulls(collectedPulls);
const version = requestedVersion || incrementStableVersion(base.version, bumpLevelForPulls(collectedPulls));
const tagName = `v${version}`;

if (!base.tag) {
  throw new Error("automatic release notes require an existing stable release or tag as the comparison base");
}
if (collected.unmatchedCommits.length > 0) {
  const examples = collected.unmatchedCommits
    .slice(0, 5)
    .map((commit) => `${commit.sha.slice(0, 7)} ${commit.title}`)
    .join("; ");
  throw new Error(
    `${collected.unmatchedCommits.length} commit(s) since ${base.tag || "the initial history"} have no merged pull request ` +
      `and would be missing from generated notes (${examples}). Release changes through a pull request.`,
  );
}

await assertReleaseDoesNotExist(tagName);

setOutputs({
  version,
  tag_name: tagName,
  head_sha: headSha,
  release_body: releaseBody(version, pulls),
});

const skippedPullCount = collectedPulls.length - pulls.length;
console.log(
  `Prepared ${tagName} from ${base.tag || "initial history"} with changelog source ${pulls.length} pull request(s)` +
    (skippedPullCount > 0 ? `; skipped ${skippedPullCount} pull request(s) labeled skip-changelog.` : "."),
);
