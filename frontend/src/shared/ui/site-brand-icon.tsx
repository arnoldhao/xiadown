import { Globe2, Panda } from "lucide-react";
import * as React from "react";
import {
  siBilibili,
  siFacebook,
  siInstagram,
  siNiconico,
  siTiktok,
  siTwitch,
  siVimeo,
  siX,
  siYoutube,
} from "simple-icons";

import { cn } from "@/lib/utils";

const SITE_BRAND_ICONS = {
  youtube: siYoutube,
  bilibili: siBilibili,
  tiktok: siTiktok,
  instagram: siInstagram,
  x: siX,
  facebook: siFacebook,
  vimeo: siVimeo,
  twitch: siTwitch,
  niconico: siNiconico,
} satisfies Record<string, { path: string; title: string; hex: string }>;

const SITE_BRAND_FALLBACK_COLORS: Record<string, string> = {
  china_private: "#d95f35",
};

export function siteBrandKey(siteKey?: string) {
  return siteKey?.trim().toLowerCase() ?? "";
}

export function siteBrandColor(siteKey?: string) {
  const normalized = siteBrandKey(siteKey);
  const fallbackColor = SITE_BRAND_FALLBACK_COLORS[normalized];
  if (fallbackColor) {
    return fallbackColor;
  }
  const hex =
    normalized && SITE_BRAND_ICONS[normalized as keyof typeof SITE_BRAND_ICONS]
      ? SITE_BRAND_ICONS[normalized as keyof typeof SITE_BRAND_ICONS].hex.trim()
      : "";
  if (!hex) {
    return "";
  }
  return hex.startsWith("#") ? hex : `#${hex}`;
}

export function siteBrandSurfaceStyle(siteKey?: string): React.CSSProperties | undefined {
  const color = siteBrandColor(siteKey);
  if (!color) {
    return undefined;
  }
  return {
    "--app-session-brand-color-default": color,
    "--app-session-brand-surface-default": `${color}1A`,
    "--app-session-brand-ring-default": `${color}33`,
    backgroundColor:
      "var(--app-session-brand-surface, var(--app-session-brand-surface-default))",
    color:
      "var(--app-session-brand-color, var(--app-session-brand-color-default))",
    boxShadow:
      "inset 0 0 0 1px var(--app-session-brand-ring, var(--app-session-brand-ring-default))",
  } as React.CSSProperties;
}

export function SiteBrandIcon(props: {
  siteKey?: string;
  className?: string;
  fallback?: "globe" | "none";
}) {
  const normalized = siteBrandKey(props.siteKey);
  if (normalized === "china_private") {
    return (
      <Panda
        className={cn("block shrink-0", props.className)}
        aria-hidden="true"
      />
    );
  }
  const icon =
    normalized && SITE_BRAND_ICONS[normalized as keyof typeof SITE_BRAND_ICONS]
      ? SITE_BRAND_ICONS[normalized as keyof typeof SITE_BRAND_ICONS]
      : undefined;
  if (!icon) {
    if (props.fallback === "none") {
      return null;
    }
    return <Globe2 className={props.className} aria-hidden="true" />;
  }
  const color = siteBrandColor(normalized);
  return (
    <svg
      viewBox="0 0 24 24"
      fill="currentColor"
      aria-hidden="true"
      className={cn("block shrink-0", props.className)}
      style={
        color
          ? {
              color:
                "var(--app-session-brand-color, var(--app-session-brand-color-default, " +
                color +
                "))",
            }
          : undefined
      }
    >
      <path d={icon.path} />
    </svg>
  );
}
