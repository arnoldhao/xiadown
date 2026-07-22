import { describe, expect, test } from "bun:test";

import {
  buildRSSArticleProgressStateRequest,
  buildRSSReadStateRequest,
  buildRSSStarredStateRequest,
  buildRSSVideoProgressStateRequest,
} from "./state-utils";
import type { RSSEntry } from "./types";

const entry = {
  id: "entry-1",
  revision: 7,
  fieldRevisions: {
    read: 2,
    starred: 3,
    articleProgress: 4,
    videoProgressSeconds: 5,
  },
} satisfies Pick<RSSEntry, "id" | "revision" | "fieldRevisions">;

describe("RSS v2 state requests", () => {
  test("uses the revision of the mutated field instead of global state revision", () => {
    expect(buildRSSReadStateRequest(entry, true)).toMatchObject({
      id: "entry-1",
      field: "read",
      read: true,
      expectedRevision: 2,
    });
    expect(buildRSSStarredStateRequest(entry, true)).toMatchObject({
      field: "starred",
      starred: true,
      expectedRevision: 3,
    });
  });

  test("binds article progress to the hydrated content revision", () => {
    expect(buildRSSArticleProgressStateRequest(entry, 1.5, "  section-2  ")).toMatchObject({
      field: "articleProgress",
      expectedRevision: 4,
      articleProgress: {
        fraction: 1,
        anchor: "section-2",
        contentRevision: 7,
      },
    });
  });

  test("normalizes video position and duration for the shared player", () => {
    expect(buildRSSVideoProgressStateRequest(entry, 140, 120)).toMatchObject({
      field: "videoProgressSeconds",
      expectedRevision: 5,
      videoProgressSeconds: 120,
      videoDurationSeconds: 120,
    });
  });
});
