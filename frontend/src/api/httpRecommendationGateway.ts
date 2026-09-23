import type { AlternativesResponse, SearchRequest, SearchResponse } from "./contracts";
import {
  CatalogUnavailableError,
  HttpApiError,
  InvalidApiResponseError,
  NetworkApiError,
  TimeoutApiError,
  ValidationApiError,
  isAbortError,
} from "./errors";
import type { RecommendationGateway } from "./recommendationGateway";
import type { GatewayOptions } from "./recommendationGateway";
import { alternativesResponseSchema, errorResponseSchema, searchResponseSchema } from "./schemas";

function recommendationUrl(baseUrl: string, suffix = ""): string {
  const normalized = baseUrl.replace(/\/$/, "");
  const recommendationsRoot = normalized.endsWith("/api")
    ? `${normalized}/recommendations`
    : `${normalized}/api/recommendations`;
  return `${recommendationsRoot}${suffix}`;
}

export class HttpRecommendationGateway implements RecommendationGateway {
  constructor(
    private readonly baseUrl: string,
    private readonly timeoutMs = 10_000,
  ) {}

  async recommend(request: SearchRequest, options?: GatewayOptions): Promise<SearchResponse> {
    const controller = new AbortController();
    let timedOut = false;
    const onExternalAbort = () => controller.abort();
    options?.signal?.addEventListener("abort", onExternalAbort, { once: true });
    const timeout = window.setTimeout(() => {
      timedOut = true;
      controller.abort();
    }, this.timeoutMs);

    try {
      const response = await fetch(recommendationUrl(this.baseUrl), {
        method: "POST",
        headers: { "Content-Type": "application/json", Accept: "application/json", "Accept-Language": options?.locale ?? "ru" },
        body: JSON.stringify(request),
        signal: controller.signal,
      });

      const payload: unknown = await response.json().catch(() => {
        throw new InvalidApiResponseError();
      });

      if (response.ok) {
        const parsed = searchResponseSchema.safeParse(payload);
        if (!parsed.success) throw new InvalidApiResponseError();
        return parsed.data as SearchResponse;
      }

      const errorPayload = errorResponseSchema.safeParse(payload);
      if (!errorPayload.success) throw new InvalidApiResponseError();
      if (response.status === 422 && errorPayload.data.error.code === "VALIDATION_ERROR") {
        throw new ValidationApiError(errorPayload.data.error.details);
      }
      if (response.status === 503 && errorPayload.data.error.code === "CATALOG_UNAVAILABLE") {
        throw new CatalogUnavailableError();
      }
      throw new HttpApiError(response.status, errorPayload.data.error.code);
    } catch (error) {
      if (timedOut) throw new TimeoutApiError();
      if (options?.signal?.aborted || isAbortError(error)) throw error;
      if (
        error instanceof ValidationApiError ||
        error instanceof CatalogUnavailableError ||
        error instanceof InvalidApiResponseError ||
        error instanceof HttpApiError
      ) {
        throw error;
      }
      throw new NetworkApiError();
    } finally {
      window.clearTimeout(timeout);
      options?.signal?.removeEventListener("abort", onExternalAbort);
    }
  }

  async getAlternatives(request: SearchRequest, options?: GatewayOptions): Promise<AlternativesResponse> {
    const controller = new AbortController();
    let timedOut = false;
    const onExternalAbort = () => controller.abort();
    options?.signal?.addEventListener("abort", onExternalAbort, { once: true });
    const timeout = window.setTimeout(() => {
      timedOut = true;
      controller.abort();
    }, this.timeoutMs);

    try {
      const response = await fetch(recommendationUrl(this.baseUrl, "/alternatives"), {
        method: "POST",
        headers: { "Content-Type": "application/json", Accept: "application/json", "Accept-Language": options?.locale ?? "ru" },
        body: JSON.stringify(request),
        signal: controller.signal,
      });

      const payload: unknown = await response.json().catch(() => {
        throw new InvalidApiResponseError();
      });

      if (response.ok) {
        const parsed = alternativesResponseSchema.safeParse(payload);
        if (!parsed.success) throw new InvalidApiResponseError();
        return parsed.data as AlternativesResponse;
      }

      const errorPayload = errorResponseSchema.safeParse(payload);
      throw new HttpApiError(
        response.status,
        errorPayload.success ? errorPayload.data.error.code : "ALTERNATIVES_UNAVAILABLE",
      );
    } catch (error) {
      if (timedOut) throw new TimeoutApiError();
      if (options?.signal?.aborted || isAbortError(error)) throw error;
      if (error instanceof InvalidApiResponseError || error instanceof HttpApiError) throw error;
      throw new NetworkApiError();
    } finally {
      window.clearTimeout(timeout);
      options?.signal?.removeEventListener("abort", onExternalAbort);
    }
  }
}
