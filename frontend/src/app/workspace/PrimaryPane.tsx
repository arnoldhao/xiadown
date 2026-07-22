import {
  type CSSProperties,
  type ElementType,
  type HTMLAttributes,
} from "react";

import {
  PRIMARY_PANE_DEFAULT_MIN_WIDTH,
  normalizeMinimumWidth,
  toPixelValue,
} from "@/app/workspace/layout";
import { useWorkspaceSurfaceStyle } from "@/app/workspace/AppShell";
import { cn } from "@/lib/utils";

interface PrimaryPaneStyle extends CSSProperties {
  "--app-workspace-primary-min-width"?: string;
}

export interface PrimaryPaneProps extends HTMLAttributes<HTMLElement> {
  as?: "main" | "section" | "div";
  minimumWidth?: number;
}

export function PrimaryPane({
  as = "main",
  minimumWidth = PRIMARY_PANE_DEFAULT_MIN_WIDTH,
  className,
  style,
  ...props
}: PrimaryPaneProps) {
  const surfaceStyle = useWorkspaceSurfaceStyle();
  const Component: ElementType = as;
  const normalizedMinimumWidth = normalizeMinimumWidth(minimumWidth);
  const paneStyle: PrimaryPaneStyle = {
    ...style,
    "--app-workspace-primary-min-width": toPixelValue(normalizedMinimumWidth),
    minWidth: toPixelValue(normalizedMinimumWidth),
  };

  return (
    <Component
      {...props}
      className={cn("app-workspace-primary-pane", className)}
      data-minimum-width={normalizedMinimumWidth}
      data-surface-density="high"
      data-surface-role="content"
      data-surface-style={surfaceStyle}
      style={paneStyle}
    />
  );
}
