import type { SearchRequest, SearchResponse } from "./contracts";

export interface RecommendationGateway {
  recommend(request: SearchRequest, options?: { signal?: AbortSignal }): Promise<SearchResponse>;
}
