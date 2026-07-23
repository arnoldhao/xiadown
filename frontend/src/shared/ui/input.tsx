import * as React from "react"

import { Input as BaseInput } from "@/components/ui/input"
import { cn } from "@/lib/utils"

export const APP_INPUT_SIZES = ["default", "compact"] as const
export type InputSize = (typeof APP_INPUT_SIZES)[number]

export type InputProps = Omit<
  React.ComponentPropsWithoutRef<typeof BaseInput>,
  "size"
> & {
  size?: InputSize
}

const Input = React.forwardRef<
  React.ElementRef<typeof BaseInput>,
  InputProps
>(({ size = "compact", className, ...props }, ref) => (
  <BaseInput
    {...props}
    ref={ref}
    className={cn(
      "app-dream-input app-motion-color",
      size === "compact" && "app-control-compact",
      className
    )}
    data-size={size}
  />
))
Input.displayName = "Input"

export { Input }
