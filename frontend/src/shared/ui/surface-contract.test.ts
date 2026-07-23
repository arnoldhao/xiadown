import { describe, expect, test } from "bun:test";

import {
  getXiaSurfaceAttributes,
  XIA_GLASS_MATERIALS,
  XIA_SURFACE_ROLES,
  XIA_SURFACE_ROLE_PRESETS,
} from "./surface-contract";

describe("Xia surface contract", () => {
  test("publishes every canonical material", () => {
    expect(XIA_GLASS_MATERIALS).toEqual([
      "regular",
      "panel",
      "clear",
      "solid",
    ]);
  });

  test("defines one exhaustive role-to-material preset", () => {
    expect(Object.keys(XIA_SURFACE_ROLE_PRESETS)).toEqual(XIA_SURFACE_ROLES);
    expect(XIA_SURFACE_ROLE_PRESETS.status.material).toBe("regular");
    expect(XIA_SURFACE_ROLE_PRESETS.overlay.material).toBe("panel");
  });

  test.each(XIA_SURFACE_ROLES)("emits canonical attributes for %s", (role) => {
    expect(getXiaSurfaceAttributes(role)).toEqual({
      "data-surface-role": role,
      "data-material": XIA_SURFACE_ROLE_PRESETS[role].material,
    });
  });
});
