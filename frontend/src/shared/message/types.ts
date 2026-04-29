export type MessageIntent = "info" | "success" | "warning" | "danger";

export type MessageKind = "toast" | "notification" | "dialog";

export interface MessageI18n {
  titleKey?: string;
  descriptionKey?: string;
  params?: Record<string, string>;
}

export interface MessageAction {
  label: string;
  labelKey?: string;
  href?: string;
  onClick?: () => void;
  intent?: MessageIntent;
}

export interface BaseMessage {
  id: string;
  kind: MessageKind;
  intent: MessageIntent;
  title?: string;
  description?: string;
  source?: string;
  ts: number;
  data?: Record<string, unknown> | unknown;
  i18n?: MessageI18n;
}

export interface ToastMessage extends BaseMessage {
  kind: "toast";
  autoCloseMs?: number; // undefined = default duration; 0 = no auto-close unless awaitFor completes
  action?: MessageAction;
  awaitFor?: Promise<unknown>;
}

export interface NotificationMessage extends BaseMessage {
  kind: "notification";
  unread?: boolean;
  persistent?: boolean;
  actions?: MessageAction[];
}

export interface DialogMessage extends BaseMessage {
  kind: "dialog";
  confirmLabel?: string;
  confirmLabelKey?: string;
  cancelLabel?: string;
  cancelLabelKey?: string;
  destructive?: boolean;
  onConfirm?: () => void | Promise<void>;
  onCancel?: () => void | Promise<void>;
}

export type Message = ToastMessage | NotificationMessage | DialogMessage;

export interface PublishToastInput
  extends Omit<Partial<ToastMessage>, "id" | "kind" | "ts"> {
  id?: string;
  ts?: number;
  intent?: MessageIntent;
}

export interface PublishNotificationInput
  extends Omit<Partial<NotificationMessage>, "id" | "kind" | "ts"> {
  id?: string;
  ts?: number;
  intent?: MessageIntent;
}

export interface PublishDialogInput
  extends Omit<Partial<DialogMessage>, "id" | "kind" | "ts"> {
  id?: string;
  ts?: number;
  intent?: MessageIntent;
}
