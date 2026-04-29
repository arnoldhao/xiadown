import { create } from "zustand";
import { persist } from "zustand/middleware";

import type { Settings } from "@/shared/contracts/settings";

interface SettingsStore {
  settings: Settings | null;
  isLoading: boolean;
  setSettings: (settings: Settings) => void;
  clearSettings: () => void;
}

export const useSettingsStore = create<SettingsStore>()(
  persist(
    (set) => ({
      settings: null,
      isLoading: true,
      setSettings: (settings) => set({ settings, isLoading: false }),
      clearSettings: () => set({ settings: null, isLoading: true }),
    }),
    {
      name: "settings-storage",
    }
  )
);
