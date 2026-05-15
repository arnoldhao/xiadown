import * as React from "react";

import {
  Dialog as BaseDialog,
  DialogPortal as BaseDialogPortal,
  DialogOverlay as BaseDialogOverlay,
  DialogClose as BaseDialogClose,
  DialogTrigger as BaseDialogTrigger,
  DialogContent as BaseDialogContent,
  DialogHeader as BaseDialogHeader,
  DialogFooter as BaseDialogFooter,
  DialogTitle as BaseDialogTitle,
  DialogDescription as BaseDialogDescription,
} from "@/components/ui/dialog";
import { cn } from "@/lib/utils";
import { Separator } from "@/shared/ui/separator";

const Dialog = BaseDialog;
const DialogPortal = BaseDialogPortal;
const DialogClose = BaseDialogClose;
const DialogTrigger = BaseDialogTrigger;

const DialogOverlay = React.forwardRef<
  React.ElementRef<typeof BaseDialogOverlay>,
  React.ComponentPropsWithoutRef<typeof BaseDialogOverlay>
>(({ className, ...props }, ref) => (
  <BaseDialogOverlay ref={ref} className={cn("app-dialog-overlay", className)} {...props} />
));
DialogOverlay.displayName = "DialogOverlay";

const DialogContent = React.forwardRef<
  React.ElementRef<typeof BaseDialogContent>,
  React.ComponentPropsWithoutRef<typeof BaseDialogContent>
>(({ className, ...props }, ref) => (
  <BaseDialogContent
    ref={ref}
    className={cn("app-dialog-content app-motion-surface", className)}
    {...props}
  />
));
DialogContent.displayName = "DialogContent";

function DialogHeader({
  className,
  ...props
}: React.HTMLAttributes<HTMLDivElement>) {
  return <BaseDialogHeader className={cn("app-dialog-header", className)} {...props} />;
}

function DialogFooter({
  className,
  ...props
}: React.HTMLAttributes<HTMLDivElement>) {
  return <BaseDialogFooter className={cn("app-dialog-footer pt-2", className)} {...props} />;
}

const DialogListCard = React.forwardRef<
  HTMLDivElement,
  React.HTMLAttributes<HTMLDivElement>
>(({ className, ...props }, ref) => (
  <div
    ref={ref}
    className={cn("app-dialog-list-card app-dream-card app-motion-surface", className)}
    {...props}
  />
));
DialogListCard.displayName = "DialogListCard";

const DialogListCardContent = React.forwardRef<
  HTMLDivElement,
  React.HTMLAttributes<HTMLDivElement>
>(({ className, ...props }, ref) => (
  <div
    ref={ref}
    className={cn("app-dialog-list-card-content", className)}
    {...props}
  />
));
DialogListCardContent.displayName = "DialogListCardContent";

const DialogRow = React.forwardRef<
  HTMLDivElement,
  React.HTMLAttributes<HTMLDivElement> & {
    interactive?: boolean;
  }
>(({ className, interactive, ...props }, ref) => (
  <div
    ref={ref}
    className={cn("app-dialog-row app-dream-row app-dream-row-compact", className)}
    data-interactive={interactive ? "true" : undefined}
    {...props}
  />
));
DialogRow.displayName = "DialogRow";

const DialogSeparator = React.forwardRef<
  React.ElementRef<typeof Separator>,
  React.ComponentPropsWithoutRef<typeof Separator>
>(({ className, ...props }, ref) => (
  <Separator
    ref={ref}
    className={cn("app-dialog-separator app-divider-soft app-divider-inset", className)}
    {...props}
  />
));
DialogSeparator.displayName = "DialogSeparator";

const DialogTitle = React.forwardRef<
  React.ElementRef<typeof BaseDialogTitle>,
  React.ComponentPropsWithoutRef<typeof BaseDialogTitle>
>(({ className, ...props }, ref) => (
  <BaseDialogTitle
    ref={ref}
    className={cn("text-lg font-semibold leading-[1.35] tracking-[-0.02em]", className)}
    {...props}
  />
));
DialogTitle.displayName = "DialogTitle";

const DialogDescription = React.forwardRef<
  React.ElementRef<typeof BaseDialogDescription>,
  React.ComponentPropsWithoutRef<typeof BaseDialogDescription>
>(({ className, ...props }, ref) => (
  <BaseDialogDescription
    ref={ref}
    className={cn("text-xs text-muted-foreground", className)}
    {...props}
  />
));
DialogDescription.displayName = "DialogDescription";

export {
  Dialog,
  DialogPortal,
  DialogOverlay,
  DialogClose,
  DialogTrigger,
  DialogContent,
  DialogHeader,
  DialogFooter,
  DialogListCard,
  DialogListCardContent,
  DialogRow,
  DialogSeparator,
  DialogTitle,
  DialogDescription,
};
