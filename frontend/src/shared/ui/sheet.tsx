import { X } from "lucide-react";
import * as React from "react";

import { cn } from "@/lib/utils";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogOverlay,
  DialogPortal,
  DialogTitle,
  DialogTrigger,
} from "@/shared/ui/dialog";

const Sheet = Dialog;
const SheetClose = DialogClose;
const SheetOverlay = DialogOverlay;
const SheetPortal = DialogPortal;
const SheetTrigger = DialogTrigger;

export const SHEET_SIDES = ["left", "right"] as const;
export type SheetSide = (typeof SHEET_SIDES)[number];
export const SHEET_SIZES = ["sm", "md", "lg"] as const;
export type SheetSize = (typeof SHEET_SIZES)[number];

export interface SheetContentProps
  extends Omit<
    React.ComponentPropsWithoutRef<typeof DialogContent>,
    "showCloseButton" | "unstyled"
  > {
  /** Edge used by the slide-over presentation. Ignored when centered is true. */
  side?: SheetSide;
  /** Presents the sheet as a window-modal panel instead of a slide-over. */
  centered?: boolean;
  /** Semantic width shared by side and centered presentations. */
  size?: SheetSize;
  /**
   * Keeps the panel inside the application content area below native window
   * chrome. Use this for tall sheets whose header must never sit behind macOS
   * traffic lights or Windows caption buttons.
   */
  windowChromeSafeArea?: boolean;
}

/**
 * Shared modal sheet built on the Dialog focus trap and portal. A Sheet is
 * always panel glass at modal elevation; callers only choose its placement
 * and semantic size.
 */
const SheetContent = React.forwardRef<
  React.ElementRef<typeof DialogContent>,
  SheetContentProps
>(
  (
    {
      centered = false,
      className,
      overlayClassName,
      side = "right",
      size = "md",
      windowChromeSafeArea = false,
      ...props
    },
    ref,
  ) => (
    <DialogContent
      {...props}
      ref={ref}
      className={cn(
        "app-sheet-content app-glass-surface app-motion-surface wails-no-drag",
        className,
      )}
      data-centered={centered ? "true" : "false"}
      data-elevation="modal"
      data-shape="panel"
      data-side={centered ? undefined : side}
      data-size={size}
      data-tint="neutral"
      data-window-chrome-safe-area={windowChromeSafeArea ? "true" : undefined}
      overlayClassName={cn("app-sheet-overlay", overlayClassName)}
      showCloseButton={false}
      surfaceRole="overlay"
      unstyled
    />
  ),
);
SheetContent.displayName = "SheetContent";

function SheetHeader({
  className,
  ...props
}: React.HTMLAttributes<HTMLElement>) {
  return (
    <header
      className={cn(
        "app-sheet-header",
        className,
      )}
      {...props}
    />
  );
}

function SheetHeading({
  className,
  ...props
}: React.HTMLAttributes<HTMLDivElement>) {
  return (
    <div
      className={cn("app-sheet-heading", className)}
      {...props}
    />
  );
}

const SheetTitle = React.forwardRef<
  React.ElementRef<typeof DialogTitle>,
  React.ComponentPropsWithoutRef<typeof DialogTitle>
>(({ className, ...props }, ref) => (
  <DialogTitle
    ref={ref}
    className={cn("app-sheet-title", className)}
    {...props}
  />
));
SheetTitle.displayName = "SheetTitle";

const SheetDescription = React.forwardRef<
  React.ElementRef<typeof DialogDescription>,
  React.ComponentPropsWithoutRef<typeof DialogDescription>
>(({ className, ...props }, ref) => (
  <DialogDescription
    ref={ref}
    className={cn(
      "app-sheet-description",
      className,
    )}
    {...props}
  />
));
SheetDescription.displayName = "SheetDescription";

const SheetBody = React.forwardRef<
  HTMLDivElement,
  React.HTMLAttributes<HTMLDivElement>
>(({ className, ...props }, ref) => (
  <div
    ref={ref}
    className={cn(
      "app-sheet-body",
      className,
    )}
    {...props}
  />
));
SheetBody.displayName = "SheetBody";

function SheetFooter({
  className,
  ...props
}: React.HTMLAttributes<HTMLElement>) {
  return (
    <footer
      className={cn(
        "app-sheet-footer app-dialog-footer",
        className,
      )}
      {...props}
    />
  );
}

export type SheetCloseButtonProps = Omit<
  React.ButtonHTMLAttributes<HTMLButtonElement>,
  "aria-label"
> & {
  "aria-label": string;
};

const SheetCloseButton = React.forwardRef<
  HTMLButtonElement,
  SheetCloseButtonProps
>(({ children, className, type = "button", ...props }, ref) => (
  <SheetClose asChild>
    <button
      ref={ref}
      className={cn("app-dialog-close app-sheet-close", className)}
      type={type}
      {...props}
    >
      {children ?? <X aria-hidden="true" className="app-sheet-close__icon" />}
    </button>
  </SheetClose>
));
SheetCloseButton.displayName = "SheetCloseButton";

export {
  Sheet,
  SheetBody,
  SheetClose,
  SheetCloseButton,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetHeading,
  SheetOverlay,
  SheetPortal,
  SheetTitle,
  SheetTrigger,
};
