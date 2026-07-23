import * as React from "react";

import { Button } from "@/shared/ui/button";
import { useI18n } from "@/shared/i18n";
import { cn } from "@/lib/utils";
import { registerWindowsTitlebarDoubleClick } from "@/components/layout/windows-titlebar";

let wailsRuntimePromise: Promise<typeof import("@wailsio/runtime")> | undefined;

function loadWailsRuntime() {
  wailsRuntimePromise ??= import("@wailsio/runtime");
  return wailsRuntimePromise;
}

export interface WindowControlsProps {
  platform: "macos" | "windows";
  owner?: WindowControlsOwner;
  className?: string;
  runtimeEnabled?: boolean;
}

export type WindowControlsOwner =
  | "primary"
  | "companion"
  | "fullscreen"
  | "settings"
  | "welcome";

function WindowsMinimiseGlyph() {
  return <span className="app-window-control-glyph app-window-control-glyph--minimize" />;
}

function WindowsMaximiseGlyph() {
  return <span className="app-window-control-glyph app-window-control-glyph--maximize" />;
}

function WindowsRestoreGlyph() {
  return (
    <span className="app-window-control-glyph app-window-control-glyph--restore">
      <span data-corner="upper" />
      <span data-corner="lower" />
    </span>
  );
}

function WindowsCloseGlyph() {
  return (
    <span className="app-window-control-glyph app-window-control-glyph--close">
      <span data-stroke="forward" />
      <span data-stroke="backward" />
    </span>
  );
}

export function WindowControls({
  platform,
  owner = "primary",
  className,
  runtimeEnabled = true,
}: WindowControlsProps) {
  const { t } = useI18n();
  const [isMaximised, setIsMaximised] = React.useState(false);
  const closeLabel = t("common.closeWindow");
  const minimizeLabel = t("common.minimizeWindow");
  const maximizeLabel = t("common.maximizeWindow");
  const restoreLabel = t("common.restoreWindow");

  React.useEffect(() => {
    if (platform !== "windows" || !runtimeEnabled) {
      setIsMaximised(false);
      return;
    }

    let cancelled = false;
    let disposeRuntimeListeners = () => undefined;

    void loadWailsRuntime()
      .then(({ Events, Window }) => {
        if (cancelled) {
          return;
        }

        const syncMaximised = async () => {
          try {
            const next = await Window.IsMaximised();
            if (!cancelled) {
              setIsMaximised(Boolean(next));
            }
          } catch {
            if (!cancelled) {
              setIsMaximised(false);
            }
          }
        };

        void syncMaximised();

        const offMaximise = Events.On(
          Events.Types.Common.WindowMaximise,
          () => {
            setIsMaximised(true);
          },
        );
        const offUnMaximise = Events.On(
          Events.Types.Common.WindowUnMaximise,
          () => {
            setIsMaximised(false);
          },
        );
        const offRestore = Events.On(Events.Types.Common.WindowRestore, () => {
          void syncMaximised();
        });

        disposeRuntimeListeners = () => {
          offMaximise();
          offUnMaximise();
          offRestore();
        };
      })
      .catch(() => {
        if (!cancelled) {
          setIsMaximised(false);
        }
      });

    return () => {
      cancelled = true;
      disposeRuntimeListeners();
    };
  }, [platform, runtimeEnabled]);

  React.useEffect(() => {
    if (
      platform !== "windows" ||
      !runtimeEnabled ||
      typeof document === "undefined"
    ) {
      return;
    }
    return registerWindowsTitlebarDoubleClick(document, () => {
      void loadWailsRuntime()
        .then(({ Window }) => Window.ToggleMaximise())
        .catch(() => undefined);
    });
  }, [platform, runtimeEnabled]);

  const handleClose = () => {
    if (!runtimeEnabled) return;
    void loadWailsRuntime()
      .then(({ Window }) => Window.Close())
      .catch(() => undefined);
  };

  const handleMinimize = () => {
    if (!runtimeEnabled) return;
    void loadWailsRuntime()
      .then(({ Window }) => Window.Minimise())
      .catch(() => undefined);
  };

  const handleToggleMaximize = () => {
    if (!runtimeEnabled) return;
    void loadWailsRuntime()
      .then(({ Window }) => Window.ToggleMaximise())
      .catch(() => undefined);
  };

  if (platform === "macos") {
    return (
      <div
        className={cn(
          "app-window-controls wails-no-drag relative z-[var(--app-layer-window-controls)] flex items-center gap-2",
          className,
        )}
        data-window-controls-owner={owner}
        data-window-controls-platform={platform}
      >
        <Button
          type="button"
          variant="ghost"
          size="icon"
          shape="circle"
          tone="neutral"
          className="app-window-control app-window-control--macos-close wails-no-drag"
          onClick={handleClose}
          aria-label={closeLabel}
        />
        <Button
          type="button"
          variant="ghost"
          size="icon"
          shape="circle"
          tone="neutral"
          className="app-window-control app-window-control--macos-minimize wails-no-drag"
          onClick={handleMinimize}
          aria-label={minimizeLabel}
        />
        <Button
          type="button"
          variant="ghost"
          size="icon"
          shape="circle"
          tone="neutral"
          className="app-window-control app-window-control--macos-maximize wails-no-drag"
          onClick={handleToggleMaximize}
          aria-label={maximizeLabel}
        />
      </div>
    );
  }

  return (
    <div
      className={cn(
        "app-window-controls wails-no-drag pointer-events-auto relative z-[var(--app-layer-window-controls)] flex h-[var(--app-windows-caption-button-height,var(--app-titlebar-height))] w-[var(--app-windows-caption-control-width)] shrink-0 self-start items-stretch",
        className,
      )}
      data-window-controls-owner={owner}
      data-window-controls-platform={platform}
    >
      <Button
        type="button"
        variant="ghost"
        size="icon"
        shape="square"
        tone="neutral"
        className="app-window-control app-window-control--windows wails-no-drag"
        onClick={handleMinimize}
        aria-label={minimizeLabel}
      >
        <WindowsMinimiseGlyph />
      </Button>
      <Button
        type="button"
        variant="ghost"
        size="icon"
        shape="square"
        tone="neutral"
        className="app-window-control app-window-control--windows wails-no-drag"
        onClick={handleToggleMaximize}
        aria-label={isMaximised ? restoreLabel : maximizeLabel}
      >
        {isMaximised ? <WindowsRestoreGlyph /> : <WindowsMaximiseGlyph />}
      </Button>
      <Button
        type="button"
        variant="ghost"
        size="icon"
        shape="square"
        tone="neutral"
        className="app-window-control app-window-control--windows app-window-control--windows-close wails-no-drag"
        onClick={handleClose}
        aria-label={closeLabel}
      >
        <WindowsCloseGlyph />
      </Button>
    </div>
  );
}
