# HTTP API HackAlem

Базовый адрес локального запуска: `http://localhost:8080`. Публичные эндпоинты этого документа не требуют авторизации. Защищённый API модерации находится в отдельном сервисе: [admin_panel/ENDPOINT.MD](../admin_panel/ENDPOINT.MD). Все ответы, кроме `/metrics` и успешного `OPTIONS`, имеют `Content-Type: application/json; charset=utf-8`. Каждый ответ содержит `X-Request-ID` и `Cache-Control: no-store`. Имена полей — `snake_case`.

## Все эндпоинты

| Метод | Путь | Назначение |
|---|---|---|
| `POST` | `/api/recommendations` | Подбор до трёх подрядчиков |
| `GET` | `/api/catalog/meta` | Справочники формы, категории по городам, версии и календарное окно |
| `GET` | `/healthz` | Liveness процесса без обращения к БД |
| `GET` | `/readyz` | Доступность PostgreSQL и целостность загруженного каталога |
| `GET` | `/metrics` | Счётчики HTTP-статусов и задержки p50/p95 |
| `OPTIONS` | Каждый из путей выше | CORS preflight для явно разрешённого origin |

Другой путь: `404 NOT_FOUND`. Неподдерживаемый метод: `405 METHOD_NOT_ALLOWED`, заголовок `Allow`. Пути точные, завершающий `/` не добавляется. CRUD и CSV-догрузка реализованы под `/admin/api/*` в отдельном сервисе и не входят в маршруты этого публичного API. CLI-импорт заменяет каталог полностью; админка сохраняет записи, отсутствующие в загружаемом файле. Бронирование не реализовано.

## POST /api/recommendations

Обязателен `Content-Type: application/json` (допускается `charset=utf-8`). Тело не более 65536 байт. Ожидается ровно один JSON-объект. Неизвестные и повторяющиеся ключи отклоняются.

| Поле | Тип | Обязательно | Проверка |
|---|---|---|---|
| `city` | string | Да | Непустое после нормализации |
| `date` | string | Да | Строгий `YYYY-MM-DD`, 2026-09-23…2026-12-31 включительно |
| `event_format` | string | Да | `свадьба`, `той`, `корпоратив`, `конференция`, `юбилей`, `день рождения` |
| `category` | string | Да | Непустая одна категория; список доступен в `/api/catalog/meta` |
| `budget_kzt` | integer | Да | От 1 до 9223372036854775807; JSON integer, без десятичной точки, экспоненты или кавычек |
| `duration_hours` | number или null | Нет | Конечное положительное число, время работы подрядчика |
| `language` | string или null | Нет | `русский`, `казахский`, `английский` |

`null` и отсутствие необязательного поля эквивалентны. Пустой язык не означает пропуск. Для обязательного поля `null` считается отсутствием. Город, категория, формат и язык сравниваются с одинаковой нормализацией Unicode/регистра/пробелов. Дата не исправляется автоматически. Неизвестные непустые город/категория допустимы и приводят к `NO_CATALOG`.

```sh
curl --fail-with-body http://localhost:8080/api/recommendations \
  -H 'Content-Type: application/json' \
  -d '{
    "city": "Алматы",
    "date": "2026-11-14",
    "event_format": "корпоратив",
    "category": "Ведущий",
    "budget_kzt": 1000000,
    "duration_hours": 6,
    "language": "русский"
  }'
```

### Успешный бизнес-ответ — HTTP 200

| Поле | Значение |
|---|---|
| `status` | `MATCHES_FOUND`, `NO_CATALOG` или `NO_MATCH` |
| `message` | Объяснение результата и количества на русском языке |
| `results` | Массив из 0–3 карточек, всегда массив, не `null` |
| `diagnostics` | Воронка, пропущенные проверки и правило сортировки |
| `metadata` | Версии исходных данных/правил/фактов и окно календаря |

**Карточка `results[]`:**

| Поле | Тип / смысл |
|---|---|
| `id` | string, исходный уникальный ID |
| `anon_name` | string, анонимизированное отображаемое имя |
| `city` | string, исходное отображаемое название города |
| `category` | string, выбранная категория в исходном написании |
| `price_from_kzt` | integer, стартовая цена за мероприятие, не за час |
| `max_hours` | number или null; null — ограничение неприменимо |
| `synthetic` | boolean, синтетический профиль, показать соответствующую метку |
| `price_imputed` | boolean, цена проставлена при подготовке датасета, показать как оценочную |
| `city_imputed` | boolean, город проставлен при подготовке датасета |
| `explanation` | string, два предложения с условиями запроса и особенностью профиля |
| `conditions_to_confirm` | Необязательный массив `{ "fact_id": string, "text": string }`; условия из описания для уточнения |

Все пользовательские строки следует выводить как текст, не через HTML-интерпретацию. Метки происхождения обязательны для корректного отображения. Начальная цена не является сметой, календарь не подтверждает бронирование, особенности из описания не гарантируют включение услуг в стоимость.

Для запроса выше на исходном каталоге из 66 профилей возвращаются следующие карточки. После модерации состав и количество могут измениться; сравнивайте metadata версий:

Полный ответ, полученный от запущенного сервера: [recommendation-response.json](examples/recommendation-response.json). Готовое тело запроса: [recommendation-request.json](examples/recommendation-request.json).

| Порядок | ID | Имя | Цена от, ₸ | max_hours |
|---|---|---|---:|---:|
| 1 | `HK-44923` | Мицури Канроджи | 650000 | 8 |
| 2 | `HK-29829` | Аня Форджер | 700000 | 6 |
| 3 | `HK-27222` | Сон Гоку | 1000000 | 10 |

Диагностика этого ответа:

```json
{
  "catalog_count": 10,
  "eligible_count": 4,
  "returned_count": 3,
  "excluded_counts": {
    "EVENT_FORMAT_UNSUPPORTED": 1,
    "BUDGET_TOO_LOW": 2,
    "LANGUAGE_UNSUPPORTED": 0,
    "DURATION_EXCEEDED": 0,
    "BUSY_ON_DATE": 3
  },
  "omitted_optional_checks": [],
  "applied_order": ["price_from_kzt:asc", "id:asc"]
}
```

`catalog_count` — число профилей только в запрошенном городе и категории. `eligible_count` — число подходящих до top-3. Четвёртый подходящий профиль не считается исключённым. `returned_count = min(eligible_count, 3) = len(results)`.

`excluded_counts` всегда содержит все пять кодов. Порядок проверок: формат, бюджет, язык, длительность, занятость. Если кандидат одновременно дорогой и занят, публичная воронка учитывает его только в бюджете. Поэтому `sum(excluded_counts) + eligible_count = catalog_count`. `omitted_optional_checks` содержит `language` и/или `duration_hours`, если они не заданы, иначе `[]`.

**Метаданные:**

```json
{
  "snapshot_version": "sha256:<64 hex characters>",
  "loader_version": "loader-v1",
  "policy_version": "eligibility-price-v1",
  "facts_version": "sha256:<64 hex characters>",
  "calendar_window": {"from": "2026-09-23", "to": "2026-12-31"}
}
```

Это схема версий, не буквальные хеши. После CLI-импорта `snapshot_version` — SHA-256 исходных байтов CSV, а `facts_version` — SHA-256 файла фактов либо `structured-only-v1`. После записи через админку версии вычисляются из сформированного CSV текущих профилей и JSON оставшихся фактов. Административная `revision` хеширует полный снимок и отличается от этих полей. Для одной версии и нормализованного запроса состав и порядок повторяемы. Реальные значения доступны в каждом бизнес-ответе и `/api/catalog/meta`.

### Пустые и неполные результаты

| Изменение контрольного запроса | status / результат |
|---|---|
| `date = "2026-12-19"` | `MATCHES_FOUND`, одна карточка `HK-35215`, Кики, от 900000 ₸; сообщение объясняет исключения |
| `category = "Флорист"`, `event_format = "свадьба"`, `budget_kzt = 300000` | `MATCHES_FOUND`, одна карточка `HK-90001`, Тихиро Огино; `synthetic: true`, `max_hours: null` |
| `budget_kzt = 100000` | `NO_MATCH`, `results: []`, `catalog_count: 10`, `eligible_count: 0` |
| `city = "Астана"`, `category = "Декоратор"` | `NO_CATALOG`, `results: []`, все счётчики нулевые |

Все эти исходы имеют **HTTP 200**. `NO_MATCH` объясняет число исключённых по конкретным условиям. `NO_CATALOG` предлагает изменить город/категорию. Если исходный пул сам меньше трёх и исключений нет, сообщение указывает размер каталога. Условия запроса никогда не ослабляются автоматически.

### Ошибки

Общая форма — отдельный объект `error`, без бизнес-поля `status`:

```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "details": [
      {
        "field": "date",
        "code": "DATE_OUT_OF_RANGE",
        "message": "Выберите дату с 2026-09-23 по 2026-12-31 включительно."
      }
    ]
  }
}
```

| HTTP | `error.code` | Когда |
|---|---|---|
| 400 | `INVALID_JSON` | Повреждённый JSON, не объект, пустое тело, второй JSON после первого |
| 403 | `ORIGIN_NOT_ALLOWED` | Preflight с неразрешённым или отсутствующим Origin |
| 404 | `NOT_FOUND` | Неизвестный путь |
| 405 | `METHOD_NOT_ALLOWED` | Неверный HTTP-метод |
| 413 | `PAYLOAD_TOO_LARGE` | Тело больше 64 KiB |
| 415 | `UNSUPPORTED_MEDIA_TYPE` | Отсутствует или неверен Content-Type |
| 422 | `VALIDATION_ERROR` | Ошибки полей |
| 500 | `INTERNAL_ERROR` | Непредвиденная внутренняя ошибка |
| 503 | `CATALOG_UNAVAILABLE` | БД недоступна, схема/каталог не загружены, нарушена целостность или истёк срок чтения |

Коды `details[]` для 422: `REQUIRED`, `INVALID_TYPE`, `INVALID_DATE`, `DATE_OUT_OF_RANGE`, `INVALID_ENUM`, `OUT_OF_RANGE`, `UNKNOWN_FIELD`. Повторяющийся ключ даёт `INVALID_TYPE`. При ошибках разбора полей проверка бизнес-значений не запускается; UI может получить следующий набор ошибок после исправления типов. У технических ошибок `details: []`, без внутренних путей и SQL.

## GET /api/catalog/meta

Параметры не требуются. HTTP 200:

```json
{
  "catalog_count": 66,
  "cities": ["Алматы", "Астана", "Зарубежье"],
  "categories": ["Банкетный зал", "Ведущий", "Ведущий церемонии", "Видеограф", "Декоратор", "Загородная площадка", "Инструменталист", "Лайв-бэнд", "Национальный ансамбль", "Отель", "Подарки и сувениры", "Ресторан", "Танцевальный коллектив", "Флорист", "Фото и видеобудки", "Фотограф", "Шоу-программа"],
  "categories_by_city": {},
  "event_formats": ["день рождения", "конференция", "корпоратив", "свадьба", "той", "юбилей"],
  "languages": ["английский", "казахский", "русский"],
  "metadata": {}
}
```

В примере сокращены только `categories_by_city` и `metadata`: в реальном ответе первое поле — объект, ключи которого города, а значения — отсортированные массивы имеющихся в городе категорий; второе содержит полные метаданные, описанные выше. «Зарубежье» — группа исходного каталога, а не конкретный город. При недоступном каталоге — `503 CATALOG_UNAVAILABLE`.

## GET /healthz

HTTP 200: `{"status":"ok"}`. Проверяет, что HTTP-процесс работает. Не подтверждает наличие БД или каталога.

## GET /readyz

HTTP 200: `{"status":"ready","catalog_count":66,"metadata":{...}}`, где metadata полная. Читает и проверяет согласованный снимок PostgreSQL. Если проверка не прошла — HTTP 503, `{"error":{"code":"CATALOG_UNAVAILABLE","details":[]}}`. Данные можно изменить через админку или отдельную CLI без перезапуска HTTP-процесса. Значение 66 в примере относится к исходному каталогу; фактическое число берётся из БД.

## GET /metrics

HTTP 200, `Content-Type: text/plain; version=0.0.4; charset=utf-8`:

```text
# TYPE hackalem_http_requests_total counter
hackalem_http_requests_total{status="200"} 12
hackalem_http_requests_total{status="422"} 2
# HELP hackalem_http_duration_seconds Rolling quantiles of the last 1024 completed HTTP requests.
# TYPE hackalem_http_duration_seconds gauge
hackalem_http_duration_seconds{quantile="0.5"} 0.004
hackalem_http_duration_seconds{quantile="0.95"} 0.009
```

Числа иллюстративные. Счётчики создаются для наблюдавшихся статусов; p50/p95 вычисляются по последним 1024 завершённым HTTP-запросам, включая служебные. Текущий запрос `/metrics` учитывается после формирования ответа. До первого запроса задержки равны нулю. Метрики не сохраняются между рестартами.

## OPTIONS и CORS

Отправьте `Origin` из `CORS_ORIGINS` на один из зарегистрированных путей. Ответ: **204**, без тела, `Access-Control-Allow-Origin` с точным origin, `Access-Control-Allow-Methods` с методом соответствующего пути и `OPTIONS`, `Access-Control-Allow-Headers: Content-Type`.

Обычные запросы с разрешённым Origin получают `Access-Control-Allow-Origin` и `Access-Control-Expose-Headers: X-Request-ID`. CORS не является авторизацией; этот MVP предоставляет публичный API чтения и подбора.

## POST /api/recommendations/alternatives

Принимает тот же JSON, Content-Type и правила валидации, что `/api/recommendations`.
Исходная рекомендация и все кандидаты вычисляются существующим matching engine
на одном snapshot, загруженном один раз за HTTP-запрос. Схема старого endpoint не меняется.

```json
{
  "alternatives": [
    {"type":"BUDGET","current_budget_kzt":100000,"suggested_budget_kzt":650000,"eligible_count":1}
  ]
}
```

Максимум две независимые альтернативы, всегда BUDGET перед DATE:

- BUDGET меняет только `budget_kzt`: минимальная цена из каталога выше текущего
  бюджета, при которой весь исходный запрос получает `MATCHES_FOUND`.
- DATE меняет только `date`: ближайшая рабочая дата в `metadata.calendar_window`.
  При равном расстоянии предпочтительна более поздняя дата. Поля элемента:
  `type`, `current_date`, `suggested_date`, `distance_days` (> 0), `eligible_count`.

`eligible_count` берётся из фактической рекомендации для конкретной альтернативы,
а не из числа показанных карточек. Изменения бюджета и даты не объединяются.
Если исходный статус не `NO_MATCH` или ни одно одиночное изменение не помогает,
возвращается HTTP 200 и `{"alternatives":[]}`. Другие поля запроса не ослабляются.

Ошибки используют общий формат `error`: 400 `INVALID_JSON`, 422 `VALIDATION_ERROR`,
503 `CATALOG_UNAVAILABLE`; ограничения размера, Content-Type, методов и CORS
совпадают с обычным endpoint. Same-origin `https://testrr.shop` не требует
добавления отдельного CORS-разрешения.

## Язык ответов: Accept-Language

`Accept-Language: ru|kk|en` выбирает язык системных `message`, `explanation`
и сообщений validation errors. Поддерживаются региональные варианты (`ru-RU`,
`kk-KZ`, `en-US`, `en-GB`). Используется первый поддерживаемый диапазон в порядке
заголовка; диапазоны с `q=0` или некорректным q пропускаются. Без поддерживаемого
языка используется русский. Ответ содержит `Content-Language: ru|kk|en` и
`Vary: Accept-Language`, сохраняя также существующий `Vary: Origin`.

JSON запроса и ответа не меняется. `request.language` по-прежнему означает язык
работы подрядчика и принимает canonical значения `русский`, `казахский`,
`английский`; UI locale через JSON не передаётся. Locale не влияет на фильтрацию,
ранжирование, ID, diagnostics или metadata. Старые Go `Validate` / `Recommend`
сохраняют RU; версии `ValidateLocalized` / `RecommendLocalized` принимают locale.

Исходные `Fact.Text` в EN/KK не переводятся и помечаются как
`Original profile note:` / `Профильдегі бастапқы мәтін:`. Тексты
`conditions_to_confirm` сохраняются без изменений. Имена городов и категории
остаются canonical; известные форматы и языки в explanation переводятся.

`/api/recommendations/alternatives` использует те же заголовки и локализованные
ошибки, при этом структурный JSON alternatives одинаков для всех locales.
