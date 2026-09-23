# Развёртывание

Production: https://testrr.shop/admin/. Сервис `hackalem-admin`, адрес `127.0.0.1:18081`, пользователь ОС `hackalem`; общий PostgreSQL с backend. Публичный доступ идёт через существующий nginx/TLS. Отдельный порт наружу не открывается.

## Сборка

Из папки `admin_panel` при наличии соседнего `backend`:

```sh
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -o bin/admin-linux-amd64 ./cmd/admin
```

Поместите бинарник `admin` и документы в `/opt/hackalem/admin_panel/releases/<release>/`. Релизы принадлежат root, бинарник 0755. Симлинк `/opt/hackalem/admin_panel/current` указывает на активный релиз.

Перед первым запуском сделайте `pg_dump -Fc` существующей БД. Скопируйте `admin.env.example` в `/etc/hackalem/admin.env` (root:hackalem, 0640). Запустите init от пользователя hackalem с `DATABASE_URL` из этого файла. Сохраните напечатанный временный пароль в защищённом месте; он не записывается в env или git. `init` не импортирует и не заменяет существующий каталог.

Установите `hackalem-admin.service` в `/etc/systemd/system/`, затем:

```sh
systemctl daemon-reload
systemctl enable --now hackalem-admin
curl --fail http://127.0.0.1:18081/readyz
```

Скопируйте `hackalem-admin.conf` в `/etc/nginx/snippets/`; в HTTPS server block `testrr.shop` добавьте `include /etc/nginx/snippets/hackalem-admin.conf;`. Сохраните все текущие маршруты фронтенда и `/api`. Проверьте `nginx -t`, затем `systemctl reload nginx`.

## Обслуживание

```sh
systemctl status hackalem-admin
journalctl -u hackalem-admin -n 80 --no-pager
curl --fail https://testrr.shop/admin/
curl --fail https://testrr.shop/api/health
```

Обновление: создать новый каталог релиза, переключить current и перезапустить `hackalem-admin`. Откат бинарника — переключить current на прежний релиз и перезапустить сервис. Перед изменениями схемы сохраните резервную копию БД. Откат бинарника не откатывает редактирование каталога; восстановление данных выполняется отдельно из проверенной резервной копии. CLI backend `import/bootstrap` заменяет каталог целиком: не запускайте их при обновлении админки.

Временный пароль меняется при первом входе. Если пароль утрачен, восстановление выполняет оператор через SSH: остановить admin, удалить сессии и пользователя admin в одной транзакции, повторить init для новой временной учётной записи, затем запустить сервис. Это не затрагивает каталог или аудит. Резервные копии БД включают хеши паролей и сессии; храните их с правами 0600.
