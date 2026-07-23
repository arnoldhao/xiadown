import type { CSSProperties, HTMLAttributes } from "react";

import {
  COMPANION_PANEL_WIDTH,
  PRIMARY_PANE_DEFAULT_MIN_WIDTH,
  normalizeMinimumWidth,
  toPixelValue,
  workspaceStageRequiredWidth,
} from "@/app/workspace/layout";
import type { CompanionPresentation } from "@/app/workspace/layout";
import { cn } from "@/lib/utils";

interface WorkspaceStageStyle extends CSSProperties {
  "--app-workspace-primary-min-width"?: string;
  "--app-workspace-companion-width"?: string;
}

export interface WorkspaceStageProps extends HTMLAttributes<HTMLDivElement> {
  primaryMinWidth?: number;
  companionOpen?: boolean;
  companionPresentation?: CompanionPresentation;
}

export function WorkspaceStage({
  primaryMinWidth = PRIMARY_PANE_DEFAULT_MIN_WIDTH,
  companionOpen = false,
  companionPresentation = "docked",
  className,
  style,
  ...props
}: WorkspaceStageProps) {
  const normalizedPrimaryMinWidth = normalizeMinimumWidth(primaryMinWidth);
  const minimumWidth = workspaceStageRequiredWidth({
    primaryMinWidth: normalizedPrimaryMinWidth,
    companionOpen,
    companionPresentation,
  });
  const stageStyle: WorkspaceStageStyle = {
    ...style,
    "--app-workspace-primary-min-width": toPixelValue(
      normalizedPrimaryMinWidth,
    ),
    "--app-workspace-companion-width": toPixelValue(COMPANION_PANEL_WIDTH),
    minWidth: toPixelValue(minimumWidth),
  };

  return (
    <div
      {...props}
      className={cn("app-workspace-stage", className)}
      data-companion-open={companionOpen}
      data-companion-presentation={companionPresentation}
      data-required-width={minimumWidth}
      style={stageStyle}
    />
  );
}
