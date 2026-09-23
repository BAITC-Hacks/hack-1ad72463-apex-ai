import { HttpRecommendationGateway } from "../api/httpRecommendationGateway";
import { MockRecommendationGateway } from "../api/mockRecommendationGateway";
import type { RecommendationGateway } from "../api/recommendationGateway";

export type ApiMode = "mock" | "http";

export interface AppConfig {
  apiMode: ApiMode;
  apiBaseUrl: string;
}

function readApiMode(value: string | undefined): ApiMode {
  return value === "http" ? "http" : "mock";
}

export const appConfig: AppConfig = {
  apiMode: readApiMode(import.meta.env.VITE_API_MODE),
  apiBaseUrl: import.meta.env.VITE_API_BASE_URL?.trim() || "http://localhost:8080",
};

export function createRecommendationGateway(config: AppConfig): RecommendationGateway {
  return config.apiMode === "http"
    ? new HttpRecommendationGateway(config.apiBaseUrl)
    : new MockRecommendationGateway();
}
