import * as React from "react";

import { cn } from "@/lib/utils";
import { Button, type ButtonProps } from "@/shared/ui/button";
import { DropdownMenuContent } from "@/shared/ui/dropdown-menu";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/shared/ui/tooltip";

export type WorkspacePrimaryHeaderActionProps = Omit<
  ButtonProps,
  | "aria-label"
  | "asChild"
  | "children"
  | "shape"
  | "size"
  | "title"
  | "type"
  | "variant"
> & {
  /** A title action always owns a real button so Radix trigger composition remains safe. */
  asChild?: never;
  label: string;
  children: React.ReactNode;
};

export interface WorkspacePrimaryHeaderActionGroupProps
  extends Omit<React.HTMLAttributes<HTMLDivElement>, "aria-label"> {
  /** Localized name for this related set of title actions. */
  label: string;
}

/**
 * Groups related borderless title actions by semantics and proximity. The
 * shared layout keeps actions inside a group compact and leaves a larger gap
 * between groups, avoiding decorative dividers in the window drag rail.
 */
export function WorkspacePrimaryHeaderActionGroup({
  children,
  className,
  label,
  ...props
}: WorkspacePrimaryHeaderActionGroupProps) {
  return (
    <div
      {...props}
      aria-label={label}
      className={cn(
        "app-workspace-primary-header-action-group wails-no-drag",
        className,
      )}
      role="group"
    >
      {children}
    </div>
  );
}

export type WorkspacePrimaryHeaderMenuContentProps = Omit<
  React.ComponentPropsWithoutRef<typeof DropdownMenuContent>,
  "align" | "alignOffset" | "side"
>;

/**
 * Canonical menu placement for a title action. Title menus always open below
 * and centered on their trigger so edge alignment never varies by Station.
 */
export const WorkspacePrimaryHeaderMenuContent = React.forwardRef<
  React.ElementRef<typeof DropdownMenuContent>,
  WorkspacePrimaryHeaderMenuContentProps
>(function WorkspacePrimaryHeaderMenuContent(
  { className, sideOffset = 8, ...props },
  ref,
) {
  return (
    <DropdownMenuContent
      {...props}
      ref={ref}
      align="center"
      className={cn("app-workspace-primary-header-menu", className)}
      side="bottom"
      sideOffset={sideOffset}
    />
  );
});

WorkspacePrimaryHeaderMenuContent.displayName =
  "WorkspacePrimaryHeaderMenuContent";

/**
 * Canonical icon-only action for a Station Primary header. The visible icon,
 * accessible name, tooltip, hit target, shape, and drag exclusion stay
 * identical across Library, Music, RSS, YouTube, and future Stations.
 */
export const WorkspacePrimaryHeaderAction = React.forwardRef<
  HTMLButtonElement,
  WorkspacePrimaryHeaderActionProps
>(function WorkspacePrimaryHeaderAction(
  { asChild: _asChild, children, className, label, ...buttonProps },
  ref,
) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <Button
          {...buttonProps}
          ref={ref}
          aria-label={label}
          className={cn(
            "app-workspace-primary-header-action wails-no-drag",
            className,
          )}
          shape="circle"
          size="compactIcon"
          type="button"
          variant="ghost"
        >
          {children}
        </Button>
      </TooltipTrigger>
      <TooltipContent side="bottom">{label}</TooltipContent>
    </Tooltip>
  );
});

WorkspacePrimaryHeaderAction.displayName = "WorkspacePrimaryHeaderAction";
