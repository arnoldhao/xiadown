import { create } from "zustand";

import type {
  DialogMessage,
  Message,
  MessageIntent,
  MessageKind,
  NotificationMessage,
  PublishDialogInput,
  PublishNotificationInput,
  PublishToastInput,
  ToastMessage,
} from "./types";

const DEFAULT_INTENT: MessageIntent = "info";
const DEFAULT_TOAST_DURATION = 5_000;
const MAX_MESSAGES = 40;

function nextId() {
  return `msg_${Date.now()}_${Math.random().toString(16).slice(2, 8)}`;
}

function trimMessageQueue(messages: Message[]) {
  if (messages.length <= MAX_MESSAGES) {
    return messages;
  }
  const dialogs = messages.filter((message) => message.kind === "dialog");
  const rest = messages
    .filter((message) => message.kind !== "dialog")
    .sort((left, right) => right.ts - left.ts)
    .slice(0, Math.max(0, MAX_MESSAGES - dialogs.length))
    .sort((left, right) => left.ts - right.ts);
  return [...rest, ...dialogs];
}

function normalizeToast(input: PublishToastInput): ToastMessage {
  return {
    id: input.id ?? nextId(),
    kind: "toast",
    ts: input.ts ?? Date.now(),
    intent: input.intent ?? DEFAULT_INTENT,
    title: input.title,
    description: input.description,
    source: input.source,
    data: input.data,
    i18n: input.i18n,
    autoCloseMs:
      input.autoCloseMs === undefined
        ? input.awaitFor
          ? 0
          : DEFAULT_TOAST_DURATION
        : Math.max(0, input.autoCloseMs),
    action: input.action,
    awaitFor: input.awaitFor,
  };
}

function normalizeNotification(input: PublishNotificationInput): NotificationMessage {
  return {
    id: input.id ?? nextId(),
    kind: "notification",
    ts: input.ts ?? Date.now(),
    intent: input.intent ?? DEFAULT_INTENT,
    title: input.title,
    description: input.description,
    source: input.source,
    data: input.data,
    i18n: input.i18n,
    unread: input.unread ?? true,
    persistent: input.persistent ?? true,
    actions: input.actions,
  };
}

function normalizeDialog(input: PublishDialogInput): DialogMessage {
  return {
    id: input.id ?? nextId(),
    kind: "dialog",
    ts: input.ts ?? Date.now(),
    intent: input.intent ?? DEFAULT_INTENT,
    title: input.title,
    description: input.description,
    source: input.source,
    data: input.data,
    i18n: input.i18n,
    confirmLabel: input.confirmLabel ?? "OK",
    confirmLabelKey: input.confirmLabelKey,
    cancelLabel: input.cancelLabel ?? "Cancel",
    cancelLabelKey: input.cancelLabelKey,
    destructive: input.destructive ?? input.intent === "danger",
    onConfirm: input.onConfirm,
    onCancel: input.onCancel,
  };
}

export interface MessageStoreState {
  messages: Message[];
  publish: (message: Message) => string;
  dismiss: (id: string) => void;
  clear: (kind?: MessageKind) => void;
  markNotificationRead: (id: string, unread?: boolean) => void;
}

export const useMessageStore = create<MessageStoreState>((set) => ({
  messages: [],
  publish: (message) => {
    const id = message.id;
    set((state) => {
      const next = state.messages.some((item) => item.id === id)
        ? state.messages.map((item) => (item.id === id ? message : item))
        : [...state.messages, message];
      return { messages: trimMessageQueue(next) };
    });
    return id;
  },
  dismiss: (id) =>
    set((state) => ({ messages: state.messages.filter((m) => m.id !== id) })),
  clear: (kind) =>
    set((state) =>
      kind ? { messages: state.messages.filter((m) => m.kind !== kind) } : { messages: [] }
    ),
  markNotificationRead: (id, unread = false) =>
    set((state) => ({
      messages: state.messages.map((m) =>
        m.kind === "notification" && m.id === id ? { ...m, unread } : m
      ),
    })),
}));

export const messageBus = {
  publishToast(input: PublishToastInput) {
    const toast = normalizeToast(input);
    return useMessageStore.getState().publish(toast);
  },
  publishNotification(input: PublishNotificationInput) {
    const notification = normalizeNotification(input);
    return useMessageStore.getState().publish(notification);
  },
  publishDialog(input: PublishDialogInput) {
    const dialog = normalizeDialog(input);
    return useMessageStore.getState().publish(dialog);
  },
  dismiss(id: string) {
    useMessageStore.getState().dismiss(id);
  },
  clear(kind?: MessageKind) {
    useMessageStore.getState().clear(kind);
  },
  markNotificationRead(id: string, unread = false) {
    useMessageStore.getState().markNotificationRead(id, unread);
  },
};

export const useMessages = () => useMessageStore((state) => state.messages);
