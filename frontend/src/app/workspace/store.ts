import { create } from "zustand";
import {
  createJSONStorage,
  persist,
  type StateStorage,
} from "zustand/middleware";

import {
  appWorkspaceReducer,
  createInitialAppWorkspaceState,
  normalizeAppWorkspaceState,
} from "@/app/workspace/reducer";
import type {
  AppStation,
  AppStationId,
  AppWorkspaceAction,
  AppWorkspaceId,
  AppWorkspaceState,
  CompanionDestination,
  WorkspaceLocation,
} from "@/app/workspace/types";

export const APP_WORKSPACE_STORAGE_KEY = "xiadown:app-workspace:v2";
export const LEGACY_APP_WORKSPACE_STORAGE_KEY = "xiadown:app-workspace:v1";

export interface AppWorkspaceStore extends AppWorkspaceState {
  dispatch: (action: AppWorkspaceAction) => void;
  activateWorkspace: (
    workspaceId: AppWorkspaceId,
    location?: WorkspaceLocation,
  ) => void;
  navigate: (
    location: WorkspaceLocation,
    workspaceId?: AppWorkspaceId,
  ) => void;
  setStations: (stations: AppStation[]) => void;
  upsertStation: (station: AppStation) => void;
  removeStation: (stationId: AppStationId) => void;
  moveStation: (stationId: AppStationId, toIndex: number) => void;
  openCompanion: (destination: CompanionDestination) => void;
  closeCompanion: (clearDestination?: boolean) => void;
  toggleCompanion: (destination?: CompanionDestination) => void;
  resetWorkspaceState: (workspaceId?: AppWorkspaceId) => void;
}

const noopStorage: StateStorage = {
  getItem: () => null,
  setItem: () => undefined,
  removeItem: () => undefined,
};

const browserStorage: StateStorage = {
  getItem: (name) => {
    if (typeof window === "undefined") {
      return null;
    }
    return (
      window.localStorage.getItem(name) ??
      (name === APP_WORKSPACE_STORAGE_KEY
        ? window.localStorage.getItem(LEGACY_APP_WORKSPACE_STORAGE_KEY)
        : null)
    );
  },
  setItem: (name, value) => {
    if (typeof window === "undefined") {
      return;
    }
    window.localStorage.setItem(name, value);
  },
  removeItem: (name) => {
    if (typeof window === "undefined") {
      return;
    }
    window.localStorage.removeItem(name);
  },
};

const storage = createJSONStorage(() =>
  typeof window === "undefined" ? noopStorage : browserStorage,
);

export const useAppWorkspaceStore = create<AppWorkspaceStore>()(
  persist(
    (set) => {
      const dispatch = (action: AppWorkspaceAction) => {
        set((current) => appWorkspaceReducer(current, action));
      };
      return {
        ...createInitialAppWorkspaceState(),
        dispatch,
        activateWorkspace: (workspaceId, location) =>
          dispatch({ type: "activate-workspace", workspaceId, location }),
        navigate: (location, workspaceId) =>
          dispatch({ type: "navigate", workspaceId, location }),
        setStations: (stations) => dispatch({ type: "set-stations", stations }),
        upsertStation: (station) =>
          dispatch({ type: "upsert-station", station }),
        removeStation: (stationId) =>
          dispatch({ type: "remove-station", stationId }),
        moveStation: (stationId, toIndex) =>
          dispatch({ type: "move-station", stationId, toIndex }),
        openCompanion: (destination) =>
          dispatch({ type: "open-companion", destination }),
        closeCompanion: (clearDestination) =>
          dispatch({ type: "close-companion", clearDestination }),
        toggleCompanion: (destination) =>
          dispatch({ type: "toggle-companion", destination }),
        resetWorkspaceState: (workspaceId) =>
          dispatch({ type: "reset", workspaceId }),
      };
    },
    {
      name: APP_WORKSPACE_STORAGE_KEY,
      version: 2,
      storage,
      partialize: (state) => ({
        schemaVersion: state.schemaVersion,
        activeWorkspaceId: state.activeWorkspaceId,
        locations: state.locations,
        stations: state.stations,
        // Keep the last destination as a preference, but never restore an
        // open transient surface before its operation/player data hydrates.
        companion: { open: false, destination: state.companion.destination },
      }),
      migrate: (persisted) =>
        normalizeAppWorkspaceState(
          persisted as Partial<AppWorkspaceState>,
          createInitialAppWorkspaceState(),
        ),
      merge: (persisted, current) => {
        const normalized = normalizeAppWorkspaceState(
          persisted as Partial<AppWorkspaceState>,
          current,
        );
        return {
          ...current,
          ...normalized,
          companion: { ...normalized.companion, open: false },
        };
      },
    },
  ),
);
