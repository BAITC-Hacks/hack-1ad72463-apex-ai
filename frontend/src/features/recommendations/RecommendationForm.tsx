import { useState } from "react";
import type { FormEvent } from "react";
import type { DateOnly, SearchRequest } from "../../api/contracts";
import { Field } from "../../shared/ui/Field";
import { DemoPresets, demoPresets } from "./DemoPresets";

export interface FormValues {
  city: string;
  category: string;
  event_format: string;
  date: string;
  budget_kzt: string;
  language: string;
  duration_hours: string;
}

type FormField = keyof FormValues;
export type FieldErrors = Partial<Record<FormField, string>>;

const cities = ["Алматы", "Астана", "Зарубежье"];
const categories = [
  "Банкетный зал", "Ведущий", "Ведущий церемонии", "Видеограф", "Декоратор", "Загородная площадка",
  "Инструменталист", "Лайв-бэнд", "Национальный ансамбль", "Отель", "Подарки и сувениры", "Ресторан",
  "Танцевальный коллектив", "Флорист", "Фото и видеобудки", "Фотограф", "Шоу-программа",
];
const formats = ["день рождения", "конференция", "корпоратив", "свадьба", "той", "юбилей"];
const languages = ["русский", "казахский", "английский"];

export const initialFormValues: FormValues = { ...demoPresets[0]!.values };

function validate(values: FormValues): FieldErrors {
  const errors: FieldErrors = {};
  if (!values.city) errors.city = "Выберите город.";
  if (!values.category) errors.category = "Выберите категорию.";
  if (!values.event_format) errors.event_format = "Выберите формат мероприятия.";
  if (!/^\d{4}-\d{2}-\d{2}$/.test(values.date)) errors.date = "Укажите дату мероприятия.";
  else if (values.date < "2026-09-23" || values.date > "2026-12-31") errors.date = "Выберите дату с 23 сентября по 31 декабря 2026 года.";

  const budget = Number(values.budget_kzt);
  if (!values.budget_kzt || !Number.isSafeInteger(budget) || budget <= 0) errors.budget_kzt = "Укажите целый бюджет больше нуля.";

  if (values.duration_hours) {
    const duration = Number(values.duration_hours);
    if (!Number.isFinite(duration) || duration <= 0) errors.duration_hours = "Укажите длительность больше нуля.";
  }
  return errors;
}

function toRequest(values: FormValues): SearchRequest {
  return {
    city: values.city,
    category: values.category,
    event_format: values.event_format,
    date: values.date as DateOnly,
    budget_kzt: Number(values.budget_kzt),
    ...(values.language ? { language: values.language } : {}),
    ...(values.duration_hours ? { duration_hours: Number(values.duration_hours) } : {}),
  };
}

interface RecommendationFormProps {
  loading: boolean;
  serverErrors: FieldErrors;
  onFieldChange: (field: FormField) => void;
  onSubmit: (request: SearchRequest) => void;
}

export function RecommendationForm({ loading, serverErrors, onFieldChange, onSubmit }: RecommendationFormProps) {
  const [values, setValues] = useState<FormValues>(initialFormValues);
  const [clientErrors, setClientErrors] = useState<FieldErrors>({});
  const errors = { ...serverErrors, ...clientErrors };

  const update = (field: FormField, value: string) => {
    setValues((current) => ({ ...current, [field]: value }));
    setClientErrors((current) => ({ ...current, [field]: undefined }));
    onFieldChange(field);
  };

  const applyPreset = (preset: FormValues) => {
    setValues({ ...preset });
    setClientErrors({});
    (Object.keys(preset) as FormField[]).forEach(onFieldChange);
  };

  const submit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const nextErrors = validate(values);
    setClientErrors(nextErrors);
    const firstError = Object.keys(nextErrors)[0] as FormField | undefined;
    if (firstError) {
      document.getElementById(firstError)?.focus();
      return;
    }
    onSubmit(toRequest(values));
  };

  const describedBy = (field: FormField) => errors[field] ? `${field}-error` : undefined;

  return (
    <div className="search-card">
      <div className="section-heading">
        <span className="step">01</span>
        <div>
          <p className="eyebrow">Параметры события</p>
          <h2>Расскажите о мероприятии</h2>
        </div>
      </div>

      <form className="search-form" onSubmit={submit} noValidate>
        <div className="field-grid">
          <Field id="city" label="Город" required error={errors.city}>
            <select id="city" value={values.city} onChange={(event) => update("city", event.target.value)} aria-invalid={Boolean(errors.city)} aria-describedby={describedBy("city")}>
              <option value="">Выберите город</option>
              {cities.map((city) => <option key={city}>{city}</option>)}
            </select>
          </Field>
          <Field id="category" label="Категория" required error={errors.category}>
            <select id="category" value={values.category} onChange={(event) => update("category", event.target.value)} aria-invalid={Boolean(errors.category)} aria-describedby={describedBy("category")}>
              <option value="">Выберите категорию</option>
              {categories.map((category) => <option key={category}>{category}</option>)}
            </select>
          </Field>
          <Field id="event_format" label="Формат мероприятия" required error={errors.event_format}>
            <select id="event_format" value={values.event_format} onChange={(event) => update("event_format", event.target.value)} aria-invalid={Boolean(errors.event_format)} aria-describedby={describedBy("event_format")}>
              <option value="">Выберите формат</option>
              {formats.map((format) => <option key={format}>{format}</option>)}
            </select>
          </Field>
          <Field id="date" label="Дата" required error={errors.date}>
            <input id="date" type="date" min="2026-09-23" max="2026-12-31" value={values.date} onChange={(event) => update("date", event.target.value)} aria-invalid={Boolean(errors.date)} aria-describedby={describedBy("date")} />
          </Field>
          <Field id="budget_kzt" label="Бюджет на подрядчика" required error={errors.budget_kzt} hint="Стартовая цена за мероприятие, ₸">
            <div className="input-suffix">
              <input id="budget_kzt" type="number" min="1" step="1" inputMode="numeric" value={values.budget_kzt} onChange={(event) => update("budget_kzt", event.target.value)} aria-invalid={Boolean(errors.budget_kzt)} aria-describedby={errors.budget_kzt ? "budget_kzt-error" : "budget_kzt-hint"} />
              <span aria-hidden="true">₸</span>
            </div>
          </Field>
          <Field id="language" label="Язык" error={errors.language}>
            <select id="language" value={values.language} onChange={(event) => update("language", event.target.value)} aria-invalid={Boolean(errors.language)} aria-describedby={describedBy("language")}>
              <option value="">Не важно</option>
              {languages.map((language) => <option key={language}>{language}</option>)}
            </select>
          </Field>
          <Field id="duration_hours" label="Длительность работы" error={errors.duration_hours} hint="Оставьте пустым, если не важно">
            <div className="input-suffix">
              <input id="duration_hours" type="number" min="0.5" step="0.5" value={values.duration_hours} onChange={(event) => update("duration_hours", event.target.value)} aria-invalid={Boolean(errors.duration_hours)} aria-describedby={errors.duration_hours ? "duration_hours-error" : "duration_hours-hint"} />
              <span aria-hidden="true">ч</span>
            </div>
          </Field>
        </div>

        <button className="primary-button" type="submit" disabled={loading}>
          <span>{loading ? "Подбираем варианты…" : "Найти подрядчиков"}</span>
          <span aria-hidden="true">→</span>
        </button>
      </form>

      <DemoPresets onSelect={applyPreset} disabled={loading} />
    </div>
  );
}
