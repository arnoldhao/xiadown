import * as React from "react";

import { cn } from "@/lib/utils";

const SidebarMenu = React.forwardRef<
  HTMLUListElement,
  React.ComponentPropsWithoutRef<"ul">
>(({ className, ...props }, ref) => (
  <ul
    ref={ref}
    className={cn("app-sidebar-menu", className)}
    {...props}
  />
));
SidebarMenu.displayName = "SidebarMenu";

const SidebarMenuItem = React.forwardRef<
  HTMLLIElement,
  React.ComponentPropsWithoutRef<"li">
>(({ className, ...props }, ref) => (
  <li ref={ref} className={cn("app-sidebar-menu__item", className)} {...props} />
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
    className={cn("app-sidebar-menu__button", className)}
    {...props}
  />
));
SidebarMenuButton.displayName = "SidebarMenuButton";

export { SidebarMenu, SidebarMenuButton, SidebarMenuItem };
