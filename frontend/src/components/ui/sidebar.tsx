import * as React from "react";

import { cn } from "@/lib/utils";

const SidebarMenu = React.forwardRef<
  HTMLUListElement,
  React.ComponentPropsWithoutRef<"ul">
>(({ className, ...props }, ref) => (
  <ul
    ref={ref}
    className={cn("flex w-full min-w-0 flex-col gap-1", className)}
    {...props}
  />
));
SidebarMenu.displayName = "SidebarMenu";

const SidebarMenuItem = React.forwardRef<
  HTMLLIElement,
  React.ComponentPropsWithoutRef<"li">
>(({ className, ...props }, ref) => (
  <li ref={ref} className={cn("relative", className)} {...props} />
));
SidebarMenuItem.displayName = "SidebarMenuItem";

const SidebarMenuButton = React.forwardRef<
  HTMLButtonElement,
  React.ComponentPropsWithoutRef<"button"> & { isActive?: boolean }
>(({ className, isActive, type = "button", ...props }, ref) => (
  <button
    ref={ref}
    type={type}
    data-active={isActive ? "true" : "false"}
    className={cn(
      "flex min-h-10 w-full items-center gap-3 rounded-2xl px-3 py-2 text-left text-sm font-medium text-muted-foreground transition",
      "hover:bg-muted/70 hover:text-foreground focus-visible:outline-none",
      "data-[active=true]:bg-primary/10 data-[active=true]:text-primary",
      className,
    )}
    {...props}
  />
));
SidebarMenuButton.displayName = "SidebarMenuButton";

export { SidebarMenu, SidebarMenuButton, SidebarMenuItem };
