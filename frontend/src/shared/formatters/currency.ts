const currency = new Intl.NumberFormat("ru-RU", { maximumFractionDigits: 0 });

export function formatPriceFrom(value: number): string {
  return `от ${currency.format(value)} ₸`;
}
