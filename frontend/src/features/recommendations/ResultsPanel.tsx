import type { SearchResponse } from "../../api/contracts";
import { ContractorCard } from "./ContractorCard";
import { DiagnosticsPanel } from "./DiagnosticsPanel";
import { EmptyState } from "./EmptyState";

interface ResultsPanelProps {
  response: SearchResponse | null;
}

function resultSummary(response: SearchResponse): string {
  const { eligible_count, returned_count } = response.diagnostics;
  if (eligible_count === 1) return "Подходит 1 подрядчик";
  if (eligible_count > returned_count) return `Подходят ${eligible_count} подрядчика — показываем первые ${returned_count}`;
  return `Подходят ${eligible_count} подрядчика`;
}

export function ResultsPanel({ response }: ResultsPanelProps) {
  if (!response) {
    return (
      <div className="idle-state">
        <div className="idle-orbit" aria-hidden="true"><span>A</span></div>
        <p className="eyebrow">Персональный подбор</p>
        <h2>Здесь появятся рекомендации</h2>
        <p>Заполните параметры или выберите быстрый сценарий. Мы покажем до трёх вариантов и объясним результат.</p>
      </div>
    );
  }

  if (response.status !== "MATCHES_FOUND") return <EmptyState response={response} />;

  return (
    <div className="results-success">
      <div className="results-intro">
        <div>
          <p className="eyebrow">Результат подбора</p>
          <h2>Нашли подходящие варианты</h2>
          <p>{resultSummary(response)}</p>
        </div>
        <span className="success-mark" aria-label="Поиск завершён">✓</span>
      </div>
      <div className="contractor-list">
        {response.results.map((contractor, index) => <ContractorCard contractor={contractor} index={index} key={contractor.id} />)}
      </div>
      <DiagnosticsPanel diagnostics={response.diagnostics} />
    </div>
  );
}
