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

const Dialog = BaseDialog;
const DialogPortal = BaseDialogPortal;
const DialogClose = BaseDialogClose;
const DialogTrigger = BaseDialogTrigger;

const DialogOverlay = React.forwardRef<
  React.ElementRef<typeof BaseDialogOverlay>,
  React.ComponentPropsWithoutRef<typeof BaseDialogOverlay>
>(({ className, ...props }, ref) => (
  <BaseDialogOverlay ref={ref} className={cn("backdrop-blur-[1px]", className)} {...props} />
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
  return <BaseDialogFooter className={cn("pt-2", className)} {...props} />;
}

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
  DialogTitle,
  DialogDescription,
};
