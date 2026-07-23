import type { HTMLAttributes, ReactNode } from "react";

import { useWorkspaceSurfaceStyle } from "@/app/workspace/AppShell";
import { cn } from "@/lib/utils";
import { GlassSurface } from "@/shared/ui/glass-surface";

export interface WorkspaceSidebarProps extends HTMLAttributes<HTMLElement> {
  header?: ReactNode;
  bottom?: ReactNode;
  footer?: ReactNode;
  glass?: boolean;
}

export function WorkspaceSidebar({
  header,
  bottom,
  footer,
  glass = true,
  children,
  className,
  "aria-label": ariaLabel = "Workspace navigation",
  ...props
}: WorkspaceSidebarProps) {
  const surfaceStyle = useWorkspaceSurfaceStyle();
  const glassHost = surfaceStyle === "glass" && glass;

  return (
    <aside
      {...props}
      aria-label={ariaLabel}
      className={cn("app-workspace-sidebar", className)}
      data-glass-host={glassHost ? "true" : "false"}
      data-surface-role="chrome"
      data-surface-style={surfaceStyle}
    >
      {glassHost ? (
        <GlassSurface
          aria-hidden="true"
          className="app-workspace-chrome-material"
          data-glass-role="sidebar"
          elevation="embedded"
          shape="panel"
          surfaceRole="chrome"
          tint="neutral"
        />
      ) : null}
      {header ? (
        <div className="app-workspace-sidebar__header">{header}</div>
      ) : null}
      <div className="app-workspace-sidebar__navigation">{children}</div>
      {bottom ? (
        <div className="app-workspace-sidebar__bottom">{bottom}</div>
      ) : null}
      {footer ? (
        <div className="app-workspace-sidebar__footer">{footer}</div>
      ) : null}
    </aside>
  );
}
