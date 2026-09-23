import type { SearchAlternative, SearchResponse } from "../../api/contracts";
import { AlternativeSuggestions } from "./AlternativeSuggestions";
import { DiagnosticsPanel } from "./DiagnosticsPanel";

interface EmptyStateProps {
  response: SearchResponse;
  alternativesLoading: boolean;
  alternatives: SearchAlternative[];
  onApplyAlternative: (alternative: SearchAlternative) => void;
}

export function EmptyState({ response, alternativesLoading, alternatives, onApplyAlternative }: EmptyStateProps) {
  const noCatalog = response.status === "NO_CATALOG";
  return (
    <div className={`empty-state ${noCatalog ? "empty-catalog" : "empty-match"}`}>
      <div className="empty-icon" aria-hidden="true">{noCatalog ? "⌁" : "≋"}</div>
      <p className="eyebrow">{noCatalog ? "Каталог" : "Условия поиска"}</p>
      <h2>{noCatalog ? "В этом городе пока нет подрядчиков этой категории" : "Подрядчики есть, но никто не подходит под выбранные условия"}</h2>
      <p>{noCatalog ? "Попробуйте изменить город или категорию." : response.message}</p>
      {!noCatalog && <DiagnosticsPanel diagnostics={response.diagnostics} />}
      {!noCatalog && (
        <AlternativeSuggestions
          loading={alternativesLoading}
          alternatives={alternatives}
          onApply={onApplyAlternative}
        />
      )}
    </div>
  );
}
