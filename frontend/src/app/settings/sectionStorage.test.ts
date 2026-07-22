import { describe, expect, test } from "bun:test";

import { isSettingsTab, resolveSettingsTab } from "./sectionStorage";

describe("settings section routing", () => {
  test("recognizes network as a first-class settings tab", () => {
    expect(isSettingsTab("network")).toBeTrue();
    expect(isSettingsTab("ai")).toBeTrue();
    expect(isSettingsTab("general")).toBeTrue();
    expect(isSettingsTab("proxy")).toBeFalse();
    expect(isSettingsTab("library-access")).toBeFalse();
  });

  test.each([
    "proxy",
    "network",
    "library",
    "library-access",
    "library_access",
    "libraryAccess",
    "remote-access",
    "tailscale",
  ])("routes the %s section alias to network", (section) => {
    expect(resolveSettingsTab(section)).toBe("network");
  });

  test("normalizes external section values without changing existing fallbacks", () => {
    expect(resolveSettingsTab(" Library-Access ")).toBe("network");
    expect(resolveSettingsTab("equalizer")).toBe("player");
    expect(resolveSettingsTab("AI")).toBe("ai");
    expect(resolveSettingsTab("unknown-section")).toBe("general");
    expect(resolveSettingsTab(null)).toBe("general");
  });
});
