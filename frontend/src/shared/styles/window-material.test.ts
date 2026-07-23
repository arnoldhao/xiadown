import { describe, expect, test } from "bun:test";

import {
  readWailsRuntimeOS,
  resolveWindowMaterialMode,
} from "./window-material";

describe("window material mode", () => {
  test("trusts only the backend-injected Wails environment", () => {
    expect(readWailsRuntimeOS({ _wails: {} })).toBe("");
    expect(
      readWailsRuntimeOS({
        _wails: { environment: { OS: " DARWIN " } },
      }),
    ).toBe("darwin");
  });

  test("uses the native underlay in main and Glass settings windows", () => {
    expect(resolveWindowMaterialMode({ runtimeOS: "darwin" })).toBe("native");
    expect(resolveWindowMaterialMode({ runtimeOS: "windows" })).toBe("native");
    expect(resolveWindowMaterialMode({ runtimeOS: "linux" })).toBe("css");
    expect(resolveWindowMaterialMode({ runtimeOS: undefined })).toBe("css");
    expect(
      resolveWindowMaterialMode({
        runtimeOS: "darwin",
        surfaceStyle: "glass",
        windowType: "settings",
      }),
    ).toBe("native");
    expect(
      resolveWindowMaterialMode({
        runtimeOS: "windows",
        windowType: "settings",
      }),
    ).toBe("native");
    expect(
      resolveWindowMaterialMode({
        runtimeOS: "darwin",
        surfaceStyle: "contrast",
        windowType: "settings",
      }),
    ).toBe("css");
    expect(
      resolveWindowMaterialMode({
        runtimeOS: "darwin",
        surfaceStyle: "glass",
        windowType: "tray-miniplayer",
      }),
    ).toBe("css");
  });

  test("lets explicit and OS transparency reduction mask every backdrop", () => {
    expect(
      resolveWindowMaterialMode({
        runtimeOS: "darwin",
        explicitReducedTransparency: true,
      }),
    ).toBe("solid");
    expect(
      resolveWindowMaterialMode({
        runtimeOS: "windows",
        prefersReducedTransparency: true,
      }),
    ).toBe("solid");
    expect(
      resolveWindowMaterialMode({
        runtimeOS: undefined,
        explicitReducedTransparency: true,
      }),
    ).toBe("solid");
  });

  test("mirrors material and Surface Style onto every eligible paint root", async () => {
    const [appSource, mainSource, settingsSource, foundationCSS, settingsShell] =
      await Promise.all([
        Bun.file(new URL("../../App.tsx", import.meta.url)).text(),
        Bun.file(new URL("../../app/main/MainApp.tsx", import.meta.url)).text(),
        Bun.file(
          new URL("../../app/settings/SettingsApp.tsx", import.meta.url),
        ).text(),
        Bun.file(new URL("./dream/foundation.css", import.meta.url)).text(),
        Bun.file(new URL("./dream/shell.css", import.meta.url)).text(),
      ]);

    expect(appSource).toContain("applyWindowMaterialMode();");
    expect(mainSource).toContain("useWindowMaterialMode()");
    expect(mainSource).toContain("data-window-material={windowMaterial}");
    expect(settingsSource).toContain(
      "useWindowMaterialMode(appearanceDraft.surfaceStyle)",
    );
    expect(settingsSource).toContain(
      "data-surface-style={appearanceDraft.surfaceStyle}",
    );
    expect(settingsSource).toContain(
      "data-window-material={windowMaterial}",
    );
    expect(foundationCSS).toMatch(
      /:root\[data-window-material="native"\],[\s\S]*?:root\[data-window-material="native"\] body,[\s\S]*?:root\[data-window-material="native"\] #root\s*\{[^}]*background:\s*transparent/s,
    );
    expect(settingsShell).toMatch(
      /\.app-settings-window\.app-dream-window\[data-surface-style="contrast"\]\s*\{[^}]*background:\s*var\(--app-surface-canvas\)/s,
    );
    expect(settingsShell).toMatch(
      /\.app-settings-window\[data-surface-style="glass"\]\[data-window-material="native"\]\s*\{[^}]*background:\s*var\(--app-surface-window-glass-wash\)/s,
    );
  });
});
