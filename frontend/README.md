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

Production frontend и Go API работают на одном origin `https://testrr.shop`. Файл `.env.production` задаёт:

```dotenv
VITE_API_MODE=http
VITE_API_BASE_URL=/api
```

Production-запрос уходит на `POST /api/recommendations`, то есть `https://testrr.shop/api/recommendations`. Между frontend и API в этой схеме нет cross-origin запроса, поэтому CORS для production-вызова не требуется.

Для локального frontend против production API используйте:

```dotenv
VITE_API_MODE=http
VITE_API_BASE_URL=https://testrr.shop/api
```

Backend из текущего `main` также предоставляет `GET /api/catalog/meta`. P0 использует проверенный fallback-справочник формы, поэтому интерфейс остаётся демонстрируемым без backend; справочники совпадают с текущим контрактом. Подключение meta endpoint можно добавить отдельно, не меняя компонент формы.

На 23 сентября 2026 года боевой API `https://testrr.shop/api` доступен: контрольный `POST /api/recommendations` и CORS для `http://localhost:5173` проверены. Это состояние внешней среды, а не гарантия постоянной доступности.

## Ожидаемый backend-контракт

- `POST /api/recommendations`
- бизнес-статусы: `MATCHES_FOUND`, `NO_CATALOG`, `NO_MATCH`
- ошибки: `422 VALIDATION_ERROR`, `503 CATALOG_UNAVAILABLE`
- `duration_hours` и `language` опускаются, если пользователь выбрал «не важно»
- дата передаётся как исходная строка `YYYY-MM-DD`, без `Date` и UTC-конвертации

Backend разрешает CORS для `http://localhost:5173`, что необходимо локальному frontend при обращении к production API. Для другого локального origin его нужно добавить в backend-переменную `CORS_ORIGINS`.

## Ограничения

- Mock — это набор контрактных фикстур, а не реализация поиска.
- Реальная интеграция требует запущенных PostgreSQL, импорта каталога и Go API.
- Стартовая цена не является итоговой сметой; доступность требует подтверждения.
- Админ-панель реализована отдельным Go-приложением в [admin_panel](../admin_panel/README.md), а не частью React-сборки.


## Связь с админ-панелью

Публичная страница находится на `/`, панель модерации — на [/admin/](https://testrr.shop/admin/). Nginx должен направлять `/admin/` и `/admin/api/*` на административный Go-сервис, сохраняя путь, а `/api/*` — на публичный backend. SPA fallback React не должен перехватывать административные маршруты.

После сохранения в панели следующий HTTP-поиск использует изменённый каталог. Уже отображённые результаты не обновляются автоматически: повторите поиск. Mock работает на фикстурах и не отражает изменения PostgreSQL. API-метаданные и ID карточек после модерации могут отличаться от исходных демопримеров.

Форма пока использует встроенные варианты города/категории и календарное окно. Новый город или категория из админки появляется в `/api/catalog/meta`, но не становится автоматически доступным в форме. Для такого изменения требуется обновить справочник frontend; полноценная загрузка meta остаётся отдельной задачей.

Авторизация, CSRF и загрузка CSV принадлежат отдельному сервису админки. Публичный gateway не должен отправлять административные токены или выполнять модерацию. Инструкции: [руководство](../admin_panel/README.md), [контракт API](../admin_panel/ENDPOINT.MD).
