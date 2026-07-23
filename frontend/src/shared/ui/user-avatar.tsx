import * as React from "react";

import type { CurrentUserProfile } from "@/shared/query/system";
import { cn } from "@/lib/utils";

export const APP_USER_AVATAR_TONES = ["neutral", "theme"] as const;
export type UserAvatarTone = (typeof APP_USER_AVATAR_TONES)[number];
export const APP_USER_AVATAR_SHAPES = ["rounded", "circle"] as const;
export type UserAvatarShape = (typeof APP_USER_AVATAR_SHAPES)[number];

export interface UserAvatarProps extends React.HTMLAttributes<HTMLDivElement> {
  profile?: CurrentUserProfile | null;
  imageClassName?: string;
  fallbackClassName?: string;
  tone?: UserAvatarTone;
  shape?: UserAvatarShape;
}

export function UserAvatar({
  profile,
  className,
  imageClassName,
  fallbackClassName,
  tone = "neutral",
  shape = "rounded",
  ...props
}: UserAvatarProps) {
  const avatarSrc = resolveUserAvatarSrc(profile);
  const initials = resolveUserInitials(profile);
  const label = resolveUserDisplayName(profile);
  const themed = tone === "theme";

  return (
    <div
      className={cn(
        "app-user-avatar",
        className
      )}
      data-tone={tone}
      data-shape={shape}
      aria-label={label}
      {...props}
    >
      {avatarSrc ? (
        <>
          <img
            src={avatarSrc}
            alt={label}
            className={cn(
              "app-user-avatar__image",
              imageClassName,
            )}
          />
          {themed ? (
            <span
              className="app-user-avatar__wash"
              aria-hidden="true"
            />
          ) : null}
        </>
      ) : (
        <span
          className={cn(
            "app-user-avatar__fallback",
            fallbackClassName,
          )}
        >
          {initials}
        </span>
      )}
      {themed ? (
        <span
          className="app-user-avatar__rim"
          aria-hidden="true"
        />
      ) : null}
    </div>
  );
}

export function resolveUserAvatarSrc(profile?: CurrentUserProfile | null) {
  const avatarBase64 = profile?.avatarBase64?.trim() ?? "";
  if (!avatarBase64) {
    return "";
  }
  const avatarMime = profile?.avatarMime?.trim() || "image/png";
  return `data:${avatarMime};base64,${avatarBase64}`;
}

export function resolveUserDisplayName(profile?: CurrentUserProfile | null) {
  return profile?.displayName?.trim() || profile?.username?.trim() || "Desktop User";
}

export function resolveUserSubtitle(profile?: CurrentUserProfile | null) {
  const username = profile?.username?.trim() ?? "";
  const displayName = profile?.displayName?.trim() ?? "";
  if (username && displayName && username !== displayName) {
    return username;
  }
  return "";
}

export function resolveUserInitials(profile?: CurrentUserProfile | null) {
  const value = profile?.initials?.trim();
  if (value) {
    return value;
  }
  const source = resolveUserDisplayName(profile);
  const segments = source.split(/\s+/).filter(Boolean);
  if (segments.length > 1) {
    return segments
      .slice(0, 2)
      .map((segment) => segment[0] ?? "")
      .join("")
      .toUpperCase();
  }
  return source.slice(0, 2).toUpperCase();
}
