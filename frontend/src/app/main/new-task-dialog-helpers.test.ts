import { describe, expect, test } from "bun:test";

import { resolveResourceSniffStartResolution } from "@/app/main/new-task-dialog-helpers";

describe("new task dialog resource sniff lifecycle helpers", () => {
  test("preserves a sniff start that was transferred to Sniff Desk", () => {
    expect(
      resolveResourceSniffStartResolution({
        requestVersion: 4,
        currentVersion: 9,
        dialogOpen: true,
        transferRequestVersion: 4,
      }),
    ).toBe("preserve");
  });

  test("cancels a stale sniff start after dialog state moved on", () => {
    expect(
      resolveResourceSniffStartResolution({
        requestVersion: 4,
        currentVersion: 5,
        dialogOpen: true,
        transferRequestVersion: null,
      }),
    ).toBe("cancel");
  });

  test("cancels a completed sniff start after the dialog closed", () => {
    expect(
      resolveResourceSniffStartResolution({
        requestVersion: 4,
        currentVersion: 4,
        dialogOpen: false,
        transferRequestVersion: null,
      }),
    ).toBe("cancel");
  });

  test("attaches a current sniff start while the dialog stays open", () => {
    expect(
      resolveResourceSniffStartResolution({
        requestVersion: 4,
        currentVersion: 4,
        dialogOpen: true,
        transferRequestVersion: null,
      }),
    ).toBe("attach");
  });
});
