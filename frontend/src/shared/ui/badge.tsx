import * as React from "react";

import { Badge as BaseBadge, badgeVariants } from "@/components/ui/badge";
import { cn } from "@/lib/utils";

export type BadgeProps = React.ComponentPropsWithoutRef<typeof BaseBadge>;

export function Badge({ className, ...props }: BadgeProps) {
  return <BaseBadge className={cn("app-motion-color", className)} {...props} />;
}

export { badgeVariants };
