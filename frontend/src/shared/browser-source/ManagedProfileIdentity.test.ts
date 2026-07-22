import { describe, expect, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";

import type { BrowserSourceProfile } from "@/shared/contracts/browserSources";
import {
  ManagedProfileAvatar,
  managedProfileDisplayLabel,
  managedProfileInitials,
} from "./ManagedProfileIdentity";

function profile(
  id: string,
  label: string,
  isDefault = false,
): BrowserSourceProfile {
  return {
    id,
    label,
    browserId: "chrome",
    available: true,
    isDefault,
  };
}

describe("managed Profile identity", () => {
  test("marks the XiaDown default without exposing an internal id", () => {
    const current = profile("opaque-profile-id", "XiaDown Chrome", true);
    expect(managedProfileDisplayLabel(current, [current], "Default profile")).toBe(
      "XiaDown Chrome · Default profile",
    );
  });

  test("gives duplicate custom display names a stable friendly ordinal", () => {
    const second = profile("profile-b", "Work");
    const first = profile("profile-a", "Work");
    expect(managedProfileDisplayLabel(first, [second, first], "Default")).toBe(
      "Work (1)",
    );
    expect(managedProfileDisplayLabel(second, [second, first], "Default")).toBe(
      "Work (2)",
    );
  });

  test("uses readable initials for a custom Profile avatar", () => {
    expect(managedProfileInitials(profile("profile", "Creator Work"))).toBe(
      "CW",
    );
  });

  test("delegates static avatar appearance to Dream", async () => {
    const current = profile("profile", "Creator Work");
    const [source, components] = await Promise.all([
      Bun.file(new URL("./ManagedProfileIdentity.tsx", import.meta.url)).text(),
      Bun.file(
        new URL("../styles/dream/components.css", import.meta.url),
      ).text(),
    ]);
    const markup = renderToStaticMarkup(
      ManagedProfileAvatar({ profile: current }),
    );

    expect(markup).toContain('class="app-managed-profile-initials"');
    expect(markup).toContain("app-managed-profile-avatar");
    expect(markup).toContain('data-tone="');
    expect(source).not.toMatch(
      /text-\[0\.68rem\]|font-bold|tracking-\[0\.08em\]/,
    );
    expect(source).not.toMatch(
      /(?:bg-|text-(?:foreground|background|muted|primary|secondary|destructive|blue|violet|emerald|amber)|border-|ring-|shadow-|rounded-|backdrop-blur|blur-|font-(?:bold|semibold|medium|mono)|tracking-|uppercase)/,
    );
    expect(components).toContain(".app-managed-profile-initials {");
    expect(components).toContain(".app-managed-profile-avatar {");
  });
});
