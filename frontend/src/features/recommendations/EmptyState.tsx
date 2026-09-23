import type { SearchResponse } from "../../api/contracts";
import { DiagnosticsPanel } from "./DiagnosticsPanel";

interface EmptyStateProps {
  response: SearchResponse;
}

export function EmptyState({ response }: EmptyStateProps) {
  const noCatalog = response.status === "NO_CATALOG";
  return (
    <div className={`empty-state ${noCatalog ? "empty-catalog" : "empty-match"}`}>
      <div className="empty-icon" aria-hidden="true">{noCatalog ? "⌁" : "≋"}</div>
      <p className="eyebrow">{noCatalog ? "Каталог" : "Условия поиска"}</p>
      <h2>{noCatalog ? "В этом городе пока нет подрядчиков этой категории" : "Подрядчики есть, но никто не подходит под выбранные условия"}</h2>
      <p>{noCatalog ? "Попробуйте изменить город или категорию." : response.message}</p>
      {!noCatalog && <DiagnosticsPanel diagnostics={response.diagnostics} />}
    </div>
  );
}
