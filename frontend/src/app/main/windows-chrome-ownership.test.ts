import { describe, expect, test } from "bun:test";

describe("main window chrome ownership", () => {
  test("moves one Windows control group between primary and companion chrome", async () => {
    const source = await Bun.file(new URL("./MainApp.tsx", import.meta.url)).text();

    expect(source).toContain(
      "isWindows && !companion.open && !playerFullscreen",
    );
    expect(source).toContain(
      "activeWorkspaceId !== APP_WORKSPACE_IDS.music",
    );
    expect(source).toContain("{primaryWindowsDragRailVisible ? (");
    expect(source).toContain('owner="primary"');
    expect(source).toContain(
      'owner={playerFullscreen ? "fullscreen" : "companion"}',
    );
    expect(source).toContain(
      "{isWindows && (companion.open || playerFullscreen) ? (",
    );
    expect(source).toContain("{primaryWindowsChromeVisible ? (");
    expect(source.match(/<WindowControls/g)).toHaveLength(2);
  });

  test("lets Music own its Windows drag hit regions so detail actions stay clickable", async () => {
    const [mainSource, pageSource] = await Promise.all([
      Bun.file(new URL("./MainApp.tsx", import.meta.url)).text(),
      Bun.file(new URL("./listen/PageView.tsx", import.meta.url)).text(),
    ]);

    expect(mainSource).toContain(
      "activeWorkspaceId !== APP_WORKSPACE_IDS.music",
    );
    expect(mainSource).toContain("{primaryWindowsDragRailVisible ? (");
    expect(pageSource).toContain("{isWindows && !props.workspaceLayout ? (");
    expect(pageSource).not.toContain(
      'workspacePageContract?.topBar === "host-owned") ? (',
    );
  });

  test("keeps primary pages free of page-owned caption buttons", async () => {
    const sources = await Promise.all(
      [
        "./RunningPage.tsx",
        "./completed/CompletedPage.tsx",
        "./listen/PageView.tsx",
        "../pets-gallery/PetsGalleryPage.tsx",
        "../sniff-desk/SniffDeskPage.tsx",
        "../../features/settings/app-sessions/index.tsx",
      ].map((path) => Bun.file(new URL(path, import.meta.url)).text()),
    );

    for (const source of sources) {
      expect(source).not.toContain("<WindowControls");
      expect(source).not.toContain(
        'from "@/components/layout/WindowControls"',
      );
    }
  });

  test("uses the companion header as a draggable caption host", async () => {
    const [panelSource, mainSource] = await Promise.all([
      Bun.file(
        new URL("../workspace/CompanionPanel.tsx", import.meta.url),
      ).text(),
      Bun.file(new URL("./MainApp.tsx", import.meta.url)).text(),
    ]);

    expect(panelSource).toContain(
      'className="app-workspace-companion__header wails-drag"',
    );
    expect(mainSource).toContain(
      "right-[var(--app-windows-caption-control-width)]",
    );
    expect(mainSource).toContain(
      "h-[var(--app-windows-caption-button-height)]",
    );
  });
});
