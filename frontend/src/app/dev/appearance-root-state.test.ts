import { describe, expect, test } from "bun:test";

import {
  applyAppearanceLabNativeVideoPreview,
  applyAppearanceLabPlatform,
  applyRootDatasetValue,
} from "./appearance-root-state";

describe("Appearance Lab root dataset transactions", () => {
  test("removes a temporary value when the host had none", () => {
    const root = { dataset: {} as Record<string, string | undefined> };
    const cleanup = applyAppearanceLabPlatform(root, "windows");

    expect(root.dataset.platform).toBe("windows");
    cleanup();
    expect(Object.hasOwn(root.dataset, "platform")).toBe(false);
  });

  test("restores the exact host value on cleanup", () => {
    const root = {
      dataset: { platform: "host-preview" } as Record<string, string | undefined>,
    };
    const cleanup = applyAppearanceLabPlatform(root, "macos");

    expect(root.dataset.platform).toBe("macos");
    cleanup();
    expect(root.dataset.platform).toBe("host-preview");
  });

  test("supports reversible values other than platform", () => {
    const root = {
      dataset: { appearance: "dark" } as Record<string, string | undefined>,
    };
    const cleanup = applyRootDatasetValue(root, "appearance", "light");

    cleanup();
    expect(root.dataset.appearance).toBe("dark");
  });

  test("preserves an explicitly owned undefined snapshot", () => {
    const root = {
      dataset: { platform: undefined } as Record<string, string | undefined>,
    };
    const cleanup = applyAppearanceLabPlatform(root, "windows");

    cleanup();
    expect(Object.hasOwn(root.dataset, "platform")).toBe(true);
    expect(root.dataset.platform).toBeUndefined();
  });

  test("replays isolated native-video provider states and restores the host", () => {
    const root = {
      dataset: {
        listenNativeVideoUnderlay: "host-underlay",
        rssSiteVideoActive: "host-rss",
      } as Record<string, string | undefined>,
    };

    const cleanup = applyAppearanceLabNativeVideoPreview(root, "youtube");

    expect(root.dataset.listenNativeVideoUnderlay).toBe("true");
    expect(root.dataset.youtubeWorkspaceVideoActive).toBe("true");
    expect(root.dataset.rssBilibiliVideoActive).toBe("false");
    expect(root.dataset.rssSiteVideoActive).toBe("false");

    cleanup();
    expect(root.dataset.listenNativeVideoUnderlay).toBe("host-underlay");
    expect(root.dataset.rssSiteVideoActive).toBe("host-rss");
    expect(Object.hasOwn(root.dataset, "youtubeWorkspaceVideoActive")).toBe(
      false,
    );
    expect(Object.hasOwn(root.dataset, "rssBilibiliVideoActive")).toBe(false);
  });

  test("maps the RSS preview to the production site-video state", () => {
    const root = { dataset: {} as Record<string, string | undefined> };
    const cleanup = applyAppearanceLabNativeVideoPreview(root, "rss");

    expect(root.dataset.listenNativeVideoUnderlay).toBe("true");
    expect(root.dataset.youtubeWorkspaceVideoActive).toBe("false");
    expect(root.dataset.rssSiteVideoActive).toBe("true");

    cleanup();
    expect(root.dataset).toEqual({});
  });
});
