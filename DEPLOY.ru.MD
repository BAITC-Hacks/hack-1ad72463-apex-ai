# Развёртывание и эксплуатация

[English](DEPLOY.MD) · [Русский](DEPLOY.ru.MD)

## 1. Область применения и текущая схема

Руководство охватывает публичный React frontend, Go API подбора, отдельную Go-админку, PostgreSQL, Nginx и TLS. Сверено с конфигурацией сервера **23 сентября 2026 года**. Команды предназначены для выполнения оператором; при написании документа production не переустанавливался и не перезапускался.

| Компонент | Размещение в production |
|---|---|
| Сервер | `2.29.54.10`, Ubuntu 26.04.1 LTS, x86_64 |
| Сайт | `https://testrr.shop/` → `/opt/hackalem/frontend/current` |
| Публичный API | `/api/*` → `127.0.0.1:18080`, `hackalem-backend.service` |
| Админка | `/admin/*` → `127.0.0.1:18081`, `hackalem-admin.service` |
| База | PostgreSQL `18/main`, база/роль `hackalem`, локальный Unix socket |
| TLS | `/etc/letsencrypt/live/testrr.shop/`, `certbot.timer` |

Оба Go-процесса работают от пользователя ОС `hackalem` с общей БД. Изменения админки сразу доступны API. В исходном наборе 66 профилей; рабочее количество может меняться. Исходный календарь: 2026-09-23…2026-12-31. Доступ к LLM во время работы не требуется.

Для существующей установки используйте разделы 2–4, 11–14. Разделы 5–10 нужны при первом запуске или осознанном изменении инфраструктуры.

## 2. Требования и правила выполнения команд

На машине сборки нужны доступ к Git, Go 1.25+, Node.js 24+ с npm, Python 3 для дополнительных проверок, SSH/SCP и tar. Используйте чистый checkout и один проверенный коммит для всех компонентов; `admin_panel/go.mod` требует соседнюю папку `backend`. Для Go race-тестов нужен нативный toolchain с поддержкой CGO.

На сервере нужны root/sudo, PostgreSQL 18, Nginx, curl и действующий TLS-сертификат. При загрузке готовых артефактов Go, Node и Docker на сервере не нужны. DNS A-записи домена должны указывать на сервер; AAAA публикуйте только при настроенном IPv6. В firewall сервера и провайдера разрешите SSH, HTTP и HTTPS; порты 5432, 18080 и 18081 оставьте закрытыми снаружи.

Если не указано иное, локальные команды выполняются из корня репозитория, серверные — в Bash от root. Замените `RELEASE` уникальным идентификатором и используйте одно значение локально и на сервере. Не выполняйте все блоки подряд: первая установка, обновление и восстановление — разные сценарии. Пароли SSH/админки, cookies, `.env` и дампы в Git не добавляются.

## 3. Локальная проверка и сборка

Сначала создайте отдельную PostgreSQL БД для тестов. Укажите её строку подключения в `TEST_DATABASE_URL`; тестовая роль должна уметь создавать схемы. Интеграционные тесты используют временные схемы. Без этой переменной они пропускаются, поэтому успешные unit-тесты не подтверждают совместимость с БД. Подробности: [тесты backend](backend/TESTING.md) и [тесты админки](admin_panel/TESTING.md).

Выполняйте в Bash. Команды собирают Linux amd64 даже на macOS. Для другой архитектуры измените GOARCH после проверки `uname -m`. HTML/CSS/JS админки встроены в Go-бинарник: каждое изменение интерфейса требует пересборки. `VITE_*` публичны и встраиваются при сборке; явные production-значения ниже перекрывают локальные настройки mock.

```bash
set -euo pipefail
: "${TEST_DATABASE_URL:?Set a dedicated test database URL}"
export TEST_DATABASE_URL
make -C backend test check
make -C admin_panel test check
(cd frontend && npm ci && npm run test:run)

RELEASE=20260923-example
COMMIT=$(git rev-parse HEAD)
BUILD="$PWD/admin_panel/.tmp/deploy-$RELEASE"
mkdir -p "$BUILD/backend" "$BUILD/admin_panel" "$BUILD/frontend"
(cd backend && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -o "$BUILD/backend/server" ./cmd/server)
(cd backend && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -o "$BUILD/backend/catalog" ./cmd/catalog)
(cd admin_panel && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -o "$BUILD/admin_panel/admin" ./cmd/admin)
cp -R backend/data "$BUILD/backend/data"
cp -R admin_panel/deploy "$BUILD/admin_panel/deploy"
(cd frontend && VITE_API_MODE=http VITE_API_BASE_URL=/api npm run build)
cp -R frontend/dist/. "$BUILD/frontend/"
printf '%s\n' "$COMMIT" > "$BUILD/COMMIT"
(cd "$BUILD" && find . -type f ! -name SHA256SUMS -exec shasum -a 256 {} + > SHA256SUMS)
COPYFILE_DISABLE=1 tar -czf "$BUILD.tar.gz" -C "$BUILD" .
scp "$BUILD.tar.gz" root@2.29.54.10:/var/tmp/
```

## 4. Подготовка релиза на сервере

Выполняйте после загрузки архива. Этот этап не меняет запущенные сервисы. Каталоги нового релиза не должны существовать. SHA256SUMS проверяет файлы по манифесту сборки; SSH обеспечивает аутентифицированную передачу. Артефакты принадлежат root и доступны сервисам для чтения. Данные БД хранятся вне каталогов релизов.

```bash
set -euo pipefail
umask 022
RELEASE=20260923-example
STAGE="/opt/hackalem/staging/$RELEASE"
install -d -m 755 /opt/hackalem/staging
mkdir "$STAGE"
tar -xzf "/var/tmp/deploy-$RELEASE.tar.gz" -C "$STAGE"
(cd "$STAGE" && sha256sum -c SHA256SUMS)
for component in backend admin_panel frontend; do
    install -d -m 755 "/opt/hackalem/$component/releases"
    mkdir "/opt/hackalem/$component/releases/$RELEASE"
    cp -R "$STAGE/$component/." "/opt/hackalem/$component/releases/$RELEASE/"
    cp "$STAGE/COMMIT" "/opt/hackalem/$component/releases/$RELEASE/COMMIT"
done
chmod 755 "/opt/hackalem/backend/releases/$RELEASE/server" "/opt/hackalem/backend/releases/$RELEASE/catalog" "/opt/hackalem/admin_panel/releases/$RELEASE/admin"
chown -R root:root "$STAGE" "/opt/hackalem/backend/releases/$RELEASE" "/opt/hackalem/admin_panel/releases/$RELEASE" "/opt/hackalem/frontend/releases/$RELEASE"
```

## 5. Первая установка: PostgreSQL и пользователь ОС

На текущем сервере этот этап пропустите. На новом Ubuntu установите `postgresql`, `postgresql-client`, `nginx` и `curl` из настроенных репозиториев ОС. Перед продолжением проверьте версию и кластер; обновление существующего кластера не является частью деплоя приложения.

Пользователь ОС, роль и база создаются один раз. Примеры предполагают, что их ещё нет. Настройте `listen_addresses = 'localhost'` и правило `local hackalem hackalem peer` в pg_hba.conf перед конфликтующими локальными правилами. Пути конфигураций определяются через `SHOW config_file;` и `SHOW hba_file;`. Перезапуск PostgreSQL нужен при изменении listen address; для HBA достаточно reload. Не заменяйте аутентификацию на `trust` ради обхода ошибки подключения.

```bash
useradd --system --user-group --home-dir /opt/hackalem --no-create-home --shell /usr/sbin/nologin hackalem
systemctl enable --now postgresql
runuser -u postgres -- createuser --no-superuser --no-createdb --no-createrole hackalem
runuser -u postgres -- createdb --owner=hackalem hackalem
runuser -u hackalem -- psql -h /var/run/postgresql -d hackalem -c 'SELECT current_user, current_database();'
pg_lsclusters
ss -lnt
```

## 6. Первая установка: окружение

На действующем сервере сначала прочитайте конфигурацию и внесите нужные изменения, не перезаписывая её целиком. EnvironmentFile systemd — не shell-скрипт: ниже DATABASE_URL backend содержит неэкранированный `&`, поэтому этот файл нельзя подключать через Bash `source`. В примерах CLI DATABASE_URL передаётся явно через `env`.

`ADMIN_ORIGIN` — точный origin браузера без `/admin/` и завершающего слеша. Внутренний HTTP за Nginx не отменяет Secure cookies: `ADMIN_INSECURE_HTTP=false`. Доверять proxy-заголовкам можно только при loopback-привязке приложения и перезаписи X-Real-IP в Nginx. Относительные пути каталога считаются от WorkingDirectory backend unit.

```bash
install -d -o root -g hackalem -m 750 /etc/hackalem
cat > /etc/hackalem/backend.env <<'ENV'
DATABASE_URL=postgresql:///hackalem?host=/var/run/postgresql&user=hackalem
HTTP_ADDR=127.0.0.1:18080
CORS_ORIGINS=https://testrr.shop,http://localhost:3000,http://localhost:5173
CATALOG_CSV=data/catalog.csv
CATALOG_META=data/catalog.meta.json
CATALOG_FACTS=data/facts.json
ENV
install -o root -g hackalem -m 640 "$STAGE/admin_panel/deploy/admin.env.example" /etc/hackalem/admin.env
chown root:hackalem /etc/hackalem/backend.env
chmod 640 /etc/hackalem/backend.env
```

## 7. Первая установка: каталог и администратор

Только для пустой БД при первом запуске: `catalog bootstrap` создаёт схему и заменяет весь каталог. Это не команда обновления. Исходные файлы проверяются вместе с метаданными и фактами. `admin init` создаёт таблицы и пользователя `admin`, один раз печатает случайный временный пароль и не меняет существующий. Сохраните пароль в менеджере паролей; общего фиксированного пароля в инструкции нет. При первом входе обязательна смена (от 12 символов, до 72 байт); после смены все сессии отзываются.

`ADMIN_INITIAL_PASSWORD` можно задать только для начальной настройки. Не оставляйте её в окружении сервиса. Если каталог уже существует, bootstrap полностью пропустите и при необходимости выполните только init.

```bash
for component in backend admin_panel frontend; do
    ln -s "/opt/hackalem/$component/releases/$RELEASE" "/opt/hackalem/$component/current"
done
(cd /opt/hackalem/backend/current && runuser -u hackalem -- ./catalog check)
(cd /opt/hackalem/backend/current && runuser -u hackalem -- env   'DATABASE_URL=postgresql:///hackalem?host=/var/run/postgresql&user=hackalem' ./catalog bootstrap)
runuser -u hackalem -- env   'DATABASE_URL=postgresql:///hackalem?host=/var/run/postgresql&user=hackalem'   /opt/hackalem/admin_panel/current/admin init
```

## 8. Первая установка: systemd

Установите приведённый backend unit и версионируемый admin unit из артефакта. Окружение, рабочие каталоги и ExecStart должны соответствовать друг другу. Admin unit содержит аналогичные ограничения сервиса. Миграции выполняются оператором явно, а не автоматически через ExecStart.

```bash
cat > /etc/systemd/system/hackalem-backend.service <<'UNIT'
[Unit]
Description=HackAlem contractor recommendation API
After=network.target postgresql.service
Wants=postgresql.service
StartLimitIntervalSec=60
StartLimitBurst=10

[Service]
Type=simple
User=hackalem
Group=hackalem
WorkingDirectory=/opt/hackalem/backend/current
EnvironmentFile=/etc/hackalem/backend.env
ExecStart=/opt/hackalem/backend/current/server
Restart=on-failure
RestartSec=3
TimeoutStopSec=15
UMask=0027
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ProtectKernelTunables=true
ProtectKernelModules=true
ProtectControlGroups=true
RestrictSUIDSGID=true
RestrictAddressFamilies=AF_UNIX AF_INET AF_INET6
CapabilityBoundingSet=
MemoryMax=512M
TasksMax=128
StandardOutput=journal
StandardError=journal
SyslogIdentifier=hackalem-backend

[Install]
WantedBy=multi-user.target
UNIT
install -m 644 "$STAGE/admin_panel/deploy/hackalem-admin.service" /etc/systemd/system/hackalem-admin.service
systemctl daemon-reload
systemctl enable --now hackalem-backend hackalem-admin
systemctl is-active hackalem-backend hackalem-admin
curl --retry 5 --retry-connrefused --retry-delay 1 --fail http://127.0.0.1:18080/readyz
curl --retry 5 --retry-connrefused --retry-delay 1 --fail http://127.0.0.1:18081/readyz
```

## 9. Маршруты Nginx

На рабочем сайте Nginx и TLS уже настроены: сохраните server blocks, параметры сертификата, frontend root и текущие API-маршруты. Перед редактированием скопируйте конфигурацию сайта. Admin snippet устанавливается один раз из артефакта. Ниже показано содержимое **внутри HTTPS server block основного домена**, а не полная конфигурация Nginx. Если такие locations уже есть в подключённых snippets, не дублируйте их.

`proxy_pass` без URI сохраняет `/api/` и `/admin/`. Точные aliases `/api` и `/api/health` намеренно меняют URI. `location ^~ /admin/` в готовом snippet не даёт SPA fallback перехватить административное API. Nginx разрешает 6 МиБ для загрузок, приложение ограничивает весь multipart-запрос 5 МиБ. Порты 18080/18081 должны слушать только loopback.

```nginx
root /opt/hackalem/frontend/current;
index index.html;

location = /api {
    proxy_pass http://127.0.0.1:18080/api/catalog/meta;
    proxy_set_header Host $host;
}
location = /api/health {
    proxy_pass http://127.0.0.1:18080/readyz;
    proxy_set_header Host $host;
}
location /api/ {
    proxy_pass http://127.0.0.1:18080;
    proxy_set_header Host $host;
    proxy_connect_timeout 3s;
    proxy_read_timeout 25s;
    client_max_body_size 1m;
}
include /etc/nginx/snippets/hackalem-admin.conf;
location / { try_files $uri $uri/ /index.html; }
location = /index.html { add_header Cache-Control "no-cache"; }
location /assets/ {
    try_files $uri =404;
    add_header Cache-Control "public, max-age=31536000, immutable";
}
location ~ /\. { deny all; }
```

## Применение проверенного изменения Nginx

На новой установке создайте site file и ссылку в sites-enabled до проверки; на текущем сервере редактируйте `/etc/nginx/sites-available/testrr.shop`. Команды ниже устанавливают только admin snippet и проверяют/перезагружают Nginx; отдельно отредактируйте site перед последними двумя командами. При ошибке проверки reload не выполняйте.

```bash
install -d -m 700 /opt/hackalem/config-backups
cp -a /etc/nginx/sites-available/testrr.shop "/opt/hackalem/config-backups/testrr.shop.$(date -u +%Y%m%dT%H%M%SZ)"
install -m 644 "$STAGE/admin_panel/deploy/hackalem-admin.conf" /etc/nginx/snippets/hackalem-admin.conf
nginx -t
systemctl reload nginx
```

## 10. TLS и другой домен

На текущем сервере сертификат уже установлен, `certbot.timer` включён в расписание. Проверка: `certbot certificates` и `systemctl status certbot.timer`. При обычном обновлении приложения новый сертификат не нужен.

Для нового сервера/домена: настройте DNS и сайт на порту 80, который отдаёт `/.well-known/acme-challenge/` из `/var/www/letsencrypt`; установите Certbot, затем выполните `certbot certonly --webroot -w /var/www/letsencrypt -d YOUR_DOMAIN` с реальным доменом и контактом оператора. После выдачи добавьте HTTPS listener и пути из `certbot certificates`. Включите HTTP→HTTPS redirect, сохранив доступность challenge. Если используется www, включите его DNS-имя в сертификат и перенаправляйте на основной домен. Настройте deploy hook успешного продления: `nginx -t && systemctl reload nginx`; проверьте `certbot renew --dry-run` и timer. Это этапы настройки нового сервера, они не выполнялись при написании инструкции.

При смене домена обновите Nginx server_name/TLS, ADMIN_ORIGIN и backend CORS_ORIGINS. Frontend `/api` остаётся на том же origin. После изменения env перезапустите соответствующие Go-сервисы. Cookies админки требуют HTTPS; insecure-режим для публичного сервера не включайте.

## 11. Резервная копия перед обновлением

Выполняйте от root перед операциями со схемой/каталогом и активацией релиза. Dump custom format включает каталог, факты, метаданные, пользователей, сессии и аудит. `pg_restore --list` проверяет оглавление архива, но не возможность восстановления: отдельно проведите пробное восстановление (раздел 15). pg_dump создаёт согласованный снимок без остановки сервисов; более поздние записи в него не попадут.

Храните зашифрованную/внешнюю копию согласно политике хранения. Команды создают только ручную копию; автоматическое расписание бэкапов приложения этой инструкцией не устанавливается. Дампы и копии конфигурации должны быть доступны только root.

```bash
umask 077
BACKUP_DIR="/opt/hackalem/backups/$(date -u +%Y%m%dT%H%M%SZ)"
mkdir -p "$BACKUP_DIR"
runuser -u postgres -- pg_dump -Fc hackalem > "$BACKUP_DIR/hackalem.dump"
pg_restore --list "$BACKUP_DIR/hackalem.dump" > "$BACKUP_DIR/archive-list.txt"
cp -a /etc/hackalem "$BACKUP_DIR/"
cp -a /etc/nginx/sites-available/testrr.shop "$BACKUP_DIR/"
for component in backend admin_panel frontend; do
    readlink -f "/opt/hackalem/$component/current" > "$BACKUP_DIR/$component.release"
done
sha256sum "$BACKUP_DIR/hackalem.dump" > "$BACKUP_DIR/hackalem.dump.sha256"
```

## 12. Обновление существующей установки

Сначала соберите, проверьте и подготовьте релиз, затем сделайте бэкап. Проверьте изменения схемы в новом коммите. Если нужно, запустите `migrate` нового бинарника catalog с production DSN: эта команда применяет только схему. До миграции оцените совместимость: откат бинарника не отменяет изменения схемы. `admin init` запускайте при необходимости инициализации административной схемы, а не для сброса пароля.

**При обычном обновлении не запускайте `catalog import` или `catalog bootstrap`.** Они заменяют весь каталог и могут удалить изменения из админки. Источником актуальных данных остаётся БД. Для намеренной полной замены датасета совместно проверьте CSV/метаданные/факты, сделайте бэкап, остановите запись через админку и явно выполните документированный CLI import.

Активируйте только изменённые компоненты. Пример обновляет админку; для API используйте `backend` и `hackalem-backend`. `mv -Tf` на Linux атомарно заменяет ссылку. Перезапуск сервиса даёт краткий перерыв; эта схема не гарантирует zero downtime.

```bash
RELEASE=20260923-example
NEXT="/opt/hackalem/admin_panel/releases/$RELEASE"
PREVIOUS=$(readlink -f /opt/hackalem/admin_panel/current)
test -x "$NEXT/admin"
printf '%s\n' "$PREVIOUS" > /opt/hackalem/admin_panel/previous-release
ln -s "$NEXT" /opt/hackalem/admin_panel/current.next
mv -Tf /opt/hackalem/admin_panel/current.next /opt/hackalem/admin_panel/current
systemctl restart hackalem-admin
curl --retry 5 --retry-connrefused --retry-delay 1 --fail http://127.0.0.1:18081/readyz
```

## Активация frontend

Используйте подготовленный frontend release с настройками реального API. Node-сервер не запускается. Сохраните прежнее значение current, затем переключите ссылку по примеру выше. При неизменной конфигурации Nginx reload не нужен. На время перехода сохраните доступность хешированных assets из недавно выданного HTML (скопируйте отсутствующие старые assets в новый релиз либо используйте общее хранилище), чтобы открытые клиенты не получали 404. Старый index.html поверх новой сборки не копируйте.

## 13. Проверки после выкладки

Выполните серверные команды ниже, затем повторите HTTPS-проверки с другой машины. Для healthcheck используйте GET: публичный API может отклонить HEAD с 405. Запрос каталога админки без сессии должен вернуть 401. Одного 200 от `/admin/` недостаточно: проверьте именно страницу входа Apex admin, а не React SPA fallback.

В браузере проверьте реальный API-режим, подбор, вход и чтение каталога админки. Новый администратор должен сменить временный пароль до доступа к каталогу. Создание/редактирование/импорт/удаление проверяйте на тестовой БД, не меняя рабочие записи ради smoke-теста. Перед запросом подбора с конкретной датой прочитайте календарные метаданные. Ожидаемые ID исходного набора могут меняться после легитимных правок каталога.

```bash
nginx -t
systemctl is-active hackalem-backend hackalem-admin nginx
systemctl is-enabled hackalem-backend hackalem-admin
curl --fail http://127.0.0.1:18080/readyz
curl --fail http://127.0.0.1:18081/readyz
curl --fail https://testrr.shop/api/health
curl --fail https://testrr.shop/api/catalog/meta
curl --fail https://testrr.shop/admin/ -o /dev/null
curl -sS -o /dev/null -w '%{http_code}\n' https://testrr.shop/admin/api/vendors
journalctl -u hackalem-backend -u hackalem-admin -n 80 --no-pager
```

## 14. Откат кода приложения

Для обновления только кода админки верните сохранённый релиз и перезапустите сервис. Файл previous-release относится к последнему обновлению; перед использованием проверьте его. Для backend применяется та же процедура, для статического frontend меняется только ссылка. Затем выполните приёмочные проверки.

Так возвращаются только бинарники/assets. Изменения каталога, пароли, сессии и миграции не отменяются. Если прежний бинарник несовместим с новой схемой, сначала спланируйте восстановление данных/схемы. Сохраняйте предыдущие релизы до приёмки. Не восстанавливайте весь старый Nginx site ради отката одного компонента: это может убрать маршруты последующих деплоев.

```bash
PREVIOUS=$(cat /opt/hackalem/admin_panel/previous-release)
test -x "$PREVIOUS/admin"
ln -s "$PREVIOUS" /opt/hackalem/admin_panel/current.rollback
mv -Tf /opt/hackalem/admin_panel/current.rollback /opt/hackalem/admin_panel/current
systemctl restart hackalem-admin
curl --retry 5 --retry-connrefused --retry-delay 1 --fail http://127.0.0.1:18081/readyz
```

## 15. Пробное восстановление БД

Задайте BACKUP_FILE путём выбранного дампа. Сначала восстановите его в новую БД; эти команды не перезаписывают production. Используйте PostgreSQL client, совместимый с версией дампа. Root открывает защищённый файл и передаёт поток pg_restore, поэтому ослаблять права не нужно. `--no-owner --role=hackalem` создаёт восстановленные объекты от роли приложения.

После восстановления проверьте целостность каталога и интеграционные/браузерные сценарии на изолированной конфигурации приложения с восстановленной БД и другими локальными портами. Одно количество записей не подтверждает готовность. CSV export — не полный бэкап: в нём нет фактов, календарных метаданных, пользователей, сессий и аудита.

```bash
BACKUP_FILE=/opt/hackalem/backups/SELECTED_TIMESTAMP/hackalem.dump
RESTORE_DB="hackalem_restore_$(date -u +%Y%m%d%H%M%S)"
runuser -u postgres -- createdb --owner=hackalem "$RESTORE_DB"
runuser -u postgres -- pg_restore --exit-on-error --single-transaction --no-owner --role=hackalem -d "$RESTORE_DB" < "$BACKUP_FILE"
runuser -u postgres -- psql -d "$RESTORE_DB" -c 'SELECT count(*) FROM vendors;'
runuser -u postgres -- psql -d "$RESTORE_DB" -c 'SELECT count(*) FROM admin_panel_users;' 
```

## Восстановление рабочих данных

Это отдельная операция обслуживания после успешной репетиции. Зафиксируйте выбранную копию и изменения, которые будут потеряны; сохраните свежий бэкап текущего состояния. Остановите оба Go-сервиса, переключите оба DATABASE_URL на проверенную восстановленную БД и запустите совместимые релизы. Peer-аутентификация должна разрешать восстановленную базу (правило HBA только для `hackalem` не совпадёт с её новым именем). Исходную БД сохраните для разбора, не удаляйте при переключении. Проверьте публичные и административные маршруты. Перед открытием админки отзовите восстановленные сессии (`DELETE FROM admin_panel_sessions`): в бэкапе могут быть ранее действовавшие сессии.

Аудит содержит автора/действие/ID и версии, но не полные прежние профили. Это не журнал отмены. Изменение описания делает прежние подтверждённые факты неактуальными; выкладка старых файлов их не восстановит.

## 16. Утерянный пароль администратора

Обычная смена пароля выполняется через интерфейс админки. При утрате пароля уполномоченный SSH-оператор может сбросить только административную учётную запись. Остановите админку, сохраните БД и выполните транзакцию ниже. Сессии этого пользователя удалятся каскадно по внешнему ключу. Каталог и аудит сохраняются. Init создаст новый временный пароль; сохраните его приватно и запустите сервис. Сам по себе повторный init не сбрасывает существующую учётную запись.

```bash
systemctl stop hackalem-admin
# Complete the backup procedure in section 11 before continuing.
runuser -u hackalem -- psql -h /var/run/postgresql -d hackalem -v ON_ERROR_STOP=1 <<'SQL'
BEGIN;
DELETE FROM admin_panel_users WHERE username = 'admin';
COMMIT;
SQL
runuser -u hackalem -- env   'DATABASE_URL=postgresql:///hackalem?host=/var/run/postgresql&user=hackalem'   /opt/hackalem/admin_panel/current/admin init
systemctl start hackalem-admin
```

## 17. Диагностика

| Симптом | Проверка / действие |
|---|---|
| Nginx 502 | Статус/journal сервиса, архитектура и права бинарника, current, loopback-порт; curl локальной readiness. |
| Readiness 503 | Доступность PostgreSQL, peer-пользователь, окружение, целостность каталога и таблицы админки. Не запускайте bootstrap как быстрый способ ремонта. |
| Вход не работает после изменения env | Точный ADMIN_ORIGIN, HTTPS, Secure cookie, основной домен и новое окружение сервиса. |
| 403 ORIGIN_REJECTED / CSRF_REJECTED | Откройте правильный origin, обновите страницу/войдите заново для нового токена; защиту не отключайте. |
| 403 PASSWORD_CHANGE_REQUIRED | Смените временный пароль и войдите повторно. |
| 409 REVISION_CONFLICT | Обновите каталог или повторите CSV preview: другая операция изменила версию. |
| 413 при загрузке | Весь multipart должен укладываться в 5 МиБ; отдельный лимит Nginx — 6 МиБ, ответ может быть HTML. Разделите файл. |
| 422 данные | Прочитайте ошибку; проверьте заголовки, типы, ID, допустимые значения и календарное окно. |
| 429 вход/пароль | Подождите 15 минут; лимит — 10 попыток на IP за окно, хранится в памяти процесса. |
| Admin API возвращает HTML | Проверьте префиксы Nginx и SPA fallback; сохраняйте URI при проксировании. |
| Frontend показывает mock | Пересоберите с VITE_API_MODE=http и VITE_API_BASE_URL=/api; изменение env не переписывает готовый JS. |
| Изменение интерфейса не видно | Админке нужна пересборка/перезапуск бинарника, React — новая dist и ссылка current. Обновите браузер. |
| Peer authentication failed | Пользователь ОС и роль БД — hackalem, DSN использует Unix socket и подходящее HBA-правило. |
| Дата подбора отклоняется | Проверьте `/api/catalog/meta`; исходный календарь имеет фиксированную конечную дату. |

Диагностика: `journalctl -u hackalem-admin -u hackalem-backend`, `/var/log/nginx/testrr.shop.error.log`, `pg_lsclusters`, `ss -lnt`. Пароли и session cookies не должны попадать в логи/метрики. `/metrics` и `/readyz` приложений — локальные операторские маршруты, Nginx их напрямую не публикует.

## 18. Связанные документы и объём проверки

- [Обзор проекта](README.ru.md)
- [Backend API](backend/ENDPOINT.MD)
- [Admin API](admin_panel/ENDPOINT.MD)
- [Шаблоны развёртывания админки](admin_panel/deploy/README.md)
- [Тесты backend](backend/TESTING.md) и [тесты админки](admin_panel/TESTING.md)

При обновлении документации проверены соответствие исходникам/конфигурации, относительные ссылки и синтаксис Bash. Настройка нового сервера, выдача сертификата и восстановление с заменой данных не выполнялись на production ради проверки текста. Для каждого реального релиза запускайте его тесты, проверки staging и приёмку; прошлые успешные выкладки их не заменяют.
