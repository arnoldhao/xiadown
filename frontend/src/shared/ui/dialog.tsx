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
    className={cn("app-dialog-content app-motion-surface gap-3 p-4", className)}
    {...props}
  />
));
DialogContent.displayName = "DialogContent";

type DialogScrollAreaProps = React.HTMLAttributes<HTMLElement> & {
  as?: "div" | "form";
};

const DialogScrollArea = React.forwardRef<HTMLElement, DialogScrollAreaProps>(
  ({ as: Component = "div", className, children, ...props }, ref) => {
    const scrollAreaRef = React.useRef<HTMLElement | null>(null);
    const hoverActiveRef = React.useRef(false);
    const focusActiveRef = React.useRef(false);
    const scrollingActiveRef = React.useRef(false);
    const scrollIdleTimeoutRef = React.useRef<number | null>(null);
    const [scrollState, setScrollState] = React.useState({
      atBottom: true,
      atTop: true,
      scrollbarVisible: false,
      scrollable: false,
    });
    const ScrollComponent = Component as React.ElementType;

    const shouldShowScrollbar = React.useCallback(
      (scrollable: boolean) =>
        scrollable &&
        (hoverActiveRef.current ||
          focusActiveRef.current ||
          scrollingActiveRef.current),
      [],
    );

    const setRefs = React.useCallback(
      (node: HTMLElement | null) => {
        scrollAreaRef.current = node;
        if (typeof ref === "function") {
          ref(node);
          return;
        }
        if (ref) {
          ref.current = node;
        }
      },
      [ref],
    );

    const updateScrollState = React.useCallback(() => {
      const node = scrollAreaRef.current;
      if (!node) {
        return;
      }
      const maxScrollTop = Math.max(0, node.scrollHeight - node.clientHeight);
      const nextState = {
        atBottom: maxScrollTop - node.scrollTop <= 1,
        atTop: node.scrollTop <= 1,
        scrollable: maxScrollTop > 1,
      };
      setScrollState((current) => {
        const nextScrollbarVisible = shouldShowScrollbar(nextState.scrollable);
        return current.atBottom === nextState.atBottom &&
          current.atTop === nextState.atTop &&
          current.scrollable === nextState.scrollable &&
          current.scrollbarVisible === nextScrollbarVisible
          ? current
          : {
              ...nextState,
              scrollbarVisible: nextScrollbarVisible,
            };
      });
    }, [shouldShowScrollbar]);

    React.useEffect(() => {
      const node = scrollAreaRef.current;
      if (!node || typeof window === "undefined") {
        return;
      }

      let frame = 0;
      const scheduleUpdate = () => {
        window.cancelAnimationFrame(frame);
        frame = window.requestAnimationFrame(updateScrollState);
      };
      const activateScrollInteraction = () => {
        scrollingActiveRef.current = true;
        if (scrollIdleTimeoutRef.current !== null) {
          window.clearTimeout(scrollIdleTimeoutRef.current);
        }
        scrollIdleTimeoutRef.current = window.setTimeout(() => {
          scrollingActiveRef.current = false;
          scheduleUpdate();
        }, 720);
      };
      const handleScroll = () => {
        activateScrollInteraction();
        scheduleUpdate();
      };
      const handlePointerEnter = () => {
        hoverActiveRef.current = true;
        scheduleUpdate();
      };
      const handlePointerLeave = () => {
        hoverActiveRef.current = false;
        scheduleUpdate();
      };
      const handleFocusIn = () => {
        focusActiveRef.current = true;
        scheduleUpdate();
      };
      const handleFocusOut = (event: FocusEvent) => {
        focusActiveRef.current = Boolean(
          event.relatedTarget instanceof Node && node.contains(event.relatedTarget),
        );
        scheduleUpdate();
      };
      const resizeObserver =
        typeof ResizeObserver === "undefined"
          ? null
          : new ResizeObserver(scheduleUpdate);
      const mutationObserver =
        typeof MutationObserver === "undefined"
          ? null
          : new MutationObserver(scheduleUpdate);

      node.addEventListener("scroll", handleScroll, { passive: true });
      node.addEventListener("pointerenter", handlePointerEnter);
      node.addEventListener("pointerleave", handlePointerLeave);
      node.addEventListener("focusin", handleFocusIn);
      node.addEventListener("focusout", handleFocusOut);
      resizeObserver?.observe(node);
      Array.from(node.children).forEach((child) => resizeObserver?.observe(child));
      mutationObserver?.observe(node, {
        childList: true,
        characterData: true,
        subtree: true,
      });
      scheduleUpdate();

      return () => {
        window.cancelAnimationFrame(frame);
        if (scrollIdleTimeoutRef.current !== null) {
          window.clearTimeout(scrollIdleTimeoutRef.current);
          scrollIdleTimeoutRef.current = null;
        }
        node.removeEventListener("scroll", handleScroll);
        node.removeEventListener("pointerenter", handlePointerEnter);
        node.removeEventListener("pointerleave", handlePointerLeave);
        node.removeEventListener("focusin", handleFocusIn);
        node.removeEventListener("focusout", handleFocusOut);
        resizeObserver?.disconnect();
        mutationObserver?.disconnect();
      };
    }, [updateScrollState]);

    return (
      <ScrollComponent
        ref={setRefs}
        className={cn("app-dialog-scroll-area", className)}
        data-at-bottom={scrollState.atBottom ? "true" : "false"}
        data-at-top={scrollState.atTop ? "true" : "false"}
        data-scrollbar-visible={scrollState.scrollbarVisible ? "true" : "false"}
        data-scrollable={scrollState.scrollable ? "true" : "false"}
        {...props}
      >
        {children}
      </ScrollComponent>
    );
  },
);
DialogScrollArea.displayName = "DialogScrollArea";

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
  return (
    <BaseDialogFooter
      className={cn(
        "app-dialog-footer flex-row flex-wrap items-center justify-end gap-2 pt-0 sm:space-x-0",
        className,
      )}
      {...props}
    />
  );
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
    className={cn("text-base font-semibold leading-[1.35] tracking-normal", className)}
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
  DialogScrollArea,
  DialogHeader,
  DialogFooter,
  DialogListCard,
  DialogListCardContent,
  DialogRow,
  DialogSeparator,
  DialogTitle,
  DialogDescription,
};
