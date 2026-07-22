import {
  createContext,
  useEffect,
  useContext,
  type CSSProperties,
  type HTMLAttributes,
  type ReactNode,
} from "react";

import {
  COMPANION_PANEL_WIDTH,
  PRIMARY_PANE_DEFAULT_MIN_WIDTH,
  WORKSPACE_SIDEBAR_WIDTH,
  normalizeMinimumWidth,
  toPixelValue,
  workspaceRequiredWidth,
} from "@/app/workspace/layout";
import type { CompanionPresentation } from "@/app/workspace/layout";
import { cn } from "@/lib/utils";
import type { XiaSurfaceStyle } from "@/shared/styles/xiadown-theme";

import "./workspace.css";

interface WorkspaceShellStyle extends CSSProperties {
  "--app-workspace-navigation-width"?: string;
  "--app-workspace-primary-min-width"?: string;
  "--app-workspace-companion-width"?: string;
}

const WorkspaceSurfaceStyleContext =
  createContext<XiaSurfaceStyle>("glass");

export function useWorkspaceSurfaceStyle() {
  return useContext(WorkspaceSurfaceStyleContext);
}

export interface AppShellProps extends HTMLAttributes<HTMLDivElement> {
  navigation: ReactNode;
  primaryMinWidth?: number;
  companionOpen?: boolean;
  companionPresentation?: CompanionPresentation;
  ambientArtworkURL?: string | null;
  surfaceStyle?: XiaSurfaceStyle;
  onMinimumWidthChange?: (minimumWidth: number) => void;
}

/**
 * Product-level shell. `onMinimumWidthChange` is the bridge point for the
 * native window layer: when the companion opens, the required width grows by
 * the fixed companion width instead of shrinking the primary pane.
 */
export function AppShell({
  navigation,
  primaryMinWidth = PRIMARY_PANE_DEFAULT_MIN_WIDTH,
  companionOpen = false,
  companionPresentation = "docked",
  ambientArtworkURL,
  surfaceStyle = "glass",
  onMinimumWidthChange,
  className,
  style,
  children,
  ...props
}: AppShellProps) {
  const normalizedAmbientArtworkURL = ambientArtworkURL?.trim() || undefined;
  const normalizedPrimaryMinWidth = normalizeMinimumWidth(primaryMinWidth);
  const minimumWidth = workspaceRequiredWidth({
    primaryMinWidth: normalizedPrimaryMinWidth,
    companionOpen,
    companionPresentation,
  });

  useEffect(() => {
    onMinimumWidthChange?.(minimumWidth);
  }, [minimumWidth, onMinimumWidthChange]);

  const shellStyle: WorkspaceShellStyle = {
    ...style,
    "--app-workspace-navigation-width": toPixelValue(WORKSPACE_SIDEBAR_WIDTH),
    "--app-workspace-primary-min-width": toPixelValue(
      normalizedPrimaryMinWidth,
    ),
    "--app-workspace-companion-width": toPixelValue(COMPANION_PANEL_WIDTH),
    minWidth: toPixelValue(minimumWidth),
  };

  return (
    <WorkspaceSurfaceStyleContext.Provider value={surfaceStyle}>
      <div
        {...props}
        className={cn("app-workspace-shell", className)}
        data-companion-open={companionOpen}
        data-companion-presentation={companionPresentation}
        data-required-width={minimumWidth}
        data-surface-role="canvas"
        data-surface-style={surfaceStyle}
        style={shellStyle}
      >
        <div
          aria-hidden="true"
          className="app-workspace-ambient-canvas"
          data-has-artwork={normalizedAmbientArtworkURL ? "true" : "false"}
        >
          {normalizedAmbientArtworkURL ? (
            <img
              alt=""
              className="app-workspace-ambient-canvas__artwork"
              draggable={false}
              src={normalizedAmbientArtworkURL}
            />
          ) : null}
        </div>
        {navigation}
        {children}
      </div>
    </WorkspaceSurfaceStyleContext.Provider>
  );
}
