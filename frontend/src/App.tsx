import { useEffect, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { Events } from "@wailsio/runtime";

import { SettingsApp } from "./app/settings";
import { MainApp } from "./app/main";
import { TrayMiniPlayerApp } from "./app/main/TrayMiniPlayerApp";
import { setLatestSettingsQueryData, useSettings } from "./shared/query/settings";
import { useSettingsStore } from "./shared/store/settings";
import { detectBrowserLanguage, getLanguage, setLanguage, t } from "./shared/i18n";
import {
  applyPlatformChrome,
  applyXiaAppearanceChange,
  applyXiaTheme,
} from "./shared/styles/theme-runtime";

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
if (typeof document !== "undefined" && initialWindowType) {
  document.documentElement.dataset.window = initialWindowType;
}

function applyAppLanguage(nextLanguage: string) {
  setLanguage(nextLanguage);
  const language = getLanguage();
  document.documentElement.lang = language;
  document.title = t("xiadown.appName", language);
}

function App() {
  const queryClient = useQueryClient();
  const { data: settings, refetch: refetchSettings } = useSettings();
  const setSettings = useSettingsStore((state) => state.setSettings);
  const [windowType, setWindowType] = useState(initialWindowType);

  useEffect(() => {
    setWindowType(readWindowType());
  }, []);

  useEffect(() => {
    if (windowType) {
      document.documentElement.dataset.window = windowType;
      return () => {
        delete document.documentElement.dataset.window;
      };
    }
    delete document.documentElement.dataset.window;
  }, [windowType]);

  useEffect(() => {
    applyAppLanguage(detectBrowserLanguage());
    applyPlatformChrome();
  }, []);

  useEffect(() => {
    if (!settings) {
      return;
    }
    setSettings(settings);
    applyXiaTheme(settings);
    applyAppLanguage(settings.language);
  }, [settings, setSettings]);

  useEffect(() => {
    if (!isWailsRuntimeReady()) {
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
  }, [queryClient, setSettings]);

  useEffect(() => {
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
  }, [refetchSettings]);

  if (windowType === "settings") {
    return <SettingsApp />;
  }
  if (windowType === "tray-miniplayer") {
    return <TrayMiniPlayerApp />;
  }
  return <MainApp />;
}

export default App;
