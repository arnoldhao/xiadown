import * as PopoverPrimitive from "@radix-ui/react-popover";
import * as React from "react";

import { cn } from "@/lib/utils";
import { getXiaSurfaceAttributes } from "@/shared/ui/surface-contract";

const Popover = PopoverPrimitive.Root;
const PopoverTrigger = PopoverPrimitive.Trigger;
const PopoverAnchor = PopoverPrimitive.Anchor;
const PopoverClose = PopoverPrimitive.Close;

type PopoverContentProps =
  React.ComponentPropsWithoutRef<typeof PopoverPrimitive.Content> & {
    portalContainer?: HTMLElement | null;
  };

const PopoverContent = React.forwardRef<
  React.ElementRef<typeof PopoverPrimitive.Content>,
  PopoverContentProps
>(
  (
    {
      align = "center",
      className,
      portalContainer,
      sideOffset = 6,
      ...props
    },
    ref,
  ) => (
    <PopoverPrimitive.Portal container={portalContainer ?? undefined}>
      <PopoverPrimitive.Content
        {...props}
        ref={ref}
        align={align}
        sideOffset={sideOffset}
        className={cn(
          "app-glass-surface app-popover-content app-motion-surface",
          className,
        )}
        data-elevation="floating"
        data-shape="control"
        data-tint="neutral"
        {...getXiaSurfaceAttributes("overlay")}
      />
    </PopoverPrimitive.Portal>
  ),
);
PopoverContent.displayName = "PopoverContent";

export {
  Popover,
  PopoverAnchor,
  PopoverClose,
  PopoverContent,
  PopoverTrigger,
};
