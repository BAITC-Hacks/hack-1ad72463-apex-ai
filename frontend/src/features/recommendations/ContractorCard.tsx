import type { Recommendation } from "../../api/contracts";
import { useLocale } from "../../i18n/LocaleContext";
import { formatMaxHours, formatPriceFrom } from "../../i18n/format";
import { categoryLabels, cityLabels, displayLabel } from "../../i18n/translations";

interface ContractorCardProps {
  contractor: Recommendation;
  index: number;
}

export function ContractorCard({ contractor, index }: ContractorCardProps) {
  const { locale, t } = useLocale();
  return (
    <article className="contractor-card">
      <div className="card-number" aria-hidden="true">{String(index + 1).padStart(2, "0")}</div>
      <div className="contractor-main">
        <div className="contractor-heading">
          <div>
            <p className="contractor-meta">{displayLabel(categoryLabels, contractor.category, locale)} · {displayLabel(cityLabels, contractor.city, locale)}</p>
            <h3>{contractor.anon_name}</h3>
          </div>
          <p className="price">{formatPriceFrom(contractor.price_from_kzt, locale)}</p>
        </div>

        <p className="explanation">{contractor.explanation}</p>

        <div className="card-tags" aria-label={t("card.tagsAria")}>
          {contractor.max_hours !== null && <span className="badge badge-neutral">{formatMaxHours(contractor.max_hours, locale)}</span>}
          {contractor.synthetic && <span className="badge badge-synthetic">{t("card.synthetic")}</span>}
          {contractor.price_imputed && <span className="badge badge-warning">{t("card.priceEstimated")}</span>}
          {contractor.city_imputed && <span className="badge badge-warning">{t("card.cityImputed")}</span>}
        </div>

        {contractor.conditions_to_confirm && contractor.conditions_to_confirm.length > 0 && (
          <div className="conditions">
            <p>{t("card.conditions")}</p>
            {locale !== "ru" && <small className="source-language-note">{t("card.originalText")}</small>}
            <ul>
              {contractor.conditions_to_confirm.map((condition) => <li key={condition.fact_id}>{condition.text}</li>)}
            </ul>
          </div>
        )}
      </div>
    </article>
  );
}
