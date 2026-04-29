import type { Sprite } from "@/shared/contracts/sprites";
import type { XiaThemePackId } from "@/shared/styles/xiadown-theme";
import type * as React from "react";

export interface SpriteCardLighting {
  cardClassName: string;
  primaryGlowClassName: string;
  directionalWashClassName: string;
  rimGlowClassName: string;
  spotlightClassName?: string;
  spriteGlowClassName?: string;
  spriteGlowStyle?: React.CSSProperties;
}

export const MAIN_SIDEBAR_ACTION_CLASS =
  "!h-[var(--app-main-sidebar-action-size)] !w-[var(--app-main-sidebar-action-size)]";
export const MAIN_SIDEBAR_ICON_CLASS =
  "h-[var(--app-main-sidebar-icon-size)] w-[var(--app-main-sidebar-icon-size)]";
export const SIDEBAR_DROPDOWN_CONTENT_CLASS_NAME =
  "w-max min-w-fit max-w-[calc(100vw-2rem)] rounded-xl bg-popover/95 p-1.5 shadow-lg backdrop-blur-sm";
export const SIDEBAR_DROPDOWN_ITEM_CLASS_NAME =
  "w-full gap-2 whitespace-nowrap rounded-lg py-2 pr-3 pl-3 text-sm outline-none";
export const SIDEBAR_DROPDOWN_CHECKBOX_ITEM_CLASS_NAME =
  "w-full gap-2 whitespace-nowrap rounded-lg py-2 pr-3 pl-8 text-sm outline-none";
export const SIDEBAR_DROPDOWN_ICON_SLOT_CLASS_NAME =
  "flex h-4 w-4 shrink-0 items-center justify-center text-muted-foreground";

export const SETTINGS_LIST_CARD_CLASS = "app-settings-card border";
export const SETTINGS_LIST_CARD_CONTENT_CLASS = "app-settings-card-content";
export const SETTINGS_ROW_BASE_CLASS = "app-settings-row";
export const SETTINGS_ROW_CLASS = `${SETTINGS_ROW_BASE_CLASS} items-center`;
export const SETTINGS_ROW_START_CLASS = `${SETTINGS_ROW_BASE_CLASS} app-settings-row-start`;
export const SETTINGS_ROW_LABEL_CLASS = "text-xs font-medium text-muted-foreground";
export const SETTINGS_ROW_LABEL_TRUNCATE_CLASS =
  "min-w-0 truncate text-xs font-medium text-muted-foreground";
export const SETTINGS_ROW_DESCRIPTION_CLASS = "text-xs text-muted-foreground";
export const SETTINGS_ROW_CONTENT_BASE_CLASS =
  "ml-auto flex min-w-0 justify-end gap-2 text-xs font-medium text-foreground";
export const SETTINGS_SEPARATOR_CLASS = "app-divider-soft my-1.5";
export const SETTINGS_CONTROL_WIDTH_CLASS =
  "w-[var(--app-settings-control-width)] min-w-0";
export const SETTINGS_WIDE_CONTROL_WIDTH_CLASS =
  "w-[var(--app-settings-control-width-wide)] min-w-0";
export const SETTINGS_COMPACT_LIST_CARD_CLASS = "overflow-hidden";
export const SETTINGS_COMPACT_LIST_CARD_CONTENT_CLASS = "p-0";
export const SETTINGS_COMPACT_ROW_CLASS = "app-settings-row-compact";
export const SETTINGS_COMPACT_ROW_LABEL_CLASS =
  "text-xs font-medium text-muted-foreground";
export const SETTINGS_COMPACT_ROW_DESCRIPTION_CLASS =
  "text-xs text-muted-foreground";
export const SETTINGS_COMPACT_ROW_CONTENT_CLASS = "text-xs";
export const SETTINGS_COMPACT_SEPARATOR_CLASS = "app-divider-inset my-0";

export const SPRITE_GALLERY_CARD_SIZE_CLASS =
  "h-[8.75rem] w-[6.125rem] min-w-[6.125rem] max-w-[6.125rem]";
export const SPRITE_GALLERY_CONTEXT_MENU_CONTENT_CLASS_NAME =
  "w-max min-w-fit max-w-[calc(100vw-2rem)] rounded-xl bg-popover/95 p-1.5 shadow-lg backdrop-blur-sm";
export const SPRITE_GALLERY_CONTEXT_MENU_ITEM_CLASS_NAME =
  "w-full gap-2 whitespace-nowrap rounded-lg py-2 pr-3 pl-3 text-sm outline-none";
export const SPRITE_GALLERY_CONTEXT_MENU_ICON_SLOT_CLASS_NAME =
  "flex h-4 w-4 shrink-0 items-center justify-center text-muted-foreground";
export const SPRITE_METADATA_TEXTAREA_CLASS =
  "min-h-[5rem] w-full rounded-lg border border-input bg-background px-2.5 py-2 text-xs shadow-sm outline-none transition";

export const COMPLETED_PREVIEW_SHELL_CLASS =
  "flex h-full min-h-0 flex-col overflow-hidden rounded-[18px] border border-border/70 bg-[#0b1118] text-white shadow-[0_20px_60px_-36px_rgba(15,23,42,0.65)]";
export const COMPLETED_PREVIEW_CONTROL_BUTTON_CLASS =
  "rounded-full text-white/90 hover:bg-white/15 hover:text-white";
export const COMPLETED_PREVIEW_CONTROL_RANGE_CLASS =
  "h-0.5 cursor-pointer accent-primary";
export const COMPLETED_PREVIEW_VOLUME_RANGE_CLASS = [
  COMPLETED_PREVIEW_CONTROL_RANGE_CLASS,
  "ml-0 w-0 min-w-0 opacity-0 transition-[margin,width,opacity] duration-150 ease-out",
  "pointer-events-none group-hover/volume:ml-1.5 group-hover/volume:w-14 group-hover/volume:opacity-100 group-hover/volume:pointer-events-auto",
  "group-focus-within/volume:ml-1.5 group-focus-within/volume:w-14 group-focus-within/volume:opacity-100 group-focus-within/volume:pointer-events-auto",
  "sm:group-hover/volume:w-16 sm:group-focus-within/volume:w-16",
].join(" ");

export const SPRITE_DISPLAY_GLOW_STYLE: React.CSSProperties = {
  backgroundImage: [
    "radial-gradient(ellipse 44% 48% at 50% 52%, hsl(var(--primary) / 0.18) 0%, hsl(var(--primary) / 0.1) 42%, transparent 76%)",
    "radial-gradient(ellipse 34% 34% at 35% 43%, hsl(var(--primary) / 0.12) 0%, transparent 72%)",
    "radial-gradient(ellipse 34% 38% at 65% 58%, hsl(var(--primary) / 0.1) 0%, transparent 74%)",
    "radial-gradient(ellipse 26% 28% at 50% 30%, hsl(var(--primary) / 0.08) 0%, transparent 72%)",
  ].join(", "),
};

export const RUNNING_SPRITE_GLOW_STYLE: React.CSSProperties = {
  backgroundImage: [
    "radial-gradient(ellipse 44% 44% at 50% 54%, hsl(var(--primary) / 0.18) 0%, hsl(var(--primary) / 0.11) 38%, transparent 76%)",
    "radial-gradient(ellipse 28% 30% at 35% 42%, hsl(var(--primary) / 0.11) 0%, transparent 72%)",
    "radial-gradient(ellipse 32% 34% at 66% 58%, hsl(var(--primary) / 0.1) 0%, transparent 74%)",
  ].join(", "),
  maskImage:
    "radial-gradient(ellipse 58% 56% at 50% 52%, black 0%, rgba(0,0,0,0.92) 38%, rgba(0,0,0,0.58) 64%, rgba(0,0,0,0.16) 84%, transparent 100%)",
  WebkitMaskImage:
    "radial-gradient(ellipse 58% 56% at 50% 52%, black 0%, rgba(0,0,0,0.92) 38%, rgba(0,0,0,0.58) 64%, rgba(0,0,0,0.16) 84%, transparent 100%)",
};

export function resolveXiaMainSidebarSurface(
  themeId: XiaThemePackId,
  shellTheme = "default",
) {
  if (shellTheme === "dream") {
    return "border-r bg-sidebar-background/70 backdrop-blur-2xl";
  }
  if (themeId === "pixel") {
    return "rounded-none border-r-2 bg-sidebar-background";
  }
  return "border-r bg-sidebar-background/82 backdrop-blur-2xl";
}

export function resolveSpriteCardLighting(
  sprite: Sprite,
  isDefault = false,
  variant?: "online",
): SpriteCardLighting {
  const defaultSpotlightClassName = isDefault
    ? "z-10 bottom-auto h-[92%] opacity-95 bg-[radial-gradient(ellipse_42%_18%_at_50%_0%,hsl(var(--background)/0.98)_0%,hsl(var(--primary)/0.62)_44%,transparent_78%),linear-gradient(180deg,hsl(var(--background)/0.92)_0%,hsl(var(--primary)/0.52)_32%,hsl(var(--primary)/0.22)_64%,transparent_88%)] [clip-path:polygon(22%_0,78%_0,98%_92%,2%_92%)] dark:bg-[radial-gradient(ellipse_42%_18%_at_50%_0%,hsl(var(--foreground)/0.72)_0%,hsl(var(--primary)/0.62)_44%,transparent_78%),linear-gradient(180deg,hsl(var(--foreground)/0.62)_0%,hsl(var(--primary)/0.56)_32%,hsl(var(--foreground)/0.20)_64%,transparent_88%)]"
    : undefined;
  const defaultSpriteGlowClassName = isDefault
    ? "scale-125 opacity-100 [--sprite-card-spotlight-top:var(--background)] dark:[--sprite-card-spotlight-top:var(--foreground)]"
    : undefined;
  const defaultSpriteGlowStyle: React.CSSProperties | undefined = isDefault
    ? {
        backgroundImage: [
          "radial-gradient(ellipse 32% 58% at 50% 18%, hsl(var(--sprite-card-spotlight-top) / 0.92) 0%, hsl(var(--primary) / 0.48) 46%, transparent 78%)",
          "radial-gradient(ellipse 58% 34% at 50% 74%, hsl(var(--primary) / 0.46) 0%, hsl(var(--primary) / 0.22) 42%, transparent 76%)",
          "radial-gradient(ellipse 40% 44% at 50% 48%, hsl(var(--sprite-card-spotlight-top) / 0.34) 0%, transparent 74%)",
        ].join(", "),
      }
    : undefined;
  const withDefaultSpotlight = (
    lighting: Omit<SpriteCardLighting, "spotlightClassName" | "spriteGlowClassName" | "spriteGlowStyle">,
  ): SpriteCardLighting => ({
    ...lighting,
    spotlightClassName: defaultSpotlightClassName,
    spriteGlowClassName: defaultSpriteGlowClassName,
    spriteGlowStyle: defaultSpriteGlowStyle,
  });

  if (variant === "online" || sprite.origin === "online") {
    return withDefaultSpotlight({
      cardClassName:
        "border-border/70 bg-card/95 shadow-[inset_0_1px_0_hsl(var(--foreground)/0.22),inset_0_16px_24px_hsl(var(--primary)/0.08),inset_0_-18px_24px_hsl(var(--background)/0.16)] hover:border-foreground/25 hover:bg-accent/20 hover:shadow-[inset_0_1px_0_hsl(var(--foreground)/0.26),inset_0_18px_26px_hsl(var(--primary)/0.11),inset_0_-20px_28px_hsl(var(--background)/0.20)]",
      primaryGlowClassName:
        "bg-[radial-gradient(circle_at_50%_0%,hsl(var(--foreground)/0.16),transparent_56%),radial-gradient(circle_at_50%_100%,hsl(var(--background)/0.20),transparent_62%)]",
      directionalWashClassName:
        "bg-[linear-gradient(180deg,hsl(var(--foreground)/0.14)_0%,hsl(var(--primary)/0.10)_44%,transparent_68%,hsl(var(--background)/0.18)_100%)]",
      rimGlowClassName:
        "bg-[radial-gradient(ellipse_at_50%_-8%,hsl(var(--foreground)/0.12),transparent_60%),radial-gradient(ellipse_at_50%_108%,hsl(var(--background)/0.16),transparent_64%)]",
    });
  }

  if (sprite.scope === "imported") {
    return withDefaultSpotlight({
      cardClassName:
        "border-primary/25 bg-card/95 shadow-[inset_0_1px_0_hsl(var(--background)/0.38),inset_0_16px_24px_hsl(var(--primary)/0.08),inset_0_-18px_24px_hsl(var(--background)/0.16)] hover:border-primary/45 hover:bg-primary/5 hover:shadow-[inset_0_1px_0_hsl(var(--background)/0.44),inset_0_18px_26px_hsl(var(--primary)/0.12),inset_0_-20px_28px_hsl(var(--background)/0.20)]",
      primaryGlowClassName:
        "bg-[radial-gradient(ellipse_at_50%_-10%,hsl(var(--background)/0.34),transparent_46%),radial-gradient(circle_at_50%_48%,hsl(var(--primary)/0.24),transparent_58%),radial-gradient(ellipse_at_50%_108%,hsl(var(--background)/0.22),transparent_64%)]",
      directionalWashClassName:
        "bg-[linear-gradient(180deg,hsl(var(--background)/0.20)_0%,hsl(var(--primary)/0.14)_48%,hsl(var(--background)/0.18)_100%)]",
      rimGlowClassName:
        "bg-[radial-gradient(ellipse_at_50%_108%,hsl(var(--background)/0.20),transparent_60%),radial-gradient(ellipse_at_50%_48%,hsl(var(--primary)/0.10),transparent_68%)]",
    });
  }

  return withDefaultSpotlight({
    cardClassName:
      "border-border/70 bg-card/95 shadow-[inset_0_1px_0_hsl(var(--background)/0.36),inset_0_16px_24px_hsl(var(--primary)/0.06),inset_0_-18px_24px_hsl(var(--foreground)/0.09)] hover:border-primary/25 hover:bg-accent/20 hover:shadow-[inset_0_1px_0_hsl(var(--background)/0.40),inset_0_18px_26px_hsl(var(--primary)/0.09),inset_0_-20px_28px_hsl(var(--foreground)/0.12)]",
    primaryGlowClassName:
      "bg-[radial-gradient(circle_at_50%_0%,hsl(var(--background)/0.18),transparent_56%),radial-gradient(circle_at_50%_52%,hsl(var(--primary)/0.12),transparent_62%)]",
    directionalWashClassName:
      "bg-[linear-gradient(180deg,hsl(var(--background)/0.16)_0%,hsl(var(--primary)/0.10)_44%,transparent_68%,hsl(var(--foreground)/0.10)_100%)]",
    rimGlowClassName:
      "bg-[radial-gradient(ellipse_at_50%_108%,hsl(var(--foreground)/0.12),transparent_60%)]",
  });
}
