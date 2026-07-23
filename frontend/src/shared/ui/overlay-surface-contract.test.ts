import { expect, test } from "bun:test";

test("shared overlays declare a role instead of choosing panel material", async () => {
  const [dropdown, dialog, sheet, reveal, message] = await Promise.all([
    Bun.file(new URL("./dropdown-menu.tsx", import.meta.url)).text(),
    Bun.file(new URL("./dialog.tsx", import.meta.url)).text(),
    Bun.file(new URL("./sheet.tsx", import.meta.url)).text(),
    Bun.file(new URL("./secondary-reveal.tsx", import.meta.url)).text(),
    Bun.file(
      new URL("../message/MessageHost.tsx", import.meta.url),
    ).text(),
  ]);

  expect(dropdown).toContain('getXiaSurfaceAttributes("overlay")');
  expect(dialog).toContain('!unstyled ? "overlay" : undefined');
  expect(dialog).toContain("getXiaSurfaceAttributes(resolvedSurfaceRole)");
  expect(sheet).toContain('surfaceRole="overlay"');
  expect(reveal).toContain('surfaceRole="overlay"');
  expect(message).toContain('surfaceRole="status"');
  expect(message).toContain('getXiaSurfaceAttributes("overlay")');

  for (const source of [dropdown, dialog, sheet, reveal, message]) {
    expect(source).not.toContain('data-material="panel"');
    expect(source).not.toContain('material="panel"');
  }
});
