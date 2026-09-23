import type { FormValues } from "./RecommendationForm";
import { useLocale } from "../../i18n/LocaleContext";
import type { TranslationKey } from "../../i18n/translations";

export interface DemoPreset {
  labelKey: TranslationKey;
  descriptionKey: TranslationKey;
  values: FormValues;
}

export const demoPresets: DemoPreset[] = [
  {
    labelKey: "presets.popular",
    descriptionKey: "presets.popularDescription",
    values: { city: "Алматы", category: "Ведущий", event_format: "корпоратив", date: "2026-11-14", budget_kzt: "1000000", language: "русский", duration_hours: "6" },
  },
  {
    labelKey: "presets.busy",
    descriptionKey: "presets.busyDescription",
    values: { city: "Алматы", category: "Ведущий", event_format: "корпоратив", date: "2026-12-19", budget_kzt: "1000000", language: "русский", duration_hours: "6" },
  },
  {
    labelKey: "presets.rare",
    descriptionKey: "presets.rareDescription",
    values: { city: "Алматы", category: "Флорист", event_format: "свадьба", date: "2026-11-14", budget_kzt: "300000", language: "русский", duration_hours: "6" },
  },
  {
    labelKey: "presets.noMatch",
    descriptionKey: "presets.noMatchDescription",
    values: { city: "Алматы", category: "Ведущий", event_format: "корпоратив", date: "2026-11-14", budget_kzt: "100000", language: "русский", duration_hours: "6" },
  },
  {
    labelKey: "presets.noCatalog",
    descriptionKey: "presets.noCatalogDescription",
    values: { city: "Астана", category: "Декоратор", event_format: "корпоратив", date: "2026-11-14", budget_kzt: "3000000", language: "", duration_hours: "" },
  },
];

interface DemoPresetsProps {
  onSelect: (values: FormValues) => void;
  disabled?: boolean;
}

export function DemoPresets({ onSelect, disabled }: DemoPresetsProps) {
  const { t } = useLocale();
  return (
    <section className="presets" aria-labelledby="presets-title">
      <div>
        <p className="eyebrow" id="presets-title">{t("presets.title")}</p>
        <p className="presets-note">{t("presets.note")}</p>
      </div>
      <div className="preset-list">
        {demoPresets.map((preset) => (
          <button
            className="preset-button"
            disabled={disabled}
            key={preset.labelKey}
            onClick={() => onSelect(preset.values)}
            title={t(preset.descriptionKey)}
            type="button"
          >
            {t(preset.labelKey)}
          </button>
        ))}
      </div>
    </section>
  );
}
