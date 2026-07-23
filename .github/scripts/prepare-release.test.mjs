import assert from "node:assert/strict";
import { randomUUID } from "node:crypto";
import { mkdtemp, readFile, rm } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import test from "node:test";

function jsonResponse(value, status = 200) {
  return new Response(JSON.stringify(value), {
    status,
    headers: { "content-type": "application/json" },
  });
}

function releaseBase() {
  return {
    tag_name: "v0.4.7",
    draft: false,
    published_at: "2026-07-01T00:00:00Z",
  };
}

function singlePullReleaseFetch({ title, labels, body = "" }) {
  return async (input) => {
    const url = new URL(String(input));
    if (url.pathname.endsWith("/releases") && url.searchParams.has("per_page")) {
      return jsonResponse([releaseBase()]);
    }
    if (url.pathname.includes("/compare/")) {
      return jsonResponse({
        status: "ahead",
        total_commits: 1,
        commits: [{ sha: "pull-commit", commit: { message: title } }],
      });
    }
    if (url.pathname.endsWith("/commits/pull-commit/pulls")) {
      return jsonResponse([{ number: 1, merged_at: "2026-07-02T00:00:00Z" }]);
    }
    if (url.pathname.endsWith("/pulls/1")) {
      return jsonResponse({
        number: 1,
        merged_at: "2026-07-02T00:00:00Z",
        title,
        user: { login: "author" },
        labels: labels.map((name) => ({ name })),
        body,
      });
    }
    if (url.pathname.includes("/git/ref/tags/") || url.pathname.includes("/releases/tags/")) {
      return jsonResponse({ message: "Not Found" }, 404);
    }
    throw new Error("unexpected GitHub API request: " + url);
  };
}

const featureReleaseFetch = singlePullReleaseFetch({
  title: "Public feature",
  labels: ["type: feature"],
});
const breakingReleaseFetch = singlePullReleaseFetch({
  title: "Breaking change",
  labels: ["type: breaking"],
});

async function runPrepare({ fetchImpl, version = "1.0.0" }) {
  const workspace = await mkdtemp(path.join(os.tmpdir(), "xiadown-prepare-release-"));
  const output = path.join(workspace, "github-output.txt");
  const originalCwd = process.cwd();
  const originalFetch = globalThis.fetch;
  const originalEnvironment = {
    GITHUB_OUTPUT: process.env.GITHUB_OUTPUT,
    GITHUB_REPOSITORY: process.env.GITHUB_REPOSITORY,
    GITHUB_SHA: process.env.GITHUB_SHA,
    GITHUB_TOKEN: process.env.GITHUB_TOKEN,
    RELEASE_VERSION: process.env.RELEASE_VERSION,
  };

  try {
    process.chdir(workspace);
    process.env.GITHUB_OUTPUT = output;
    process.env.GITHUB_REPOSITORY = "arnoldhao/xiadown";
    process.env.GITHUB_SHA = "release-head";
    process.env.GITHUB_TOKEN = "test-token";
    process.env.RELEASE_VERSION = version;
    globalThis.fetch = fetchImpl;
    await import("./prepare-release.mjs?test=" + randomUUID());
    return await readFile(output, "utf8");
  } finally {
    process.chdir(originalCwd);
    globalThis.fetch = originalFetch;
    for (const [key, value] of Object.entries(originalEnvironment)) {
      if (value === undefined) {
        delete process.env[key];
      } else {
        process.env[key] = value;
      }
    }
    await rm(workspace, { recursive: true, force: true });
  }
}

test("automatic changelog fails closed when a direct commit has no merged pull request", async () => {
  await assert.rejects(
    runPrepare({
      fetchImpl: async (input) => {
        const url = new URL(String(input));
        if (url.pathname.endsWith("/releases") && url.searchParams.has("per_page")) {
          return jsonResponse([releaseBase()]);
        }
        if (url.pathname.includes("/compare/")) {
          return jsonResponse({
            status: "ahead",
            total_commits: 1,
            commits: [{ sha: "direct-commit", commit: { message: "Large direct update" } }],
          });
        }
        if (url.pathname.endsWith("/commits/direct-commit/pulls")) {
          return jsonResponse([]);
        }
        if (url.pathname.includes("/git/ref/tags/") || url.pathname.includes("/releases/tags/")) {
          return jsonResponse({ message: "Not Found" }, 404);
        }
        throw new Error("unexpected GitHub API request: " + url);
      },
    }),
    /have no merged pull request.*Release changes through a pull request/,
  );
});

test("breaking label bumps a 0.4.x release to 1.0.0", async () => {
  const output = await runPrepare({
    version: "",
    fetchImpl: breakingReleaseFetch,
  });

  assert.match(output, /^version=1\.0\.0$/m);
  assert.match(output, /^tag_name=v1\.0\.0$/m);
  assert.match(output, /### Breaking Changes\n- Breaking change \(#1\) by @author/);
  assert.doesNotMatch(output, /^base_tag=/m);
  assert.doesNotMatch(output, /^release_name=/m);
});

test("feature label uses automatic notes with an automatically calculated minor version", async () => {
  const output = await runPrepare({
    version: "",
    fetchImpl: featureReleaseFetch,
  });

  assert.match(output, /^version=0\.5\.0$/m);
  assert.match(output, /^tag_name=v0\.5\.0$/m);
  assert.match(output, /Public feature \(#1\) by @author/);
});

test("automatic changelog omits skip-changelog pull requests", async () => {
  const output = await runPrepare({
    fetchImpl: async (input) => {
      const url = new URL(String(input));
      if (url.pathname.endsWith("/releases") && url.searchParams.has("per_page")) {
        return jsonResponse([releaseBase()]);
      }
      if (url.pathname.includes("/compare/")) {
        return jsonResponse({
          status: "ahead",
          total_commits: 2,
          commits: [
            { sha: "feature-commit", commit: { message: "Feature" } },
            { sha: "internal-commit", commit: { message: "Internal" } },
          ],
        });
      }
      if (url.pathname.endsWith("/commits/feature-commit/pulls")) {
        return jsonResponse([{ number: 1, merged_at: "2026-07-02T00:00:00Z" }]);
      }
      if (url.pathname.endsWith("/commits/internal-commit/pulls")) {
        return jsonResponse([{ number: 2, merged_at: "2026-07-03T00:00:00Z" }]);
      }
      if (url.pathname.endsWith("/pulls/1")) {
        return jsonResponse({
          number: 1,
          merged_at: "2026-07-02T00:00:00Z",
          title: "Public feature",
          user: { login: "author" },
          labels: [{ name: "type: feature" }],
          body: "",
        });
      }
      if (url.pathname.endsWith("/pulls/2")) {
        return jsonResponse({
          number: 2,
          merged_at: "2026-07-03T00:00:00Z",
          title: "Internal maintenance",
          user: { login: "author" },
          labels: [{ name: "skip-changelog" }],
          body: "",
        });
      }
      if (url.pathname.includes("/git/ref/tags/") || url.pathname.includes("/releases/tags/")) {
        return jsonResponse({ message: "Not Found" }, 404);
      }
      throw new Error("unexpected GitHub API request: " + url);
    },
  });

  assert.match(output, /Public feature \(#1\) by @author/);
  assert.doesNotMatch(output, /Internal maintenance/);
});

test("automatic changelog fails closed when merged pull request details cannot be loaded", async () => {
  await assert.rejects(
    runPrepare({
      fetchImpl: async (input) => {
        const url = new URL(String(input));
        if (url.pathname.endsWith("/releases") && url.searchParams.has("per_page")) {
          return jsonResponse([releaseBase()]);
        }
        if (url.pathname.includes("/compare/")) {
          return jsonResponse({
            status: "ahead",
            total_commits: 1,
            commits: [{ sha: "pull-commit", commit: { message: "Feature" } }],
          });
        }
        if (url.pathname.endsWith("/commits/pull-commit/pulls")) {
          return jsonResponse([{ number: 1, merged_at: "2026-07-02T00:00:00Z" }]);
        }
        if (url.pathname.endsWith("/pulls/1")) {
          return jsonResponse({ message: "Not Found" }, 404);
        }
        throw new Error("unexpected GitHub API request: " + url);
      },
    }),
    /could not load merged pull request #1.*refusing to generate incomplete release notes/,
  );
});
