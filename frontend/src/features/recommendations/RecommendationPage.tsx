import { useEffect, useRef, useState } from "react";
import type { SearchAlternative, SearchRequest, SearchResponse } from "../../api/contracts";
import {
  CatalogUnavailableError,
  HttpApiError,
  InvalidApiResponseError,
  NetworkApiError,
  TimeoutApiError,
  ValidationApiError,
  isAbortError,
} from "../../api/errors";
import type { RecommendationGateway } from "../../api/recommendationGateway";
import { useLocale } from "../../i18n/LocaleContext";
import { ErrorState } from "./ErrorState";
import type { TechnicalErrorKind } from "./ErrorState";
import { LoadingState } from "./LoadingState";
import { RecommendationForm } from "./RecommendationForm";
import type { FieldErrors, FormValues } from "./RecommendationForm";
import { ResultsPanel } from "./ResultsPanel";

interface RecommendationPageProps {
  gateway: RecommendationGateway;
}

const knownFields = new Set<keyof FormValues>(["city", "category", "event_format", "date", "budget_kzt", "language", "duration_hours"]);

export function RecommendationPage({ gateway }: RecommendationPageProps) {
  const { locale, t } = useLocale();
  const [response, setResponse] = useState<SearchResponse | null>(null);
  const [loading, setLoading] = useState(false);
  const [fieldErrors, setFieldErrors] = useState<FieldErrors>({});
  const [technicalError, setTechnicalError] = useState<TechnicalErrorKind | null>(null);
  const [validationNotice, setValidationNotice] = useState<string | null>(null);
  const [alternativesLoading, setAlternativesLoading] = useState(false);
  const [alternatives, setAlternatives] = useState<SearchAlternative[]>([]);
  const [appliedRequest, setAppliedRequest] = useState<SearchRequest | null>(null);
  const lastRequest = useRef<SearchRequest | null>(null);
  const activeController = useRef<AbortController | null>(null);
  const alternativesController = useRef<AbortController | null>(null);
  const requestSequence = useRef(0);
  const alternativesSequence = useRef(0);

  useEffect(() => () => {
    activeController.current?.abort();
    alternativesController.current?.abort();
  }, []);

  const loadAlternatives = async (request: SearchRequest) => {
    const controller = new AbortController();
    alternativesController.current = controller;
    const sequence = ++alternativesSequence.current;
    setAlternativesLoading(true);

    try {
      const result = await gateway.getAlternatives(request, { signal: controller.signal, locale });
      if (sequence === alternativesSequence.current) setAlternatives(result.alternatives);
    } catch {
      if (sequence === alternativesSequence.current) setAlternatives([]);
    } finally {
      if (sequence === alternativesSequence.current) setAlternativesLoading(false);
    }
  };

  const recommend = async (request: SearchRequest) => {
    activeController.current?.abort();
    alternativesController.current?.abort();
    alternativesController.current = null;
    alternativesSequence.current += 1;
    const controller = new AbortController();
    activeController.current = controller;
    const sequence = ++requestSequence.current;
    lastRequest.current = request;
    setLoading(true);
    setResponse(null);
    setFieldErrors({});
    setTechnicalError(null);
    setValidationNotice(null);
    setAlternativesLoading(false);
    setAlternatives([]);

    try {
      const nextResponse = await gateway.recommend(request, { signal: controller.signal, locale });
      if (sequence === requestSequence.current) {
        setResponse(nextResponse);
        if (nextResponse.status === "NO_MATCH") void loadAlternatives(request);
      }
    } catch (error) {
      if (controller.signal.aborted || isAbortError(error) || sequence !== requestSequence.current) return;
      if (error instanceof ValidationApiError) {
        const nextFieldErrors: FieldErrors = {};
        const general: string[] = [];
        for (const detail of error.details) {
          if (knownFields.has(detail.field as keyof FormValues)) nextFieldErrors[detail.field as keyof FormValues] = detail.message;
          else general.push(detail.message);
        }
        setFieldErrors(nextFieldErrors);
        setValidationNotice(general[0] ?? t("validation.default"));
      } else if (error instanceof CatalogUnavailableError) {
        setTechnicalError("catalog");
      } else if (error instanceof NetworkApiError || error instanceof TimeoutApiError) {
        setTechnicalError("network");
      } else if (error instanceof InvalidApiResponseError) {
        setTechnicalError("invalid");
      } else if (error instanceof HttpApiError) {
        setTechnicalError("unexpected");
      } else {
        setTechnicalError("unexpected");
      }
    } finally {
      if (sequence === requestSequence.current) setLoading(false);
    }
  };

  const retry = () => {
    if (lastRequest.current) void recommend(lastRequest.current);
  };

  const applyAlternative = (alternative: SearchAlternative) => {
    const current = lastRequest.current;
    if (!current) return;
    const nextRequest: SearchRequest = alternative.type === "BUDGET"
      ? { ...current, budget_kzt: alternative.suggested_budget_kzt }
      : { ...current, date: alternative.suggested_date };
    setAppliedRequest(nextRequest);
    void recommend(nextRequest);
  };

  return (
    <main id="main">
      <section className="intro">
        <p className="eyebrow">{t("intro.eyebrow")}</p>
        <h1>{t("intro.title")}</h1>
        <p>{t("intro.copy")}</p>
      </section>

      <div className="workspace">
        <RecommendationForm
          loading={loading}
          appliedRequest={appliedRequest}
          serverErrors={fieldErrors}
          onFieldChange={(field) => setFieldErrors((current) => ({ ...current, [field]: undefined }))}
          onSubmit={(request) => void recommend(request)}
        />
        <section className="results-card" aria-label={t("results.aria")} aria-live="polite" aria-busy={loading}>
          {loading ? <LoadingState /> : technicalError ? <ErrorState kind={technicalError} onRetry={retry} /> : validationNotice ? (
            <div className="validation-state" role="alert">
              <div className="empty-icon danger" aria-hidden="true">!</div>
              <p className="eyebrow">{t("validation.eyebrow")}</p>
              <h2>{t("validation.title")}</h2>
              <p>{validationNotice}</p>
            </div>
          ) : (
            <ResultsPanel
              response={response}
              alternativesLoading={alternativesLoading}
              alternatives={alternatives}
              onApplyAlternative={applyAlternative}
            />
          )}
        </section>
      </div>
      <p className="disclaimer">{t("disclaimer")}</p>
    </main>
  );
}
