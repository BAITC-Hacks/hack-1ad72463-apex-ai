import type { SearchDiagnostics } from "../../api/contracts";

const reasonLabels = {
  EVENT_FORMAT_UNSUPPORTED: "Не подходит формат",
  BUDGET_TOO_LOW: "Выше бюджета",
  LANGUAGE_UNSUPPORTED: "Не подходит язык",
  DURATION_EXCEEDED: "Не подходит длительность",
  BUSY_ON_DATE: "Заняты на эту дату",
} as const;

interface DiagnosticsPanelProps {
  diagnostics: SearchDiagnostics;
}

export function DiagnosticsPanel({ diagnostics }: DiagnosticsPanelProps) {
  return (
    <details className="diagnostics">
      <summary>Как сформирован результат</summary>
      <div className="diagnostics-content">
        <dl className="diagnostics-totals">
          <div><dt>В категории</dt><dd>{diagnostics.catalog_count}</dd></div>
          <div><dt>Подходят</dt><dd>{diagnostics.eligible_count}</dd></div>
          <div><dt>Показаны</dt><dd>{diagnostics.returned_count}</dd></div>
        </dl>
        <div className="diagnostic-reasons">
          <p className="diagnostics-label">Причины исключения</p>
          <ul>
            {Object.entries(reasonLabels).map(([code, label]) => (
              <li key={code}><span>{label}</span><strong>{diagnostics.excluded_counts[code as keyof typeof reasonLabels]}</strong></li>
            ))}
          </ul>
        </div>
        <p className="order-note">Порядок API: {diagnostics.applied_order.join(" → ") || "не указан"}</p>
      </div>
    </details>
  );
}
