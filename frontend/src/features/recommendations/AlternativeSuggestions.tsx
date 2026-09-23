import type { SearchAlternative } from "../../api/contracts";

interface AlternativeSuggestionsProps {
  loading: boolean;
  alternatives: SearchAlternative[];
  onApply: (alternative: SearchAlternative) => void;
}

const budgetFormatter = new Intl.NumberFormat("ru-RU");
const monthNames = [
  "января", "февраля", "марта", "апреля", "мая", "июня",
  "июля", "августа", "сентября", "октября", "ноября", "декабря",
];

function formatDate(value: string): string {
  const [, month, day] = value.split("-").map(Number);
  return `${day} ${monthNames[month! - 1]}`;
}

function variantsText(count: number): string {
  const lastTwo = count % 100;
  const last = count % 10;
  if (lastTwo >= 11 && lastTwo <= 14) return `Появятся ${count} подходящих вариантов`;
  if (last === 1) return `Появится ${count} подходящий вариант`;
  if (last >= 2 && last <= 4) return `Появятся ${count} подходящих варианта`;
  return `Появятся ${count} подходящих вариантов`;
}

function daysLabel(count: number): string {
  const lastTwo = count % 100;
  const last = count % 10;
  if (lastTwo >= 11 && lastTwo <= 14) return "дней";
  if (last === 1) return "день";
  if (last >= 2 && last <= 4) return "дня";
  return "дней";
}

export function AlternativeSuggestions({ loading, alternatives, onApply }: AlternativeSuggestionsProps) {
  if (!loading && alternatives.length === 0) return null;

  return (
    <section className="alternative-suggestions" aria-labelledby="alternatives-title">
      <div className="alternatives-heading">
        <h3 id="alternatives-title">Что можно изменить?</h3>
        <p>Мы проверили варианты, изменяя только одно условие за раз.</p>
      </div>
      {loading ? (
        <p className="alternatives-loading" role="status">
          <span className="loading-dot" aria-hidden="true" />
          Ищем, что можно изменить…
        </p>
      ) : (
        <div className="alternatives-grid">
          {alternatives.map((alternative) => (
            <article className="alternative-card" key={alternative.type}>
              <p className="alternative-type">
                {alternative.type === "BUDGET" ? "Увеличить бюджет" : "Изменить дату"}
              </p>
              <p className="alternative-value">
                {alternative.type === "BUDGET"
                  ? `до ${budgetFormatter.format(alternative.suggested_budget_kzt)} ₸`
                  : formatDate(alternative.suggested_date)}
              </p>
              <p className="alternative-result">{variantsText(alternative.eligible_count)}</p>
              {alternative.type === "DATE" && (
                <p className="alternative-distance">±{alternative.distance_days} {daysLabel(alternative.distance_days)}</p>
              )}
              <button className="alternative-button" type="button" onClick={() => onApply(alternative)}>
                Применить и найти
              </button>
            </article>
          ))}
        </div>
      )}
    </section>
  );
}
