import type { ExclusionCode, SearchDiagnostics } from "../../api/contracts";
import { useLocale } from "../../i18n/LocaleContext";
import type { TranslationKey } from "../../i18n/translations";

interface FunnelDefinition {
  code: ExclusionCode;
  label: TranslationKey;
  omittedKey?: "language" | "duration_hours";
  omittedLabel?: TranslationKey;
}

const funnelOrder: FunnelDefinition[] = [
  { code: "EVENT_FORMAT_UNSUPPORTED", label: "diagnostics.format" },
  { code: "BUDGET_TOO_LOW", label: "diagnostics.budget" },
  { code: "LANGUAGE_UNSUPPORTED", label: "diagnostics.language", omittedKey: "language", omittedLabel: "diagnostics.languageOmitted" },
  { code: "DURATION_EXCEEDED", label: "diagnostics.duration", omittedKey: "duration_hours", omittedLabel: "diagnostics.durationOmitted" },
  { code: "BUSY_ON_DATE", label: "diagnostics.availability" },
];

interface DiagnosticsPanelProps {
  diagnostics: SearchDiagnostics;
  mode: "matches" | "empty";
}

interface FunnelStep {
  code: string;
  label: TranslationKey;
  remaining: number;
  excluded: number;
  omitted: boolean;
}

export function buildFunnel(diagnostics: SearchDiagnostics): FunnelStep[] {
  let remaining = diagnostics.catalog_count;
  const steps: FunnelStep[] = [{ code: "CATALOG", label: "diagnostics.catalog", remaining, excluded: 0, omitted: false }];

  for (const definition of funnelOrder) {
    const omitted = Boolean(definition.omittedKey && diagnostics.omitted_optional_checks.includes(definition.omittedKey));
    const excluded = omitted ? 0 : diagnostics.excluded_counts[definition.code];
    remaining -= excluded;
    steps.push({
      code: definition.code,
      label: omitted ? definition.omittedLabel! : definition.label,
      remaining,
      excluded,
      omitted,
    });
  }

  steps.push({ code: "RETURNED", label: "diagnostics.shown", remaining: diagnostics.returned_count, excluded: 0, omitted: false });
  return steps;
}

export function DiagnosticsPanel({ diagnostics, mode }: DiagnosticsPanelProps) {
  const { t } = useLocale();
  const steps = buildFunnel(diagnostics);
  const sortRule = diagnostics.applied_order[0] === "price_from_kzt:asc" && diagnostics.applied_order[1] === "id:asc"
    ? t("diagnostics.sortPriceId")
    : t("diagnostics.sortApi");

  return (
    <details className="diagnostics" open={mode === "matches"}>
      <summary>{t("diagnostics.summary")}</summary>
      <div className="diagnostics-content">
        <div className="diagnostics-heading">
          <h3>{t(mode === "matches" ? "diagnostics.title" : "diagnostics.emptyTitle")}</h3>
          <p>{t("diagnostics.subtitle")}</p>
        </div>
        <ol className="funnel-list">
          {steps.map((step) => {
            const width = diagnostics.catalog_count === 0 ? 0 : Math.max(0, Math.min(100, (step.remaining / diagnostics.catalog_count) * 100));
            return (
              <li className={step.omitted ? "funnel-step omitted" : "funnel-step"} key={step.code}>
                <div className="funnel-step-heading">
                  <span>{t(step.label)}</span>
                  <strong>{step.remaining}</strong>
                </div>
                <div className="funnel-track" aria-hidden="true"><span style={{ width: `${width}%` }} /></div>
                {step.excluded > 0 && <small>−{step.excluded}</small>}
              </li>
            );
          })}
        </ol>
        <p className="diagnostics-result">
          {t("diagnostics.eligible")}: <strong>{diagnostics.eligible_count}</strong>
          <span aria-hidden="true"> · </span>
          {t("diagnostics.shown")}: <strong>{diagnostics.returned_count}</strong>
        </p>
        <p className="order-note" title={diagnostics.applied_order.join(" → ")}>{sortRule}</p>
      </div>
    </details>
  );
}
