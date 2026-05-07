import type * as React from "react";

export type DreamFMHiddenEngineStyle = React.CSSProperties & {
  [name: `--${string}`]: string | number | null | undefined;
};

export const DREAM_FM_LIST_SECTION_TITLE_CLASS =
  "mb-2 px-2 text-xs font-semibold text-sidebar-foreground/58";
export const DREAM_FM_LIST_ITEM_BUTTON_CLASS =
  "rounded-2xl border border-transparent bg-sidebar-background/30 text-sidebar-foreground/86 shadow-[inset_0_0_0_1px_hsl(var(--foreground)/0.045),inset_0_1px_0_hsl(var(--background)/0.16)] transition-[transform,background-color,color,box-shadow,border-color] duration-200 [transition-timing-function:cubic-bezier(0.2,_0.8,_0.2,_1)] hover:-translate-y-0.5 hover:bg-sidebar-background/56 hover:text-sidebar-foreground active:translate-y-0 active:scale-[0.985] data-[active=true]:bg-sidebar-primary/11 data-[active=true]:text-sidebar-foreground data-[active=true]:shadow-[inset_0_0_0_1px_hsl(var(--sidebar-primary)/0.20),0_12px_30px_-27px_hsl(var(--sidebar-primary)/0.46)]";
export const DREAM_FM_CONTROL_SURFACE_CLASS =
  "dream-fm-list-control-surface inline-flex shrink-0 items-center gap-0.5 overflow-hidden rounded-full p-0.5";
export const DREAM_FM_CONTROL_ICON_BUTTON_CLASS =
  "border-transparent bg-transparent text-sidebar-foreground/55 shadow-none transition-[transform,background-color,color,box-shadow,opacity] duration-200 ease-out hover:bg-sidebar-background/54 hover:text-sidebar-foreground active:scale-95 disabled:opacity-35";
export const DREAM_FM_PILL_BUTTON_CLASS =
  "rounded-full border-transparent bg-sidebar-background/30 text-sidebar-foreground/62 shadow-[inset_0_0_0_1px_hsl(var(--foreground)/0.055),inset_0_1px_0_hsl(var(--background)/0.18)] transition-[transform,background-color,color,box-shadow,opacity] duration-200 ease-out hover:-translate-y-0.5 hover:bg-sidebar-background/54 hover:text-sidebar-foreground active:translate-y-0 active:scale-[0.985] disabled:opacity-35";
export const DREAM_FM_NOTICE_CARD_CLASS =
  "dream-fm-list-control-surface dream-fm-list-control-surface-top whitespace-pre-wrap rounded-2xl px-3.5 py-3 text-sm leading-5 text-sidebar-foreground/64";

export const DREAM_FM_NOW_PLAYING_PANEL_CLASS =
  "h-[132px] w-[300px] overflow-hidden rounded-[22px] border border-[hsl(var(--tray-control-foreground)/0.18)] bg-[hsl(var(--tray-control-color))] text-[hsl(var(--tray-control-foreground))] shadow-none";
export const DREAM_FM_MINI_SIDE_CONTROL_CLASS =
  "flex h-7 w-7 shrink-0 items-center justify-center rounded-full text-[hsl(var(--tray-control-foreground)/0.74)] transition hover:bg-[hsl(var(--tray-control-foreground)/0.13)] hover:text-[hsl(var(--tray-control-foreground))] focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-[hsl(var(--tray-control-foreground)/0.34)] disabled:cursor-not-allowed disabled:opacity-35 [&>svg]:block [&>svg]:shrink-0";
export const DREAM_FM_MINI_PRIMARY_CONTROL_CLASS =
  "flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-[hsl(var(--tray-control-foreground))] text-[hsl(var(--tray-control-color))] shadow-[0_14px_28px_-20px_hsl(var(--tray-control-foreground)/0.86)] transition hover:scale-[1.03] focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-[hsl(var(--tray-control-foreground)/0.48)] disabled:cursor-not-allowed disabled:opacity-60 [&>svg]:block [&>svg]:shrink-0";

export const DREAM_FM_PLAYER_SURFACE_WIDTH_CLASS =
  "w-[min(18rem,62vw,42vh)] sm:w-[min(22rem,50vw,44vh)] xl:w-[min(25rem,34vw,46vh)]";
export const DREAM_FM_PLAYER_ICON_BUTTON_CLASS =
  "flex h-10 w-10 items-center justify-center rounded-full bg-transparent text-sidebar-foreground/55 shadow-none transition-[transform,background-color,color,box-shadow,opacity] duration-200 ease-out hover:scale-[1.04] hover:bg-sidebar-background/54 hover:text-sidebar-foreground active:scale-95 focus-visible:outline-none disabled:pointer-events-none disabled:opacity-40 data-[active=true]:bg-[hsl(var(--dream-shell-top)/0.58)] data-[active=true]:text-sidebar-primary data-[active=true]:shadow-[0_8px_22px_-18px_hsl(var(--foreground)/0.45)] dark:data-[active=true]:bg-white/10";
export const DREAM_FM_PLAYER_FOOTER_ICON_BUTTON_CLASS =
  "bg-transparent text-sidebar-foreground/55 shadow-none hover:bg-transparent hover:text-sidebar-primary data-[active=true]:bg-transparent data-[active=true]:shadow-none";
export const DREAM_FM_DROPDOWN_CONTENT_CLASS =
  "dream-fm-dropdown-content w-max min-w-fit max-w-[calc(100vw-2rem)]";
export const DREAM_FM_DROPDOWN_ITEM_CLASS =
  "dream-fm-dropdown-item w-full gap-2 whitespace-nowrap px-2 py-1.5 text-xs font-medium outline-none data-[disabled]:opacity-40";
export const DREAM_FM_DROPDOWN_ICON_SLOT_CLASS =
  "flex h-5 w-5 shrink-0 items-center justify-center rounded-full text-inherit opacity-70";

export const DREAM_FM_HIDDEN_ENGINE_STYLE: DreamFMHiddenEngineStyle = {
  position: "fixed",
  left: -10000,
  top: 0,
  width: 1,
  height: 1,
  opacity: 0,
  overflow: "hidden",
  pointerEvents: "none",
};
