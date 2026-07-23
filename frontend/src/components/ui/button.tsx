import * as React from "react";
import { Slot } from "@radix-ui/react-slot";
import { cva, type VariantProps } from "class-variance-authority";

import { cn } from "@/lib/utils";

const buttonVariants = cva(
  "app-base-button",
  {
    variants: {
      variant: {
        default: "app-base-button--default",
        destructive: "app-base-button--destructive",
        outline: "app-base-button--outline",
        secondary: "app-base-button--secondary",
        ghost: "app-base-button--ghost",
        link: "app-base-button--link",
      },
      size: {
        default: "app-base-button-size--default",
        sm: "app-base-button-size--sm",
        lg: "app-base-button-size--lg",
        icon: "app-base-button-size--icon",
      },
    },
    defaultVariants: {
      variant: "default",
      size: "default",
    },
  },
);

export interface ButtonProps
  extends
    React.ButtonHTMLAttributes<HTMLButtonElement>,
    VariantProps<typeof buttonVariants> {
  asChild?: boolean;
}

const Button = React.forwardRef<HTMLButtonElement, ButtonProps>(
  ({ className, variant, size, asChild = false, ...props }, ref) => {
    const Comp = asChild ? Slot : "button";
    const resolvedVariant = variant ?? "default";
    const resolvedSize = size ?? "default";
    return (
      <Comp
        className={cn(
          buttonVariants({
            variant: resolvedVariant,
            size: resolvedSize,
            className,
          }),
        )}
        data-base-size={resolvedSize}
        data-base-variant={resolvedVariant}
        ref={ref}
        {...props}
      />
    );
  },
);
Button.displayName = "Button";

export { Button, buttonVariants };
