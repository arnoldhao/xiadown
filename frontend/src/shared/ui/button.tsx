import * as React from "react"

import {
  Button as BaseButton,
  buttonVariants as baseButtonVariants,
  type ButtonProps as BaseButtonProps,
} from "@/components/ui/button"
import { cn } from "@/lib/utils"
import { type VariantProps } from "class-variance-authority"

type AppButtonSize = "compact" | "compactIcon"
type AppButtonVariant = "sidebar"

export type ButtonProps = Omit<BaseButtonProps, "size" | "variant"> & {
  size?: BaseButtonProps["size"] | AppButtonSize
  variant?: BaseButtonProps["variant"] | AppButtonVariant
}

type BaseButtonVariants = VariantProps<typeof baseButtonVariants>
type ButtonVariantsProps = Omit<BaseButtonVariants, "size" | "variant"> & {
  size?: BaseButtonVariants["size"] | AppButtonSize
  variant?: BaseButtonVariants["variant"] | AppButtonVariant
  className?: string
}

function resolveSizeClass(size: BaseButtonVariants["size"] | AppButtonSize | undefined) {
  if (size === "compact") {
    return {
      controlClass: "app-control-compact",
      textClass: "text-xs",
      gapClass: "gap-1.5",
      mappedSize: "sm" as const,
    }
  }

  if (size === "compactIcon") {
    return {
      controlClass: "app-control-compact-icon",
      textClass: "text-xs",
      gapClass: "gap-1.5",
      mappedSize: "icon" as const,
    }
  }

  return {
    controlClass: undefined,
    textClass: "text-xs",
    gapClass: undefined,
    mappedSize: size,
  }
}

const buttonVariants = ({ size, variant, className }: ButtonVariantsProps = {}) => {
  const { controlClass, textClass, gapClass, mappedSize } = resolveSizeClass(size)
  const mappedVariant = variant === "sidebar" ? "default" : variant
  const radiusClass = variant === "sidebar" ? "rounded-[var(--app-sidebar-radius)]" : undefined

  return cn(
    "app-dream-button app-motion-surface app-motion-press",
    baseButtonVariants({ variant: mappedVariant, size: mappedSize }),
    controlClass,
    textClass,
    gapClass,
    radiusClass,
    className
  )
}

const Button = React.forwardRef<HTMLButtonElement, ButtonProps>(
  ({ size, variant, className, ...props }, ref) => {
    const { controlClass, textClass, gapClass, mappedSize } = resolveSizeClass(size)
    const mappedVariant = variant === "sidebar" ? "default" : variant
    const radiusClass = variant === "sidebar" ? "rounded-[var(--app-sidebar-radius)]" : undefined

    return (
      <BaseButton
        ref={ref}
        size={mappedSize}
        variant={mappedVariant}
        className={cn("app-dream-button app-motion-surface app-motion-press", controlClass, textClass, gapClass, radiusClass, className)}
        {...props}
      />
    )
  }
)
Button.displayName = "Button"

export { Button, buttonVariants }
