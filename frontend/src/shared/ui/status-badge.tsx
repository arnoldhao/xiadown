import * as React from "react";

import { cn } from "@/lib/utils";

export const DREAM_STATUS_TONES = [
  "neutral",
  "accent",
  "busy",
  "success",
  "warning",
  "danger",
  "muted",
] as const;
export type DreamStatusTone = (typeof DREAM_STATUS_TONES)[number];

export interface StatusBadgeProps
  extends React.HTMLAttributes<HTMLSpanElement> {
  tone?: DreamStatusTone;
  icon?: React.ReactNode;
  iconOnly?: boolean;
  marker?: boolean;
}

/**
 * Canonical compact state label. Product code supplies a localized label and
 * semantic tone; Dream CSS owns geometry, color, contrast, and icon sizing.
 */
export const StatusBadge = React.forwardRef<HTMLSpanElement, StatusBadgeProps>(
  function StatusBadge(
    {
      children,
      className,
      icon,
      iconOnly = false,
      marker = false,
      tone = "neutral",
      ...props
    },
    ref,
  ) {
    return (
      <span
        ref={ref}
        className={cn("app-dream-status-badge", className)}
        data-app-status-badge="true"
        data-icon-only={iconOnly ? "true" : undefined}
        data-tone={tone}
        {...props}
      >
        {icon ? (
          <span className="app-dream-status-badge__icon" aria-hidden="true">
            {icon}
          </span>
        ) : marker ? (
          <span className="app-dream-status-badge__marker" aria-hidden="true" />
        ) : null}
        {!iconOnly ? (
          <span className="app-dream-status-badge__label">{children}</span>
        ) : null}
      </span>
    );
  },
);
