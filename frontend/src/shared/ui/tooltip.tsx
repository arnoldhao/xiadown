import * as React from "react";
import { createPortal } from "react-dom";

import { cn } from "@/lib/utils";

type TooltipSide = "top" | "bottom" | "left" | "right";
type TooltipAlign = "start" | "center" | "end";

type TooltipContextValue = {
  contentId: string;
  open: boolean;
  setOpen: React.Dispatch<React.SetStateAction<boolean>>;
  triggerRef: React.MutableRefObject<HTMLElement | null>;
};

type TooltipPosition = {
  arrowClassName: string;
  left: number;
  top: number;
  transform: string;
};

const TooltipContext = React.createContext<TooltipContextValue | null>(null);

function useTooltipContext(name: string) {
  const context = React.useContext(TooltipContext);
  if (!context) {
    throw new Error(`${name} must be used within Tooltip`);
  }
  return context;
}

function setRef<T>(ref: React.Ref<T> | undefined, value: T) {
  if (typeof ref === "function") {
    ref(value);
    return;
  }
  if (ref && typeof ref === "object") {
    (ref as React.MutableRefObject<T>).current = value;
  }
}

function composeRefs<T>(...refs: Array<React.Ref<T> | undefined>) {
  return (value: T) => {
    refs.forEach((ref) => setRef(ref, value));
  };
}

function composeEventHandlers<E>(
  theirs?: (event: E) => void,
  ours?: (event: E) => void,
) {
  return (event: E) => {
    theirs?.(event);
    if (!(event as { defaultPrevented?: boolean }).defaultPrevented) {
      ours?.(event);
    }
  };
}

function resolveTooltipPosition(
  rect: DOMRect,
  side: TooltipSide,
  align: TooltipAlign,
  sideOffset: number,
): TooltipPosition {
  if (side === "bottom") {
    return {
      top: rect.bottom + sideOffset,
      left:
        align === "start"
          ? rect.left
          : align === "end"
            ? rect.right
            : rect.left + rect.width / 2,
      transform:
        align === "start"
          ? "translate(0, 0)"
          : align === "end"
            ? "translate(-100%, 0)"
            : "translate(-50%, 0)",
      arrowClassName: "left-1/2 top-0 -translate-x-1/2 -translate-y-1/2",
    };
  }

  if (side === "left") {
    return {
      top:
        align === "start"
          ? rect.top
          : align === "end"
            ? rect.bottom
            : rect.top + rect.height / 2,
      left: rect.left - sideOffset,
      transform:
        align === "start"
          ? "translate(-100%, 0)"
          : align === "end"
            ? "translate(-100%, -100%)"
            : "translate(-100%, -50%)",
      arrowClassName: "left-full top-1/2 -translate-x-1/2 -translate-y-1/2",
    };
  }

  if (side === "right") {
    return {
      top:
        align === "start"
          ? rect.top
          : align === "end"
            ? rect.bottom
            : rect.top + rect.height / 2,
      left: rect.right + sideOffset,
      transform:
        align === "start"
          ? "translate(0, 0)"
          : align === "end"
            ? "translate(0, -100%)"
            : "translate(0, -50%)",
      arrowClassName: "left-0 top-1/2 -translate-x-1/2 -translate-y-1/2",
    };
  }

  return {
    top: rect.top - sideOffset,
    left:
      align === "start"
        ? rect.left
        : align === "end"
          ? rect.right
          : rect.left + rect.width / 2,
    transform:
      align === "start"
        ? "translate(0, -100%)"
        : align === "end"
          ? "translate(-100%, -100%)"
          : "translate(-50%, -100%)",
    arrowClassName: "left-1/2 top-full -translate-x-1/2 -translate-y-1/2",
  };
}

function TooltipProvider({
  children,
}: React.PropsWithChildren<{ delayDuration?: number }>) {
  return <>{children}</>;
}

function Tooltip({ children }: React.PropsWithChildren) {
  const [open, setOpen] = React.useState(false);
  const triggerRef = React.useRef<HTMLElement | null>(null);
  const contentId = React.useId();

  return (
    <TooltipContext.Provider value={{ contentId, open, setOpen, triggerRef }}>
      {children}
    </TooltipContext.Provider>
  );
}

type TooltipTriggerProps = {
  asChild?: boolean;
  children: React.ReactElement;
  openOnFocus?: boolean;
};

const TooltipTrigger = React.forwardRef<HTMLElement, TooltipTriggerProps>(
  ({ asChild, children, openOnFocus = true }, forwardedRef) => {
    const context = useTooltipContext("TooltipTrigger");
    const child = React.Children.only(children) as React.ReactElement<any>;
    const childRef = (child as any).ref as React.Ref<HTMLElement> | undefined;

    const triggerProps = {
      ref: composeRefs(forwardedRef, childRef, (node: HTMLElement | null) => {
        context.triggerRef.current = node;
      }),
      onMouseEnter: composeEventHandlers(child.props.onMouseEnter, () => context.setOpen(true)),
      onMouseLeave: composeEventHandlers(child.props.onMouseLeave, () => context.setOpen(false)),
      onFocus: composeEventHandlers(child.props.onFocus, () => {
        if (openOnFocus) {
          context.setOpen(true);
        }
      }),
      onBlur: composeEventHandlers(child.props.onBlur, () => {
        if (openOnFocus) {
          context.setOpen(false);
        }
      }),
      onKeyDown: composeEventHandlers(
        child.props.onKeyDown,
        (event: React.KeyboardEvent<HTMLElement>) => {
          if (event.key === "Escape") {
            context.setOpen(false);
          }
        },
      ),
      "aria-describedby": context.open ? context.contentId : undefined,
    };

    if (asChild) {
      return React.cloneElement(child, triggerProps);
    }

    return React.createElement("button", { type: "button", ...triggerProps }, child);
  },
);
TooltipTrigger.displayName = "TooltipTrigger";

type TooltipContentProps = React.HTMLAttributes<HTMLDivElement> & {
  align?: TooltipAlign;
  multiline?: boolean;
  side?: TooltipSide;
  sideOffset?: number;
};

const TooltipContent = React.forwardRef<HTMLDivElement, TooltipContentProps>(
  (
    {
      align = "center",
      children,
      className,
      multiline = false,
      side = "top",
      sideOffset = 8,
      style,
      ...props
    },
    ref,
  ) => {
    const context = useTooltipContext("TooltipContent");
    const [position, setPosition] = React.useState<TooltipPosition | null>(null);

    React.useLayoutEffect(() => {
      if (!context.open || !context.triggerRef.current || typeof window === "undefined") {
        return;
      }

      const update = () => {
        if (!context.triggerRef.current) {
          return;
        }
        setPosition(
          resolveTooltipPosition(
            context.triggerRef.current.getBoundingClientRect(),
            side,
            align,
            sideOffset,
          ),
        );
      };

      update();
      window.addEventListener("resize", update);
      window.addEventListener("scroll", update, true);
      return () => {
        window.removeEventListener("resize", update);
        window.removeEventListener("scroll", update, true);
      };
    }, [align, context.open, context.triggerRef, side, sideOffset]);

    if (!context.open || !position || typeof document === "undefined") {
      return null;
    }

    return createPortal(
      <div
        ref={ref}
        id={context.contentId}
        role="tooltip"
        className={cn(
          "pointer-events-none fixed z-50 select-none rounded-md bg-foreground px-2 py-1 text-[10px] font-medium text-background shadow-lg shadow-black/15",
          "animate-in fade-in-0 zoom-in-95",
          className,
          multiline
            ? "max-w-[min(22rem,calc(100vw-1rem))] whitespace-pre-line break-words"
            : "max-w-[min(28rem,calc(100vw-1rem))] overflow-hidden text-ellipsis whitespace-nowrap",
        )}
        style={{
          left: position.left,
          top: position.top,
          transform: position.transform,
          ...style,
        }}
        {...props}
      >
        {children}
        <span
          aria-hidden="true"
          className={cn("absolute h-2 w-2 rotate-45 bg-foreground", position.arrowClassName)}
        />
      </div>,
      document.body,
    );
  },
);
TooltipContent.displayName = "TooltipContent";

export { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger };
