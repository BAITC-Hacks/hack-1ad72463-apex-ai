import type { AlternativesResponse, SearchRequest, SearchResponse } from "./contracts";

export interface RecommendationGateway {
  recommend(request: SearchRequest, options?: { signal?: AbortSignal }): Promise<SearchResponse>;
  getAlternatives(request: SearchRequest, options?: { signal?: AbortSignal }): Promise<AlternativesResponse>;
}
