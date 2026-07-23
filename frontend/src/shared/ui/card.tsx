import * as React from "react"

import {
  Card as BaseCard,
  CardContent as BaseCardContent,
  CardDescription as BaseCardDescription,
  CardFooter as BaseCardFooter,
  CardHeader as BaseCardHeader,
  CardTitle as BaseCardTitle,
} from "@/components/ui/card"
import { cn } from "@/lib/utils"

export const APP_CARD_SECTION_SIZES = ["default", "compact"] as const
export type CardSectionSize = (typeof APP_CARD_SECTION_SIZES)[number]

const Card = React.forwardRef<
  HTMLDivElement,
  React.ComponentPropsWithoutRef<typeof BaseCard>
>(({ className, ...props }, ref) => (
  <BaseCard
    {...props}
    ref={ref}
    className={cn("app-dream-card app-motion-surface", className)}
    data-app-card="true"
  />
))
Card.displayName = "Card"

type CardSectionProps<T extends React.ElementType> = React.ComponentPropsWithoutRef<T> & {
  size?: CardSectionSize
}

const CardHeader = React.forwardRef<
  HTMLDivElement,
  CardSectionProps<typeof BaseCardHeader>
>(({ className, size = "default", ...props }, ref) => (
  <BaseCardHeader
    {...props}
    ref={ref}
    className={cn("app-dream-card__header", className)}
    data-section-size={size}
  />
))
CardHeader.displayName = "CardHeader"

const CardContent = React.forwardRef<
  HTMLDivElement,
  CardSectionProps<typeof BaseCardContent>
>(({ className, size = "default", ...props }, ref) => (
  <BaseCardContent
    {...props}
    ref={ref}
    className={cn("app-dream-card__content", className)}
    data-section-size={size}
  />
))
CardContent.displayName = "CardContent"

const CardFooter = React.forwardRef<
  HTMLDivElement,
  CardSectionProps<typeof BaseCardFooter>
>(({ className, size = "default", ...props }, ref) => (
  <BaseCardFooter
    {...props}
    ref={ref}
    className={cn("app-dream-card__footer", className)}
    data-section-size={size}
  />
))
CardFooter.displayName = "CardFooter"

const CardTitle = React.forwardRef<
  React.ElementRef<typeof BaseCardTitle>,
  React.ComponentPropsWithoutRef<typeof BaseCardTitle>
>(({ className, ...props }, ref) => (
  <BaseCardTitle
    ref={ref}
    className={cn("app-dream-card__title", className)}
    {...props}
  />
))
CardTitle.displayName = "CardTitle"

const CardDescription = React.forwardRef<
  React.ElementRef<typeof BaseCardDescription>,
  React.ComponentPropsWithoutRef<typeof BaseCardDescription>
>(({ className, ...props }, ref) => (
  <BaseCardDescription
    ref={ref}
    className={cn("app-dream-card__description", className)}
    {...props}
  />
))
CardDescription.displayName = "CardDescription"

export { Card, CardHeader, CardContent, CardFooter, CardTitle, CardDescription }
