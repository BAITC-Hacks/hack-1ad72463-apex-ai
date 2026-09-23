import type { SearchAlternative } from "../../api/contracts";
import { useLocale } from "../../i18n/LocaleContext";
import { formatAlternativeCount, formatBudgetTarget, formatDateOnly, formatDistanceDays } from "../../i18n/format";

interface AlternativeSuggestionsProps {
  loading: boolean;
  alternatives: SearchAlternative[];
  onApply: (alternative: SearchAlternative) => void;
}

export function AlternativeSuggestions({ loading, alternatives, onApply }: AlternativeSuggestionsProps) {
  const { locale, t } = useLocale();
  if (!loading && alternatives.length === 0) return null;

  return (
    <section className="alternative-suggestions" aria-labelledby="alternatives-title">
      <div className="alternatives-heading">
        <h3 id="alternatives-title">{t("alternatives.title")}</h3>
        <p>{t("alternatives.copy")}</p>
      </div>
      {loading ? (
        <p className="alternatives-loading" role="status">
          <span className="loading-dot" aria-hidden="true" />
          {t("alternatives.loading")}
        </p>
      ) : (
        <div className="alternatives-grid">
          {alternatives.map((alternative) => (
            <article className="alternative-card" key={alternative.type}>
              <p className="alternative-type">
                {alternative.type === "BUDGET" ? t("alternatives.budget") : t("alternatives.date")}
              </p>
              <p className="alternative-value">
                {alternative.type === "BUDGET"
                  ? formatBudgetTarget(alternative.suggested_budget_kzt, locale)
                  : formatDateOnly(alternative.suggested_date, locale)}
              </p>
              <p className="alternative-result">{formatAlternativeCount(alternative.eligible_count, locale)}</p>
              {alternative.type === "DATE" && (
                <p className="alternative-distance">{formatDistanceDays(alternative.distance_days, locale)}</p>
              )}
              <button className="alternative-button" type="button" onClick={() => onApply(alternative)}>
                {t("alternatives.apply")}
              </button>
            </article>
          ))}
        </div>
      )}
    </section>
  );
}
