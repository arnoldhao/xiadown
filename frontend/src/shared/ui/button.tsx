import * as React from "react"

import {
  Button as BaseButton,
  buttonVariants as baseButtonVariants,
  type ButtonProps as BaseButtonProps,
} from "@/components/ui/button"
import { cn } from "@/lib/utils"
import { type VariantProps } from "class-variance-authority"

export type AppButtonSize = "compact" | "compactIcon"
export type AppButtonVariant = "sidebar" | "glass"

export const APP_BUTTON_VARIANTS = [
  "default",
  "destructive",
  "outline",
  "secondary",
  "ghost",
  "link",
  "sidebar",
  "glass",
] as const satisfies readonly NonNullable<
  BaseButtonProps["variant"] | AppButtonVariant
>[]

export const APP_BUTTON_TONES = [
  "neutral",
  "accent",
  "destructive",
  "success",
  "warning",
] as const
export type AppButtonTone = (typeof APP_BUTTON_TONES)[number]

export const APP_BUTTON_SHAPES = [
  "control",
  "capsule",
  "circle",
  "square",
] as const
export type AppButtonShape = (typeof APP_BUTTON_SHAPES)[number]

export const APP_BUTTON_SIZES = [
  "default",
  "sm",
  "lg",
  "icon",
  "compact",
  "compactIcon",
] as const satisfies readonly NonNullable<BaseButtonProps["size"] | AppButtonSize>[]

export type ButtonProps = Omit<BaseButtonProps, "size" | "variant"> & {
  size?: BaseButtonProps["size"] | AppButtonSize
  variant?: BaseButtonProps["variant"] | AppButtonVariant
  tone?: AppButtonTone
  shape?: AppButtonShape
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
      mappedSize: "sm" as const,
    }
  }

  if (size === "compactIcon") {
    return {
      controlClass: "app-control-compact-icon",
      mappedSize: "icon" as const,
    }
  }

  return {
    controlClass: undefined,
    mappedSize: size,
  }
}

const buttonVariants = ({ size, variant, className }: ButtonVariantsProps = {}) => {
  const { controlClass, mappedSize } = resolveSizeClass(size)
  const mappedVariant =
    variant === "sidebar" ? "default" : variant === "glass" ? "outline" : variant

  return cn(
    "app-dream-button app-motion-surface app-motion-press",
    baseButtonVariants({ variant: mappedVariant, size: mappedSize }),
    controlClass,
    variant === "sidebar" && "app-dream-button--sidebar",
    className
  )
}

const Button = React.forwardRef<HTMLButtonElement, ButtonProps>(
  ({ size, variant, tone, shape, className, ...props }, ref) => {
    const { controlClass, mappedSize } = resolveSizeClass(size)
    const resolvedVariant = variant ?? "default"
    const mappedVariant =
      resolvedVariant === "sidebar"
        ? "default"
        : resolvedVariant === "glass"
          ? "outline"
          : resolvedVariant
    const resolvedTone: AppButtonTone =
      tone ??
      (resolvedVariant === "destructive"
        ? "destructive"
        : resolvedVariant === "default"
          ? "accent"
          : "neutral")

    return (
      <BaseButton
        {...props}
        ref={ref}
        size={mappedSize}
        variant={mappedVariant}
        className={cn(
          "app-dream-button app-motion-surface app-motion-press",
          resolvedVariant === "glass" && "app-glass-surface",
          controlClass,
          resolvedVariant === "sidebar" && "app-dream-button--sidebar",
          className,
        )}
        data-app-button="true"
        data-elevation={resolvedVariant === "glass" ? "floating" : undefined}
        data-material={resolvedVariant === "glass" ? "regular" : undefined}
        data-shape={
          shape ?? (resolvedVariant === "sidebar" ? undefined : "control")
        }
        data-size={size ?? "default"}
        data-tint={resolvedVariant === "glass" ? "neutral" : undefined}
        data-tone={resolvedTone}
        data-variant={resolvedVariant}
      />
    )
  }
)
Button.displayName = "Button"

export { Button, buttonVariants }
