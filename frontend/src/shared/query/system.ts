import { useQuery } from "@tanstack/react-query";
import { Call } from "@wailsio/runtime";

export interface CurrentUserProfile {
  username: string;
  displayName: string;
  initials?: string;
  avatarPath?: string;
  avatarBase64?: string;
  avatarMime?: string;
}

export const CURRENT_USER_PROFILE_QUERY_KEY = ["system", "current-user-profile"];
export const FONT_FAMILIES_QUERY_KEY = ["system", "font-families"];
const SYSTEM_HANDLER_SERVICE = "xiadown/internal/presentation/wails.SystemHandler";

export async function openExternalURL(url: string): Promise<void> {
  await Call.ByName(`${SYSTEM_HANDLER_SERVICE}.OpenURL`, { url });
}

export function useCurrentUserProfile() {
  return useQuery({
    queryKey: CURRENT_USER_PROFILE_QUERY_KEY,
    queryFn: async (): Promise<CurrentUserProfile> => {
      const result = await Call.ByName(`${SYSTEM_HANDLER_SERVICE}.GetCurrentUserProfile`);
      return normalizeCurrentUserProfile(result as Partial<CurrentUserProfile> | null | undefined);
    },
    staleTime: Infinity,
    refetchInterval: 60 * 60 * 1_000,
    retry: false,
  });
}

export function useFontFamilies() {
  return useQuery({
    queryKey: FONT_FAMILIES_QUERY_KEY,
    queryFn: async (): Promise<string[]> => {
      const result = await Call.ByName(`${SYSTEM_HANDLER_SERVICE}.ListFontFamilies`);
      return normalizeFontFamilies(result);
    },
    staleTime: Infinity,
    retry: false,
  });
}

function normalizeCurrentUserProfile(raw: Partial<CurrentUserProfile> | null | undefined): CurrentUserProfile {
  const anyRaw = (raw ?? {}) as Record<string, unknown>;
  return {
    username: stringOrEmpty(raw?.username) || stringOrEmpty(anyRaw.Username),
    displayName: stringOrEmpty(raw?.displayName) || stringOrEmpty(anyRaw.DisplayName),
    initials: stringOrEmpty(raw?.initials) || stringOrEmpty(anyRaw.Initials) || undefined,
    avatarPath: stringOrEmpty(raw?.avatarPath) || stringOrEmpty(anyRaw.AvatarPath) || undefined,
    avatarBase64: stringOrEmpty(raw?.avatarBase64) || stringOrEmpty(anyRaw.AvatarBase64) || undefined,
    avatarMime: stringOrEmpty(raw?.avatarMime) || stringOrEmpty(anyRaw.AvatarMime) || undefined,
  };
}

function normalizeFontFamilies(raw: unknown): string[] {
  if (!Array.isArray(raw)) {
    return [];
  }
  return Array.from(
    new Set(
      raw
        .map((item) => stringOrEmpty(item))
        .filter(Boolean),
    ),
  ).sort((left, right) => left.localeCompare(right));
}

function stringOrEmpty(value: unknown): string {
  return typeof value === "string" ? value.trim() : "";
}
