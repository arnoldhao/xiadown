import { describe, expect, test } from "bun:test";

import {
  completeStartupPresentation,
  hasNativeWailsHost,
  hasNativeWailsTransport,
  isMainStartupWindow,
  readStartupWindowType,
  STARTUP_ERROR_EVENT,
  STARTUP_READY_EVENT,
  reportStartupFailure,
  waitForNativeWailsHost,
} from "./startup-presentation";

const read = (relativePath: string) =>
  Bun.file(new URL(relativePath, import.meta.url)).text();

describe("startup presentation routing", () => {
  test("recognises only the primary URL as the main startup window", () => {
    expect(readStartupWindowType("")).toBe("");
    expect(readStartupWindowType("?window=settings")).toBe("settings");
    expect(readStartupWindowType("?window=tray-miniplayer")).toBe(
      "tray-miniplayer",
    );
    expect(isMainStartupWindow("")).toBeTrue();
    expect(isMainStartupWindow("?window=settings")).toBeFalse();
  });

  test("publishes stable lifecycle event names", () => {
    expect(STARTUP_READY_EVENT).toBe("xiadown:startup-ready");
    expect(STARTUP_ERROR_EVENT).toBe("xiadown:startup-error");
  });

  test("waits for the navigation-finished Wails environment injection", async () => {
    const nativeHost = {
      _wails: {},
      webkit: {
        messageHandlers: {
          external: { postMessage() {} },
        },
      },
      setTimeout(callback: TimerHandler) {
        this._wails = { environment: { OS: "darwin" } };
        if (typeof callback === "function") callback();
        return 1;
      },
    } as unknown as Window;

    expect(hasNativeWailsHost(nativeHost)).toBeFalse();
    expect(hasNativeWailsTransport(nativeHost)).toBeTrue();
    expect(await waitForNativeWailsHost(nativeHost, 100)).toBeTrue();
    expect(hasNativeWailsHost(nativeHost)).toBeTrue();
  });

  test("removes the duplicate HTML shell immediately when native transport owns the transition", () => {
    const attributes = new Map<string, string>();
    let shellRemoved = false;
    const applicationRoot = {
      setAttribute(name: string, value: string) {
        attributes.set(name, value);
      },
      removeAttribute(name: string) {
        attributes.delete(name);
      },
    };
    const shell = {
      setAttribute() {},
      remove() {
        shellRemoved = true;
      },
    };
    const documentRef = {
      documentElement: { dataset: { startupState: "boot" } },
      getElementById(id: string) {
        if (id === "root") return applicationRoot;
        if (id === "xiadown-startup") return shell;
        return null;
      },
    } as unknown as Document;
    const host = {
      location: { search: "" },
      webkit: { messageHandlers: { external: { postMessage() {} } } },
      dispatchEvent() {
        return true;
      },
    } as unknown as Window;

    completeStartupPresentation(documentRef, host);

    expect(shellRemoved).toBeTrue();
    expect(documentRef.documentElement.dataset.startupState).toBe("ready");
    expect(attributes.get("aria-busy")).toBe("false");
    expect(attributes.has("aria-hidden")).toBeFalse();
    expect(attributes.has("inert")).toBeFalse();
  });

  test("does not hide an already-visible root after a later React error", () => {
    const documentRef = {
      documentElement: { dataset: { startupState: "ready" } },
    } as unknown as Document;
    const host = {
      dispatchEvent() {
        return true;
      },
    } as unknown as Window;
    const previousConsoleError = console.error;
    console.error = () => {};
    try {
      reportStartupFailure(new Error("late render"), documentRef, host);
    } finally {
      console.error = previousConsoleError;
    }
    expect(documentRef.documentElement.dataset.startupState).toBe("ready");
  });
});

describe("startup presentation contract", () => {
  test("ships a self-contained boot shell before the application module", async () => {
    const html = await read("../index.html");
    const shellIndex = html.indexOf('id="xiadown-startup"');
    const moduleIndex = html.indexOf('type="module" src="/src/main.tsx"');

    expect(shellIndex).toBeGreaterThan(0);
    expect(moduleIndex).toBeGreaterThan(shellIndex);
    expect(html).toContain('href="/appicon_startup.png"');
    expect(html).toContain(
      'id="root" aria-busy="true" aria-hidden="true" inert',
    );
    expect(html).toContain("visibility: hidden");
    expect(html).toContain("visibility: visible");
    expect(html).toContain('data-startup-state="ready"');
    expect(html).toContain("prefers-reduced-motion: reduce");
    expect(html).toContain("prefers-reduced-transparency: reduce");
    expect(html).toContain("root.dataset.startupWindow");
    expect(html).toContain('localStorage.getItem("xiadown:startup-theme")');
    expect(html).toContain('data-startup-window="main"');
    expect(html).toContain("12_000");
    expect(html).toContain('id="xiadown-startup-retry"');
    expect(Bun.file(new URL("../public/appicon_startup.png", import.meta.url)).size)
      .toBeLessThan(100_000);
  });

  test("releases startup only after settings/theme and a committed content frame", async () => {
    const [
      main,
      app,
      mainApp,
      providers,
      settingsQuery,
      requestCoordinator,
      startup,
      nativeOverlay,
    ] = await Promise.all([
      read("./main.tsx"),
      read("./App.tsx"),
      read("./app/main/MainApp.tsx"),
      read("./app/providers/AppProviders.tsx"),
      read("./shared/query/settings.ts"),
      read("./shared/query/initial-request-coordinator.ts"),
      read("./startup-presentation.ts"),
      read("../../internal/presentation/wails/main_window_startup_overlay_darwin.go"),
    ]);

    expect(main).toContain('import "./index.css"');
    expect(main).not.toContain('import("./index.css")');
    expect(main).not.toContain("startupPresentationGate");
    expect(main).toContain("loadWindowApplication(windowType)");
    expect(main).toContain("preloadMainAppInitialSurface");
    expect(main).toContain("preloadStartupSettings(windowType)");
    expect(main).toContain("waitForNativeWailsHost(window)");
    expect(main).toContain('runtimeEnabled={windowType !== "tray-miniplayer"}');
    expect(main).toContain('telemetryEnabled={windowType === ""}');
    expect(main).toContain("onStartupReady");
    expect(main).toContain("scheduleStartupCompletion(document, window)");
    expect(app).toContain("useLayoutEffect");
    expect(app).toContain("(!settings && !settingsFailed)");
    expect(app).toContain("!startupSurfaceReady");
    expect(app).toContain("STARTUP_SURFACE_READY_EVENT");
    expect(app.indexOf("applyXiaTheme(settings)"))
      .toBeLessThan(app.indexOf("onStartupReady?.()"));
    expect(app).not.toContain("fallback={null}");
    expect(app).not.toContain("lazy(");
    expect(mainApp).toContain("<MainStartupSurfaceReady />");
    expect(mainApp).toContain("root.dataset.startupSurface = \"ready\"");
    expect(providers).not.toContain(
      'import { TelemetryManager } from "@/shared/telemetry/manager"',
    );
    expect(providers).toContain('import("@/shared/telemetry/manager")');
    expect(providers).toContain('"xiadown:startup-ready"');
    expect(settingsQuery).toContain("createInitialRequestCoordinator");
    expect(settingsQuery).toContain("queryFn: requestInitialSettings");
    expect(requestCoordinator).toContain("if (initialRequested)");
    expect(requestCoordinator).toContain("await pendingRequest");
    expect(requestCoordinator).toContain("startupPreload ??= request()");
    expect(startup).toContain("SettingsHandler.MarkMainWindowBootReady");
    expect(startup).toContain("SettingsHandler.MarkMainWindowBootFailed");
    expect(startup).not.toContain("MINIMUM_NATIVE_STARTUP_VISIBLE_MS");
    expect(startup).toContain("requestAnimationFrame(notifyNativeHost)");
    expect(startup).toContain("hasNativeWailsTransport(host)");
    expect(startup).toContain("STARTUP_TRANSITION_FALLBACK_MS");
    expect(nativeOverlay).toContain(
      "events.Mac.WebViewDidStartProvisionalNavigation",
    );
    expect(nativeOverlay).toContain("xiadownInstallMainStartupOverlay");
    expect(nativeOverlay).toContain("addSubview:overlay positioned:NSWindowAbove");
    expect(nativeOverlay).toContain("minimumMainStartupOverlayVisible = 120");
    expect(nativeOverlay).toContain("duration = reduceMotion ? 0.01 : 0.12");
  });

  test("loads MainApp without serialising it behind the initial route preload", async () => {
    const main = await read("./main.tsx");
    const backgroundPreload = "void preloadMainAppInitialSurface().catch(";

    expect(main).toContain(backgroundPreload);
    expect(main).not.toContain("await preloadMainAppInitialSurface()");
    expect(main.indexOf(backgroundPreload)).toBeLessThan(
      main.indexOf("return MainApp;"),
    );
  });

  test("keeps startup readiness gated by the mounted main surface", async () => {
    const [app, mainApp] = await Promise.all([
      read("./App.tsx"),
      read("./app/main/MainApp.tsx"),
    ]);

    expect(app).toContain("!startupSurfaceReady");
    expect(app).toContain("STARTUP_SURFACE_READY_EVENT");
    expect(mainApp).toContain("<MainStartupSurfaceReady />");
    expect(mainApp).toContain('root.dataset.startupSurface = "ready"');
  });

  test("uses one three-dot loading language in native and HTML startup surfaces", async () => {
    const [html, nativeOverlay] = await Promise.all([
      read("../index.html"),
      read("../../internal/presentation/wails/main_window_startup_overlay_darwin.go"),
    ]);

    expect(html).toContain("xiadown-startup__progress");
    expect(html).toContain("xiadown-startup-dot 1.2s");
    expect(html).toContain("animation-delay: 120ms");
    expect(nativeOverlay).not.toContain("NSProgressIndicator");
    expect(nativeOverlay).toContain("NSStackView *progress");
    expect(nativeOverlay).toContain("index < 3");
    expect(nativeOverlay).toContain("wave.duration = 1.2");
    expect(nativeOverlay).toContain("index * 0.12");
    expect(nativeOverlay).toContain("accessibilityDisplayShouldReduceMotion");
  });
});
