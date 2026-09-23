import type { SearchAlternative, SearchResponse } from "../../api/contracts";
import { useLocale } from "../../i18n/LocaleContext";
import { formatResultSummary } from "../../i18n/format";
import { ContractorCard } from "./ContractorCard";
import { DiagnosticsPanel } from "./DiagnosticsPanel";
import { EmptyState } from "./EmptyState";

interface ResultsPanelProps {
  response: SearchResponse | null;
  alternativesLoading: boolean;
  alternatives: SearchAlternative[];
  onApplyAlternative: (alternative: SearchAlternative) => void;
}

export function ResultsPanel({ response, alternativesLoading, alternatives, onApplyAlternative }: ResultsPanelProps) {
  const { locale, t } = useLocale();
  if (!response) {
    return (
      <div className="idle-state">
        <div className="idle-orbit" aria-hidden="true"><span>A</span></div>
        <p className="eyebrow">{t("results.idleEyebrow")}</p>
        <h2>{t("results.idleTitle")}</h2>
        <p>{t("results.idleCopy")}</p>
      </div>
    );
  }

  if (response.status !== "MATCHES_FOUND") {
    return (
      <EmptyState
        response={response}
        alternativesLoading={alternativesLoading}
        alternatives={alternatives}
        onApplyAlternative={onApplyAlternative}
      />
    );
  }

  return (
    <div className="results-success">
      <div className="results-intro">
        <div>
          <p className="eyebrow">{t("results.eyebrow")}</p>
          <h2>{t("results.title")}</h2>
          <p>{formatResultSummary(response.diagnostics.eligible_count, response.diagnostics.returned_count, locale)}</p>
        </div>
        <span className="success-mark" aria-label={t("results.successAria")}>✓</span>
      </div>
      <div className="contractor-list">
        {response.results.map((contractor, index) => <ContractorCard contractor={contractor} index={index} key={contractor.id} />)}
      </div>
      <DiagnosticsPanel diagnostics={response.diagnostics} mode="matches" />
    </div>
  );
}
