import { Slot } from "@radix-ui/react-slot";
import * as React from "react";

import { cn } from "@/lib/utils";
import {
  getXiaSurfaceAttributes,
  type GlassMaterial,
  type XiaSurfaceRole,
} from "@/shared/ui/surface-contract";

export type { GlassMaterial, XiaSurfaceRole } from "@/shared/ui/surface-contract";
export const GLASS_ELEVATIONS = ["embedded", "floating", "modal"] as const;
export type GlassElevation = (typeof GLASS_ELEVATIONS)[number];
export const GLASS_SHAPES = ["control", "card", "panel", "capsule"] as const;
export type GlassShape = (typeof GLASS_SHAPES)[number];
export const GLASS_TINTS = ["neutral", "accent", "artwork"] as const;
export type GlassTint = (typeof GLASS_TINTS)[number];

export interface GlassSurfaceProps
  extends React.HTMLAttributes<HTMLDivElement> {
  asChild?: boolean;
  /** Visual surface intent; independent from the element's ARIA `role`. */
  surfaceRole?: XiaSurfaceRole;
  material?: GlassMaterial;
  elevation?: GlassElevation;
  shape?: GlassShape;
  tint?: GlassTint;
  interactive?: boolean;
  focusRing?: boolean;
}

export const GlassSurface = React.forwardRef<HTMLDivElement, GlassSurfaceProps>(
  (
    {
      asChild = false,
      surfaceRole,
      material = "regular",
      elevation = "floating",
      shape = "panel",
      tint = "neutral",
      interactive = false,
      focusRing = false,
      className,
      ...props
    },
    ref,
  ) => {
    const Component = asChild ? Slot : "div";

    return (
      <Component
        ref={ref}
        className={cn("app-glass-surface", className)}
        data-elevation={elevation}
        data-focus-ring={focusRing ? "true" : undefined}
        data-interactive={interactive ? "true" : undefined}
        data-shape={shape}
        data-tint={tint}
        {...props}
        {...(surfaceRole
          ? getXiaSurfaceAttributes(surfaceRole)
          : { "data-material": material })}
      />
    );
  },
);
GlassSurface.displayName = "GlassSurface";

export interface GlassGroupProps extends GlassSurfaceProps {}

export const GlassGroup = React.forwardRef<HTMLDivElement, GlassGroupProps>(
  ({ className, shape = "capsule", ...props }, ref) => (
    <GlassSurface
      ref={ref}
      className={cn("app-glass-group", className)}
      shape={shape}
      {...props}
    />
  ),
);
GlassGroup.displayName = "GlassGroup";
