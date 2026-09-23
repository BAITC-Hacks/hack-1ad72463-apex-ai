# HTTP API админки

Production origin: `https://testrr.shop`. Prefix: `/admin/api`. Публичные `/api/*` описаны в `../backend/ENDPOINT.MD`.

## Авторизация и заголовки

Войти через POST login; сервер выдаёт cookie `hackalem_admin_session` и JSON с `username`, `csrf_token`, `must_change_password`, `expires_at`. Cookie действует 8 часов. После первичного входа разрешены только session/logout/password до смены временного пароля.

Все POST/PUT/DELETE требуют `Origin: https://testrr.shop`. Авторизованные операции также требуют `X-CSRF-Token` из session/login. JSON-запросы: `Content-Type: application/json`.

POST vendors, PUT/DELETE vendors/{id} и POST import требуют `If-Match: "<revision>"` из GET vendors или preview. После изменения ответ содержит новую revision и ETag. При конфликте перезагрузите каталог/preview; не повторяйте запись автоматически.

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

Все успешные API-операции возвращают 200. Изменения каталога возвращают `{ "ok": true, "revision": "…", "catalog_count": 67 }`. Создание и CSV с совпадающими ID без update_existing не меняют данные и возвращают 409. Импорт транзакционный, существующие строки целиком заменяются только при явном разрешении. Preview показывает первые 8 строк.

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
