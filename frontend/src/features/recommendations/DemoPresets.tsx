import type { FormValues } from "./RecommendationForm";

export interface DemoPreset {
  label: string;
  description: string;
  values: FormValues;
}

export const demoPresets: DemoPreset[] = [
  {
    label: "Популярный запрос",
    description: "Три карточки из четырёх подходящих",
    values: { city: "Алматы", category: "Ведущий", event_format: "корпоратив", date: "2026-11-14", budget_kzt: "1000000", language: "русский", duration_hours: "6" },
  },
  {
    label: "Занятая дата",
    description: "Та же заявка, но остаётся один вариант",
    values: { city: "Алматы", category: "Ведущий", event_format: "корпоратив", date: "2026-12-19", budget_kzt: "1000000", language: "русский", duration_hours: "6" },
  },
  {
    label: "Редкая категория",
    description: "Один флорист, длительность не указана",
    values: { city: "Алматы", category: "Флорист", event_format: "свадьба", date: "2026-11-14", budget_kzt: "300000", language: "русский", duration_hours: "6" },
  },
  {
    label: "Нет совпадений",
    description: "Категория есть, бюджет исключает всех",
    values: { city: "Алматы", category: "Ведущий", event_format: "корпоратив", date: "2026-11-14", budget_kzt: "100000", language: "русский", duration_hours: "6" },
  },
  {
    label: "Нет категории",
    description: "В городе нет выбранной категории",
    values: { city: "Астана", category: "Декоратор", event_format: "корпоратив", date: "2026-11-14", budget_kzt: "3000000", language: "", duration_hours: "" },
  },
];

interface DemoPresetsProps {
  onSelect: (values: FormValues) => void;
  disabled?: boolean;
}

export function DemoPresets({ onSelect, disabled }: DemoPresetsProps) {
  return (
    <section className="presets" aria-labelledby="presets-title">
      <div>
        <p className="eyebrow" id="presets-title">Быстрые сценарии</p>
        <p className="presets-note">Заполняют форму, но не запускают поиск</p>
      </div>
      <div className="preset-list">
        {demoPresets.map((preset) => (
          <button
            className="preset-button"
            disabled={disabled}
            key={preset.label}
            onClick={() => onSelect(preset.values)}
            title={preset.description}
            type="button"
          >
            {preset.label}
          </button>
        ))}
      </div>
    </section>
  );
}
