import type { HTMLAttributes } from "react";

import { cn } from "@/lib/utils";

export type ActivityDockProps = HTMLAttributes<HTMLElement>;

export function ActivityDock({
  children,
  className,
  "aria-label": ariaLabel = "Current activity",
  ...props
}: ActivityDockProps) {
  if (children == null) {
    return null;
  }

  return (
    <section
      {...props}
      aria-label={ariaLabel}
      className={cn("app-workspace-activity-dock", className)}
    >
      {children}
    </section>
  );
}
