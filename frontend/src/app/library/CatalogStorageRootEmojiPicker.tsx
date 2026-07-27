import { EmojiPicker } from "frimousse";
import * as React from "react";

import { useI18n } from "@/shared/i18n";

import type { LibraryWorkspaceLabels } from "./types";

type FrimousseLocale = React.ComponentProps<typeof EmojiPicker.Root>["locale"];

function emojiPickerLocale(language: string): FrimousseLocale {
  switch (language.trim().toLocaleLowerCase()) {
    case "zh-tw":
      return "zh-hant";
    case "zh-cn":
      return "zh";
    case "ja-jp":
      return "ja";
    case "ko-kr":
      return "ko";
    case "pt-br":
      return "pt";
    case "es-419":
      return "es-mx";
    case "vi-vn":
      return "vi";
    case "id-id":
      return "en";
    default:
      return "en";
  }
}

export interface CatalogStorageRootEmojiPickerProps {
  labels: Pick<LibraryWorkspaceLabels, "loading" | "search">;
  onEmojiSelect: (emoji: string) => void;
}

export default function CatalogStorageRootEmojiPicker(
  props: CatalogStorageRootEmojiPickerProps,
) {
  const { language } = useI18n();
  return (
    <EmojiPicker.Root
      className="app-storage-root-emoji-picker"
      columns={8}
      locale={emojiPickerLocale(language)}
      onEmojiSelect={({ emoji }) => props.onEmojiSelect(emoji)}
    >
      <EmojiPicker.Search
        aria-label={props.labels.search}
        className="app-storage-root-emoji-picker__search"
        placeholder={`${props.labels.search}…`}
      />
      <EmojiPicker.Viewport className="app-storage-root-emoji-picker__viewport">
        <EmojiPicker.Loading className="app-storage-root-emoji-picker__state">
          {props.labels.loading}
        </EmojiPicker.Loading>
        <EmojiPicker.Empty className="app-storage-root-emoji-picker__state">
          <span aria-hidden="true">—</span>
          <span className="sr-only">{props.labels.search}</span>
        </EmojiPicker.Empty>
        <EmojiPicker.List
          className="app-storage-root-emoji-picker__list"
          components={{
            CategoryHeader: ({ category, ...categoryProps }) => (
              <div
                {...categoryProps}
                className="app-storage-root-emoji-picker__category"
              >
                {category.label}
              </div>
            ),
            Emoji: ({ emoji, ...emojiProps }) => (
              <button
                {...emojiProps}
                className="app-storage-root-emoji-picker__emoji"
                type="button"
              >
                {emoji.emoji}
              </button>
            ),
            Row: ({ children, ...rowProps }) => (
              <div
                {...rowProps}
                className="app-storage-root-emoji-picker__row"
              >
                {children}
              </div>
            ),
          }}
        />
      </EmojiPicker.Viewport>
    </EmojiPicker.Root>
  );
}
