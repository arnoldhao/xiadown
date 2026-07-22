import type {
  XiaSurfaceStyle,
  XiaThemePackId,
} from "@/shared/styles/xiadown-theme";

export const MAIN_SIDEBAR_ACTION_CLASS =
  "app-main-sidebar-action";
export const MAIN_SIDEBAR_ICON_CLASS =
  "app-main-sidebar-action-icon";
export const SIDEBAR_DROPDOWN_CONTENT_CLASS_NAME =
  "app-sidebar-dropdown-content";
export const SIDEBAR_DROPDOWN_ITEM_CLASS_NAME =
  "app-sidebar-dropdown-item";
export const SIDEBAR_DROPDOWN_CHECKBOX_ITEM_CLASS_NAME =
  "app-sidebar-dropdown-item";
export const SIDEBAR_DROPDOWN_ICON_SLOT_CLASS_NAME =
  "app-sidebar-dropdown-icon-slot";

export const SETTINGS_LIST_CARD_CLASS = "app-settings-list-card";
export const SETTINGS_LIST_CARD_CONTENT_CLASS = "app-settings-list-card-content";
export const SETTINGS_ROW_BASE_CLASS = "app-settings-row";
export const SETTINGS_ROW_CLASS = SETTINGS_ROW_BASE_CLASS;
export const SETTINGS_ROW_START_CLASS = `${SETTINGS_ROW_BASE_CLASS} app-settings-row-start`;
export const SETTINGS_ROW_LABEL_CLASS = "app-settings-row-label";
export const SETTINGS_ROW_LABEL_TRUNCATE_CLASS = "app-settings-row-label";
export const SETTINGS_ROW_DESCRIPTION_CLASS = "app-settings-row-description";
export const SETTINGS_ROW_CONTENT_BASE_CLASS =
  "app-settings-row-content";
export const SETTINGS_SEPARATOR_CLASS = "app-divider-soft";
export const SETTINGS_CONTROL_WIDTH_CLASS = "app-settings-control-width";
export const SETTINGS_WIDE_CONTROL_WIDTH_CLASS = "app-settings-control-width-wide";
export const SETTINGS_COMPACT_LIST_CARD_CLASS = "app-settings-list-card-compact";
export const SETTINGS_COMPACT_LIST_CARD_CONTENT_CLASS = "app-settings-list-card-content-compact";
export const SETTINGS_COMPACT_ROW_CLASS = "app-settings-row-compact";
export const SETTINGS_COMPACT_ROW_LABEL_CLASS = "app-settings-row-label";
export const SETTINGS_COMPACT_ROW_DESCRIPTION_CLASS =
  "app-settings-row-description";
export const SETTINGS_COMPACT_ROW_CONTENT_CLASS = "app-settings-row-content-compact";
export const SETTINGS_COMPACT_SEPARATOR_CLASS = "app-divider-inset";

export const PET_GALLERY_CONTEXT_MENU_CONTENT_CLASS_NAME =
  "app-pet-gallery-context-menu-content";
export const PET_GALLERY_CONTEXT_MENU_ITEM_CLASS_NAME =
  "app-pet-gallery-context-menu-item";
export const PET_GALLERY_CONTEXT_MENU_ICON_SLOT_CLASS_NAME =
  "app-pet-gallery-context-menu-icon-slot";

export const VIDSTACK_PREVIEW_SHELL_CLASS =
  "app-completed-preview-shell";
export const VIDSTACK_PREVIEW_CONTROL_BUTTON_CLASS =
  "app-completed-preview-control-button";
export const VIDSTACK_PREVIEW_CONTROL_RANGE_CLASS =
  "app-completed-preview-range";
export const VIDSTACK_PREVIEW_VOLUME_RANGE_CLASS =
  "app-completed-preview-range app-completed-preview-volume-range";
export const COMPLETED_PREVIEW_SHELL_CLASS = VIDSTACK_PREVIEW_SHELL_CLASS;
export const COMPLETED_PREVIEW_CONTROL_BUTTON_CLASS =
  VIDSTACK_PREVIEW_CONTROL_BUTTON_CLASS;
export const COMPLETED_PREVIEW_CONTROL_RANGE_CLASS =
  VIDSTACK_PREVIEW_CONTROL_RANGE_CLASS;
export const COMPLETED_PREVIEW_VOLUME_RANGE_CLASS =
  VIDSTACK_PREVIEW_VOLUME_RANGE_CLASS;

export function resolveXiaMainSidebarSurface(
  themeId: XiaThemePackId,
  surfaceStyle: XiaSurfaceStyle = "glass",
  shellTheme = "default",
) {
  void themeId;
  void shellTheme;
  if (surfaceStyle === "contrast") {
    return "app-main-sidebar-surface app-main-sidebar-surface--contrast";
  }
  return "app-main-sidebar-surface app-main-sidebar-surface--glass";
}
