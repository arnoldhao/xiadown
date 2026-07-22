import { describe, expect, test } from "bun:test";

import type { BrowserSourceProfile } from "@/shared/contracts/browserSources";
import { browserProfileDisplayLabel } from "./browserProfileDisplayLabel";

function profile(
  label: string,
  overrides: Partial<BrowserSourceProfile> = {},
): BrowserSourceProfile {
  return {
    id: "opaque-profile",
    browserId: "chrome",
    label,
    isDefault: true,
    available: true,
    ...overrides,
  };
}

describe("browser profile display label", () => {
  test("localizes Chrome's generated Your Chrome alias", () => {
    expect(
      browserProfileDisplayLabel(
        profile("Your Chrome"),
        "chrome",
        "Localized default",
        "Localized others",
      ),
    ).toBe("Localized default");
  });

  test("localizes a generated default alias while preserving its channel", () => {
    expect(
      browserProfileDisplayLabel(
        profile("Your Chrome · Beta"),
        "chrome",
        "Localized default",
        "Localized others",
      ),
    ).toBe("Localized default · Beta");
  });

  test("preserves user-selected profile names", () => {
    expect(
      browserProfileDisplayLabel(
        profile("Work account"),
        "chrome",
        "Localized default",
        "Localized others",
      ),
    ).toBe("Work account");
    expect(
      browserProfileDisplayLabel(
        profile("Your Chrome", { isDefault: false }),
        "chrome",
        "Localized default",
        "Localized others",
      ),
    ).toBe("Your Chrome");
  });

  test("localizes unavailable Safari's aggregate placeholder", () => {
    expect(
      browserProfileDisplayLabel(
        profile("Other Profiles", {
          browserId: "safari",
          available: false,
          isDefault: false,
        }),
        "safari",
        "Localized default",
        "Localized others",
      ),
    ).toBe("Localized others");
  });
});
