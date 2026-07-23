import {
  Component,
  useLayoutEffect,
  type ComponentType,
  type ErrorInfo,
  type PropsWithChildren,
} from "react";
import ReactDOM from "react-dom/client";

import "./index.css";
import {
  completeStartupPresentation,
  readStartupWindowType,
  reportStartupFailure,
  scheduleStartupCompletion,
  waitForNativeWailsHost,
} from "./startup-presentation";

function isAppearanceLabWindow() {
  return (
    new URLSearchParams(window.location.search).get("window") ===
    "appearance-lab"
  );
}

const root = ReactDOM.createRoot(
  document.getElementById("root") as HTMLElement,
);

const startupCopySource = document.getElementById("xiadown-startup");
const startupCrashCopy = {
  title:
    startupCopySource?.dataset.crashTitle ?? "XiaDown ran into a problem",
  detail:
    startupCopySource?.dataset.crashDetail ??
    "Reload the interface. Your downloads and library data will not be deleted.",
  retry: startupCopySource?.dataset.crashRetry ?? "Reload",
};

function StartupReady({ children }: PropsWithChildren) {
  useLayoutEffect(
    () => scheduleStartupCompletion(document, window),
    [],
  );
  return children;
}

function ReactCrashFallback() {
  return (
    <div id="xiadown-react-crash" role="alert">
      <img src="/appicon_startup.png" alt="" width="72" height="72" />
      <strong>{startupCrashCopy.title}</strong>
      <span>{startupCrashCopy.detail}</span>
      <button type="button" onClick={() => window.location.reload()}>
        {startupCrashCopy.retry}
      </button>
    </div>
  );
}

class StartupErrorBoundary extends Component<
  PropsWithChildren,
  { failed: boolean }
> {
  state = { failed: false };

  static getDerivedStateFromError() {
    return { failed: true };
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    reportStartupFailure({ error, componentStack: info.componentStack });
  }

  render() {
    return this.state.failed ? <ReactCrashFallback /> : this.props.children;
  }
}

async function loadWindowApplication(
  windowType: string,
): Promise<ComponentType> {
  if (windowType === "settings") {
    const { SettingsApp } = await import("./app/settings");
    return SettingsApp;
  }
  if (windowType === "tray-miniplayer") {
    const { TrayMiniPlayerApp } = await import("./app/main/TrayMiniPlayerApp");
    return TrayMiniPlayerApp;
  }
  return import("./app/main").then(
    ({ MainApp, preloadMainAppInitialSurface }) => {
      void preloadMainAppInitialSurface().catch(() => undefined);
      return MainApp;
    },
  );
}

async function preloadStartupSettings(windowType: string) {
  if (windowType === "appearance-lab" || !(await waitForNativeWailsHost(window))) {
    return;
  }
  const { preloadSettings } = await import("./shared/query/settings");
  await preloadSettings();
}

async function renderApplication() {
  const windowType = readStartupWindowType();
  if (isAppearanceLabWindow()) {
    if (!import.meta.env.DEV) {
      root.render(null);
      completeStartupPresentation();
      return;
    }
    const { AppearanceLab } = await import("./app/dev/AppearanceLab");
    root.render(
      <StartupErrorBoundary>
        <StartupReady>
          <AppearanceLab />
        </StartupReady>
      </StartupErrorBoundary>,
    );
    return;
  }

  // Start the only readiness-blocking RPC while the active application chunk
  // is still downloading and parsing. useSettings shares this in-flight call.
  void preloadStartupSettings(windowType).catch(() => undefined);

  const [
    { AppProviders },
    { default: App },
    { MessageHost },
    WindowApplication,
  ] =
    await Promise.all([
      import("./app/providers/AppProviders"),
      import("./App"),
      import("./shared/message"),
      loadWindowApplication(windowType),
    ]);

  root.render(
    <StartupErrorBoundary>
      <AppProviders
        runtimeEnabled={windowType !== "tray-miniplayer"}
        telemetryEnabled={windowType === ""}
      >
        <App
          onStartupReady={() => {
            return scheduleStartupCompletion(document, window);
          }}
        >
          <WindowApplication />
        </App>
        <MessageHost />
      </AppProviders>
    </StartupErrorBoundary>,
  );
}

void renderApplication().catch(reportStartupFailure);
