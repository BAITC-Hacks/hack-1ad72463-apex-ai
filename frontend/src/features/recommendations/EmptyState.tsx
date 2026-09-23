import type { SearchAlternative, SearchResponse } from "../../api/contracts";
import { useLocale } from "../../i18n/LocaleContext";
import { AlternativeSuggestions } from "./AlternativeSuggestions";
import { DiagnosticsPanel } from "./DiagnosticsPanel";

interface EmptyStateProps {
  response: SearchResponse;
  alternativesLoading: boolean;
  alternatives: SearchAlternative[];
  onApplyAlternative: (alternative: SearchAlternative) => void;
}

export function EmptyState({ response, alternativesLoading, alternatives, onApplyAlternative }: EmptyStateProps) {
  const { t } = useLocale();
  const noCatalog = response.status === "NO_CATALOG";
  return (
    <div className={`empty-state ${noCatalog ? "empty-catalog" : "empty-match"}`}>
      <div className="empty-icon" aria-hidden="true">{noCatalog ? "⌁" : "≋"}</div>
      <p className="eyebrow">{noCatalog ? t("empty.catalogEyebrow") : t("empty.matchEyebrow")}</p>
      <h2>{noCatalog ? t("empty.catalogTitle") : t("empty.matchTitle")}</h2>
      <p>{noCatalog ? t("empty.catalogCopy") : response.message}</p>
      {!noCatalog && <DiagnosticsPanel diagnostics={response.diagnostics} mode="empty" />}
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
