import * as React from "react";
import { cva, type VariantProps } from "class-variance-authority";

import { cn } from "@/lib/utils";

const badgeVariants = cva(
  "app-base-badge",
  {
    variants: {
      variant: {
        default: "app-base-badge--default",
        secondary: "app-base-badge--secondary",
        destructive: "app-base-badge--destructive",
        outline: "app-base-badge--outline",
        subtle: "app-base-badge--subtle",
        ghost: "app-base-badge--ghost",
      },
    },
    defaultVariants: {
      variant: "default",
    },
  },
);

function Badge({
  className,
  variant,
  ...props
}: React.HTMLAttributes<HTMLDivElement> & VariantProps<typeof badgeVariants>) {
  const resolvedVariant = variant ?? "default";
  return (
    <div
      className={cn(badgeVariants({ variant: resolvedVariant }), className)}
      {...props}
      data-variant={resolvedVariant}
    />
  );
}

export { Badge, badgeVariants };
