import {
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
  type PropsWithChildren,
} from "react";
import { useQueryClient } from "@tanstack/react-query";
import { Events } from "@wailsio/runtime";

import { LIBRARY_RESOURCE_SNIFF_RESOURCES_QUERY_KEY } from "./shared/query/library";
import { setLatestSettingsQueryData, useSettings } from "./shared/query/settings";
import { useSettingsStore } from "./shared/store/settings";
import { detectBrowserLanguage, getLanguage, setLanguage, t } from "./shared/i18n";
import {
  applyPlatformChrome,
  applyXiaAppearanceChange,
  applyXiaTheme,
} from "./shared/styles/theme-runtime";
import { applyWindowMaterialMode } from "./shared/styles/window-material";
import { STARTUP_SURFACE_READY_EVENT } from "./startup-presentation";

function isWailsRuntimeReady() {
  return typeof window !== "undefined" && typeof (window as any)._wails?.dispatchWailsEvent === "function";
}

function readWindowType() {
  if (typeof window === "undefined") {
    return "";
  }
  const params = new URLSearchParams(window.location.search);
  return params.get("window") || "";
}

const initialWindowType = readWindowType();
applyWindowMaterialMode();
if (typeof document !== "undefined" && initialWindowType) {
  document.documentElement.dataset.window = initialWindowType;
}

function applyAppLanguage(nextLanguage: string) {
  setLanguage(nextLanguage);
  const language = getLanguage();
  document.documentElement.lang = language;
  document.title = t("xiadown.appName", language);
}

function App({
  children,
  onStartupReady,
}: PropsWithChildren<{ onStartupReady?: () => void | (() => void) }>) {
  const queryClient = useQueryClient();
  const [windowType, setWindowType] = useState(initialWindowType);
  const {
    data: settings,
    isError: settingsFailed,
    refetch: refetchSettings,
  } = useSettings(
    windowType !== "appearance-lab",
  );
  const setSettings = useSettingsStore((state) => state.setSettings);
  const [startupSurfaceReady, setStartupSurfaceReady] = useState(
    () =>
      initialWindowType !== "" ||
      document.documentElement.dataset.startupSurface === "ready",
  );
  const startupReady = useRef(false);
  const startupCleanup = useRef<(() => void) | undefined>(undefined);

  useEffect(() => {
    setWindowType(readWindowType());
  }, []);

  useLayoutEffect(() => {
    if (windowType !== "") {
      setStartupSurfaceReady(true);
      return;
    }
    const markSurfaceReady = () => setStartupSurfaceReady(true);
    if (document.documentElement.dataset.startupSurface === "ready") {
      markSurfaceReady();
      return;
    }
    window.addEventListener(STARTUP_SURFACE_READY_EVENT, markSurfaceReady, {
      once: true,
    });
    return () => {
      window.removeEventListener(STARTUP_SURFACE_READY_EVENT, markSurfaceReady);
    };
  }, [windowType]);

  useEffect(() => {
    if (windowType) {
      document.documentElement.dataset.window = windowType;
      return () => {
        delete document.documentElement.dataset.window;
      };
    }
    delete document.documentElement.dataset.window;
  }, [windowType]);

  useLayoutEffect(() => {
    applyAppLanguage(detectBrowserLanguage());
    applyPlatformChrome();
  }, []);

  useLayoutEffect(() => {
    if (!settings || windowType === "appearance-lab") {
      return;
    }
    setSettings(settings);
    applyXiaTheme(settings);
    applyAppLanguage(settings.language);
  }, [settings, setSettings, windowType]);

  useLayoutEffect(() => {
    if (
      startupReady.current ||
      windowType === "appearance-lab" ||
      !startupSurfaceReady ||
      (!settings && !settingsFailed)
    ) {
      return;
    }
    startupReady.current = true;
    const cleanup = onStartupReady?.();
    startupCleanup.current = typeof cleanup === "function" ? cleanup : undefined;
  }, [onStartupReady, settings, settingsFailed, startupSurfaceReady, windowType]);

  useLayoutEffect(
    () => () => {
      startupCleanup.current?.();
    },
    [],
  );

  useEffect(() => {
    if (!isWailsRuntimeReady() || windowType === "appearance-lab") {
      return;
    }

    const offSettingsUpdated = Events.On("settings:updated", (event: any) => {
      const payload = event?.data ?? event;
      if (!payload) {
        return;
      }
      const next = setLatestSettingsQueryData(queryClient, payload);
      if (!next) {
        return;
      }
      setSettings(next);
      applyXiaTheme(next);
      if (next.language) {
        applyAppLanguage(next.language);
      }
      void queryClient.invalidateQueries({
        queryKey: LIBRARY_RESOURCE_SNIFF_RESOURCES_QUERY_KEY,
      });
    });

    const offThemeChanged = Events.On("theme:changed", (event: any) => {
      const appearance = event?.data ?? event;
      const current = useSettingsStore.getState().settings;
      applyXiaAppearanceChange(appearance, current);
    });

    return () => {
      offSettingsUpdated();
      offThemeChanged();
    };
  }, [queryClient, setSettings, windowType]);

  useEffect(() => {
    if (windowType === "appearance-lab") {
      return;
    }
    const reconcileSettings = () => {
      if (document.visibilityState === "hidden") {
        return;
      }
      void refetchSettings();
    };

    window.addEventListener("focus", reconcileSettings);
    document.addEventListener("visibilitychange", reconcileSettings);
    return () => {
      window.removeEventListener("focus", reconcileSettings);
      document.removeEventListener("visibilitychange", reconcileSettings);
    };
  }, [refetchSettings, windowType]);

  return children;
}

export default App;
