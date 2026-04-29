import { Dialogs } from "@wailsio/runtime";

type OpenFileDialogOptions = Parameters<typeof Dialogs.OpenFile>[0];

export function isDialogCancelError(error: unknown) {
  const message =
    error instanceof Error
      ? error.message
      : typeof error === "string"
        ? error
        : String(error ?? "");
  const normalized = message.trim().toLowerCase();
  if (!normalized) {
    return false;
  }
  return [
    "shellitem is nil",
    "shell item is nil",
    "operation was canceled",
    "operation was cancelled",
    "operation canceled",
    "operation cancelled",
    "canceled by the user",
    "cancelled by the user",
    "user canceled",
    "user cancelled",
    "dialog was closed",
    "0x800704c7",
  ].some((marker) => normalized.includes(marker));
}

export async function openFileDialog(options: OpenFileDialogOptions) {
  try {
    return await Dialogs.OpenFile(options);
  } catch (error) {
    if (isDialogCancelError(error)) {
      return options.AllowsMultipleSelection ? [] : "";
    }
    throw error;
  }
}
