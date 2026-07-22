import { describe, expect, test } from "bun:test";

const noticesURL = new URL("../../../public/THIRD_PARTY_NOTICES.txt", import.meta.url);
const settingsAppURL = new URL("./SettingsApp.tsx", import.meta.url);

describe("bundled third-party notices", () => {
  test("ships the asset requested by Settings", async () => {
    const [notices, settingsSource] = await Promise.all([
      Bun.file(noticesURL).text(),
      Bun.file(settingsAppURL).text(),
    ]);

    expect(settingsSource).toContain('const THIRD_PARTY_NOTICES_SRC = "/THIRD_PARTY_NOTICES.txt";');
    expect(notices).toContain("XiaDown Third-Party Notices");
    expect(notices).toContain("Frontend runtime components (");
    expect(notices).toContain("Go production components (");
    expect(notices).toContain("Component: Go standard library and runtime@go1.25.12");
    expect(notices).toContain("----- PATENTS -----");
  });

  test("preserves the selected Simple Icons CC BY attribution", async () => {
    const notices = await Bun.file(noticesURL).text();

    expect(notices).toContain("Vivaldi (siVivaldi, slug: vivaldi)");
    expect(notices).toContain("License: CC-BY-4.0");
    expect(notices).toContain("https://vivaldi.com/press");
    expect(notices).toContain("no change to upstream SVG path data");
  });

  test("does not present build-only tools as shipped runtime dependencies", async () => {
    const notices = await Bun.file(noticesURL).text();

    expect(notices).not.toContain("Component: vite@");
    expect(notices).not.toContain("Component: typescript@");
    expect(notices).not.toContain("Component: @vitejs/plugin-react@");
  });

  test("includes MIT terms without inventing copyright years for reviewed incomplete archives", async () => {
    const notices = await Bun.file(noticesURL).text();

    expect(notices).toContain("Component: react-remove-scroll-bar@2.3.8");
    expect(notices).toContain("Component: webworkify-webpack@2.1.5");
    expect(notices).toContain("MIT standard permission terms (upstream archive omitted LICENSE)");
    expect(notices).toContain("copyright/author metadata is not synthesized");
  });
});
