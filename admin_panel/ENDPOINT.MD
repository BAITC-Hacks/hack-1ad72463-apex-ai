# HTTP API админки

Production origin: `https://testrr.shop`. Prefix: `/admin/api`. Публичные `/api/*` описаны в [backend/ENDPOINT.MD](../backend/ENDPOINT.MD). Локальный origin по `.env.example`: `http://127.0.0.1:18081`; заголовок Origin должен соответствовать `ADMIN_ORIGIN` конкретного окружения.

## Авторизация и заголовки

Войти через POST login; сервер выдаёт cookie `hackalem_admin_session` и JSON с `username`, `csrf_token`, `must_change_password`, `expires_at`. Cookie действует 8 часов. После первичного входа разрешены только session/logout/password до смены временного пароля.

Все POST/PUT/DELETE требуют `Origin: https://testrr.shop`. Авторизованные операции также требуют `X-CSRF-Token` из session/login. JSON-запросы: `Content-Type: application/json`.

POST vendors, PUT/DELETE vendors/{id} и POST import требуют `If-Match: "<revision>"` из GET vendors или preview. Revision — хеш полного снимка, а не `metadata.snapshot_version`. После изменения ответ содержит новую revision и ETag. При конфликте перезагрузите каталог/preview; не повторяйте запись автоматически.

## Маршруты

| Метод | Путь | Назначение и тело |
|---|---|---|
| GET, HEAD | `/admin/` | Интерфейс входа и управления |
| GET, HEAD | `/admin/app.js`, `/admin/style.css` | Встроенные ресурсы |
| любой | `/admin` | Redirect 308 на `/admin/` |
| POST | `/admin/api/login` | `{ "username": "admin", "password": "…" }`; возвращает сессию |
| GET | `/admin/api/session` | Текущая сессия и CSRF-токен |
| POST | `/admin/api/logout` | Удаляет текущую сессию; `{ "ok": true }` |
| POST | `/admin/api/password` | `{ "current_password": "…", "new_password": "…" }`; отзывает все сессии |
| GET | `/admin/api/vendors` | `{ vendors: […], revision, metadata, event_formats, languages }`, ETag |
| POST | `/admin/api/vendors` | Создать один профиль (объект ниже) |
| PUT | `/admin/api/vendors/{id}` | Полностью заменить поля профиля; ID менять нельзя |
| DELETE | `/admin/api/vendors/{id}` | Удалить профиль и связанные факты |
| POST | `/admin/api/import/preview` | multipart: `file`; проверка без изменения БД, возвращает total/new_count/existing_count/existing_ids/sample/revision |
| POST | `/admin/api/import` | multipart: `file`, `update_existing=true` только для осознанного обновления совпавших ID |
| GET | `/admin/api/export` | CSV текущего каталога |
| GET | `/admin/api/template` | CSV с заголовками для заполнения |
| GET | `/admin/api/audit` | Последние 50 событий: id, actor, action, vendor_ids, created_at |
| GET | `/readyz` | `{ "status": "ready" }`; только напрямую к локальному сервису, nginx этот маршрут не публикует |

Все успешные API-операции возвращают 200. JSON-ответы имеют `Content-Type: application/json`; export/template возвращают CSV, а audit — JSON-массив. Административные ответы имеют `Cache-Control: no-store` и `X-Request-ID`. Изменения каталога возвращают `{ "ok": true, "revision": "…", "catalog_count": 67 }`. Создание и CSV с совпадающими ID без update_existing не меняют данные и возвращают 409. Импорт транзакционный, существующие строки целиком заменяются только при явном разрешении. Preview показывает первые 8 строк. Сам preview не требует `If-Match`, но требует сессию, Origin и CSRF. Он не сохраняет файл: финальный import снова получает файл и проверяет revision в транзакции. Сервер не связывает запросы идентификатором preview. Совпадение preview revision не отменяет проверку на момент записи.

## Профиль

```json
{
  "id": "HK-NEW-001",
  "anon_name": "Новый фотограф",
  "categories": ["Фотограф"],
  "city": "Алматы",
  "city_imputed": false,
  "synthetic": false,
  "price_from_kzt": 150000,
  "price_imputed": false,
  "event_formats": ["свадьба"],
  "languages": ["русский", "казахский"],
  "max_hours": 5,
  "busy_dates": [],
  "description": "Фотосъёмка мероприятий."
}
```

`max_hours` может быть null. Поля валидируются общим парсером backend; допустимые форматы/языки и календарное окно возвращает GET vendors. Для CSV используйте GET template/export: заголовки совпадают с ключами объекта, массивы записываются через `|`, boolean — `True`/`False`, пустой max_hours соответствует null. Максимум 5000 строк за импорт, multipart-запрос до 5 МиБ; JSON до 512 КиБ.

## Ошибки

Формат: `{ "error": { "code": "REVISION_CONFLICT", "message": "…" } }`. Ответ содержит X-Request-ID. Основные статусы: 401 — нет сессии/неверный пароль, 403 — Origin/CSRF/нужна смена пароля, 404 — нет записи/маршрута, 405 — метод, 409 — конфликт revision/ID/последняя запись, 413 — загрузка слишком велика или некорректный multipart, 415 — нужен JSON, 422 — невалидные данные, 428 — отсутствует If-Match, 429 — слишком много попыток, 503 — БД/каталог недоступны или транзакция не сохранена.


## Примеры ответов и последовательность вызовов

Вход и `GET /admin/api/session` возвращают объект одинаковой формы:

```json
{
  "username": "admin",
  "csrf_token": "<session-csrf-token>",
  "must_change_password": false,
  "expires_at": "2026-09-23T20:00:00Z"
}
```

Значения токена и срока в примере иллюстративные. При `must_change_password: true` сначала выполните смену пароля, затем новый login: прежняя cookie и CSRF больше не действуют. `password` и `logout` не требуют revision каталога.

Для создания/редактирования: login → смена пароля при необходимости → session → GET vendors → POST/PUT с Origin, cookie, CSRF и If-Match → сохранить новую revision из ответа. PUT заменяет профиль полностью, не является PATCH; `id` в теле обязан совпадать с ID пути. GET отдельного `/vendors/{id}` не предусмотрен — данные находятся в массиве GET vendors.

Для CSV: login → preview с multipart `file` → проверка результата → import с тем же выбранным файлом, If-Match из preview и осознанным `update_existing=true` при необходимости. Заголовок multipart с boundary формирует HTTP-клиент; тело не является JSON.

Пример preview:

```json
{
  "total": 2,
  "new_count": 1,
  "existing_count": 1,
  "existing_ids": ["HK-44733"],
  "sample": [
    {
      "id": "HK-NEW-001", "anon_name": "Новый фотограф",
      "categories": ["Фотограф"], "city": "Алматы",
      "city_imputed": false, "synthetic": true,
      "price_from_kzt": 150000, "price_imputed": false,
      "event_formats": ["свадьба"], "languages": ["русский"],
      "max_hours": 5, "busy_dates": [], "description": "Тестовый профиль."
    },
    {
      "id": "HK-44733", "anon_name": "Буллма",
      "categories": ["Ведущий"], "city": "Алматы",
      "city_imputed": false, "synthetic": false,
      "price_from_kzt": 1000000, "price_imputed": false,
      "event_formats": ["корпоратив"], "languages": ["русский", "английский"],
      "max_hours": 6, "busy_dates": [], "description": "Пример нового описания."
    }
  ],
  "revision": "sha256:<full-snapshot-hash>"
}
```

Пример иллюстрирует формат preview для тестового файла, а не текущие данные указанных профилей. При большем файле sample ограничен первыми 8 объектами. Preview не запускает импорт автоматически.

Успешная запись:

```json
{
  "ok": true,
  "revision": "sha256:<new-full-snapshot-hash>",
  "catalog_count": 67
}
```

Число профилей зависит от актуального каталога. При конфликте возвращается:

```json
{
  "error": {
    "code": "REVISION_CONFLICT",
    "message": "Каталог уже изменился. Обновите список и повторите действие."
  }
}
```

### Коды ошибок

| HTTP | Коды | Условия |
|---|---|---|
| 401 | `UNAUTHORIZED`, `INVALID_CREDENTIALS` | Нет действующей сессии или неверный логин/пароль |
| 403 | `ORIGIN_REJECTED`, `CSRF_REJECTED`, `PASSWORD_CHANGE_REQUIRED` | Нарушена защита запроса или не сменён временный пароль |
| 404 | `NOT_FOUND` | Неизвестный маршрут или отсутствующий профиль |
| 405 | `METHOD_NOT_ALLOWED` | Метод не поддерживается; возвращается Allow |
| 409 | `REVISION_CONFLICT`, `DUPLICATE_ID`, `LAST_VENDOR` | Версия изменилась, ID уже есть или удаляется последний профиль |
| 413 | `INVALID_UPLOAD` | Multipart не разобран, в том числе превышен лимит приложения |
| 415 | `UNSUPPORTED_MEDIA_TYPE` | Для JSON-маршрута не передан application/json |
| 422 | `INVALID_JSON`, `INVALID_VENDOR`, `INVALID_CSV`, `FILE_REQUIRED`, `TOO_MANY_RECORDS`, `INVALID_UPLOAD`, `PASSWORD_REJECTED` | Некорректные поля/файл/пароль; превышение лимита JSON также даёт INVALID_JSON |
| 428 | `REVISION_REQUIRED` | Нет If-Match для записи каталога |
| 429 | `RATE_LIMITED` | Лимит попыток; Retry-After: 900 |
| 500 | `INTERNAL_ERROR` | Непредвиденная ошибка обработчика |
| 503 | `DATABASE_UNAVAILABLE`, `CATALOG_UNAVAILABLE`, `SAVE_FAILED`, `NOT_READY` | Ошибка БД, каталога, сохранения или readiness |

Ответ 413, сформированный самим Nginx до приложения, может иметь HTML-тело; JSON-формат ошибок относится к ответам приложения. Для неавторизованного запроса проверка сессии/Origin может сработать раньше проверки маршрута или метода.

`GET /admin/api/audit` возвращает массив до 50 объектов `{id, actor, action, vendor_ids, created_at}`. Возможные действия: `create`, `update`, `delete`, `import`, `password_changed`. Revision до/после сохраняются в БД, но этим endpoint не возвращаются. История не содержит копии прежних полей и не предоставляет rollback API.
