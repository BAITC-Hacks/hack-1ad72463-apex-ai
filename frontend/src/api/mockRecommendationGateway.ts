import type { AlternativesResponse, Recommendation, SearchMetadata, SearchRequest, SearchResponse } from "./contracts";
import { CatalogUnavailableError, ValidationApiError } from "./errors";
import type { RecommendationGateway } from "./recommendationGateway";
import type { GatewayOptions } from "./recommendationGateway";
import { alternativesResponseSchema, searchResponseSchema } from "./schemas";

export type MockFailure = "none" | "validation" | "catalog-unavailable";

const metadata: SearchMetadata = {
  snapshot_version: "demo-fixtures-v1",
  loader_version: "mock-loader-v1",
  policy_version: "contract-fixtures-v1",
  facts_version: "mock-facts-v1",
  calendar_window: { from: "2026-09-23", to: "2026-12-31" },
};

const excludedZero = {
  EVENT_FORMAT_UNSUPPORTED: 0,
  BUDGET_TOO_LOW: 0,
  LANGUAGE_UNSUPPORTED: 0,
  DURATION_EXCEEDED: 0,
  BUSY_ON_DATE: 0,
};

const cards: Recommendation[] = [
  {
    id: "FIX-LEAD-01",
    anon_name: "Алекс Меридиан",
    city: "Алматы",
    category: "Ведущий",
    price_from_kzt: 650_000,
    max_hours: 8,
    synthetic: true,
    price_imputed: false,
    city_imputed: false,
    explanation: "Профиль поддерживает корпоративы на русском языке до 8 часов, а стартовая цена укладывается в выбранный бюджет. В описании отмечены персональный сценарий и современные интерактивы.",
    conditions_to_confirm: [{ fact_id: "FIX-FACT-01", text: "Уточните состав интерактивной программы для вашей площадки." }],
  },
  {
    id: "FIX-LEAD-02",
    anon_name: "Мира Асыл",
    city: "Алматы",
    category: "Ведущий",
    price_from_kzt: 700_000,
    max_hours: 6,
    synthetic: true,
    price_imputed: true,
    city_imputed: false,
    explanation: "Профиль соответствует формату корпоратива, русскому языку и запрошенной длительности 6 часов. В описании акцент сделан на динамичной программе без длинных речей.",
  },
  {
    id: "FIX-LEAD-03",
    anon_name: "Данияр Сценарий",
    city: "Алматы",
    category: "Ведущий",
    price_from_kzt: 1_000_000,
    max_hours: 10,
    synthetic: true,
    price_imputed: false,
    city_imputed: true,
    explanation: "Стартовая цена равна указанному бюджету, а лимит времени покрывает шестичасовую программу. В профиле отмечена современная подача с уважением к традициям.",
  },
];

function diagnostics(catalog: number, eligible: number, returned: number, excluded = excludedZero) {
  return {
    catalog_count: catalog,
    eligible_count: eligible,
    returned_count: returned,
    excluded_counts: excluded,
    omitted_optional_checks: [] as Array<"language" | "duration_hours">,
    applied_order: ["price_from_kzt:asc", "id:asc"],
  };
}

const popular: SearchResponse = {
  status: "MATCHES_FOUND",
  message: "Подходят 4 подрядчика; показаны первые 3 по стартовой цене.",
  results: [...cards],
  diagnostics: diagnostics(10, 4, 3, { ...excludedZero, EVENT_FORMAT_UNSUPPORTED: 1, BUDGET_TOO_LOW: 2, BUSY_ON_DATE: 3 }),
  metadata,
};

const busyDate: SearchResponse = {
  status: "MATCHES_FOUND",
  message: "Подходит 1 подрядчик; остальные заняты на выбранную дату или не прошли условия.",
  results: [{
    ...cards[0]!,
    id: "FIX-LEAD-04",
    anon_name: "Кира Нова",
    price_from_kzt: 900_000,
    max_hours: 7,
    explanation: "Профиль соответствует корпоративу на русском языке и доступен по календарю демонстрационной фикстуры. Стартовая цена укладывается в бюджет, длительность до 7 часов покрывает запрос.",
  }],
  diagnostics: diagnostics(10, 1, 1, { ...excludedZero, EVENT_FORMAT_UNSUPPORTED: 1, BUDGET_TOO_LOW: 2, BUSY_ON_DATE: 6 }),
  metadata,
};

const rare: SearchResponse = {
  status: "MATCHES_FOUND",
  message: "Подходит 1 подрядчик из 2 профилей этой категории.",
  results: [{
    id: "FIX-FLORIST-01",
    anon_name: "Студия Айша",
    city: "Алматы",
    category: "Флорист",
    price_from_kzt: 250_000,
    max_hours: null,
    synthetic: true,
    price_imputed: false,
    city_imputed: false,
    explanation: "Студия работает со свадебным форматом, а стартовая цена укладывается в бюджет. Для флористики ограничение часов неприменимо; один другой профиль занят на выбранную дату.",
  }],
  diagnostics: diagnostics(2, 1, 1, { ...excludedZero, BUSY_ON_DATE: 1 }),
  metadata,
};

const noMatch: SearchResponse = {
  status: "NO_MATCH",
  message: "Подрядчики этой категории есть, но стартовая цена каждого выше выбранного бюджета.",
  results: [],
  diagnostics: diagnostics(10, 0, 0, { ...excludedZero, EVENT_FORMAT_UNSUPPORTED: 1, BUDGET_TOO_LOW: 9 }),
  metadata,
};

const noCatalog: SearchResponse = {
  status: "NO_CATALOG",
  message: "В Астане пока нет подрядчиков категории «Декоратор».",
  results: [],
  diagnostics: diagnostics(0, 0, 0),
  metadata,
};

function fixtureKey(request: SearchRequest): string {
  return [request.city, request.category, request.event_format, request.date, request.budget_kzt, request.language ?? "", request.duration_hours ?? ""].join("|");
}

const fixtures = new Map<string, SearchResponse>([
  ["Алматы|Ведущий|корпоратив|2026-11-14|1000000|русский|6", popular],
  ["Алматы|Ведущий|корпоратив|2026-12-19|1000000|русский|6", busyDate],
  ["Алматы|Флорист|свадьба|2026-11-14|300000|русский|6", rare],
  ["Алматы|Ведущий|корпоратив|2026-11-14|100000|русский|6", noMatch],
  ["Астана|Декоратор|корпоратив|2026-11-14|3000000||", noCatalog],
]);

const alternativesFixtures = new Map<string, AlternativesResponse>([
  ["Алматы|Ведущий|корпоратив|2026-11-14|100000|русский|6", {
    alternatives: [
      { type: "BUDGET", current_budget_kzt: 100_000, suggested_budget_kzt: 650_000, eligible_count: 4 },
      { type: "DATE", current_date: "2026-11-14", suggested_date: "2026-11-15", distance_days: 1, eligible_count: 2 },
    ],
  }],
]);

export class MockRecommendationGateway implements RecommendationGateway {
  constructor(private readonly failure: MockFailure = "none") {}

  async recommend(request: SearchRequest, options?: GatewayOptions): Promise<SearchResponse> {
    if (options?.signal?.aborted) throw new DOMException("Aborted", "AbortError");
    if (this.failure === "validation") {
      throw new ValidationApiError([{ field: "date", code: "DATE_OUT_OF_RANGE", message: "Дата вне календарного окна." }]);
    }
    if (this.failure === "catalog-unavailable") throw new CatalogUnavailableError();

    const fixture = fixtures.get(fixtureKey(request)) ?? {
      ...noMatch,
      message: "Для этой комбинации в DEMO-режиме нет отдельной контрактной фикстуры. Условия не были изменены.",
    };
    return searchResponseSchema.parse(fixture) as SearchResponse;
  }

  async getAlternatives(request: SearchRequest, options?: GatewayOptions): Promise<AlternativesResponse> {
    if (options?.signal?.aborted) throw new DOMException("Aborted", "AbortError");
    const fixture = alternativesFixtures.get(fixtureKey(request)) ?? { alternatives: [] };
    return alternativesResponseSchema.parse(fixture) as AlternativesResponse;
  }
}

export const mockFixtures = { popular, busyDate, rare, noMatch, noCatalog };
