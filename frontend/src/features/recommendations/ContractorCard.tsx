import type { Recommendation } from "../../api/contracts";
import { formatPriceFrom } from "../../shared/formatters/currency";

interface ContractorCardProps {
  contractor: Recommendation;
  index: number;
}

export function ContractorCard({ contractor, index }: ContractorCardProps) {
  return (
    <article className="contractor-card">
      <div className="card-number" aria-hidden="true">{String(index + 1).padStart(2, "0")}</div>
      <div className="contractor-main">
        <div className="contractor-heading">
          <div>
            <p className="contractor-meta">{contractor.category} · {contractor.city}</p>
            <h3>{contractor.anon_name}</h3>
          </div>
          <p className="price">{formatPriceFrom(contractor.price_from_kzt)}</p>
        </div>

        <p className="explanation">{contractor.explanation}</p>

        <div className="card-tags" aria-label="Условия и происхождение данных">
          {contractor.max_hours !== null && <span className="badge badge-neutral">до {contractor.max_hours} ч</span>}
          {contractor.synthetic && <span className="badge badge-synthetic">Синтетический профиль</span>}
          {contractor.price_imputed && <span className="badge badge-warning">Цена оценочная</span>}
          {contractor.city_imputed && <span className="badge badge-warning">Город восстановлен</span>}
        </div>

        {contractor.conditions_to_confirm && contractor.conditions_to_confirm.length > 0 && (
          <div className="conditions">
            <p>Стоит уточнить</p>
            <ul>
              {contractor.conditions_to_confirm.map((condition) => <li key={condition.fact_id}>{condition.text}</li>)}
            </ul>
          </div>
        )}
      </div>
    </article>
  );
}
