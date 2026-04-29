import * as Dialog from "@radix-ui/react-dialog";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import type { ComponentType, ReactNode, SVGProps } from "react";
import { createPortal } from "react-dom";
import { AlertTriangle, CheckCircle2, Info, X, XCircle } from "lucide-react";

import { useI18n } from "@/shared/i18n";
import { Button } from "@/shared/ui/button";
import { cn } from "@/lib/utils";
import { messageBus, useMessages } from "./store";
import type {
  DialogMessage,
  MessageIntent,
  NotificationMessage,
  ToastMessage,
} from "./types";

type IntentIcon = {
  icon: ComponentType<SVGProps<SVGSVGElement>>;
};

const INTENT_ICON: Record<MessageIntent, IntentIcon> = {
  info: { icon: Info },
  success: { icon: CheckCircle2 },
  warning: { icon: AlertTriangle },
  danger: { icon: XCircle },
};

const DEFAULT_ITEM_HEIGHT = 48;
const STACK_GAP = 8;
const STACK_OFFSET = 10;
const STACK_SCALE_STEP = 0.025;
const STACK_OPACITY_STEP = 0.09;
const STACK_MIN_SCALE = 0.88;
const STACK_MIN_OPACITY = 0.45;

type StackLayout = {
  stacked: boolean;
  offset: number;
};

function useViewportHeight() {
  const [height, setHeight] = useState(() =>
    typeof window === "undefined" ? 0 : window.innerHeight
  );

  useEffect(() => {
    const handleResize = () => setHeight(window.innerHeight);
    handleResize();
    window.addEventListener("resize", handleResize);
    return () => window.removeEventListener("resize", handleResize);
  }, []);

  return height;
}

function useElementHeight<T extends HTMLElement>() {
  const [height, setHeight] = useState(0);
  const observerRef = useRef<ResizeObserver | null>(null);

  const ref = useCallback((node: T | null) => {
    if (observerRef.current) {
      observerRef.current.disconnect();
      observerRef.current = null;
    }
    if (!node) {
      return;
    }
    const update = () => setHeight(node.getBoundingClientRect().height);
    update();
    observerRef.current = new ResizeObserver(update);
    observerRef.current.observe(node);
  }, []);

  useEffect(() => () => observerRef.current?.disconnect(), []);

  return { ref, height };
}

function getStackLayout(count: number, itemHeight: number, maxHeight: number): StackLayout {
  if (!count || !maxHeight) {
    return { stacked: false, offset: STACK_OFFSET };
  }
  const resolvedHeight = itemHeight > 0 ? itemHeight : DEFAULT_ITEM_HEIGHT;
  const totalHeight = count * (resolvedHeight + STACK_GAP) - STACK_GAP;
  if (totalHeight <= maxHeight) {
    return { stacked: false, offset: STACK_OFFSET };
  }
  const offset =
    maxHeight > resolvedHeight
      ? Math.min(STACK_OFFSET, (maxHeight - resolvedHeight) / Math.max(1, count - 1))
      : 0;
  return { stacked: true, offset: Math.max(0, offset) };
}

function getStackItemStyle(index: number, total: number, layout: StackLayout) {
  if (!layout.stacked) {
    return undefined;
  }
  const depthIndex = index;
  const translateY = depthIndex * layout.offset;
  const scale = Math.max(STACK_MIN_SCALE, 1 - depthIndex * STACK_SCALE_STEP);
  const opacity = Math.max(STACK_MIN_OPACITY, 1 - depthIndex * STACK_OPACITY_STEP);

  return {
    transform: `translateY(${translateY}px) scale(${scale})`,
    transformOrigin: "top center",
    opacity,
    zIndex: 100 + (total - index),
  } as const;
}

function formatMessageText(template: string, params?: Record<string, string>) {
  if (!params) {
    return template;
  }
  let output = template;
  Object.entries(params).forEach(([key, value]) => {
    output = output.split(`{${key}}`).join(value ?? "");
  });
  return output;
}

function resolveMessageText(
  raw: string | undefined,
  key: string | undefined,
  params: Record<string, string> | undefined,
  t: (key: string) => string
) {
  if (key) {
    return formatMessageText(t(key), params);
  }
  return raw ?? "";
}

function MessageSurface({
  message,
  onActivate,
  children,
}: {
  message: ToastMessage | NotificationMessage;
  onActivate: () => void;
  children: ReactNode;
}) {
  const intentIcon = INTENT_ICON[message.intent];
  const Icon = intentIcon.icon;
  const liveMode = message.intent === "danger" || message.intent === "warning" ? "assertive" : "polite";

  return (
    <div
      role={liveMode === "assertive" ? "alert" : "status"}
      aria-live={liveMode}
      data-intent={message.intent}
      className={cn(
        "app-message-surface app-motion-surface flex text-foreground"
      )}
    >
      <button type="button" className="app-message-button" onClick={onActivate}>
        <Icon className="app-message-icon" />
        <div className="app-message-text">
          {children}
        </div>
      </button>
    </div>
  );
}

function MessageText({ title, description }: { title: string; description: string }) {
  return (
    <>
      {title ? <span className="app-message-title">{title}</span> : null}
      {title && description ? <span className="app-message-separator">·</span> : null}
      {description ? (
        <span className={cn("app-message-description", !title && "text-foreground")}>
          {description}
        </span>
      ) : null}
    </>
  );
}

function ToastItem({ message }: { message: ToastMessage }) {
  const { t } = useI18n();
  const title = resolveMessageText(message.title, message.i18n?.titleKey, message.i18n?.params, t);
  const description = resolveMessageText(
    message.description,
    message.i18n?.descriptionKey,
    message.i18n?.params,
    t
  );

  useEffect(() => {
    let timer: number | null = null;
    let cancelled = false;

    const scheduleDismiss = () => {
      if (!message.autoCloseMs || message.autoCloseMs <= 0) {
        return;
      }
      timer = window.setTimeout(() => messageBus.dismiss(message.id), message.autoCloseMs);
    };

    if (message.awaitFor) {
      message.awaitFor
        .catch(() => null)
        .finally(() => {
          if (cancelled) {
            return;
          }
          if (!message.autoCloseMs || message.autoCloseMs <= 0) {
            messageBus.dismiss(message.id);
            return;
          }
          scheduleDismiss();
        });
    } else {
      scheduleDismiss();
    }

    return () => {
      cancelled = true;
      if (timer !== null) {
        window.clearTimeout(timer);
      }
    };
  }, [message.id, message.autoCloseMs, message.awaitFor]);

  return (
    <MessageSurface
      message={message}
      onActivate={() => {
        message.action?.onClick?.();
        messageBus.dismiss(message.id);
      }}
    >
      <MessageText title={title} description={description} />
    </MessageSurface>
  );
}

function NotificationItem({ message }: { message: NotificationMessage }) {
  const { t } = useI18n();
  const title = resolveMessageText(message.title, message.i18n?.titleKey, message.i18n?.params, t);
  const description = resolveMessageText(
    message.description,
    message.i18n?.descriptionKey,
    message.i18n?.params,
    t
  );

  const handleActivate = () => {
    message.actions?.[0]?.onClick?.();
    messageBus.markNotificationRead(message.id, false);
    messageBus.dismiss(message.id);
  };

  return (
    <MessageSurface
      message={message}
      onActivate={handleActivate}
    >
      <MessageText title={title} description={description} />
    </MessageSurface>
  );
}

function DialogHost({ message }: { message: DialogMessage }) {
  const { t } = useI18n();
  const title = resolveMessageText(message.title, message.i18n?.titleKey, message.i18n?.params, t);
  const description = resolveMessageText(
    message.description,
    message.i18n?.descriptionKey,
    message.i18n?.params,
    t
  );
  const cancelLabel = message.cancelLabelKey ? t(message.cancelLabelKey) : message.cancelLabel ?? "Cancel";
  const confirmLabel = message.confirmLabelKey ? t(message.confirmLabelKey) : message.confirmLabel ?? "Confirm";

  const handleClose = () => {
    message.onCancel?.();
    messageBus.dismiss(message.id);
  };

  const handleConfirm = async () => {
    try {
      await message.onConfirm?.();
    } finally {
      messageBus.dismiss(message.id);
    }
  };

  return (
    <Dialog.Root open onOpenChange={(open) => (!open ? handleClose() : null)}>
      <Dialog.Portal>
        <Dialog.Overlay className="fixed inset-0 z-50 bg-black/80 data-[state=open]:animate-in data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0" />
        <Dialog.Content
          className={cn(
            "fixed left-1/2 top-1/2 z-50 grid max-h-[min(28rem,calc(100vh-2rem))] w-[min(32rem,calc(100vw-2rem))] max-w-none -translate-x-1/2 -translate-y-1/2 grid-rows-[minmax(0,1fr)_auto] overflow-hidden border bg-background shadow-lg duration-200",
            "data-[state=open]:animate-in data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0",
            "data-[state=closed]:zoom-out-95 data-[state=open]:zoom-in-95",
            "data-[state=closed]:slide-out-to-left-1/2 data-[state=closed]:slide-out-to-top-[48%]",
            "data-[state=open]:slide-in-from-left-1/2 data-[state=open]:slide-in-from-top-[48%]",
            "sm:rounded-lg"
          )}
        >
          <div className="min-h-0 min-w-0 overflow-hidden px-6 pt-6 pb-4 text-center sm:text-left">
            {title ? (
              <Dialog.Title className="overflow-hidden break-words pr-6 text-lg font-semibold leading-[1.35] tracking-tight [display:-webkit-box] [-webkit-box-orient:vertical] [-webkit-line-clamp:2]">
                {title}
              </Dialog.Title>
            ) : null}
            {description ? (
              <Dialog.Description className="mt-1.5 overflow-hidden break-words text-sm leading-5 text-muted-foreground [display:-webkit-box] [-webkit-box-orient:vertical] [-webkit-line-clamp:4]">
                {description}
              </Dialog.Description>
            ) : null}
          </div>

          <div className="flex shrink-0 flex-col-reverse gap-2 px-6 py-4 sm:flex-row sm:justify-end">
            <Button variant="outline" size="compact" onClick={handleClose}>
              {cancelLabel}
            </Button>
            <Button
              variant={message.destructive ? "destructive" : "default"}
              size="compact"
              onClick={handleConfirm}
            >
              {confirmLabel}
            </Button>
          </div>

          <Dialog.Close asChild>
            <Button
              variant="ghost"
              size="compactIcon"
              className="absolute right-4 top-4"
              aria-label="close"
            >
              <X className="h-4 w-4" />
            </Button>
          </Dialog.Close>
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  );
}

export function MessageHost() {
  const viewportHeight = useViewportHeight();
  const stackMeasure = useElementHeight<HTMLDivElement>();
  const messages = useMessages();
  const stackedMessages = useMemo(
    () =>
      messages
        .filter((m): m is ToastMessage | NotificationMessage => m.kind !== "dialog")
        .sort((a, b) => b.ts - a.ts),
    [messages]
  );
  const dialog = useMemo(() => messages.find((m): m is DialogMessage => m.kind === "dialog"), [messages]);
  const maxStackHeight = viewportHeight * 0.5;
  const stackLayout = getStackLayout(stackedMessages.length, stackMeasure.height, maxStackHeight);

  if (typeof document === "undefined") {
    return null;
  }

  return createPortal(
    <>
      <div
        className={cn(
          "pointer-events-none fixed left-1/2 top-[calc(var(--app-titlebar-height)+0.875rem)] z-[90] w-auto max-w-[calc(100vw-1.5rem)] -translate-x-1/2 max-h-[50vh]",
          stackLayout.stacked ? "h-[50vh] overflow-hidden" : "flex flex-col items-center gap-2"
        )}
      >
        {stackedMessages.map((message, index) => (
          <div
            key={message.id}
            ref={index === 0 ? stackMeasure.ref : undefined}
            className={cn(
              "pointer-events-auto w-fit max-w-full",
              stackLayout.stacked ? "absolute left-0 right-0 top-0" : null
            )}
            style={getStackItemStyle(index, stackedMessages.length, stackLayout)}
          >
            <div className="animate-in slide-in-from-top-2 duration-200">
              {message.kind === "toast" ? (
                <ToastItem message={message} />
              ) : (
                <NotificationItem message={message} />
              )}
            </div>
          </div>
        ))}
      </div>

      {dialog ? <DialogHost message={dialog} /> : null}
    </>,
    document.body
  );
}
