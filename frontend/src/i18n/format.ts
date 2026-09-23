import type { Locale } from "./translations";

const intlLocales: Record<Locale, string> = { ru: "ru-KZ", kk: "kk-KZ", en: "en-KZ" };

function number(value: number, locale: Locale): string {
  return new Intl.NumberFormat(intlLocales[locale], { maximumFractionDigits: 0 }).format(value);
}

export function formatPriceFrom(value: number, locale: Locale): string {
  const formatted = number(value, locale);
  if (locale === "kk") return `${formatted} ₸ бастап`;
  if (locale === "en") return `from KZT ${formatted}`;
  return `от ${formatted} ₸`;
}

export function formatBudgetTarget(value: number, locale: Locale): string {
  const formatted = number(value, locale);
  if (locale === "kk") return `${formatted} ₸ дейін`;
  if (locale === "en") return `up to KZT ${formatted}`;
  return `до ${formatted} ₸`;
}

export function formatDateOnly(value: string, locale: Locale): string {
  const [year, month, day] = value.split("-").map(Number);
  const date = new Date(Date.UTC(year!, month! - 1, day));
  return new Intl.DateTimeFormat(intlLocales[locale], {
    day: "numeric",
    month: "long",
    timeZone: "UTC",
  }).format(date);
}

export function formatResultSummary(eligible: number, returned: number, locale: Locale): string {
  if (locale === "kk") {
    return eligible > returned
      ? `${eligible} мердігер сәйкес келеді — алғашқы ${returned} нұсқа көрсетілді`
      : `${eligible} мердігер сәйкес келеді`;
  }
  if (locale === "en") {
    return eligible > returned
      ? `${eligible} contractors are eligible — showing the first ${returned}`
      : `${eligible} ${eligible === 1 ? "contractor is" : "contractors are"} eligible`;
  }
  if (eligible === 1) return "Подходит 1 подрядчик";
  if (eligible > returned) return `Подходят ${eligible} подрядчика — показываем первые ${returned}`;
  return `Подходят ${eligible} подрядчика`;
}

export function formatAlternativeCount(count: number, locale: Locale): string {
  if (locale === "kk") return `${count} сәйкес нұсқа пайда болады`;
  if (locale === "en") return `${count} suitable ${count === 1 ? "option" : "options"} will appear`;
  const lastTwo = count % 100;
  const last = count % 10;
  if (lastTwo >= 11 && lastTwo <= 14) return `Появятся ${count} подходящих вариантов`;
  if (last === 1) return `Появится ${count} подходящий вариант`;
  if (last >= 2 && last <= 4) return `Появятся ${count} подходящих варианта`;
  return `Появятся ${count} подходящих вариантов`;
}

export function formatDistanceDays(count: number, locale: Locale): string {
  if (locale === "kk") return `±${number(count, locale)} күн`;
  if (locale === "en") return `±${number(count, locale)} ${count === 1 ? "day" : "days"}`;
  const lastTwo = count % 100;
  const last = count % 10;
  const unit = lastTwo >= 11 && lastTwo <= 14 ? "дней" : last === 1 ? "день" : last >= 2 && last <= 4 ? "дня" : "дней";
  return `±${number(count, locale)} ${unit}`;
}

export function formatMaxHours(hours: number, locale: Locale): string {
  const formatted = new Intl.NumberFormat(intlLocales[locale], { maximumFractionDigits: 1 }).format(hours);
  if (locale === "kk") return `${formatted} сағ дейін`;
  if (locale === "en") return `up to ${formatted} h`;
  return `до ${formatted} ч`;
}
