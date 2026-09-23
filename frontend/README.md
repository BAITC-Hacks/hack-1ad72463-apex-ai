# Apex Match frontend

Публичный одностраничный интерфейс подбора event-подрядчиков. Frontend отображает решение Go backend и не фильтрует, не ранжирует и не пересортировывает подрядчиков самостоятельно.

## Stack

- React + TypeScript + Vite
- Zod для runtime-проверки ответа API
- native `fetch`
- обычный CSS
- Vitest + Testing Library для минимальных component tests

## Запуск

Требуется актуальная LTS-версия Node.js с npm.

```bash
cd frontend
npm install
npm run dev
```

Production-сборка и тесты:

```bash
npm run build
npm test -- --run
```

## Режим API

Скопируйте `.env.example` в локальный `.env` и выберите режим:

```dotenv
VITE_API_MODE=mock
VITE_API_BASE_URL=https://testrr.shop/api
```

- `mock` — автономное демо на заранее подготовленных контрактных фикстурах. В интерфейсе всегда видна метка `DEMO / MOCK API`. Быстрые сценарии только заполняют форму; ответ выбирает mock adapter по точному запросу.
- `http` — `HttpRecommendationGateway` отправляет запрос в endpoint рекомендаций. Base URL может быть корнем (`http://localhost:8080`) или API-префиксом (`https://testrr.shop/api`); в обоих случаях получится корректный `/api/recommendations`. Ответ проверяется Zod-схемой и дополнительными инвариантами до отображения.

Backend из текущего `main` также предоставляет `GET /api/catalog/meta`. P0 использует проверенный fallback-справочник формы, поэтому интерфейс остаётся демонстрируемым без backend; справочники совпадают с текущим контрактом. Подключение meta endpoint можно добавить отдельно, не меняя компонент формы.

На 23 сентября 2026 года боевой API `https://testrr.shop/api` доступен: контрольный `POST /api/recommendations` и CORS для `http://localhost:5173` проверены. Это состояние внешней среды, а не гарантия постоянной доступности.

## Ожидаемый backend-контракт

- `POST /api/recommendations`
- бизнес-статусы: `MATCHES_FOUND`, `NO_CATALOG`, `NO_MATCH`
- ошибки: `422 VALIDATION_ERROR`, `503 CATALOG_UNAVAILABLE`
- `duration_hours` и `language` опускаются, если пользователь выбрал «не важно»
- дата передаётся как исходная строка `YYYY-MM-DD`, без `Date` и UTC-конвертации

Локальный и текущий боевой Go backend разрешают CORS для `http://localhost:5173`. Для другого домена frontend его origin нужно добавить в backend-переменную `CORS_ORIGINS`.

## Ограничения

- Mock — это набор контрактных фикстур, а не реализация поиска.
- Реальная интеграция требует запущенных PostgreSQL, импорта каталога и Go API.
- Стартовая цена не является итоговой сметой; доступность требует подтверждения.
- Admin panel запланирована на отдельный этап и здесь не реализована.
