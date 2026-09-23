import type { AlternativesResponse, SearchRequest, SearchResponse } from "./contracts";
import type { Locale } from "../i18n/translations";

export interface GatewayOptions {
  signal?: AbortSignal;
  locale?: Locale;
}

export interface RecommendationGateway {
  recommend(request: SearchRequest, options?: GatewayOptions): Promise<SearchResponse>;
  getAlternatives(request: SearchRequest, options?: GatewayOptions): Promise<AlternativesResponse>;
}
