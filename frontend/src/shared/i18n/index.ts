import { useCallback, useSyncExternalStore } from "react";
import enTranslations from "./locales/en.json";
import es419Translations from "./locales/es-419.json";
import idIDTranslations from "./locales/id-ID.json";
import jaJPTranslations from "./locales/ja-JP.json";
import koKRTranslations from "./locales/ko-KR.json";
import ptBRTranslations from "./locales/pt-BR.json";
import viVNTranslations from "./locales/vi-VN.json";
import zhCNTranslations from "./locales/zh-CN.json";
import zhTWTranslations from "./locales/zh-TW.json";

export type SupportedLanguage =
  | "en"
  | "zh-CN"
  | "zh-TW"
  | "ja-JP"
  | "ko-KR"
  | "es-419"
  | "pt-BR"
  | "id-ID"
  | "vi-VN";
export type TFunction = (key: string) => string;

const I18N_PREFIX = "i18n:";

const SUPPORTED_LANGUAGE_OPTIONS: Array<{
  value: SupportedLanguage;
  labelKey: string;
}> = [
  { value: "en", labelKey: "settings.language.option.en" },
  { value: "zh-CN", labelKey: "settings.language.option.zhCN" },
  { value: "zh-TW", labelKey: "settings.language.option.zhTW" },
  { value: "ja-JP", labelKey: "settings.language.option.jaJP" },
  { value: "ko-KR", labelKey: "settings.language.option.koKR" },
  { value: "es-419", labelKey: "settings.language.option.es419" },
  { value: "pt-BR", labelKey: "settings.language.option.ptBR" },
  { value: "id-ID", labelKey: "settings.language.option.idID" },
  { value: "vi-VN", labelKey: "settings.language.option.viVN" },
];

type TranslationMap = Record<string, string>;

function flattenTranslations(input: Record<string, any>, prefix = "", output: TranslationMap = {}): TranslationMap {
  Object.entries(input).forEach(([key, value]) => {
    const nextKey = prefix ? `${prefix}.${key}` : key;
    if (value && typeof value === "object" && !Array.isArray(value)) {
      flattenTranslations(value, nextKey, output);
    } else {
      output[nextKey] = String(value);
    }
  });
  return output;
}

const translations: Record<SupportedLanguage, TranslationMap> = {
  en: flattenTranslations(enTranslations as Record<string, any>),
  "zh-CN": flattenTranslations(zhCNTranslations as Record<string, any>),
  "zh-TW": flattenTranslations(zhTWTranslations as Record<string, any>),
  "ja-JP": flattenTranslations(jaJPTranslations as Record<string, any>),
  "ko-KR": flattenTranslations(koKRTranslations as Record<string, any>),
  "es-419": flattenTranslations(es419Translations as Record<string, any>),
  "pt-BR": flattenTranslations(ptBRTranslations as Record<string, any>),
  "id-ID": flattenTranslations(idIDTranslations as Record<string, any>),
  "vi-VN": flattenTranslations(viVNTranslations as Record<string, any>),
};

let currentLanguage: SupportedLanguage = "en";
const subscribers = new Set<() => void>();

function notifySubscribers() {
  subscribers.forEach((callback) => callback());
}

function subscribe(callback: () => void) {
  subscribers.add(callback);
  return () => subscribers.delete(callback);
}

export function t(key: string, language: SupportedLanguage = currentLanguage): string {
  const langTranslations = translations[language] ?? translations.en;
  return langTranslations[key] ?? translations.en[key] ?? key;
}

function formatTemplate(template: string, params?: Record<string, string>) {
  if (!params) {
    return template;
  }
  let output = template;
  Object.entries(params).forEach(([key, value]) => {
    output = output.split(`{${key}}`).join(value ?? "");
  });
  return output;
}

export function resolveI18nText(
  value: string | undefined,
  language: SupportedLanguage = currentLanguage,
): string {
  const raw = (value ?? "").trim();
  if (!raw.startsWith(I18N_PREFIX)) {
    return raw;
  }

  const payload = raw.slice(I18N_PREFIX.length);
  const [keyPart, queryPart = ""] = payload.split("?", 2);
  const key = keyPart.trim();
  if (!key) {
    return "";
  }

  const params: Record<string, string> = {};
  new URLSearchParams(queryPart).forEach((paramValue, paramKey) => {
    params[paramKey] = paramValue;
  });

  return formatTemplate(t(key, language), Object.keys(params).length > 0 ? params : undefined);
}

export function setLanguage(language: string) {
  const normalized = normalizeLanguage(language);
  if (normalized === currentLanguage) {
    return;
  }
  currentLanguage = normalized;
  notifySubscribers();
}

export function getLanguage(): SupportedLanguage {
  return currentLanguage;
}

export function detectBrowserLanguage(): SupportedLanguage {
  const navigatorLanguage = typeof navigator !== "undefined" ? navigator.language : "";
  return normalizeLanguage(navigatorLanguage);
}

export function useI18n() {
  const language = useSyncExternalStore(subscribe, getLanguage);

  const boundT = useCallback<TFunction>((key) => t(key, language), [language]);
  const supportedLanguages = SUPPORTED_LANGUAGE_OPTIONS.map((option) => ({
    value: option.value,
    label: t(option.labelKey, language),
  }));

  return {
    t: boundT,
    language,
    setLanguage,
    supportedLanguages,
  };
}

export function normalizeLanguage(language: string): SupportedLanguage {
  const normalized = language?.trim().replace(/_/g, "-").toLowerCase();
  if (
    normalized?.startsWith("zh-tw") ||
    normalized?.startsWith("zh-hant") ||
    normalized?.startsWith("zh-hk") ||
    normalized?.startsWith("zh-mo")
  ) {
    return "zh-TW";
  }
  if (
    normalized?.startsWith("zh-cn") ||
    normalized?.startsWith("zh-hans") ||
    normalized?.startsWith("zh-sg")
  ) {
    return "zh-CN";
  }
  if (normalized?.startsWith("zh")) {
    return "zh-CN";
  }
  if (normalized?.startsWith("ja")) {
    return "ja-JP";
  }
  if (normalized?.startsWith("ko")) {
    return "ko-KR";
  }
  if (normalized === "es-419" || normalized?.startsWith("es")) {
    return "es-419";
  }
  if (normalized?.startsWith("pt")) {
    return "pt-BR";
  }
  if (normalized?.startsWith("id")) {
    return "id-ID";
  }
  if (normalized?.startsWith("vi")) {
    return "vi-VN";
  }
  return "en";
}
