# Apex Match — frontend-контракт интеграции

## 1. Статус контракта

Фактические endpoint ещё не предоставлены. [`ARCHITECTURE.md`](../ARCHITECTURE.md) предлагает `POST /api/recommendations`; до согласования с backend-командой это **ожидаемый**, а не реализованный маршрут.

Общие правила:

- JSON использует `snake_case`.
- Frontend-компоненты зависят от `RecommendationGateway`, а не от URL или `fetch`.
- `MockRecommendationGateway` и `HttpRecommendationGateway` реализуют один интерфейс и проходят одну runtime-схему.
- Типы в этом файле зеркалят текущий backend-проект. Если backend изменит схему, сначала меняются этот контракт и contract tests, затем код.

## 2. Gateway

```ts
export interface RecommendationGateway {
  recommend(
    request: SearchRequest,
    options?: { signal?: AbortSignal },
  ): Promise<SearchResponse>;
}

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
```

### Правила запроса

| Поле | Frontend-валидация |
|---|---|
| `city` | Непустая строка. Неизвестная backend пара город+категория может дать `NO_CATALOG`, а не ошибку. |
| `date` | Реальная date-only дата точно `YYYY-MM-DD`, в окне 2026-09-23…2026-12-31. Не преобразовывать в `Date.toISOString()`. |
| `event_format` | Одно из: `свадьба`, `той`, `корпоратив`, `конференция`, `юбилей`, `день рождения` до появления meta endpoint. |
| `category` | Непустая строка; fallback-справочник версионируется отдельно от компонентов. |
| `budget_kzt` | Safe integer `> 0`; бюджет на одного подрядчика. |
| `duration_hours` | Опциональное конечное число `> 0`; пустое значение опускается или передаётся `null` по финальной договорённости. |
| `language` | Опционально: `русский`, `казахский`, `английский`; пустое значение опускается или передаётся `null`. |

Frontend отправляет только поля контракта. Никакие mock-селекторы, UI-флаги или подписи в HTTP body не попадают.

## 3. Успешный ответ

```ts
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
```

### Инварианты runtime-валидации

- `results.length === diagnostics.returned_count`.
- `returned_count === min(eligible_count, 3)` и лежит в 0…3.
- `sum(excluded_counts) + eligible_count === catalog_count`.
- `MATCHES_FOUND` требует `eligible_count >= 1` и `results.length >= 1`.
- `NO_CATALOG` требует `catalog_count === 0`, `eligible_count === 0`, `results=[]`.
- `NO_MATCH` требует `catalog_count > 0`, `eligible_count === 0`, `results=[]`.
- `price_from_kzt` — положительное safe integer; в UI всегда «от».
- `max_hours` не интерпретируется frontend как бесконечная длительность; `null` можно показать как «не применимо».
- Нарушение любого инварианта даёт `InvalidApiResponseError`; частичный UI не рендерится.

## 4. Ошибки

```ts
export type FieldErrorCode =
  | "REQUIRED"
  | "INVALID_TYPE"
  | "INVALID_DATE"
  | "INVALID_ENUM"
  | "OUT_OF_RANGE"
  | "DATE_OUT_OF_RANGE"
  | "UNKNOWN_FIELD";

export interface ErrorDetail {
  field: string;
  code: FieldErrorCode | string;
  message: string;
}

export interface ErrorResponse {
  error: {
    code: "VALIDATION_ERROR" | "CATALOG_UNAVAILABLE" | string;
    details?: ErrorDetail[];
  };
}
```

| HTTP/сбой | Frontend-поведение |
|---|---|
| `422 VALIDATION_ERROR` | Известные `details.field` привязываются к полям; неизвестные остаются в общем alert. |
| `503 CATALOG_UNAVAILABLE` | Техническая ошибка с retry; не показывать `NO_CATALOG`. |
| Network/таймаут | Общая техническая ошибка; форма и последний подтверждённый запрос сохраняются. |
| Invalid JSON/schema | `InvalidApiResponseError`; не пытаться угадать формат. |

## 5. HTTP-адаптер

После появления backend:

```text
POST {API_BASE_URL}/api/recommendations
Content-Type: application/json
Accept: application/json
```

- Base URL задаётся через публичную build-time переменную; секретов в ней нет.
- Таймаут frontend — 10 секунд до сигнала ошибки; фактический backend-бюджет согласуется отдельно.
- Каждый новый запрос отменяет предыдущий; отмена не показывается как ошибка.
- После JSON parse ответ обязательно проходит runtime-схему и инварианты.
- CORS ещё не согласован. До этого нельзя обещать интеграцию из браузера.

## 6. Mock-адаптер

Mock нужен для параллельной frontend-работы, но не имитирует сам алгоритм подбора. Он возвращает заранее описанные contract fixtures:

- `MATCHES_FOUND` с 1, 2 и 3 карточками;
- `NO_CATALOG`;
- `NO_MATCH`;
- `VALIDATION_ERROR` с ошибками каждого поля;
- `CATALOG_UNAVAILABLE`, network error, timeout и invalid response.

Все профили mock имеют `FIX-*` и `synthetic: true`. Режим mock видим в development UI и не может быть production-значением по умолчанию.

## 7. Опции формы

В backend-документах нет готового meta endpoint. До его появления frontend использует `CatalogOptionsSource`:

```ts
export interface CatalogOptions {
  cities: string[];
  categories: string[];
  event_formats: string[];
  languages: string[];
  calendar_window: { from: DateOnly; to: DateOnly };
  source: "fallback" | "api";
  version: string;
}

export interface CatalogOptionsSource {
  getOptions(options?: { signal?: AbortSignal }): Promise<CatalogOptions>;
}
```

Fallback содержит только справочники, проверенные по CSV, и явное `source: "fallback"`. Он не содержит карточки, календари или логику поиска. После согласования meta endpoint добавляется HTTP-реализация без изменения формы.

## 8. P1-расширения, пока не согласованы

Backend пока не описал контракты для альтернатив «что изменить», meta и AI-интерпретации. Frontend не придумывает их URL и payload. До появления backend-схем для них можно создать только изолированные UI-компоненты и contract fixtures, не заявляя сквозную работу.

## 9. Вопросы к backend-команде до интеграции

1. Подтвердить URL, method, base URL и CORS.
2. Подтвердить: опциональные поля опускаются или передаются как `null`.
3. Подтвердить полные JSON Schema/OpenAPI для успеха, `422` и `503`; SDK для P0 не нужен.
4. Согласовать источник опций формы: meta endpoint или версионный общий файл.
5. Уточнить, будут ли `conditions_to_confirm`, какие поля диагностики стабильны и можно ли безопасно показывать `message` как user-facing текст.
