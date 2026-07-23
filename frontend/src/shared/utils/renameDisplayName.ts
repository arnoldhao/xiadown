export interface RenameDisplayNameValidationMessages {
  required: string;
  invalid: string;
  tooLong: string;
}

const RENAME_DISPLAY_NAME_MAX_LENGTH = 160;
const RENAME_INVALID_NAME_PATTERN = /[<>:"/\\|?*\u0000-\u001f\u007f]/;
const RENAME_RESERVED_NAMES = new Set([
  "CON",
  "PRN",
  "AUX",
  "NUL",
  "COM1",
  "COM2",
  "COM3",
  "COM4",
  "COM5",
  "COM6",
  "COM7",
  "COM8",
  "COM9",
  "LPT1",
  "LPT2",
  "LPT3",
  "LPT4",
  "LPT5",
  "LPT6",
  "LPT7",
  "LPT8",
  "LPT9",
]);

export function validateRenameDisplayName(
  name: string,
  messages: RenameDisplayNameValidationMessages,
) {
  const trimmed = name.trim();
  if (!trimmed) {
    return messages.required;
  }
  if ([...trimmed].length > RENAME_DISPLAY_NAME_MAX_LENGTH) {
    return messages.tooLong;
  }
  if (
    RENAME_INVALID_NAME_PATTERN.test(trimmed) ||
    trimmed.replace(/\./g, "") === "" ||
    trimmed.endsWith(".")
  ) {
    return messages.invalid;
  }
  const reservedCandidate = trimmed.split(".")[0]?.toUpperCase() ?? "";
  if (RENAME_RESERVED_NAMES.has(reservedCandidate)) {
    return messages.invalid;
  }
  return "";
}

export interface ProtectedFileDisplayName {
  stem: string;
  extension: string;
}

/**
 * Keeps the visible final extension outside the editable field. This protects
 * the file-format suffix while still allowing dots inside the ordinary name.
 */
export function splitProtectedFileDisplayName(
  displayName: string,
): ProtectedFileDisplayName {
  const trimmed = displayName.trim();
  const dotIndex = trimmed.lastIndexOf(".");
  if (dotIndex <= 0 || dotIndex >= trimmed.length - 1) {
    return { stem: trimmed, extension: "" };
  }
  return {
    stem: trimmed.slice(0, dotIndex),
    extension: trimmed.slice(dotIndex),
  };
}

export function composeProtectedFileDisplayName(
  draftStem: string,
  extension: string,
) {
  const normalizedExtension = extension.trim();
  let stem = draftStem.trim();
  if (
    normalizedExtension &&
    stem.length > normalizedExtension.length &&
    stem.toLocaleLowerCase().endsWith(normalizedExtension.toLocaleLowerCase())
  ) {
    stem = stem.slice(0, -normalizedExtension.length).trimEnd();
  }
  return `${stem}${normalizedExtension}`;
}
