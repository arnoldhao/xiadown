import * as React from "react";

import {
  DropdownMenu as BaseDropdownMenu,
  DropdownMenuTrigger as BaseDropdownMenuTrigger,
  DropdownMenuContent as BaseDropdownMenuContent,
  DropdownMenuItem as BaseDropdownMenuItem,
  DropdownMenuCheckboxItem as BaseDropdownMenuCheckboxItem,
  DropdownMenuRadioGroup as BaseDropdownMenuRadioGroup,
  DropdownMenuRadioItem as BaseDropdownMenuRadioItem,
  DropdownMenuLabel as BaseDropdownMenuLabel,
  DropdownMenuSeparator as BaseDropdownMenuSeparator,
} from "@/components/ui/dropdown-menu";
import { cn } from "@/lib/utils";
import { getXiaSurfaceAttributes } from "@/shared/ui/surface-contract";

const DropdownMenu = BaseDropdownMenu;
const DropdownMenuTrigger = BaseDropdownMenuTrigger;

export const APP_DROPDOWN_MENU_ITEM_TONES = [
  "neutral",
  "destructive",
] as const;
export type AppDropdownMenuItemTone =
  (typeof APP_DROPDOWN_MENU_ITEM_TONES)[number];

const DropdownMenuContent = React.forwardRef<
  React.ElementRef<typeof BaseDropdownMenuContent>,
  React.ComponentPropsWithoutRef<typeof BaseDropdownMenuContent>
>(({ className, sideOffset = 6, ...props }, ref) => (
  <BaseDropdownMenuContent
    {...props}
    ref={ref}
    sideOffset={sideOffset}
    className={cn(
      "app-glass-surface app-menu-content app-motion-surface",
      className,
    )}
    data-menu-part="content"
    data-elevation="floating"
    data-shape="control"
    data-tint="neutral"
    {...getXiaSurfaceAttributes("overlay")}
  />
));
DropdownMenuContent.displayName = "DropdownMenuContent";

const DropdownMenuItem = React.forwardRef<
  React.ElementRef<typeof BaseDropdownMenuItem>,
  React.ComponentPropsWithoutRef<typeof BaseDropdownMenuItem> & {
    tone?: AppDropdownMenuItemTone;
  }
>(({ className, tone = "neutral", ...props }, ref) => (
  <BaseDropdownMenuItem
    {...props}
    ref={ref}
    className={cn("app-menu-item app-motion-color", className)}
    data-menu-part="item"
    data-tone={tone}
  />
));
DropdownMenuItem.displayName = "DropdownMenuItem";

const DropdownMenuCheckboxItem = React.forwardRef<
  React.ElementRef<typeof BaseDropdownMenuCheckboxItem>,
  React.ComponentPropsWithoutRef<typeof BaseDropdownMenuCheckboxItem>
>(({ className, ...props }, ref) => (
  <BaseDropdownMenuCheckboxItem
    {...props}
    ref={ref}
    className={cn("app-menu-item app-motion-color", className)}
    data-menu-part="checkbox-item"
  />
));
DropdownMenuCheckboxItem.displayName = "DropdownMenuCheckboxItem";

const DropdownMenuRadioGroup = BaseDropdownMenuRadioGroup;

const DropdownMenuRadioItem = React.forwardRef<
  React.ElementRef<typeof BaseDropdownMenuRadioItem>,
  React.ComponentPropsWithoutRef<typeof BaseDropdownMenuRadioItem>
>(({ className, ...props }, ref) => (
  <BaseDropdownMenuRadioItem
    {...props}
    ref={ref}
    className={cn("app-menu-item app-menu-item--radio app-motion-color", className)}
    data-menu-part="radio-item"
  />
));
DropdownMenuRadioItem.displayName = "DropdownMenuRadioItem";

const DropdownMenuLabel = React.forwardRef<
  React.ElementRef<typeof BaseDropdownMenuLabel>,
  React.ComponentPropsWithoutRef<typeof BaseDropdownMenuLabel>
>(({ className, ...props }, ref) => (
  <BaseDropdownMenuLabel
    {...props}
    ref={ref}
    className={cn("app-menu-label", className)}
    data-menu-part="label"
  />
));
DropdownMenuLabel.displayName = "DropdownMenuLabel";

const DropdownMenuSeparator = React.forwardRef<
  React.ElementRef<typeof BaseDropdownMenuSeparator>,
  React.ComponentPropsWithoutRef<typeof BaseDropdownMenuSeparator>
>(({ className, ...props }, ref) => (
  <BaseDropdownMenuSeparator
    ref={ref}
    className={cn("app-menu-separator", className)}
    {...props}
  />
));
DropdownMenuSeparator.displayName = "DropdownMenuSeparator";

function DropdownMenuShortcut({
  className,
  ...props
}: React.HTMLAttributes<HTMLSpanElement>) {
  return (
    <span
      {...props}
      className={cn("app-menu-shortcut", className)}
      data-menu-part="shortcut"
    />
  );
}

export {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuCheckboxItem,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuShortcut,
};
