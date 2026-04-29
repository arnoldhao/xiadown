import { useCallback, useSyncExternalStore } from "react";
import enTranslations from "./locales/en.json";
import zhCNTranslations from "./locales/zh-CN.json";

export type SupportedLanguage = "en" | "zh-CN";
export type TFunction = (key: string) => string;

const I18N_PREFIX = "i18n:";

const SUPPORTED_LANGUAGE_OPTIONS: Array<{
  value: SupportedLanguage;
  labelKey: string;
}> = [
  { value: "en", labelKey: "settings.language.option.en" },
  { value: "zh-CN", labelKey: "settings.language.option.zh-CN" },
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

function normalizeLanguage(language: string): SupportedLanguage {
  if (language === "zh-CN" || language?.toLowerCase() === "zh-cn") {
    return "zh-CN";
  }
  if (language?.toLowerCase().startsWith("zh")) {
    return "zh-CN";
  }
  return "en";
}
