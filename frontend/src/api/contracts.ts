export type DateOnly = `${number}-${number}-${number}`;

export interface SearchRequest {
  city: string;
  date: DateOnly;
  event_format: string;
  category: string;
  budget_kzt: number;
  duration_hours?: number | null;
  language?: string | null;
}

export interface BudgetAlternative {
  type: "BUDGET";
  current_budget_kzt: number;
  suggested_budget_kzt: number;
  eligible_count: number;
}

export interface DateAlternative {
  type: "DATE";
  current_date: DateOnly;
  suggested_date: DateOnly;
  distance_days: number;
  eligible_count: number;
}

export type SearchAlternative = BudgetAlternative | DateAlternative;

export interface AlternativesResponse {
  alternatives: SearchAlternative[];
}

export type SearchStatus = "MATCHES_FOUND" | "NO_CATALOG" | "NO_MATCH";

export type ExclusionCode =
  | "EVENT_FORMAT_UNSUPPORTED"
  | "BUDGET_TOO_LOW"
  | "LANGUAGE_UNSUPPORTED"
  | "DURATION_EXCEEDED"
  | "BUSY_ON_DATE";

export interface ConditionToConfirm {
  fact_id: string;
  text: string;
}

export interface Recommendation {
  id: string;
  anon_name: string;
  city: string;
  category: string;
  price_from_kzt: number;
  max_hours: number | null;
  synthetic: boolean;
  price_imputed: boolean;
  city_imputed: boolean;
  explanation: string;
  conditions_to_confirm?: ConditionToConfirm[];
}

export interface SearchDiagnostics {
  catalog_count: number;
  eligible_count: number;
  returned_count: number;
  excluded_counts: Record<ExclusionCode, number>;
  omitted_optional_checks: Array<"language" | "duration_hours">;
  applied_order: string[];
}

export interface SearchMetadata {
  snapshot_version: string;
  loader_version: string;
  policy_version: string;
  facts_version: string;
  calendar_window: { from: DateOnly; to: DateOnly };
}

export interface SearchResponse {
  status: SearchStatus;
  message: string;
  results: Recommendation[];
  diagnostics: SearchDiagnostics;
  metadata: SearchMetadata;
}

export interface ErrorDetail {
  field: string;
  code: string;
  message: string;
}

export interface ErrorResponse {
  error: {
    code: string;
    details: ErrorDetail[];
  };
}
