import assert from "node:assert/strict";
import test from "node:test";

import {
  collectComparedCommits,
  compareStableVersions,
  filterChangelogPulls,
  incrementStableVersion,
  normalizeStableVersion,
} from "./release-notes.mjs";

test("stable release versions reject prerelease/build suffixes and compare numerically", () => {
  assert.equal(normalizeStableVersion("v1.0.0"), "1.0.0");
  assert.equal(normalizeStableVersion("1.0.0-beta.1"), "");
  assert.equal(normalizeStableVersion("1.0.0+build.1"), "");
  assert.equal(compareStableVersions("1.0.0", "0.4.7"), 1);
  assert.equal(incrementStableVersion("0.4.7", "major"), "1.0.0");
});

test("comparison pagination is complete and fails closed", async () => {
  const firstPage = Array.from({ length: 100 }, (_, index) => ({ sha: `first-${index}` }));
  const commits = await collectComparedCommits({
    baseTag: "v0.4.7",
    headSha: "release-head",
    fetchPage: async (page) => ({
      status: "ahead",
      total_commits: 101,
      commits: page === 1 ? firstPage : [{ sha: "last" }],
    }),
  });
  assert.equal(commits.length, 101);
  assert.equal(commits.at(-1).sha, "last");

  await assert.rejects(
    collectComparedCommits({
      baseTag: "v0.4.7",
      headSha: "missing",
      fetchPage: async () => null,
    }),
    /refusing to generate incomplete release notes/,
  );
  await assert.rejects(
    collectComparedCommits({
      baseTag: "v0.4.7",
      headSha: "same",
      fetchPage: async () => ({ status: "identical", total_commits: 0, commits: [] }),
    }),
    /must be ahead/,
  );
});

test("skip-changelog excludes a pull request case-insensitively", () => {
  const included = filterChangelogPulls([
    { number: 1, labels: ["feat"] },
    { number: 2, labels: ["Skip-Changelog", "chore"] },
    { number: 3 },
  ]);
  assert.deepEqual(included.map((pull) => pull.number), [1, 3]);
});
