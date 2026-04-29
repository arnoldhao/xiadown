import * as React from "react";

import {
  DropdownMenu as BaseDropdownMenu,
  DropdownMenuTrigger as BaseDropdownMenuTrigger,
  DropdownMenuContent as BaseDropdownMenuContent,
  DropdownMenuItem as BaseDropdownMenuItem,
  DropdownMenuCheckboxItem as BaseDropdownMenuCheckboxItem,
  DropdownMenuLabel as BaseDropdownMenuLabel,
  DropdownMenuSeparator as BaseDropdownMenuSeparator,
} from "@/components/ui/dropdown-menu";
import { cn } from "@/lib/utils";

const DropdownMenu = BaseDropdownMenu;
const DropdownMenuTrigger = BaseDropdownMenuTrigger;

const DropdownMenuContent = React.forwardRef<
  React.ElementRef<typeof BaseDropdownMenuContent>,
  React.ComponentPropsWithoutRef<typeof BaseDropdownMenuContent>
>(({ className, sideOffset = 6, ...props }, ref) => (
  <BaseDropdownMenuContent
    ref={ref}
    sideOffset={sideOffset}
    className={cn("app-menu-content app-motion-surface text-xs", className)}
    {...props}
  />
));
DropdownMenuContent.displayName = "DropdownMenuContent";

const DropdownMenuItem = React.forwardRef<
  React.ElementRef<typeof BaseDropdownMenuItem>,
  React.ComponentPropsWithoutRef<typeof BaseDropdownMenuItem>
>(({ className, ...props }, ref) => (
  <BaseDropdownMenuItem
    ref={ref}
    className={cn("app-menu-item app-motion-color text-xs leading-[1.35]", className)}
    {...props}
  />
));
DropdownMenuItem.displayName = "DropdownMenuItem";

const DropdownMenuCheckboxItem = React.forwardRef<
  React.ElementRef<typeof BaseDropdownMenuCheckboxItem>,
  React.ComponentPropsWithoutRef<typeof BaseDropdownMenuCheckboxItem>
>(({ className, ...props }, ref) => (
  <BaseDropdownMenuCheckboxItem
    ref={ref}
    className={cn("app-menu-item app-motion-color text-xs leading-[1.35]", className)}
    {...props}
  />
));
DropdownMenuCheckboxItem.displayName = "DropdownMenuCheckboxItem";

const DropdownMenuLabel = React.forwardRef<
  React.ElementRef<typeof BaseDropdownMenuLabel>,
  React.ComponentPropsWithoutRef<typeof BaseDropdownMenuLabel>
>(({ className, ...props }, ref) => (
  <BaseDropdownMenuLabel
    ref={ref}
    className={cn("app-menu-label text-2xs font-semibold uppercase tracking-[0.08em] text-muted-foreground", className)}
    {...props}
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
  return <span className={cn("app-menu-shortcut text-2xs tracking-[0.08em] text-muted-foreground", className)} {...props} />;
}

export {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuCheckboxItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuShortcut,
};
