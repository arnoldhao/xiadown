import {
AlertCircle,
CheckCircle2,
CircleHelp,
Copy,
Download,
DownloadCloud,
Eye,
Grid3X3,
Loader2,
Pencil,
RefreshCw,
RotateCcw,
Search,
Sparkles,
Store,
Trash2,
Upload,
X
} from "lucide-react";
import * as React from "react";

import { LocalSpriteGalleryCard } from "@/features/sprites/card";
import { resolveSpriteCardLighting } from "@/features/sprites/shared";
import { getXiaText } from "@/features/xiadown/shared";
import { cn } from "@/lib/utils";
import type { OnlineSpriteCatalogCategory,OnlineSpriteCatalogItem } from "@/shared/contracts/online-sprites";
import type { Sprite,SpriteManifest,SpriteSliceGrid } from "@/shared/contracts/sprites";
import { messageBus } from "@/shared/message/store";
import { SPRITE_ROW_BY_ANIMATION, type SpriteAnimation } from "@/shared/sprites/animation";
import {
Dialog,
DialogClose,
DialogContent,
DialogDescription,
DialogHeader,
DialogTitle,
} from "@/shared/ui/dialog";
import {
DropdownMenu,
DropdownMenuContent,
DropdownMenuItem,
DropdownMenuSeparator,
DropdownMenuTrigger,
} from "@/shared/ui/dropdown-menu";
import { SpriteDisplay } from "@/shared/ui/sprite-player";
import {
SPRITE_GALLERY_CARD_SIZE_CLASS,
} from "@/shared/styles/xiadown";
import {
DREAM_FM_CONTROL_ICON_BUTTON_CLASS,
DREAM_FM_CONTROL_SURFACE_CLASS,
DREAM_FM_PILL_BUTTON_CLASS,
} from "@/shared/styles/dreamfm";
import {
buildAssetPreviewURL
} from "@/shared/utils/resourceHelpers";
import { formatBytes } from "@/shared/utils/formatBytes";

type XiaText = ReturnType<typeof getXiaText>;

export type SpriteStudioNavigation = {
  action: "gallery" | "edit" | "import";
  spriteId?: string;
  nonce: number;
};

type StudioTab = "edit" | "preview";
type GenerationGuideTab = "conversation" | "code";
type SaveState = "idle" | "saving" | "saved" | "error";
export type OnlineSpriteInstallState = {
  status: "downloading" | "importing" | "installed" | "error";
  progress: number;
  error?: string;
};
type DragTarget = { axis: "x" | "y"; index: number; pointerId: number };
type SpriteMetadataDraft = {
  name: string;
  authorDisplayName: string;
  version: string;
  description: string;
};

const SPRITE_ACTIONS: SpriteAnimation[] = [
  "greeting",
  "snoring",
  "upset",
  "celebrate",
  "seeking",
  "embarrassed",
  "working",
  "listeningToMusic",
];
const MIN_SLICE_GAP_PX = 4;
const ONLINE_SECTION_COLLAPSED_ITEM_COUNT = 8;
const SPRITE_MAGENTA_KEY_RED_MIN = 250;
const SPRITE_MAGENTA_KEY_GREEN_MAX = 5;
const SPRITE_MAGENTA_KEY_BLUE_MIN = 250;
const imageCache = new Map<string, Promise<HTMLImageElement | null>>();
const SPRITE_STUDIO_TAB_SURFACE_CLASS = cn(
  DREAM_FM_CONTROL_SURFACE_CLASS,
  "dream-fm-list-control-surface-top min-w-0 max-w-full rounded-[18px] text-sidebar-foreground transition-[width,background-color,box-shadow] duration-200 ease-out [--sprite-segment-tab-radius:16px]",
);
const IOS_TOOLBAR_CLASS =
  cn(
    SPRITE_STUDIO_TAB_SURFACE_CLASS,
    "min-w-0 max-w-full",
  );
const IOS_DIVIDER_CLASS = "mx-0.5 h-4 w-px bg-sidebar-foreground/12";
const IOS_MENU_CONTENT_CLASS =
  "w-max min-w-fit max-w-[calc(100vw-2rem)] rounded-[20px] border border-white/25 bg-background/88 p-1.5 shadow-[0_22px_60px_-34px_hsl(var(--foreground)/0.55),inset_0_1px_0_hsl(var(--background)/0.55)] backdrop-blur-2xl dark:border-white/10";
const IOS_MENU_ITEM_CLASS =
  "w-full gap-2 whitespace-nowrap rounded-2xl py-2 pr-3 pl-3 text-sm outline-none transition data-[highlighted]:bg-foreground/7 data-[disabled]:opacity-40";
const IOS_MENU_ICON_SLOT_CLASS =
  "flex h-5 w-5 shrink-0 items-center justify-center rounded-full text-muted-foreground";
const IOS_DROPDOWN_PANEL_CLASS =
  "dream-fm-floating-surface rounded-[24px] p-3 text-sidebar-foreground shadow-[0_24px_72px_-42px_hsl(var(--foreground)/0.58),inset_0_0_0_1px_hsl(var(--foreground)/0.07),inset_0_1px_0_hsl(var(--dream-shell-top)/0.18)]";
const IOS_PANEL_CLASS =
  "rounded-[28px] border border-white/25 bg-background/58 shadow-[0_22px_60px_-42px_hsl(var(--foreground)/0.46),inset_0_1px_0_hsl(var(--background)/0.56)] backdrop-blur-2xl dark:border-white/10";
const IOS_INSET_PANEL_CLASS =
  "rounded-[22px] border border-white/20 bg-background/48 shadow-[inset_0_1px_0_hsl(var(--background)/0.48)] backdrop-blur-xl dark:border-white/10";
const IOS_FORM_CONTROL_CLASS =
  "app-motion-color [color-scheme:light] border border-[hsl(var(--foreground)/0.14)] bg-[hsl(var(--background)/0.76)] text-foreground caret-sidebar-primary shadow-[inset_0_1px_0_hsl(var(--background)/0.28),inset_0_0_0_1px_hsl(var(--foreground)/0.04)] outline-none transition placeholder:text-muted-foreground/70 focus:border-sidebar-primary/45 focus:bg-[hsl(var(--background)/0.90)] disabled:cursor-not-allowed disabled:bg-[hsl(var(--background)/0.58)] disabled:text-foreground/68 disabled:placeholder:text-muted-foreground/50 dark:[color-scheme:dark] dark:border-white/12 dark:!bg-[hsl(var(--sidebar-background)/0.78)] dark:!text-[hsl(var(--sidebar-foreground))] dark:placeholder:text-[hsl(var(--sidebar-foreground)/0.48)] dark:focus:!bg-[hsl(var(--sidebar-background)/0.90)] dark:disabled:!bg-[hsl(var(--sidebar-background)/0.62)] dark:disabled:!text-[hsl(var(--sidebar-foreground)/0.72)]";
const IOS_TEXTAREA_CLASS =
  cn(IOS_FORM_CONTROL_CLASS, "min-h-[5rem] w-full resize-none rounded-[18px] px-3 py-2 text-xs font-medium");
const IOS_DANGER_BUTTON_CLASS =
  "rounded-full !border-transparent !bg-destructive !text-destructive-foreground shadow-[0_18px_40px_-24px_hsl(var(--destructive)/0.72),inset_0_1px_0_hsl(var(--background)/0.18)] transition-[transform,background-color,color,box-shadow,opacity] duration-200 ease-out hover:-translate-y-0.5 hover:!bg-destructive/90 hover:!text-destructive-foreground active:translate-y-0 active:scale-[0.985] focus-visible:ring-2 focus-visible:ring-destructive/28 disabled:opacity-35";

type IOSButtonProps = React.ButtonHTMLAttributes<HTMLButtonElement> & {
  size?: "compact" | "default";
  variant?: "outline" | "ghost" | "destructive";
  tone?: "primary" | "secondary" | "plain" | "danger";
};

const IOSButton = React.forwardRef<HTMLButtonElement, IOSButtonProps>(
  ({ className, size = "compact", variant, tone, type = "button", ...props }, ref) => {
    const resolvedTone =
      tone ??
      (variant === "destructive"
        ? "danger"
        : variant === "ghost"
          ? "plain"
          : variant === "outline"
            ? "secondary"
            : "primary");
    return (
      <button
        ref={ref}
        type={type}
        className={cn(
          "app-motion-color inline-flex min-w-0 shrink-0 items-center justify-center gap-1.5 border text-xs font-semibold outline-none focus-visible:outline-none disabled:pointer-events-none",
          size === "compact" ? "h-8 px-3" : "h-9 px-4",
          resolvedTone === "primary" &&
            "rounded-full border-transparent bg-sidebar-primary text-sidebar-primary-foreground shadow-[0_18px_40px_-24px_hsl(var(--sidebar-primary)/0.68),inset_0_1px_0_hsl(var(--background)/0.18)] transition-[transform,background-color,color,box-shadow,opacity] duration-200 ease-out hover:-translate-y-0.5 hover:bg-sidebar-primary/90 hover:text-sidebar-primary-foreground active:translate-y-0 active:scale-[0.985] disabled:opacity-35",
          resolvedTone === "secondary" &&
            DREAM_FM_PILL_BUTTON_CLASS,
          resolvedTone === "plain" &&
            cn(DREAM_FM_CONTROL_ICON_BUTTON_CLASS, "rounded-xl"),
          resolvedTone === "danger" &&
            IOS_DANGER_BUTTON_CLASS,
          className,
        )}
        {...props}
      />
    );
  },
);
IOSButton.displayName = "IOSButton";

function iosSegmentClass(active: boolean) {
  return cn(
    "flex h-8 min-w-0 items-center justify-center overflow-hidden text-xs font-medium text-sidebar-foreground/55 outline-none transition-[width,padding,background-color,color,box-shadow,transform] duration-200 ease-out active:scale-95 [border-radius:var(--sprite-segment-tab-radius,16px)]",
    "hover:scale-[1.03] hover:bg-sidebar-background/54 hover:text-sidebar-primary focus-visible:outline-none",
    "data-[active=true]:bg-sidebar-accent data-[active=true]:text-sidebar-primary data-[active=true]:shadow-sm",
    active &&
      "bg-sidebar-accent text-sidebar-primary shadow-sm",
  );
}

function spriteStudioGroupButtonClass(active = false) {
  return cn(
    "h-8 min-w-0 overflow-hidden border-0 bg-transparent px-2 shadow-none [border-radius:var(--sprite-segment-tab-radius,16px)]",
    "text-sidebar-foreground/55 hover:bg-sidebar-background/54 hover:text-sidebar-primary",
    "focus-visible:outline-none",
    active && "bg-sidebar-accent text-sidebar-primary shadow-sm",
  );
}

const SPRITE_STUDIO_TAB_LABEL_CLASS =
  "block min-w-0 truncate text-xs font-medium transition-[margin,max-width,opacity,transform] duration-200 ease-out";
const SPRITE_STUDIO_SEARCH_FIELD_CLASS =
  "dream-fm-list-control-surface dream-fm-list-control-surface-top flex h-9 min-w-0 items-center gap-2 rounded-2xl px-3 text-sidebar-foreground transition-[width,box-shadow,border-color,background-color] duration-200 ease-out";

const IOSInput = React.forwardRef<HTMLInputElement, React.ComponentPropsWithoutRef<"input">>(
  ({ className, type, ...props }, ref) => (
    <input
      ref={ref}
      type={type}
      className={cn(
        IOS_FORM_CONTROL_CLASS,
        "h-8 w-full rounded-full px-3 text-xs font-medium",
        className,
      )}
      {...props}
    />
  ),
);
IOSInput.displayName = "IOSInput";

const IOSSelect = React.forwardRef<HTMLSelectElement, React.SelectHTMLAttributes<HTMLSelectElement>>(
  ({ className, ...props }, ref) => (
    <select
      ref={ref}
      className={cn(
        IOS_FORM_CONTROL_CLASS,
        "h-8 rounded-full px-3 text-xs font-medium [&>option]:bg-background [&>option]:text-foreground dark:[&>option]:bg-sidebar-background dark:[&>option]:text-sidebar-foreground",
        className,
      )}
      {...props}
    />
  ),
);
IOSSelect.displayName = "IOSSelect";

async function copyTextToClipboard(value: string) {
  const text = value.trim();
  if (!text) {
    return;
  }
  if (navigator.clipboard?.writeText) {
    try {
      await navigator.clipboard.writeText(text);
      return;
    } catch {
      // Embedded WebViews can expose clipboard but still reject writes.
    }
  }
  const textarea = document.createElement("textarea");
  textarea.value = text;
  textarea.setAttribute("readonly", "true");
  textarea.style.position = "fixed";
  textarea.style.left = "-10000px";
  textarea.style.top = "0";
  document.body.appendChild(textarea);
  textarea.select();
  try {
    const clipboardCommand = ["co", "py"].join("");
    if (!document.execCommand(clipboardCommand)) {
      throw new Error("clipboard command failed");
    }
  } finally {
    document.body.removeChild(textarea);
  }
}

export function SpriteGallery(props: {
  text: XiaText;
  sprites: Sprite[];
  httpBaseURL: string;
  isLoading: boolean;
  importPending: boolean;
  defaultSpriteId: string;
  defaultPending: boolean;
  exportPending: boolean;
  deletePending: boolean;
  onImportSprite: () => void;
  onOpenSprite: (spriteId: string) => void;
  onSetDefaultSprite: (sprite: Sprite) => Promise<void>;
  onExportSprite: (sprite: Sprite) => Promise<void>;
  onDeleteSprite: (sprite: Sprite) => Promise<void>;
}) {
  const [contextMenuTarget, setContextMenuTarget] = React.useState<{
    spriteId: string;
    x: number;
    y: number;
  } | null>(null);
  const [deleteConfirmSpriteId, setDeleteConfirmSpriteId] = React.useState("");
  const [deleteConfirmError, setDeleteConfirmError] = React.useState("");
  const [generationGuideOpen, setGenerationGuideOpen] = React.useState(false);
  const [generationGuideTab, setGenerationGuideTab] = React.useState<GenerationGuideTab>("conversation");
  const [expanded, setExpanded] = React.useState(false);
  const contextMenuSprite = React.useMemo(
    () => props.sprites.find((sprite) => sprite.id === contextMenuTarget?.spriteId) ?? null,
    [contextMenuTarget?.spriteId, props.sprites],
  );
  const deleteConfirmSprite = React.useMemo(
    () => props.sprites.find((sprite) => sprite.id === deleteConfirmSpriteId) ?? null,
    [deleteConfirmSpriteId, props.sprites],
  );
  const canExportContextSprite = Boolean(contextMenuSprite) && !props.exportPending;
  const canSetDefaultContextSprite =
    Boolean(contextMenuSprite && contextMenuSprite.id !== props.defaultSpriteId) && !props.defaultPending;
  const canDeleteContextSprite =
    Boolean(contextMenuSprite && contextMenuSprite.scope === "imported") && !props.deletePending;
  const generationGuideText = props.text.spriteStudio.generationGuide;
  const activeGenerationGuideText =
    generationGuideTab === "conversation"
      ? generationGuideText.prompts.conversation
      : generationGuideText.prompts.code;
  const generationGuideTabs: Array<{ id: GenerationGuideTab; label: string }> = [
    { id: "conversation", label: generationGuideText.tabs.conversation },
    { id: "code", label: generationGuideText.tabs.code },
  ];
  const visibleSprites = expanded ? props.sprites : props.sprites.slice(0, 24);
  const hasMoreSprites = props.sprites.length > visibleSprites.length;

  React.useEffect(() => {
    setExpanded(false);
  }, [props.sprites.length]);

  const openSpriteContextMenu = React.useCallback((event: React.MouseEvent, sprite: Sprite) => {
    event.preventDefault();
    event.stopPropagation();
    setContextMenuTarget({
      spriteId: sprite.id,
      x: event.clientX,
      y: event.clientY,
    });
  }, []);

  const handleViewContextSprite = React.useCallback(() => {
    if (!contextMenuSprite) {
      return;
    }
    props.onOpenSprite(contextMenuSprite.id);
    setContextMenuTarget(null);
  }, [contextMenuSprite, props]);

  const handleSetDefaultContextSprite = React.useCallback(async () => {
    if (!contextMenuSprite || contextMenuSprite.id === props.defaultSpriteId || props.defaultPending) {
      return;
    }
    setContextMenuTarget(null);
    await props.onSetDefaultSprite(contextMenuSprite);
  }, [contextMenuSprite, props]);

  const handleExportContextSprite = React.useCallback(async () => {
    if (!contextMenuSprite || props.exportPending) {
      return;
    }
    setContextMenuTarget(null);
    await props.onExportSprite(contextMenuSprite);
  }, [contextMenuSprite, props]);

  const handleRequestDeleteContextSprite = React.useCallback(() => {
    if (!contextMenuSprite || contextMenuSprite.scope !== "imported") {
      return;
    }
    setContextMenuTarget(null);
    setDeleteConfirmError("");
    setDeleteConfirmSpriteId(contextMenuSprite.id);
  }, [contextMenuSprite]);

  const handleDeleteDialogOpenChange = React.useCallback(
    (open: boolean) => {
      if (props.deletePending) {
        return;
      }
      if (!open) {
        setDeleteConfirmSpriteId("");
        setDeleteConfirmError("");
      }
    },
    [props.deletePending],
  );

  const handleConfirmDeleteSprite = React.useCallback(async () => {
    if (!deleteConfirmSprite || deleteConfirmSprite.scope !== "imported") {
      return;
    }
    setDeleteConfirmError("");
    try {
      await props.onDeleteSprite(deleteConfirmSprite);
      setDeleteConfirmSpriteId("");
    } catch (error) {
      setDeleteConfirmError(resolveSpriteError(error));
    }
  }, [deleteConfirmSprite, props]);

  const handleCopyGenerationGuide = React.useCallback(async () => {
    try {
      await copyTextToClipboard(activeGenerationGuideText);
      messageBus.publishToast({
        id: "sprite-generation-guide-copied",
        intent: "success",
        title: generationGuideText.clipboardSucceeded,
        source: "xiadown.spriteStudio",
        autoCloseMs: 2200,
      });
    } catch (error) {
      messageBus.publishToast({
        id: "sprite-generation-guide-clipboard-failed",
        intent: "danger",
        title: generationGuideText.clipboardFailed,
        description: resolveSpriteError(error),
        source: "xiadown.spriteStudio",
      });
    }
  }, [activeGenerationGuideText, generationGuideText.clipboardSucceeded, generationGuideText.clipboardFailed]);

  if (props.isLoading) {
    return (
      <div className="flex h-full min-h-[20rem] items-center justify-center text-muted-foreground">
        <Loader2 className="h-5 w-5 animate-spin" />
      </div>
    );
  }

  return (
    <>
      <div className="w-full min-w-0">
        <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
          <div className="flex min-w-0 items-center gap-2 text-sm font-semibold text-foreground">
            <Sparkles className="h-4 w-4 shrink-0 text-primary" />
            <span className="truncate">{props.text.spriteStudio.localGallery}</span>
          </div>
          <div className="flex min-w-0 flex-wrap items-center justify-end gap-2">
            <IOSButton
              type="button"
              size="compact"
              variant="outline"
              onClick={() => setGenerationGuideOpen(true)}
            >
              <CircleHelp className="h-4 w-4" />
              {generationGuideText.action}
            </IOSButton>
            <IOSButton
              type="button"
              size="compact"
              variant="outline"
              onClick={() => props.onImportSprite()}
              disabled={props.importPending}
            >
              {props.importPending ? (
                <Loader2 className="h-4 w-4 animate-spin" />
              ) : (
                <Upload className="h-4 w-4" />
              )}
              {props.text.spriteStudio.importAction}
            </IOSButton>
          </div>
        </div>
        {props.sprites.length === 0 ? (
          <div className={cn(IOS_INSET_PANEL_CLASS, "flex min-h-[10rem] items-center justify-center px-4 py-5 text-center text-sm font-medium text-muted-foreground")}>
            {props.text.spriteStudio.empty}
          </div>
        ) : (
          <div className="flex flex-wrap gap-4">
            {visibleSprites.map((sprite) => (
              <LocalSpriteGalleryCard
                key={sprite.id}
                sprite={sprite}
                imageUrl={buildAssetPreviewURL(props.httpBaseURL, sprite.spritePath, sprite.updatedAt)}
                isDefault={sprite.id === props.defaultSpriteId}
                onClick={() => props.onOpenSprite(sprite.id)}
                onContextMenu={(event) => openSpriteContextMenu(event, sprite)}
              />
            ))}
          </div>
        )}
        {hasMoreSprites || expanded ? (
          <div className="mt-4 flex justify-center">
            <IOSButton
              type="button"
              size="compact"
              variant="outline"
              onClick={() => setExpanded((current) => !current)}
            >
              {expanded ? props.text.settings.sprites.showLess : props.text.settings.sprites.showMore}
            </IOSButton>
          </div>
        ) : null}
      </div>

      <Dialog open={generationGuideOpen} onOpenChange={setGenerationGuideOpen}>
        <DialogContent className={cn(IOS_PANEL_CLASS, "grid h-[min(36rem,calc(100vh-6.5rem))] w-[min(58rem,calc(100vw-2rem))] max-w-none grid-rows-[auto_minmax(0,1fr)] gap-3 overflow-hidden p-4")}>
          <DialogHeader className="min-w-0">
            <DialogTitle className="overflow-hidden break-words pr-6 text-left leading-[1.35] [display:-webkit-box] [-webkit-box-orient:vertical] [-webkit-line-clamp:2]">
              {generationGuideText.title}
            </DialogTitle>
            <DialogDescription className="overflow-hidden break-words text-left leading-relaxed [display:-webkit-box] [-webkit-box-orient:vertical] [-webkit-line-clamp:2]">
              {generationGuideText.description}
            </DialogDescription>
          </DialogHeader>

          <div className={cn(IOS_INSET_PANEL_CLASS, "flex min-h-0 min-w-0 flex-col overflow-hidden")}>
            <div className="flex shrink-0 flex-wrap items-center justify-between gap-2 border-b border-foreground/10 px-3 py-2">
              <div
                role="tablist"
                aria-label={generationGuideText.title}
                className={cn(SPRITE_STUDIO_TAB_SURFACE_CLASS, "h-9")}
              >
                {generationGuideTabs.map((tab) => (
                  <button
                    key={tab.id}
                    type="button"
                    role="tab"
                    aria-selected={generationGuideTab === tab.id}
                    data-active={generationGuideTab === tab.id ? "true" : "false"}
                    onClick={() => setGenerationGuideTab(tab.id)}
                    className={cn(
                      "min-w-[6rem] px-3",
                      iosSegmentClass(generationGuideTab === tab.id),
                    )}
                  >
                    <span className={SPRITE_STUDIO_TAB_LABEL_CLASS}>{tab.label}</span>
                  </button>
                ))}
              </div>
              <IOSButton
                type="button"
                size="compact"
                variant="outline"
                className="shrink-0"
                onClick={() => void handleCopyGenerationGuide()}
              >
                <Copy className="h-4 w-4" />
                {generationGuideText.clipboardAction}
              </IOSButton>
            </div>
            <pre className="min-h-0 flex-1 select-text overflow-auto whitespace-pre-wrap break-words bg-background/42 p-3 font-mono text-[11px] leading-relaxed text-foreground">{activeGenerationGuideText}</pre>
          </div>
        </DialogContent>
      </Dialog>

      <DropdownMenu
        open={Boolean(contextMenuTarget)}
        onOpenChange={(open) => {
          if (!open) {
            setContextMenuTarget(null);
          }
        }}
      >
        {contextMenuTarget ? (
          <DropdownMenuTrigger asChild>
            <button
              type="button"
              aria-hidden="true"
              tabIndex={-1}
              className="fixed z-50 h-px w-px opacity-0 outline-none"
              style={{
                left: contextMenuTarget.x,
                top: contextMenuTarget.y,
              }}
            />
          </DropdownMenuTrigger>
        ) : null}
        <DropdownMenuContent
          side="bottom"
          align="start"
          sideOffset={2}
          className={IOS_MENU_CONTENT_CLASS}
        >
          <div className="p-1">
            <DropdownMenuItem
              className={IOS_MENU_ITEM_CLASS}
              disabled={!canSetDefaultContextSprite}
              onSelect={() => void handleSetDefaultContextSprite()}
            >
              <div className={IOS_MENU_ICON_SLOT_CLASS}>
                {props.defaultPending ? (
                  <Loader2 className="h-4 w-4 animate-spin text-muted-foreground" />
                ) : (
                  <CheckCircle2 className="h-4 w-4 text-muted-foreground" />
                )}
              </div>
              <span className="truncate font-medium text-foreground">
                {props.text.settings.sprites.setDefaultAction}
              </span>
            </DropdownMenuItem>
            <DropdownMenuItem
              className={IOS_MENU_ITEM_CLASS}
              disabled={!contextMenuSprite}
              onSelect={handleViewContextSprite}
            >
              <div className={IOS_MENU_ICON_SLOT_CLASS}>
                <Eye className="h-4 w-4 text-muted-foreground" />
              </div>
              <span className="truncate font-medium text-foreground">{props.text.actions.view}</span>
            </DropdownMenuItem>
            <DropdownMenuItem
              className={IOS_MENU_ITEM_CLASS}
              disabled={!canExportContextSprite}
              onSelect={() => void handleExportContextSprite()}
            >
              <div className={IOS_MENU_ICON_SLOT_CLASS}>
                {props.exportPending ? (
                  <Loader2 className="h-4 w-4 animate-spin text-muted-foreground" />
                ) : (
                  <Download className="h-4 w-4 text-muted-foreground" />
                )}
              </div>
              <span className="truncate font-medium text-foreground">
                {props.text.settings.sprites.exportAction}
              </span>
            </DropdownMenuItem>
            <DropdownMenuSeparator className="my-1 bg-foreground/10" />
            <DropdownMenuItem
              className={cn(IOS_MENU_ITEM_CLASS, "text-destructive focus:text-destructive")}
              disabled={!canDeleteContextSprite}
              onSelect={handleRequestDeleteContextSprite}
            >
              <div className={IOS_MENU_ICON_SLOT_CLASS}>
                <Trash2 className="h-4 w-4 text-destructive" />
              </div>
              <span className="truncate font-medium">{props.text.settings.sprites.deleteAction}</span>
            </DropdownMenuItem>
          </div>
        </DropdownMenuContent>
      </DropdownMenu>

      <Dialog open={Boolean(deleteConfirmSpriteId)} onOpenChange={handleDeleteDialogOpenChange}>
        <DialogContent className={cn(IOS_PANEL_CLASS, "grid h-[min(14rem,calc(100vh-2rem))] w-[min(24rem,calc(100vw-2rem))] max-w-none grid-rows-[auto_minmax(0,1fr)_auto] gap-3 overflow-hidden p-4")}>
          <DialogHeader className="min-w-0">
            <DialogTitle className="overflow-hidden break-words pr-6 text-left leading-[1.35] [display:-webkit-box] [-webkit-box-orient:vertical] [-webkit-line-clamp:2]">
              {props.text.settings.sprites.deleteTitle}
            </DialogTitle>
            <DialogDescription className="overflow-hidden break-words text-left leading-5 [display:-webkit-box] [-webkit-box-orient:vertical] [-webkit-line-clamp:3]">
              {formatDeleteMessage(
                props.text.settings.sprites.deleteMessage,
                deleteConfirmSprite?.name ?? "",
              )}
            </DialogDescription>
          </DialogHeader>

          <div className="min-h-0 overflow-hidden">
            {deleteConfirmError ? (
              <div className="overflow-hidden break-words text-xs leading-5 text-destructive [display:-webkit-box] [-webkit-box-orient:vertical] [-webkit-line-clamp:2]">
                {deleteConfirmError}
              </div>
            ) : null}
          </div>

          <div className="flex flex-nowrap items-center justify-between gap-2">
            <DialogClose asChild>
              <IOSButton size="compact" variant="outline" disabled={props.deletePending}>
                {props.text.actions.cancelDialog}
              </IOSButton>
            </DialogClose>
            <IOSButton
              size="compact"
              variant="destructive"
              disabled={!deleteConfirmSprite || props.deletePending}
              onClick={() => void handleConfirmDeleteSprite()}
            >
              {props.deletePending ? <Loader2 className="h-4 w-4 animate-spin" /> : null}
              {props.text.settings.sprites.deleteAction}
            </IOSButton>
          </div>
        </DialogContent>
      </Dialog>
    </>
  );
}

export function OnlineSpriteGallery(props: {
  text: XiaText;
  items: OnlineSpriteCatalogItem[];
  categories: OnlineSpriteCatalogCategory[];
  installedSpriteIds: Set<string>;
  defaultSpriteId: string;
  installStates: Record<string, OnlineSpriteInstallState | undefined>;
  isLoading: boolean;
  isFetching: boolean;
  hasError: boolean;
  onRefresh: () => void;
  onInstallSprite: (item: OnlineSpriteCatalogItem) => Promise<void>;
}) {
  const [query, setQuery] = React.useState("");
  const [expandedSectionIds, setExpandedSectionIds] = React.useState<Set<string>>(() => new Set());
  const [selectedItemId, setSelectedItemId] = React.useState("");
  const [contextMenuItemId, setContextMenuItemId] = React.useState("");
  const [contextMenuPoint, setContextMenuPoint] = React.useState({ x: 0, y: 0 });
  const normalizedQuery = query.trim().toLowerCase();
  const catalogSections = React.useMemo(
    () => buildOnlineCatalogSections(props.items, props.categories),
    [props.categories, props.items],
  );
  const selectedItem = React.useMemo(
    () => props.items.find((item) => item.id === selectedItemId) ?? null,
    [props.items, selectedItemId],
  );
  const contextMenuItem = React.useMemo(
    () => props.items.find((item) => item.id === contextMenuItemId) ?? null,
    [contextMenuItemId, props.items],
  );
  const searchItems = React.useMemo(
    () => (normalizedQuery ? filterOnlineCatalogItems(props.items, normalizedQuery) : []),
    [normalizedQuery, props.items],
  );
  const sections = normalizedQuery
    ? [{ id: "search", label: props.text.spriteStudio.onlineSearchResults, items: searchItems }]
    : catalogSections;
  const visibleSections = sections.filter((section) => section.items.length > 0);

  React.useEffect(() => {
    setExpandedSectionIds(new Set());
  }, [normalizedQuery, props.categories.length, props.items.length]);

  React.useEffect(() => {
    if (selectedItemId && !selectedItem) {
      setSelectedItemId("");
    }
  }, [selectedItem, selectedItemId]);

  React.useEffect(() => {
    if (contextMenuItemId && !contextMenuItem) {
      setContextMenuItemId("");
    }
  }, [contextMenuItem, contextMenuItemId]);

  const openOnlineSpriteContextMenu = React.useCallback((event: React.MouseEvent, item: OnlineSpriteCatalogItem) => {
    event.preventDefault();
    setContextMenuPoint({ x: event.clientX, y: event.clientY });
    setContextMenuItemId(item.id);
  }, []);

  const toggleSectionExpanded = React.useCallback((sectionId: string) => {
    setExpandedSectionIds((current) => {
      const next = new Set(current);
      if (next.has(sectionId)) {
        next.delete(sectionId);
      } else {
        next.add(sectionId);
      }
      return next;
    });
  }, []);

  if (props.isLoading) {
    return (
      <div className="flex h-full min-h-[20rem] items-center justify-center text-muted-foreground">
        <Loader2 className="h-5 w-5 animate-spin" />
      </div>
    );
  }

  return (
    <div className="w-full min-w-0 space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex min-w-0 items-center gap-2 text-sm font-semibold text-foreground">
          <Store className="h-4 w-4 shrink-0 text-primary" />
          <span className="truncate">{props.text.spriteStudio.onlineGallery}</span>
        </div>
        <div className="flex min-w-0 flex-1 flex-wrap items-center justify-end gap-2 sm:flex-none">
          <div className={cn(SPRITE_STUDIO_SEARCH_FIELD_CLASS, "min-w-[11rem] flex-1 sm:w-[15rem] sm:flex-none")}>
            <Search className="h-4 w-4 shrink-0 text-sidebar-foreground/55" />
            <input
              value={query}
              onChange={(event) => setQuery(event.target.value)}
              placeholder={props.text.spriteStudio.searchOnline}
              className="app-motion-color h-auto min-w-0 flex-1 border-0 bg-transparent px-0 text-xs font-medium text-sidebar-foreground shadow-none outline-none placeholder:text-sidebar-foreground/45"
            />
            <span
              className={cn(
                "block shrink-0 overflow-hidden transition-[width,opacity,transform] duration-200 ease-out",
                query ? "w-5 translate-x-0 opacity-100" : "w-0 -translate-x-1 opacity-0",
              )}
            >
              <button
                type="button"
                aria-label={props.text.actions.clear}
                onClick={() => setQuery("")}
                disabled={!query}
                tabIndex={query ? 0 : -1}
                className="flex h-5 w-5 items-center justify-center rounded-full text-sidebar-foreground/55 transition hover:bg-sidebar-background/54 hover:text-sidebar-foreground focus-visible:outline-none disabled:pointer-events-none"
              >
                <X className="h-3.5 w-3.5" />
              </button>
            </span>
          </div>
          <IOSButton
            type="button"
            size="compact"
            variant="outline"
            className={cn("h-9 shrink-0", DREAM_FM_PILL_BUTTON_CLASS)}
            onClick={props.onRefresh}
            disabled={props.isFetching}
          >
            {props.isFetching ? <Loader2 className="h-4 w-4 animate-spin" /> : <RefreshCw className="h-4 w-4" />}
            {props.text.spriteStudio.refreshCatalog}
          </IOSButton>
        </div>
      </div>

      {props.hasError && props.items.length === 0 ? (
        <OnlineSpriteCatalogErrorState
          text={props.text}
          pending={props.isFetching}
          onRefresh={props.onRefresh}
        />
      ) : visibleSections.length === 0 ? (
        <OnlineSpriteEmptyState message={props.text.spriteStudio.onlineEmpty} />
      ) : (
        <div className="space-y-6">
          {visibleSections.map((section) => {
            const expanded = expandedSectionIds.has(section.id);
            const visibleItems = expanded
              ? section.items
              : section.items.slice(0, ONLINE_SECTION_COLLAPSED_ITEM_COUNT);
            const hasMore = section.items.length > visibleItems.length;
            return (
              <section key={section.id} className="min-w-0 space-y-3">
                <div className="flex min-w-0 items-center justify-between gap-3">
                  <div className="flex min-w-0 items-center gap-2">
                    <div className="min-w-0 truncate text-xs font-semibold text-muted-foreground">
                      {section.label}
                    </div>
                    <div className="shrink-0 rounded-md bg-muted px-1.5 py-0.5 text-[10px] font-medium text-muted-foreground">
                      {formatCountText(props.text.spriteStudio.onlineCount, section.items.length)}
                    </div>
                  </div>
                  {hasMore || expanded ? (
                    <IOSButton
                      type="button"
                      size="compact"
                      variant="outline"
                      onClick={() => toggleSectionExpanded(section.id)}
                    >
                      {expanded ? props.text.settings.sprites.showLess : props.text.settings.sprites.showMore}
                    </IOSButton>
                  ) : null}
                </div>
                <div className="flex flex-wrap gap-4">
                  {visibleItems.map((item) => (
                    <OnlineSpriteCard
                      key={item.id}
                      text={props.text}
                      item={item}
                      installed={isOnlineSpriteInstalled(
                        item,
                        props.installedSpriteIds,
                        props.installStates,
                      )}
                      isDefault={item.id === props.defaultSpriteId}
                      installState={props.installStates[item.id]}
                      onOpenSprite={() => setSelectedItemId(item.id)}
                      onContextMenu={(event) => openOnlineSpriteContextMenu(event, item)}
                    />
                  ))}
                </div>
              </section>
            );
          })}
        </div>
      )}
      <OnlineSpriteDetailDialog
        text={props.text}
        item={selectedItem}
        installed={
          selectedItem
            ? isOnlineSpriteInstalled(
                selectedItem,
                props.installedSpriteIds,
                props.installStates,
              )
            : false
        }
        isDefault={selectedItem?.id === props.defaultSpriteId}
        installState={selectedItem ? props.installStates[selectedItem.id] : undefined}
        onOpenChange={(open) => {
          if (!open) {
            setSelectedItemId("");
          }
        }}
        onInstallSprite={props.onInstallSprite}
      />
      <DropdownMenu
        open={Boolean(contextMenuItem)}
        onOpenChange={(open) => {
          if (!open) {
            setContextMenuItemId("");
          }
        }}
      >
        {contextMenuItem ? (
          <DropdownMenuTrigger asChild>
            <button
              type="button"
              aria-hidden="true"
              className="fixed h-px w-px opacity-0"
              style={{ left: contextMenuPoint.x, top: contextMenuPoint.y }}
            />
          </DropdownMenuTrigger>
        ) : null}
        <DropdownMenuContent
          side="bottom"
          align="start"
          sideOffset={2}
          className={IOS_MENU_CONTENT_CLASS}
        >
          <div className="p-1">
            <DropdownMenuItem
              className={IOS_MENU_ITEM_CLASS}
              disabled={!contextMenuItem}
              onSelect={() => {
                if (contextMenuItem) {
                  setSelectedItemId(contextMenuItem.id);
                }
                setContextMenuItemId("");
              }}
            >
              <div className={IOS_MENU_ICON_SLOT_CLASS}>
                <Eye className="h-4 w-4 text-muted-foreground" />
              </div>
              <span className="truncate font-medium text-foreground">{props.text.actions.view}</span>
            </DropdownMenuItem>
          </div>
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
  );
}

function OnlineSpriteEmptyState(props: { message: string }) {
  return (
    <div className={cn(IOS_INSET_PANEL_CLASS, "flex min-h-[8rem] items-center justify-center px-4 py-5 text-center")}>
      <div className="flex min-w-0 flex-col items-center gap-2">
        <div className="flex h-8 w-8 items-center justify-center rounded-full border border-white/25 bg-background/70 text-muted-foreground shadow-[inset_0_1px_0_hsl(var(--background)/0.45)] dark:border-white/10">
          <Search className="h-4 w-4" />
        </div>
        <div className="max-w-[18rem] text-sm font-medium leading-5 text-muted-foreground">
          {props.message}
        </div>
      </div>
    </div>
  );
}

function OnlineSpriteCatalogErrorState(props: {
  text: XiaText;
  pending: boolean;
  onRefresh: () => void;
}) {
  return (
    <div className={cn(IOS_INSET_PANEL_CLASS, "flex min-h-[8rem] items-center justify-center px-4 py-5 text-center")}>
      <div className="flex min-w-0 flex-col items-center gap-3">
        <div className="flex h-8 w-8 items-center justify-center rounded-full border border-destructive/20 bg-destructive/10 text-destructive shadow-[inset_0_1px_0_hsl(var(--background)/0.38)]">
          <AlertCircle className="h-4 w-4" />
        </div>
        <div className="max-w-[18rem] text-sm font-medium leading-5 text-muted-foreground">
          {props.text.spriteStudio.onlineLoadFailed}
        </div>
        <IOSButton
          type="button"
          size="compact"
          variant="outline"
          disabled={props.pending}
          onClick={props.onRefresh}
        >
          {props.pending ? <Loader2 className="h-4 w-4 animate-spin" /> : <RefreshCw className="h-4 w-4" />}
          {props.text.spriteStudio.refreshCatalog}
        </IOSButton>
      </div>
    </div>
  );
}

type OnlineSpriteSection = {
  id: string;
  label: string;
  items: OnlineSpriteCatalogItem[];
};

function buildOnlineCatalogSections(
  items: OnlineSpriteCatalogItem[],
  categories: OnlineSpriteCatalogCategory[],
): OnlineSpriteSection[] {
  const groupedItems = new Map<string, OnlineSpriteCatalogItem[]>();
  items.forEach((item) => {
    const categoryId = item.category.trim() || "featured";
    const current = groupedItems.get(categoryId) ?? [];
    current.push(item);
    groupedItems.set(categoryId, current);
  });

  const categoryLabels = new Map(categories.map((category) => [category.id, category.label]));
  const orderedCategoryIds = categories.map((category) => category.id);
  const trailingCategoryIds = Array.from(groupedItems.keys())
    .filter((categoryId) => !categoryLabels.has(categoryId))
    .sort((left, right) => left.localeCompare(right, undefined, { sensitivity: "base" }));

  return [...orderedCategoryIds, ...trailingCategoryIds]
    .map((categoryId) => ({
      id: categoryId,
      label: categoryLabels.get(categoryId) || categoryId,
      items: groupedItems.get(categoryId) ?? [],
    }))
    .filter((section) => section.items.length > 0);
}

function filterOnlineCatalogItems(items: OnlineSpriteCatalogItem[], normalizedQuery: string) {
  return items.filter((item) =>
    [
      item.name,
      item.description,
      item.authorDisplayName,
      item.category,
      ...item.tags,
    ].some((value) => value.toLowerCase().includes(normalizedQuery)),
  );
}

function isOnlineSpriteInstalled(
  item: OnlineSpriteCatalogItem,
  installedSpriteIds: Set<string>,
  installStates: Record<string, OnlineSpriteInstallState | undefined>,
) {
  return (
    installedSpriteIds.has(item.id) ||
    installStates[item.id]?.status === "installed"
  );
}

function OnlineSpriteCard(props: {
  text: XiaText;
  item: OnlineSpriteCatalogItem;
  installed: boolean;
  isDefault: boolean;
  installState?: OnlineSpriteInstallState;
  onOpenSprite: () => void;
  onContextMenu: (event: React.MouseEvent) => void;
}) {
  return (
    <button
      type="button"
      onClick={props.onOpenSprite}
      onContextMenu={props.onContextMenu}
      className={cn("group transition duration-200 hover:-translate-y-0.5 active:translate-y-0 active:scale-[0.985]", SPRITE_GALLERY_CARD_SIZE_CLASS)}
      title={props.item.description || props.item.name}
    >
      <OnlineSpriteCardVisual
        text={props.text}
        item={props.item}
        installed={props.installed}
        isDefault={props.isDefault}
        installState={props.installState}
        idleLabel={props.text.actions.view}
      />
    </button>
  );
}

function OnlineSpriteCardVisual(props: {
  text: XiaText;
  item: OnlineSpriteCatalogItem;
  installed: boolean;
  isDefault: boolean;
  installState?: OnlineSpriteInstallState;
  className?: string;
  idleLabel?: string;
}) {
  const status = resolveOnlineSpriteInstallStatus(props.text, props.installed, props.installState, props.idleLabel);
  const lightingSprite = buildOnlineSpriteLightingStub(props.item);
  const lighting = resolveSpriteCardLighting(lightingSprite, props.isDefault, "online");
  return (
    <div
      className={cn(
        "relative isolate flex h-full w-full flex-col items-center overflow-hidden rounded-[22px] border px-3 pb-3 pt-3 text-center",
        status.failed
          ? "border-destructive/35 bg-background/72 shadow-[inset_0_1px_0_hsl(var(--background)/0.36),inset_0_16px_24px_hsl(var(--destructive)/0.10),inset_0_-18px_24px_hsl(var(--destructive)/0.06)]"
          : lighting.cardClassName,
        props.className,
      )}
    >
      <div
        className={cn(
          "pointer-events-none absolute inset-0 z-0 rounded-[22px]",
          lighting.primaryGlowClassName,
        )}
      />
      <div
        className={cn(
          "pointer-events-none absolute inset-0 z-0 rounded-[22px]",
          lighting.directionalWashClassName,
        )}
      />
      <div
        className={cn(
          "pointer-events-none absolute inset-0 z-0 rounded-[22px]",
          lighting.rimGlowClassName,
        )}
      />
      {lighting.spotlightClassName ? (
        <div
          className={cn(
            "pointer-events-none absolute inset-0 rounded-[22px]",
            lighting.spotlightClassName,
          )}
        />
      ) : null}

      <div className="relative z-20 flex min-h-0 w-full flex-1 items-center justify-center">
        {lighting.spriteGlowClassName ? (
          <div
            aria-hidden="true"
            className={cn(
              "pointer-events-none absolute left-1/2 top-1/2 h-48 w-64 -translate-x-1/2 -translate-y-1/2 blur-xl",
              lighting.spriteGlowClassName,
            )}
            style={lighting.spriteGlowStyle}
          />
        ) : null}
        <img
          src={props.item.previewUrl}
          alt={props.item.name}
          draggable={false}
          loading="lazy"
          decoding="async"
          className="relative z-10 max-h-[5rem] max-w-full select-none object-contain drop-shadow-sm"
        />
      </div>

      <div className="relative z-30 mt-2 w-full min-w-0 px-1">
        <div className="truncate text-sm font-medium leading-5 text-foreground">{props.item.name}</div>
        <div
          className={cn(
            "mt-0.5 flex min-w-0 items-center justify-center gap-1 truncate text-[11px] font-medium text-muted-foreground",
            status.failed && "text-destructive",
            status.pending && "text-primary",
            props.installed && "text-emerald-700 dark:text-emerald-200",
          )}
        >
          {status.pending ? (
            <Loader2 className="h-3 w-3 shrink-0 animate-spin" />
          ) : props.installed ? (
            <CheckCircle2 className="h-3 w-3 shrink-0" />
          ) : status.failed ? (
            <AlertCircle className="h-3 w-3 shrink-0" />
          ) : (
            <DownloadCloud className="h-3 w-3 shrink-0" />
          )}
          <span className="truncate">{status.pending ? `${status.label} ${status.progress}%` : status.label}</span>
        </div>
      </div>

      {status.pending ? (
        <div className="absolute inset-x-3 bottom-2 z-40 h-1 overflow-hidden rounded-full bg-muted">
          <div className="h-full rounded-full bg-primary transition-all" style={{ width: `${status.progress}%` }} />
        </div>
      ) : null}
    </div>
  );
}

function OnlineSpriteDetailDialog(props: {
  text: XiaText;
  item: OnlineSpriteCatalogItem | null;
  installed: boolean;
  isDefault: boolean;
  installState?: OnlineSpriteInstallState;
  onOpenChange: (open: boolean) => void;
  onInstallSprite: (item: OnlineSpriteCatalogItem) => Promise<void>;
}) {
  const item = props.item;
  const status = resolveOnlineSpriteInstallStatus(props.text, props.installed, props.installState);
  const updatedLabel = item ? formatOnlineSpriteDate(item.updatedAt) : "";
  return (
    <Dialog open={Boolean(item)} onOpenChange={props.onOpenChange}>
      {item ? (
        <DialogContent className={cn(IOS_PANEL_CLASS, "grid max-h-[min(42rem,calc(100vh-2rem))] w-[min(30rem,calc(100vw-2rem))] max-w-none grid-rows-[auto_auto_minmax(0,1fr)_auto] gap-4 overflow-hidden p-4")}>
          <DialogHeader className="min-w-0">
            <DialogTitle className="overflow-hidden break-words pr-6 text-left leading-[1.35] [display:-webkit-box] [-webkit-box-orient:vertical] [-webkit-line-clamp:2]">
              {item.name}
            </DialogTitle>
            <DialogDescription className="overflow-hidden break-words text-left leading-relaxed [display:-webkit-box] [-webkit-box-orient:vertical] [-webkit-line-clamp:2]">
              {item.description}
            </DialogDescription>
          </DialogHeader>

          <div className="flex shrink-0 justify-center">
            <div className={SPRITE_GALLERY_CARD_SIZE_CLASS}>
              <OnlineSpriteCardVisual
                text={props.text}
                item={item}
                installed={props.installed}
                isDefault={props.isDefault}
                installState={props.installState}
              />
            </div>
          </div>

          <div className={cn(IOS_INSET_PANEL_CLASS, "min-h-0 min-w-0 overflow-y-auto overflow-x-hidden p-3")}>
            <div className="grid grid-cols-2 gap-2">
              <OnlineSpriteInfoItem label={props.text.settings.sprites.author} value={item.authorDisplayName} />
              <OnlineSpriteInfoItem label={props.text.spriteStudio.versionLabel} value={item.version} />
              <OnlineSpriteInfoItem label={props.text.spriteStudio.sizeLabel} value={formatBytes(item.size)} />
              <OnlineSpriteInfoItem label={props.text.spriteStudio.updatedLabel} value={updatedLabel} />
            </div>
            {item.tags.length > 0 ? (
              <div className="mt-3 flex flex-wrap gap-1.5">
                {item.tags.map((tag) => (
                  <span
                    key={tag}
                    className="max-w-full break-words rounded-full border border-sidebar-foreground/10 bg-sidebar-background/36 px-2 py-0.5 text-[11px] font-medium text-sidebar-foreground/62"
                  >
                    {tag}
                  </span>
                ))}
              </div>
            ) : null}
            {props.installState?.status === "error" && props.installState.error ? (
              <div className="mt-3 overflow-hidden break-words rounded-2xl border border-destructive/20 bg-destructive/10 px-3 py-2 text-xs leading-5 text-destructive [display:-webkit-box] [-webkit-box-orient:vertical] [-webkit-line-clamp:4]">
                {props.installState.error}
              </div>
            ) : null}
          </div>

          <div className="flex shrink-0 items-center justify-end gap-2">
            <IOSButton
              type="button"
              size="default"
              disabled={props.installed || status.pending}
              onClick={() => void props.onInstallSprite(item)}
            >
              {status.pending ? (
                <Loader2 className="h-4 w-4 animate-spin" />
              ) : props.installed ? (
                <CheckCircle2 className="h-4 w-4" />
              ) : status.failed ? (
                <AlertCircle className="h-4 w-4" />
              ) : (
                <DownloadCloud className="h-4 w-4" />
              )}
              {status.pending ? `${status.label} ${status.progress}%` : status.label}
            </IOSButton>
          </div>
        </DialogContent>
      ) : null}
    </Dialog>
  );
}

function OnlineSpriteInfoItem(props: { label: string; value: string }) {
  return (
    <div className="min-w-0 rounded-2xl bg-background/36 px-3 py-2 shadow-[inset_0_1px_0_hsl(var(--background)/0.34)]">
      <div className="truncate text-[11px] font-medium text-muted-foreground">{props.label}</div>
      <div className="mt-0.5 truncate text-xs font-semibold text-foreground">{props.value || "-"}</div>
    </div>
  );
}

function resolveOnlineSpriteInstallStatus(
  text: XiaText,
  installed: boolean,
  installState?: OnlineSpriteInstallState,
  idleLabel?: string,
) {
  const pending = installState?.status === "downloading" || installState?.status === "importing";
  const progress = clampInt(Math.round(installState?.progress ?? 0), 0, 100);
  const failed = installState?.status === "error";
  const label =
    installState?.status === "downloading"
      ? text.spriteStudio.downloading
      : installState?.status === "importing"
        ? text.spriteStudio.importing
        : installState?.status === "error"
          ? text.spriteStudio.installFailed
          : installed
            ? text.spriteStudio.installed
            : idleLabel || text.actions.install;
  return { failed, label, pending, progress };
}

function formatOnlineSpriteDate(value: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return date.toLocaleDateString();
}

function buildOnlineSpriteLightingStub(item: OnlineSpriteCatalogItem): Sprite {
  return {
    id: item.id,
    name: item.name,
    description: item.description,
    frameCount: 64,
    columns: 8,
    rows: 8,
    spriteFile: "sprite.png",
    spritePath: "",
    sourceType: "zip",
    origin: "online",
    scope: "builtin",
    status: "ready",
    imageWidth: 0,
    imageHeight: 0,
    author: {
      id: "",
      displayName: item.authorDisplayName,
    },
    createdAt: item.updatedAt,
    updatedAt: item.updatedAt,
    version: item.version,
  };
}

export function SegmentedTabs(props: {
  activeTab: StudioTab;
  text: XiaText;
  onChange: (tab: StudioTab) => void;
}) {
  const items: Array<{ id: StudioTab; label: string; icon: React.ReactNode }> = [
    { id: "edit", label: props.text.spriteStudio.editTab, icon: <Pencil className="h-4 w-4" /> },
    { id: "preview", label: props.text.spriteStudio.previewTab, icon: <Eye className="h-4 w-4" /> },
  ];
  return (
    <div className={cn(SPRITE_STUDIO_TAB_SURFACE_CLASS, "h-9")}>
      {items.map((item) => (
        <button
          key={item.id}
          type="button"
          aria-label={item.label}
          data-active={props.activeTab === item.id ? "true" : "false"}
          onClick={() => props.onChange(item.id)}
          className={cn(
            "w-[5.25rem] min-w-8 px-1.5",
            iosSegmentClass(props.activeTab === item.id),
          )}
        >
          {item.icon}
          <span className={cn(SPRITE_STUDIO_TAB_LABEL_CLASS, "ml-1.5 max-w-12")}>{item.label}</span>
        </button>
      ))}
    </div>
  );
}

export function SpriteDetailHeader(props: {
  text: XiaText;
  sprite: Sprite;
  manifest: SpriteManifest;
  activeTab: StudioTab;
  saveState: SaveState;
  exportPending: boolean;
  updatePending: boolean;
  deletePending: boolean;
  previewAnimation: SpriteAnimation;
  onPreviewAnimationChange: (animation: SpriteAnimation) => void;
  onExportSprite: () => void;
  onSaveSpriteMetadata: (draft: SpriteMetadataDraft) => Promise<void>;
  onDeleteSprite: () => Promise<void>;
  onResetGrid: () => void;
}) {
  const metaText = props.sprite.author.displayName;
  return (
    <div className="flex min-w-0 items-center justify-between gap-4 border-b border-border/50 pb-4">
      <div className="min-w-0 max-w-[calc(50%_-_0.5rem)] space-y-2 overflow-hidden">
        <div className="flex min-w-0 items-baseline gap-2">
          <div
            className={cn(
              "min-w-0 truncate text-base font-semibold leading-6 text-foreground",
              metaText ? "max-w-[55%] shrink-0" : "max-w-full",
            )}
          >
            {props.sprite.name}
          </div>
          {metaText ? (
            <div className="min-w-0 flex-1 truncate text-xs font-medium text-muted-foreground/75">{metaText}</div>
          ) : null}
        </div>
        <SpriteMetadataActions
          text={props.text}
          sprite={props.sprite}
          exportPending={props.exportPending}
          updatePending={props.updatePending}
          deletePending={props.deletePending}
          onExportSprite={props.onExportSprite}
          onSaveSpriteMetadata={props.onSaveSpriteMetadata}
          onDeleteSprite={props.onDeleteSprite}
        />
      </div>

      <div className="wails-no-drag flex min-w-0 max-w-[calc(50%_-_0.5rem)] items-center justify-end overflow-hidden">
        {props.activeTab === "edit" ? (
          <EditHeaderActions
            text={props.text}
            manifest={props.manifest}
            saveState={props.saveState}
            onResetGrid={props.onResetGrid}
          />
        ) : (
          <AnimationButtonGroup
            text={props.text}
            animation={props.previewAnimation}
            onAnimationChange={props.onPreviewAnimationChange}
          />
        )}
      </div>
    </div>
  );
}

export function SpriteMetadataActions(props: {
  text: XiaText;
  sprite: Sprite;
  exportPending: boolean;
  updatePending: boolean;
  deletePending: boolean;
  onExportSprite: () => void;
  onSaveSpriteMetadata: (draft: SpriteMetadataDraft) => Promise<void>;
  onDeleteSprite: () => Promise<void>;
}) {
  const canMutate = props.sprite.scope === "imported";
  return (
    <div className={cn(IOS_TOOLBAR_CLASS, "h-9")}>
      <SpriteMetadataEditDropdown
        text={props.text}
        sprite={props.sprite}
        disabled={!canMutate}
        pending={props.updatePending}
        onSave={props.onSaveSpriteMetadata}
      />
      <span className={IOS_DIVIDER_CLASS} aria-hidden="true" />
      <IOSButton
        type="button"
        variant="ghost"
        size="compact"
        className={spriteStudioGroupButtonClass()}
        disabled={props.exportPending}
        onClick={props.onExportSprite}
      >
        {props.exportPending ? <Loader2 className="h-4 w-4 animate-spin" /> : <Download className="h-4 w-4" />}
        <span className="min-w-0 truncate">{props.text.settings.sprites.exportAction}</span>
      </IOSButton>
      <span className={IOS_DIVIDER_CLASS} aria-hidden="true" />
      <SpriteDeleteDropdown
        text={props.text}
        sprite={props.sprite}
        disabled={!canMutate}
        pending={props.deletePending}
        onDelete={props.onDeleteSprite}
      />
    </div>
  );
}

export function SpriteMetadataEditDropdown(props: {
  text: XiaText;
  sprite: Sprite;
  disabled: boolean;
  pending: boolean;
  onSave: (draft: SpriteMetadataDraft) => Promise<void>;
}) {
  const [open, setOpen] = React.useState(false);
  const [draft, setDraft] = React.useState<SpriteMetadataDraft>(() => buildSpriteMetadataDraft(props.sprite));
  const [initialDraft, setInitialDraft] = React.useState<SpriteMetadataDraft>(() => buildSpriteMetadataDraft(props.sprite));
  const [error, setError] = React.useState("");
  const authorLocked = props.sprite.sourceType === "zip";

  const handleOpenChange = React.useCallback(
    (nextOpen: boolean) => {
      setOpen(nextOpen);
      setError("");
      if (nextOpen) {
        const nextDraft = buildSpriteMetadataDraft(props.sprite);
        setDraft(nextDraft);
        setInitialDraft(nextDraft);
      }
    },
    [props.sprite],
  );

  React.useEffect(() => {
    if (!open) {
      return;
    }
    const nextDraft = buildSpriteMetadataDraft(props.sprite);
    setDraft(nextDraft);
    setInitialDraft(nextDraft);
  }, [open, props.sprite]);

  const handleSave = async () => {
    setError("");
    try {
      await props.onSave({
        ...draft,
        authorDisplayName: authorLocked ? props.sprite.author.displayName : draft.authorDisplayName,
      });
      setOpen(false);
    } catch (saveError) {
      setError(resolveSpriteError(saveError));
    }
  };

  return (
    <DropdownMenu open={open} onOpenChange={handleOpenChange}>
      <DropdownMenuTrigger asChild>
        <IOSButton
          type="button"
          variant="ghost"
          size="compact"
          className={spriteStudioGroupButtonClass()}
          disabled={props.disabled}
        >
          {props.pending ? <Loader2 className="h-4 w-4 animate-spin" /> : <Pencil className="h-4 w-4" />}
          <span className="min-w-0 truncate">{props.text.settings.sprites.editAction}</span>
        </IOSButton>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="start" className={cn(IOS_DROPDOWN_PANEL_CLASS, "w-80")} onCloseAutoFocus={(event) => event.preventDefault()}>
        <div className="space-y-3">
          {error ? <div className="text-xs text-destructive">{error}</div> : null}
          <SpriteFormField label={props.text.settings.sprites.name}>
            <IOSInput
              value={draft.name}
              onChange={(event) => setDraft((current) => ({ ...current, name: event.target.value }))}
              placeholder={props.text.settings.sprites.namePlaceholder}
            />
          </SpriteFormField>
          <div className="grid grid-cols-2 gap-3">
            <SpriteFormField label={props.text.settings.sprites.author}>
              <IOSInput
                value={draft.authorDisplayName}
                onChange={(event) => setDraft((current) => ({ ...current, authorDisplayName: event.target.value }))}
                placeholder={props.text.settings.sprites.authorPlaceholder}
                disabled={authorLocked}
              />
            </SpriteFormField>
            <SpriteFormField label={props.text.settings.sprites.version}>
              <IOSSelect
                value={draft.version}
                onChange={(event) => setDraft((current) => ({ ...current, version: event.target.value }))}
                className="w-full"
              >
                <option value="1.0">1.0</option>
              </IOSSelect>
            </SpriteFormField>
          </div>
          <SpriteFormField label={props.text.settings.sprites.description}>
            <textarea
              value={draft.description}
              onChange={(event) => setDraft((current) => ({ ...current, description: event.target.value }))}
              placeholder={props.text.settings.sprites.descriptionPlaceholder}
              className={IOS_TEXTAREA_CLASS}
            />
          </SpriteFormField>
          <div className="flex items-center justify-between gap-2 border-t border-sidebar-foreground/10 pt-3">
            <IOSButton
              type="button"
              size="compact"
              variant="outline"
              disabled={props.pending}
              onClick={() => {
                setDraft(initialDraft);
                setError("");
              }}
            >
              <RotateCcw className="h-4 w-4" />
              {props.text.actions.reset}
            </IOSButton>
            <IOSButton
              type="button"
              size="compact"
              className="shadow-[0_18px_40px_-24px_hsl(var(--sidebar-primary)/0.68)]"
              disabled={!draft.name.trim() || props.pending}
              onClick={() => void handleSave()}
            >
              {props.pending ? <Loader2 className="h-4 w-4 animate-spin" /> : null}
              {props.text.actions.save}
            </IOSButton>
          </div>
        </div>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

export function SpriteDeleteDropdown(props: {
  text: XiaText;
  sprite: Sprite;
  disabled: boolean;
  pending: boolean;
  onDelete: () => Promise<void>;
}) {
  const [open, setOpen] = React.useState(false);
  const [error, setError] = React.useState("");

  const handleOpenChange = React.useCallback(
    (nextOpen: boolean) => {
      if (props.pending) {
        return;
      }
      setOpen(nextOpen);
      if (!nextOpen) {
        setError("");
      }
    },
    [props.pending],
  );

  const handleDelete = async () => {
    setError("");
    try {
      await props.onDelete();
      setOpen(false);
    } catch (deleteError) {
      setError(resolveSpriteError(deleteError));
    }
  };

  return (
    <DropdownMenu open={open} onOpenChange={handleOpenChange}>
      <DropdownMenuTrigger asChild>
        <IOSButton
          type="button"
          variant="ghost"
          size="compact"
          className={cn(
            spriteStudioGroupButtonClass(),
            "text-destructive hover:bg-destructive/10 hover:text-destructive",
          )}
          disabled={props.disabled || props.pending}
        >
          {props.pending ? <Loader2 className="h-4 w-4 animate-spin" /> : <Trash2 className="h-4 w-4" />}
          <span className="min-w-0 truncate">{props.text.settings.sprites.deleteAction}</span>
        </IOSButton>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="start" className={cn(IOS_DROPDOWN_PANEL_CLASS, "w-72")} onCloseAutoFocus={(event) => event.preventDefault()}>
        <div className="space-y-3">
          <div className="text-sm font-semibold text-foreground">{props.text.settings.sprites.deleteTitle}</div>
          <div className="text-xs leading-relaxed text-muted-foreground">
            {formatDeleteMessage(props.text.settings.sprites.deleteMessage, props.sprite.name)}
          </div>
          {error ? <div className="text-xs text-destructive">{error}</div> : null}
          <div className="flex items-center justify-between gap-2 border-t border-foreground/10 pt-3">
            <IOSButton type="button" size="compact" variant="outline" disabled={props.pending} onClick={() => setOpen(false)}>
              {props.text.actions.cancelDialog}
            </IOSButton>
            <IOSButton
              type="button"
              size="compact"
              variant="destructive"
              disabled={props.pending}
              onClick={() => void handleDelete()}
            >
              {props.pending ? <Loader2 className="h-4 w-4 animate-spin" /> : null}
              {props.text.settings.sprites.deleteAction}
            </IOSButton>
          </div>
        </div>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

export function SpriteFormField(props: { label: string; children: React.ReactNode }) {
  return (
    <label className="flex min-w-0 flex-col gap-1.5">
      <span className="text-xs font-medium text-sidebar-foreground/62">{props.label}</span>
      {props.children}
    </label>
  );
}

export function SpriteMetricSegments(props: {
  text: XiaText;
  manifest: SpriteManifest;
  grid: SpriteSliceGrid;
}) {
  const firstFrameWidth = props.grid.x[1] - props.grid.x[0];
  const firstFrameHeight = props.grid.y[1] - props.grid.y[0];
  const metrics = [
    {
      label: props.text.spriteStudio.sheetSize,
      value: `${props.manifest.sheetWidth} × ${props.manifest.sheetHeight}`,
    },
    {
      label: props.text.spriteStudio.gridSize,
      value: `${props.manifest.columns} × ${props.manifest.rows}`,
    },
    {
      label: props.text.spriteStudio.firstFrame,
      value: `${firstFrameWidth} × ${firstFrameHeight}`,
    },
  ];
  return (
    <div className={cn(IOS_TOOLBAR_CLASS, "max-w-full")}>
      {metrics.map((metric, index) => (
        <div
          key={metric.label}
          className={cn("flex h-8 min-w-0 items-center gap-1.5 px-2.5 text-xs", index > 0 && "border-l border-foreground/10")}
        >
          <span className="min-w-0 truncate text-muted-foreground">{metric.label}</span>
          <span className="shrink-0 font-semibold tabular-nums text-foreground">{metric.value}</span>
        </div>
      ))}
    </div>
  );
}

export function EditHeaderActions(props: {
  text: XiaText;
  manifest: SpriteManifest;
  saveState: SaveState;
  onResetGrid: () => void;
}) {
  return (
    <div className={cn(IOS_TOOLBAR_CLASS, "h-9")}>
      <IOSButton
        type="button"
        variant="ghost"
        size="compact"
        className={spriteStudioGroupButtonClass()}
        disabled={!props.manifest.canEdit}
        onClick={props.onResetGrid}
      >
        <RotateCcw className="h-4 w-4" />
        <span className="min-w-0 truncate">{props.text.spriteStudio.resetEqual}</span>
      </IOSButton>
      <span className={IOS_DIVIDER_CLASS} aria-hidden="true" />
      <SaveStateBadge
        text={props.text}
        canEdit={props.manifest.canEdit}
        saveState={props.saveState}
        className="h-8 min-w-0 justify-center overflow-hidden border-0 bg-transparent px-2 [border-radius:var(--sprite-segment-tab-radius,16px)]"
      />
    </div>
  );
}

export function AnimationButtonGroup(props: {
  text: XiaText;
  animation: SpriteAnimation;
  onAnimationChange: (animation: SpriteAnimation) => void;
}) {
  return (
    <div className={cn(SPRITE_STUDIO_TAB_SURFACE_CLASS, "inline-grid grid-cols-4 gap-1 rounded-[22px] p-1 [--sprite-segment-tab-radius:18px]")}>
      {SPRITE_ACTIONS.map((action) => (
        <button
          key={action}
          type="button"
          aria-pressed={props.animation === action}
          data-active={props.animation === action ? "true" : "false"}
          onClick={() => props.onAnimationChange(action)}
          className={cn(
            "h-7 px-2",
            iosSegmentClass(props.animation === action),
          )}
        >
          <span className={SPRITE_STUDIO_TAB_LABEL_CLASS}>{props.text.spriteStudio.animations[action]}</span>
        </button>
      ))}
    </div>
  );
}

export function SpriteSheetEditor(props: {
  text: XiaText;
  manifest: SpriteManifest;
  imageUrl: string;
  grid: SpriteSliceGrid;
  onGridChange: (grid: SpriteSliceGrid, immediate?: boolean) => void;
}) {
  const { text, grid, imageUrl, manifest, onGridChange } = props;
  const stageRef = React.useRef<HTMLDivElement | null>(null);
  const dragRef = React.useRef<DragTarget | null>(null);
  const canEdit = manifest.canEdit;

  const moveLine = React.useCallback(
    (axis: "x" | "y", index: number, clientX: number, clientY: number, immediate = false) => {
      const rect = stageRef.current?.getBoundingClientRect();
      if (!rect) {
        return;
      }
      const values = axis === "x" ? grid.x : grid.y;
      const total = axis === "x" ? manifest.sheetWidth : manifest.sheetHeight;
      const pointerValue =
        axis === "x"
          ? ((clientX - rect.left) / Math.max(1, rect.width)) * total
          : ((clientY - rect.top) / Math.max(1, rect.height)) * total;
      const min = values[index - 1] + MIN_SLICE_GAP_PX;
      const max = values[index + 1] - MIN_SLICE_GAP_PX;
      const nextValue = clampInt(Math.round(pointerValue), min, max);
      if (nextValue === values[index]) {
        return;
      }
      const nextValues = [...values];
      nextValues[index] = nextValue;
      onGridChange(axis === "x" ? { ...grid, x: nextValues } : { ...grid, y: nextValues }, immediate);
    },
    [grid, manifest.sheetHeight, manifest.sheetWidth, onGridChange],
  );

  const handlePointerDown = (axis: "x" | "y", index: number, event: React.PointerEvent<HTMLButtonElement>) => {
    if (!canEdit) {
      return;
    }
    event.preventDefault();
    dragRef.current = { axis, index, pointerId: event.pointerId };
    event.currentTarget.setPointerCapture(event.pointerId);
    moveLine(axis, index, event.clientX, event.clientY);
  };

  const handlePointerMove = (event: React.PointerEvent<HTMLButtonElement>) => {
    const drag = dragRef.current;
    if (!drag || drag.pointerId !== event.pointerId) {
      return;
    }
    event.preventDefault();
    moveLine(drag.axis, drag.index, event.clientX, event.clientY);
  };

  const handlePointerUp = (event: React.PointerEvent<HTMLButtonElement>) => {
    const drag = dragRef.current;
    if (!drag || drag.pointerId !== event.pointerId) {
      return;
    }
    event.preventDefault();
    dragRef.current = null;
    if (event.currentTarget.hasPointerCapture(event.pointerId)) {
      event.currentTarget.releasePointerCapture(event.pointerId);
    }
    moveLine(drag.axis, drag.index, event.clientX, event.clientY, true);
  };

  const imageWidthPercent = Math.min(100, (manifest.imageWidth / Math.max(1, manifest.sheetWidth)) * 100);
  const imageHeightPercent = Math.min(100, (manifest.imageHeight / Math.max(1, manifest.sheetHeight)) * 100);

  return (
    <div className={cn(IOS_PANEL_CLASS, "min-w-0 p-3")}>
      <div className="mb-3 flex items-center gap-2 text-xs font-medium text-muted-foreground">
        <Grid3X3 className="h-4 w-4 text-primary" />
        {canEdit ? text.spriteStudio.dragHint : text.spriteStudio.readonlyHint}
      </div>
      <div
        ref={stageRef}
        className="relative mx-auto w-full overflow-hidden rounded-[22px] border border-white/20 bg-[length:18px_18px] shadow-[inset_0_1px_0_hsl(var(--background)/0.42)] dark:border-white/10"
        style={{
          aspectRatio: `${Math.max(1, manifest.sheetWidth)} / ${Math.max(1, manifest.sheetHeight)}`,
          backgroundColor: "hsl(var(--muted) / 0.42)",
          backgroundImage:
            "linear-gradient(45deg, hsl(var(--foreground) / 0.08) 25%, transparent 25%), linear-gradient(-45deg, hsl(var(--foreground) / 0.08) 25%, transparent 25%), linear-gradient(45deg, transparent 75%, hsl(var(--foreground) / 0.08) 75%), linear-gradient(-45deg, transparent 75%, hsl(var(--foreground) / 0.08) 75%)",
          backgroundPosition: "0 0, 0 9px, 9px -9px, -9px 0px",
        }}
      >
        {imageUrl ? (
          <img
            src={imageUrl}
            alt={manifest.name}
            draggable={false}
            className="absolute left-0 top-0 select-none"
            style={{
              width: `${imageWidthPercent}%`,
              height: `${imageHeightPercent}%`,
              userSelect: "none",
            }}
          />
        ) : null}

        {grid.x.slice(1, -1).map((value, index) => (
          <button
            key={`x-${index}`}
            type="button"
            aria-label={text.spriteStudio.verticalLine}
            className={cn(
              "absolute top-0 z-20 h-full w-4 -translate-x-1/2 cursor-col-resize bg-transparent p-0",
              !canEdit && "cursor-default",
            )}
            style={{ left: `${(value / Math.max(1, manifest.sheetWidth)) * 100}%` }}
            onPointerDown={(event) => handlePointerDown("x", index + 1, event)}
            onPointerMove={handlePointerMove}
            onPointerUp={handlePointerUp}
            onPointerCancel={handlePointerUp}
            disabled={!canEdit}
          >
            <span className="mx-auto block h-full w-px bg-primary/90 shadow-[0_0_0_1px_hsl(var(--background)/0.65)]" />
          </button>
        ))}

        {grid.y.slice(1, -1).map((value, index) => (
          <button
            key={`y-${index}`}
            type="button"
            aria-label={text.spriteStudio.horizontalLine}
            className={cn(
              "absolute left-0 z-20 h-4 w-full -translate-y-1/2 cursor-row-resize bg-transparent p-0",
              !canEdit && "cursor-default",
            )}
            style={{ top: `${(value / Math.max(1, manifest.sheetHeight)) * 100}%` }}
            onPointerDown={(event) => handlePointerDown("y", index + 1, event)}
            onPointerMove={handlePointerMove}
            onPointerUp={handlePointerUp}
            onPointerCancel={handlePointerUp}
            disabled={!canEdit}
          >
            <span className="block h-px w-full bg-primary/90 shadow-[0_0_0_1px_hsl(var(--background)/0.65)]" />
          </button>
        ))}
      </div>
    </div>
  );
}

export function SpriteStudioPreview(props: {
  text: XiaText;
  manifest: SpriteManifest;
  imageUrl: string;
  displayImageUrl: string;
  sprite: Sprite;
  grid: SpriteSliceGrid;
  animation: SpriteAnimation;
}) {
  const rowIndex = SPRITE_ROW_BY_ANIMATION[props.animation] ?? 0;

  return (
    <div className={cn(IOS_PANEL_CLASS, "min-w-0 p-4")}>
      <div className="min-w-0 space-y-4">
        <div className="grid min-w-0 grid-cols-[minmax(0,1fr)_minmax(0,1fr)] gap-4">
          <div className={cn(IOS_INSET_PANEL_CLASS, "flex min-w-0 flex-col p-5")}>
            <div className="mb-3 truncate text-left text-xs font-medium text-muted-foreground">
              {props.text.spriteStudio.animations[props.animation]}
            </div>
            <div className="flex min-h-36 flex-1 items-center justify-center">
              <SlicedSpriteCanvas
                imageUrl={props.imageUrl}
                manifest={props.manifest}
                grid={props.grid}
                rowIndex={rowIndex}
                frameIndex={0}
                size={152}
                animate
              />
            </div>
          </div>

          <div className={cn(IOS_INSET_PANEL_CLASS, "flex min-w-0 flex-col p-5")}>
            <div className="mb-3 truncate text-left text-xs font-medium text-muted-foreground">
              {props.text.spriteStudio.previewTab}
            </div>
            <div className="flex min-h-36 flex-1 items-center justify-center">
              <SpriteDisplay
                sprite={props.sprite}
                imageUrl={props.displayImageUrl}
                alt={props.sprite.name}
                animation={props.animation}
                animate
                className="mx-auto h-36 w-36"
                spriteClassName="!h-28 !w-28"
              />
            </div>
          </div>
        </div>

        <div className="grid grid-cols-4 gap-2 sm:grid-cols-8">
          {Array.from({ length: props.manifest.columns }, (_, index) => (
            <div
              key={index}
              className={cn(IOS_INSET_PANEL_CLASS, "flex items-center justify-center p-1")}
            >
              <SlicedSpriteCanvas
                imageUrl={props.imageUrl}
                manifest={props.manifest}
                grid={props.grid}
                rowIndex={rowIndex}
                frameIndex={index}
                size={54}
                animate={false}
              />
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}

export function SlicedSpriteCanvas(props: {
  imageUrl: string;
  manifest: SpriteManifest;
  grid: SpriteSliceGrid;
  rowIndex: number;
  frameIndex: number;
  size: number;
  animate: boolean;
}) {
  const { animate, frameIndex, grid, imageUrl, manifest, rowIndex, size } = props;
  const canvasRef = React.useRef<HTMLCanvasElement | null>(null);
  const image = useLoadedImage(imageUrl);

  React.useEffect(() => {
    const canvas = canvasRef.current;
    const context = canvas?.getContext("2d");
    if (!canvas || !context) {
      return;
    }

    const dpr = window.devicePixelRatio || 1;
    canvas.width = Math.round(size * dpr);
    canvas.height = Math.round(size * dpr);
    canvas.style.width = `${size}px`;
    canvas.style.height = `${size}px`;

    const draw = (currentFrame: number) => {
      context.setTransform(dpr, 0, 0, dpr, 0, 0);
      context.clearRect(0, 0, size, size);
      if (!image) {
        return;
      }
      drawSlicedFrame(context, image, manifest, grid, rowIndex, currentFrame, size);
    };

    draw(frameIndex);
    if (!animate || !image) {
      return;
    }

    let frameHandle = 0;
    const startedAt = performance.now();
    const tick = (timestamp: number) => {
      const nextFrame = Math.floor((timestamp - startedAt) / 440) % Math.max(1, manifest.columns);
      draw(nextFrame);
      frameHandle = window.requestAnimationFrame(tick);
    };
    frameHandle = window.requestAnimationFrame(tick);
    return () => window.cancelAnimationFrame(frameHandle);
  }, [animate, frameIndex, grid, image, manifest, rowIndex, size]);

  return <canvas ref={canvasRef} className="block select-none" aria-hidden="true" />;
}

export function drawSlicedFrame(
  context: CanvasRenderingContext2D,
  image: HTMLImageElement,
  manifest: SpriteManifest,
  grid: SpriteSliceGrid,
  rowIndex: number,
  frameIndex: number,
  size: number,
) {
  const safeColumn = clampInt(frameIndex, 0, Math.max(0, manifest.columns - 1));
  const safeRow = clampInt(rowIndex, 0, Math.max(0, manifest.rows - 1));
  const sourceLeft = grid.x[safeColumn] ?? 0;
  const sourceTop = grid.y[safeRow] ?? 0;
  const sourceRight = grid.x[safeColumn + 1] ?? manifest.sheetWidth;
  const sourceBottom = grid.y[safeRow + 1] ?? manifest.sheetHeight;
  const sourceWidth = Math.max(1, sourceRight - sourceLeft);
  const sourceHeight = Math.max(1, sourceBottom - sourceTop);
  const drawLeft = clampInt(sourceLeft, 0, image.naturalWidth);
  const drawTop = clampInt(sourceTop, 0, image.naturalHeight);
  const drawRight = clampInt(sourceRight, 0, image.naturalWidth);
  const drawBottom = clampInt(sourceBottom, 0, image.naturalHeight);
  const drawWidth = drawRight - drawLeft;
  const drawHeight = drawBottom - drawTop;
  if (drawWidth <= 0 || drawHeight <= 0) {
    return;
  }
  const targetLeft = ((drawLeft - sourceLeft) / sourceWidth) * size;
  const targetTop = ((drawTop - sourceTop) / sourceHeight) * size;
  const targetWidth = (drawWidth / sourceWidth) * size;
  const targetHeight = (drawHeight / sourceHeight) * size;
  context.imageSmoothingEnabled = true;
  context.imageSmoothingQuality = "high";
  drawChromaKeyedImage(
    context,
    image,
    drawLeft,
    drawTop,
    drawWidth,
    drawHeight,
    targetLeft,
    targetTop,
    targetWidth,
    targetHeight,
  );
}

function drawChromaKeyedImage(
  targetContext: CanvasRenderingContext2D,
  image: HTMLImageElement,
  sourceLeft: number,
  sourceTop: number,
  sourceWidth: number,
  sourceHeight: number,
  targetLeft: number,
  targetTop: number,
  targetWidth: number,
  targetHeight: number,
) {
  const frameCanvas = document.createElement("canvas");
  frameCanvas.width = Math.max(1, sourceWidth);
  frameCanvas.height = Math.max(1, sourceHeight);
  const frameContext = frameCanvas.getContext("2d", { willReadFrequently: true });
  if (!frameContext) {
    targetContext.drawImage(
      image,
      sourceLeft,
      sourceTop,
      sourceWidth,
      sourceHeight,
      targetLeft,
      targetTop,
      targetWidth,
      targetHeight,
    );
    return;
  }

  frameContext.drawImage(
    image,
    sourceLeft,
    sourceTop,
    sourceWidth,
    sourceHeight,
    0,
    0,
    sourceWidth,
    sourceHeight,
  );

  try {
    const imageData = frameContext.getImageData(0, 0, frameCanvas.width, frameCanvas.height);
    removeMagentaKeyPixels(imageData.data);
    frameContext.putImageData(imageData, 0, 0);
    targetContext.drawImage(
      frameCanvas,
      0,
      0,
      frameCanvas.width,
      frameCanvas.height,
      targetLeft,
      targetTop,
      targetWidth,
      targetHeight,
    );
  } catch {
    targetContext.drawImage(
      image,
      sourceLeft,
      sourceTop,
      sourceWidth,
      sourceHeight,
      targetLeft,
      targetTop,
      targetWidth,
      targetHeight,
    );
  }
}

function removeMagentaKeyPixels(pixels: Uint8ClampedArray) {
  for (let index = 0; index < pixels.length; index += 4) {
    if (
      pixels[index + 3] > 0 &&
      pixels[index] >= SPRITE_MAGENTA_KEY_RED_MIN &&
      pixels[index + 1] <= SPRITE_MAGENTA_KEY_GREEN_MAX &&
      pixels[index + 2] >= SPRITE_MAGENTA_KEY_BLUE_MIN
    ) {
      pixels[index] = 0;
      pixels[index + 1] = 0;
      pixels[index + 2] = 0;
      pixels[index + 3] = 0;
    }
  }
}

export function SaveStateBadge(props: { text: XiaText; canEdit: boolean; saveState: SaveState; className?: string }) {
  if (!props.canEdit) {
    return (
      <span
        className={cn(
          "inline-flex h-7 min-w-0 items-center gap-1.5 rounded-full border border-white/20 bg-background/44 px-2.5 text-xs font-semibold text-muted-foreground shadow-[inset_0_1px_0_hsl(var(--background)/0.45)] dark:border-white/10",
          props.className,
        )}
      >
        <AlertCircle className="h-3.5 w-3.5" />
        <span className="min-w-0 truncate">{props.text.spriteStudio.readonly}</span>
      </span>
    );
  }
  if (props.saveState === "saving") {
    return (
      <span
        className={cn(
          "inline-flex h-7 min-w-0 items-center gap-1.5 rounded-full border border-primary/18 bg-primary/10 px-2.5 text-xs font-semibold text-primary shadow-[inset_0_1px_0_hsl(var(--background)/0.38)]",
          props.className,
        )}
      >
        <Loader2 className="h-3.5 w-3.5 animate-spin" />
        <span className="min-w-0 truncate">{props.text.spriteStudio.saving}</span>
      </span>
    );
  }
  if (props.saveState === "error") {
    return (
      <span
        className={cn(
          "inline-flex h-7 min-w-0 items-center gap-1.5 rounded-full border border-destructive/20 bg-destructive/10 px-2.5 text-xs font-semibold text-destructive shadow-[inset_0_1px_0_hsl(var(--background)/0.38)]",
          props.className,
        )}
      >
        <AlertCircle className="h-3.5 w-3.5" />
        <span className="min-w-0 truncate">{props.text.spriteStudio.saveError}</span>
      </span>
    );
  }
  return (
    <span
      className={cn(
        "inline-flex h-7 min-w-0 items-center gap-1.5 rounded-full border border-emerald-500/18 bg-emerald-500/10 px-2.5 text-xs font-semibold text-emerald-700 shadow-[inset_0_1px_0_hsl(var(--background)/0.38)] dark:text-emerald-200",
        props.className,
      )}
    >
      <CheckCircle2 className="h-3.5 w-3.5" />
      <span className="min-w-0 truncate">{props.text.spriteStudio.saved}</span>
    </span>
  );
}

export function useLoadedImage(imageUrl: string) {
  const [image, setImage] = React.useState<HTMLImageElement | null>(null);
  React.useEffect(() => {
    let active = true;
    setImage(null);
    if (!imageUrl) {
      return () => {
        active = false;
      };
    }
    void loadImage(imageUrl).then((nextImage) => {
      if (active) {
        setImage(nextImage);
      }
    });
    return () => {
      active = false;
    };
  }, [imageUrl]);
  return image;
}

export function loadImage(imageUrl: string) {
  const cached = imageCache.get(imageUrl);
  if (cached) {
    return cached;
  }
  const next = new Promise<HTMLImageElement | null>((resolve) => {
    const image = new Image();
    image.crossOrigin = "anonymous";
    image.decoding = "async";
    image.onload = () => resolve(image);
    image.onerror = () => resolve(null);
    image.src = imageUrl;
  });
  imageCache.set(imageUrl, next);
  return next;
}

export function normalizeGrid(
  grid: SpriteSliceGrid,
  columns: number,
  rows: number,
  sheetWidth: number,
  sheetHeight: number,
): SpriteSliceGrid {
  if (isValidGrid(grid, columns, rows)) {
    return cloneGrid(grid);
  }
  return defaultGrid(columns, rows, sheetWidth, sheetHeight);
}

export function defaultGrid(columns: number, rows: number, sheetWidth: number, sheetHeight: number): SpriteSliceGrid {
  return {
    x: equalBoundaries(Math.max(1, sheetWidth), Math.max(1, columns)),
    y: equalBoundaries(Math.max(1, sheetHeight), Math.max(1, rows)),
  };
}

export function equalBoundaries(total: number, segments: number) {
  return Array.from({ length: segments + 1 }, (_, index) => Math.round((total * index) / segments));
}

export function isValidGrid(grid: SpriteSliceGrid, columns: number, rows: number) {
  return isValidBoundaryList(grid.x, columns) && isValidBoundaryList(grid.y, rows);
}

export function isValidBoundaryList(values: number[], segments: number) {
  if (values.length !== segments + 1 || values[0] !== 0) {
    return false;
  }
  for (let index = 1; index < values.length; index += 1) {
    if (values[index] <= values[index - 1]) {
      return false;
    }
  }
  return true;
}

export function cloneGrid(grid: SpriteSliceGrid): SpriteSliceGrid {
  return {
    x: [...grid.x],
    y: [...grid.y],
  };
}

export function gridKey(grid: SpriteSliceGrid) {
  return JSON.stringify([grid.x, grid.y]);
}

export function clampInt(value: number, min: number, max: number) {
  return Math.min(Math.max(value, min), max);
}

export function buildSpriteMetadataDraft(sprite: Sprite): SpriteMetadataDraft {
  return {
    name: sprite.name,
    authorDisplayName: sprite.author.displayName,
    version: sprite.version || "1.0",
    description: sprite.description,
  };
}

export function formatDeleteMessage(template: string, spriteName: string) {
  return template.replace("{name}", spriteName);
}

export function formatCountText(template: string, count: number) {
  return template.replace("{count}", String(count));
}

export function buildSpriteArchivePath(directoryPath: string, spriteName: string) {
  const trimmedDirectory = directoryPath.trim().replace(/[\\/]+$/, "");
  const archiveName = `${sanitizeSpriteArchiveName(spriteName)}.zip`;
  if (!trimmedDirectory) {
    return archiveName;
  }
  const separator =
    trimmedDirectory.includes("\\") && !trimmedDirectory.includes("/")
      ? "\\"
      : "/";
  return `${trimmedDirectory}${separator}${archiveName}`;
}

export function sanitizeSpriteArchiveName(value: string) {
  const cleaned = value.replace(/[<>:"/\\|?*\u0000-\u001F]/g, "_").trim();
  return cleaned || "sprite";
}

export function resolveSpriteError(error: unknown) {
  return sanitizeSpriteErrorMessage(extractSpriteErrorMessage(error));
}

export function resolveSpriteImportFailure(error: unknown, text: XiaText) {
  const message = resolveSpriteError(error);
  return {
    title: text.spriteStudio.importFailedTitle,
    description: resolveSpriteImportFailureDescription(message, text),
  };
}

function resolveSpriteImportFailureDescription(message: string, text: XiaText) {
  const normalized = message.toLowerCase();
  if (!normalized) {
    return text.spriteStudio.importErrors.fallback;
  }
  if (
    normalized.includes("only supports png or zip") ||
    normalized.includes("unsupported file") ||
    normalized.includes("unsupported format")
  ) {
    return text.spriteStudio.importErrors.unsupportedFormat;
  }
  if (
    normalized.includes("open sprite archive") ||
    normalized.includes("not a valid zip file") ||
    normalized.includes("not a zip file") ||
    normalized.includes("zip: ")
  ) {
    return text.spriteStudio.importErrors.invalidArchive;
  }
  if (
    normalized.includes("must be at most") ||
    normalized.includes("archive contents exceed") ||
    normalized.includes("too large")
  ) {
    return text.spriteStudio.importErrors.archiveTooLarge;
  }
  if (
    normalized.includes("decode manifest") ||
    normalized.includes("sprite manifest app") ||
    normalized.includes("manifest") && normalized.includes("not supported")
  ) {
    return text.spriteStudio.importErrors.invalidManifest;
  }
  if (
    normalized.includes("does not contain a png sprite sheet") ||
    normalized.includes("sprite image not found") ||
    normalized.includes("sprite sheet") && normalized.includes("not found")
  ) {
    return text.spriteStudio.importErrors.missingSheet;
  }
  if (
    normalized.includes("decode sprite image") ||
    normalized.includes("decode image") ||
    normalized.includes("unknown format")
  ) {
    return text.spriteStudio.importErrors.invalidImage;
  }
  if (normalized.includes("must be square")) {
    return text.spriteStudio.importErrors.notSquare;
  }
  if (normalized.includes("too small") || normalized.includes("at least")) {
    return text.spriteStudio.importErrors.tooSmall;
  }
  if (normalized.includes("exceeds the maximum")) {
    return text.spriteStudio.importErrors.tooLarge;
  }
  if (normalized.includes("escapes the sprite directory")) {
    return text.spriteStudio.importErrors.invalidArchive;
  }
  return summarizeSpriteErrorMessage(message) || text.spriteStudio.importErrors.fallback;
}

function extractSpriteErrorMessage(error: unknown, depth = 0): string {
  if (depth > 4) {
    return "";
  }
  if (error instanceof Error) {
    return extractSpriteErrorMessage(error.message, depth + 1);
  }
  if (typeof error === "string") {
    return extractSpriteErrorMessageFromString(error, depth);
  }
  if (!error || typeof error !== "object") {
    return String(error ?? "");
  }

  const record = error as Record<string, unknown>;
  for (const key of ["message", "error", "detail", "reason", "description"]) {
    const value = record[key];
    const extracted = extractSpriteErrorMessage(value, depth + 1).trim();
    if (extracted) {
      return extracted;
    }
  }
  return "";
}

function extractSpriteErrorMessageFromString(value: string, depth: number) {
  const trimmed = value.trim();
  if (!trimmed) {
    return "";
  }

  const parsed = parseSpriteErrorJSON(trimmed);
  if (parsed !== null) {
    const extracted = extractSpriteErrorMessage(parsed, depth + 1).trim();
    if (extracted) {
      return extracted;
    }
  }

  const embeddedJSON = trimmed.match(/\{[\s\S]*\}/)?.[0] ?? "";
  if (embeddedJSON && embeddedJSON !== trimmed) {
    const parsedEmbedded = parseSpriteErrorJSON(embeddedJSON);
    if (parsedEmbedded !== null) {
      const extracted = extractSpriteErrorMessage(parsedEmbedded, depth + 1).trim();
      if (extracted) {
        return extracted;
      }
    }
  }

  return trimmed.replace(/^error:\s*/i, "").trim();
}

function parseSpriteErrorJSON(value: string) {
  if (!value.startsWith("{") && !value.startsWith("[")) {
    return null;
  }
  try {
    return JSON.parse(value) as unknown;
  } catch {
    return null;
  }
}

function sanitizeSpriteErrorMessage(message: string) {
  return summarizeSpriteErrorMessage(message.replace(/\s+/g, " ").trim());
}

function summarizeSpriteErrorMessage(message: string) {
  const cleaned = message.trim();
  if (!cleaned || cleaned.startsWith("{") || cleaned.startsWith("[")) {
    return "";
  }
  if (cleaned.length <= 120) {
    return cleaned;
  }
  return `${cleaned.slice(0, 117).trim()}...`;
}
