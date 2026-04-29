import * as React from "react";


import type { SetupState } from "@/app/main/types";
export {
  MAIN_SIDEBAR_ACTION_CLASS,
  MAIN_SIDEBAR_ICON_CLASS,
  SIDEBAR_DROPDOWN_CHECKBOX_ITEM_CLASS_NAME,
  SIDEBAR_DROPDOWN_CONTENT_CLASS_NAME,
  SIDEBAR_DROPDOWN_ICON_SLOT_CLASS_NAME,
  SIDEBAR_DROPDOWN_ITEM_CLASS_NAME,
} from "@/shared/styles/xiadown";

export const SETUP_STORAGE_KEY = "xiadown-setup-v1";
export const CORE_DEPENDENCIES = ["yt-dlp", "ffmpeg", "bun"] as const;
export const TASK_DIALOG_DEPENDENCIES = CORE_DEPENDENCIES;
export const COMPLETED_TASK_PAGE_SIZE_OPTIONS = [15, 30, 45, 60] as const;
export const COMPLETED_FILE_PAGE_SIZE_OPTIONS = [15, 30, 45, 60] as const;
export const COMPLETED_PREVIEW_SUPPORT_CACHE = new Map<string, boolean>();

export function readSetupState(): SetupState {
  if (typeof window === "undefined") {
    return { completed: false };
  }
  try {
    const raw = window.localStorage.getItem(SETUP_STORAGE_KEY);
    if (!raw) {
      return { completed: false };
    }
    const parsed = JSON.parse(raw) as Partial<SetupState>;
    return { completed: parsed.completed === true };
  } catch {
    return { completed: false };
  }
}

export function writeSetupState(value: SetupState) {
  if (typeof window === "undefined") {
    return;
  }
  window.localStorage.setItem(SETUP_STORAGE_KEY, JSON.stringify(value));
}

export function useSetupState() {
  const [state, setState] = React.useState<SetupState>(() => readSetupState());

  React.useEffect(() => {
    writeSetupState(state);
  }, [state]);

  return [state, setState] as const;
}
