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

type CardSectionSize = "default" | "compact"

const Card = React.forwardRef<
  HTMLDivElement,
  React.ComponentPropsWithoutRef<typeof BaseCard>
>(({ className, ...props }, ref) => (
  <BaseCard ref={ref} className={cn("app-motion-surface", className)} {...props} />
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
    ref={ref}
    className={cn(size === "compact" && "p-3", className)}
    {...props}
  />
))
CardHeader.displayName = "CardHeader"

const CardContent = React.forwardRef<
  HTMLDivElement,
  CardSectionProps<typeof BaseCardContent>
>(({ className, size = "default", ...props }, ref) => (
  <BaseCardContent
    ref={ref}
    className={cn(size === "compact" && "p-3", className)}
    {...props}
  />
))
CardContent.displayName = "CardContent"

const CardFooter = React.forwardRef<
  HTMLDivElement,
  CardSectionProps<typeof BaseCardFooter>
>(({ className, size = "default", ...props }, ref) => (
  <BaseCardFooter
    ref={ref}
    className={cn(size === "compact" && "p-3", className)}
    {...props}
  />
))
CardFooter.displayName = "CardFooter"

const CardTitle = BaseCardTitle
const CardDescription = BaseCardDescription

export { Card, CardHeader, CardContent, CardFooter, CardTitle, CardDescription }
