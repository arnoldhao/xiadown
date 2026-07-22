import * as React from "react";

import { Badge as BaseBadge, badgeVariants } from "@/components/ui/badge";
import { cn } from "@/lib/utils";

export type BadgeProps = React.ComponentPropsWithoutRef<typeof BaseBadge>;

export const APP_BADGE_VARIANTS = [
  "default",
  "secondary",
  "destructive",
  "outline",
  "subtle",
  "ghost",
] as const satisfies readonly NonNullable<BadgeProps["variant"]>[];

export function Badge({ className, ...props }: BadgeProps) {
  return <BaseBadge className={cn("app-dream-badge app-motion-color", className)} {...props} />;
}

export { badgeVariants };
