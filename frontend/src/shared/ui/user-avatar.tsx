import * as React from "react";

import type { CurrentUserProfile } from "@/shared/query/system";
import { cn } from "@/lib/utils";

export interface UserAvatarProps extends React.HTMLAttributes<HTMLDivElement> {
  profile?: CurrentUserProfile | null;
  imageClassName?: string;
  fallbackClassName?: string;
  tone?: "neutral" | "theme";
}

export function UserAvatar({
  profile,
  className,
  imageClassName,
  fallbackClassName,
  tone = "neutral",
  ...props
}: UserAvatarProps) {
  const avatarSrc = resolveUserAvatarSrc(profile);
  const initials = resolveUserInitials(profile);
  const label = resolveUserDisplayName(profile);
  const themed = tone === "theme";

  return (
    <div
      className={cn(
        "relative flex shrink-0 items-center justify-center overflow-hidden rounded-xl",
        themed
          ? "isolate bg-[radial-gradient(circle_at_32%_18%,hsl(var(--background)/0.62),transparent_40%),linear-gradient(135deg,hsl(var(--sidebar-accent)),hsl(var(--primary)/0.18))] text-sidebar-accent-foreground shadow-[inset_0_0_0_1px_hsl(var(--primary)/0.28),inset_0_1px_0_hsl(var(--background)/0.42),0_12px_26px_-22px_hsl(var(--primary)/0.75)]"
          : "bg-muted text-muted-foreground dark:bg-muted/80 dark:text-foreground/80",
        className
      )}
      aria-label={label}
      {...props}
    >
      {avatarSrc ? (
        <>
          <img
            src={avatarSrc}
            alt={label}
            className={cn(
              "h-full w-full object-cover",
              themed ? "contrast-[1.02] saturate-[0.9]" : null,
              imageClassName,
            )}
          />
          {themed ? (
            <span
              className="pointer-events-none absolute inset-0 bg-[linear-gradient(135deg,hsl(var(--primary)/0.24),transparent_48%,hsl(var(--sidebar-accent)/0.28))] mix-blend-soft-light"
              aria-hidden="true"
            />
          ) : null}
        </>
      ) : (
        <span
          className={cn(
            "relative z-10 text-sm font-semibold tracking-[0.16em]",
            themed ? "text-sidebar-accent-foreground" : null,
            fallbackClassName,
          )}
        >
          {initials}
        </span>
      )}
      {themed ? (
        <span
          className="pointer-events-none absolute inset-0 rounded-[inherit] shadow-[inset_0_0_0_1px_hsl(var(--primary)/0.24),inset_0_-10px_18px_hsl(var(--primary)/0.10)]"
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
