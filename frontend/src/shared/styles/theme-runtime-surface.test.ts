import { expect, test } from "bun:test";

test("publishes the window-wide surface style and clears the legacy attribute", async () => {
  const source = await Bun.file(
    new URL("./theme-runtime.ts", import.meta.url),
  ).text();

  expect(source).toContain(
    "document.documentElement.dataset.xiadownSurfaceStyle = appearance.surfaceStyle",
  );
  expect(source).toContain(
    "delete document.documentElement.dataset.xiadownSidebarStyle",
  );
  expect(source).not.toContain(
    "dataset.xiadownSidebarStyle = appearance.sidebarStyle",
  );
});
