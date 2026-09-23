import { createContext, useContext, useEffect, useMemo, useState } from "react";
import type { ReactNode } from "react";
import { translations } from "./translations";
import type { Locale, TranslationKey } from "./translations";

const STORAGE_KEY = "apex-match-locale";
const supportedLocales: Locale[] = ["ru", "kk", "en"];

function isLocale(value: string | null): value is Locale {
  return supportedLocales.includes(value as Locale);
}

function initialLocale(): Locale {
  const stored = window.localStorage.getItem(STORAGE_KEY);
  if (isLocale(stored)) return stored;
  return "ru";
}

interface LocaleContextValue {
  locale: Locale;
  setLocale: (locale: Locale) => void;
  t: (key: TranslationKey) => string;
}

const defaultContext: LocaleContextValue = {
  locale: "ru",
  setLocale: () => undefined,
  t: (key) => translations.ru[key],
};

const LocaleContext = createContext<LocaleContextValue>(defaultContext);

export function LocaleProvider({ children }: { children: ReactNode }) {
  const [locale, setLocale] = useState<Locale>(initialLocale);

  useEffect(() => {
    window.localStorage.setItem(STORAGE_KEY, locale);
    document.documentElement.lang = locale;
  }, [locale]);

  const value = useMemo<LocaleContextValue>(() => ({
    locale,
    setLocale,
    t: (key) => translations[locale][key],
  }), [locale]);

  return <LocaleContext.Provider value={value}>{children}</LocaleContext.Provider>;
}

export function useLocale(): LocaleContextValue {
  return useContext(LocaleContext);
}
