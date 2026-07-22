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
  siXiaohongshu,
  siYoutube,
} from "simple-icons";

import { cn } from "@/lib/utils";

const SITE_BRAND_ICONS = {
  youtube: siYoutube,
  bilibili: siBilibili,
  douyin: siTiktok,
  xiaohongshu: siXiaohongshu,
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
        className={cn("app-site-brand-glyph", props.className)}
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
    return (
      <Globe2
        className={cn("app-site-brand-glyph", props.className)}
        aria-hidden="true"
      />
    );
  }
  const color = siteBrandColor(normalized);
  return (
    <svg
      viewBox="0 0 24 24"
      fill="currentColor"
      aria-hidden="true"
      className={cn(
        "app-site-brand-icon app-site-brand-glyph",
        props.className,
      )}
      style={
        color
          ? {
              "--app-site-brand-fallback-color": color,
            } as React.CSSProperties
          : undefined
      }
    >
      <path d={icon.path} />
    </svg>
  );
}
