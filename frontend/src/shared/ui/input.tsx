import * as React from "react"

import { Input as BaseInput } from "@/components/ui/input"
import { cn } from "@/lib/utils"

type InputSize = "default" | "compact"

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
    ref={ref}
    className={cn(
      "app-dream-input app-motion-color text-xs placeholder:text-xs file:text-xs",
      size === "compact" && "app-control-compact",
      className
    )}
    {...props}
  />
))
Input.displayName = "Input"

export { Input }
