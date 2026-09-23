# Deployment and operations

[English](DEPLOY.MD) · [Русский](DEPLOY.ru.MD)

## 1. Scope and live topology

This guide covers the public React frontend, Go recommendation API, separate Go admin panel, PostgreSQL, Nginx and TLS. Verified against the server configuration on **September 23, 2026**. Commands are procedures for an operator; writing this document did not reinstall or restart production.

| Component | Production location |
|---|---|
| Server | `2.29.54.10`, Ubuntu 26.04.1 LTS, x86_64 |
| Website | `https://testrr.shop/` → `/opt/hackalem/frontend/current` |
| Public API | `/api/*` → `127.0.0.1:18080`, `hackalem-backend.service` |
| Admin | `/admin/*` → `127.0.0.1:18081`, `hackalem-admin.service` |
| Database | PostgreSQL `18/main`, database/role `hackalem`, local Unix socket |
| TLS | `/etc/letsencrypt/live/testrr.shop/`, `certbot.timer` |

Both Go processes run as OS user `hackalem` and use the same database. Admin changes are immediately visible to the API. The baseline dataset has 66 profiles; the live count may change. The supplied calendar covers 2026-09-23 through 2026-12-31. Runtime LLM access is not required.

For an existing installation, follow sections 2–4, 11–14. Sections 5–10 are for initial installation or deliberate infrastructure changes.

## 2. Prerequisites and command conventions

Build machine: authenticated Git access, Go 1.25+, Node.js 24+ with npm, Python 3 for optional checks, SSH/SCP and tar. Use a clean checkout and one reviewed commit for all components; `admin_panel/go.mod` requires the adjacent `backend` directory. Go race tests require a native build toolchain with CGO support.

Production server: root/sudo access, PostgreSQL 18, Nginx, curl and an existing valid TLS certificate. Go, Node and Docker are unnecessary on the server when deploying prebuilt artifacts. DNS A records for the domain must point to the server; publish AAAA only if IPv6 is configured. Permit SSH, HTTP and HTTPS in the host/provider firewalls; keep 5432, 18080 and 18081 private.

Unless labelled otherwise, local commands start in the repository root; server commands run in a root Bash shell. Replace `RELEASE` with a unique release ID and use the same value locally and remotely. Do not execute every block blindly: initial setup, update and recovery are separate workflows. SSH passwords, admin passwords, cookies, `.env` files and dumps are never committed.

## 3. Test and build locally

Create a dedicated PostgreSQL test database first. Set `TEST_DATABASE_URL` to its connection string; the test role must be able to create schemas. Integration tests use temporary schemas. Without that variable, integration tests are skipped, so a green unit-test run is insufficient evidence of DB compatibility. See [backend tests](backend/TESTING.md) and [admin tests](admin_panel/TESTING.md).

Run in a Bash shell. The commands build Linux amd64 binaries even on macOS. For a different server architecture, change GOARCH after checking `uname -m`. Admin HTML/CSS/JS are embedded in the Go binary: every interface edit requires rebuilding it. Frontend `VITE_*` values are public and embedded at build time; explicit production values below override local mock settings.

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

## 4. Stage the release on the server

Run after uploading the archive. These steps do not change running services. New release directories must not already exist. SHA256SUMS verifies uploaded files against the build manifest; SSH provides authenticated transport. Keep artifacts and directories root-owned, readable by the service users. Database storage stays outside release directories.

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

## 5. First installation: PostgreSQL and OS user

Skip on the existing server. On a fresh Ubuntu host install `postgresql`, `postgresql-client`, `nginx` and `curl` using the configured OS package repositories. Confirm that the resulting cluster/version matches the target before proceeding; do not upgrade an existing cluster as part of application deployment.

Create the OS account, database role and database once. The examples assume they do not exist. Configure `listen_addresses = 'localhost'` and a `local hackalem hackalem peer` rule in pg_hba.conf before any conflicting local rule. Locate actual configuration files with `SHOW config_file;` and `SHOW hba_file;`. Restart PostgreSQL only if the listen address changed; reload for HBA changes. Never switch authentication to `trust` to bypass a failed connection.

```bash
useradd --system --user-group --home-dir /opt/hackalem --no-create-home --shell /usr/sbin/nologin hackalem
systemctl enable --now postgresql
runuser -u postgres -- createuser --no-superuser --no-createdb --no-createrole hackalem
runuser -u postgres -- createdb --owner=hackalem hackalem
runuser -u hackalem -- psql -h /var/run/postgresql -d hackalem -c 'SELECT current_user, current_database();'
pg_lsclusters
ss -lnt
```

## 6. First installation: environment

On an existing server, inspect and merge configuration rather than overwriting it. systemd EnvironmentFile values are not shell scripts: the backend DATABASE_URL below contains `&` unquoted, so do not source that file with Bash. The administrative CLI examples pass DATABASE_URL explicitly via `env`.

Keep `ADMIN_ORIGIN` as the exact browser origin without `/admin/` or a trailing slash. Loopback HTTP behind Nginx still requires Secure cookies: `ADMIN_INSECURE_HTTP=false`. Trust proxy headers only when the app binds to loopback and Nginx overwrites X-Real-IP. Relative catalog paths resolve from the backend unit WorkingDirectory.

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

## 7. First installation: catalog and administrator

Only for an empty initial database: `catalog bootstrap` creates schema and replaces the complete catalog. It is not an update command. The source files are validated together, including metadata and facts. `admin init` creates admin tables and user `admin`, prints a random temporary password once and leaves an existing password unchanged. Save the generated password in your password manager; no shared default password is published here. The first login requires a password change (12+ characters, at most 72 bytes); all sessions are revoked after a change.

`ADMIN_INITIAL_PASSWORD` is optional for initial setup only. Do not retain it in the service environment. If the catalog already exists, skip bootstrap entirely and run only init when needed.

```bash
for component in backend admin_panel frontend; do
    ln -s "/opt/hackalem/$component/releases/$RELEASE" "/opt/hackalem/$component/current"
done
(cd /opt/hackalem/backend/current && runuser -u hackalem -- ./catalog check)
(cd /opt/hackalem/backend/current && runuser -u hackalem -- env   'DATABASE_URL=postgresql:///hackalem?host=/var/run/postgresql&user=hackalem' ./catalog bootstrap)
runuser -u hackalem -- env   'DATABASE_URL=postgresql:///hackalem?host=/var/run/postgresql&user=hackalem'   /opt/hackalem/admin_panel/current/admin init
```

## 8. First installation: systemd

Install the backend unit below and the versioned admin unit from the artifact. Their environment, working directories and ExecStart paths must agree. The admin unit includes the same service isolation protections. Database migrations are an explicit operator step, not an automatic ExecStart task.

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

## 9. Nginx routes

The live site already has Nginx and TLS: preserve its server blocks, certificate settings, frontend root and existing API routes. Back up the site before editing. Install the admin snippet once from the artifact. The following is the required content **inside the canonical HTTPS server block**, not a complete Nginx configuration. If equivalent locations already exist in included snippets, do not add duplicates.

`proxy_pass` without a URI preserves `/api/` and `/admin/`. The exact `/api` and `/api/health` aliases intentionally rewrite the URI. `location ^~ /admin/` in the shipped snippet prevents the SPA fallback from serving admin API requests. Nginx allows 6 MiB for admin uploads, while the application caps the complete multipart request at 5 MiB. Ports 18080/18081 must remain on loopback.

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

## Apply a reviewed Nginx change

On a fresh installation create the site file and sites-enabled symlink before testing; on the existing host edit `/etc/nginx/sites-available/testrr.shop`. The commands below install only the admin snippet and validate/reload Nginx; edit the site separately before the final two commands. Never reload after a failed configuration test.

```bash
install -d -m 700 /opt/hackalem/config-backups
cp -a /etc/nginx/sites-available/testrr.shop "/opt/hackalem/config-backups/testrr.shop.$(date -u +%Y%m%dT%H%M%SZ)"
install -m 644 "$STAGE/admin_panel/deploy/hackalem-admin.conf" /etc/nginx/snippets/hackalem-admin.conf
nginx -t
systemctl reload nginx
```

## 10. TLS and a different domain

The current certificate is already installed and `certbot.timer` is scheduled. Check it with `certbot certificates` and `systemctl status certbot.timer`. Do not request a replacement certificate during routine app updates.

For a new server/domain: configure DNS and a port-80 site serving `/.well-known/acme-challenge/` from `/var/www/letsencrypt`; install Certbot, then run `certbot certonly --webroot -w /var/www/letsencrypt -d YOUR_DOMAIN` using the real domain and operator contact. Only after issuance add the HTTPS listener and certificate paths from `certbot certificates`. Redirect HTTP to HTTPS while keeping the challenge location reachable. If serving www, include its DNS name in the certificate and redirect it to the canonical domain. Configure a successful-renewal deploy hook to run `nginx -t && systemctl reload nginx`; test with `certbot renew --dry-run` and verify the timer. These are new-host setup steps, not commands executed while writing this guide.

When changing domain update Nginx server_name/TLS, ADMIN_ORIGIN and backend CORS_ORIGINS. Frontend `/api` stays same-origin. Restart the affected Go services after changing env files. Admin cookies require HTTPS; do not enable insecure mode for a public deployment.

## 11. Backup before an update

Run as root before schema/catalog operations and before release activation. A custom-format dump includes catalog, facts, metadata, admin users, sessions and audit. `pg_restore --list` checks the archive table of contents, not recoverability: perform a separate restore rehearsal (section 15). pg_dump gives a consistent database snapshot without stopping services; later writes are not included.

Keep an encrypted/off-server copy according to your retention policy. The commands below create a manual backup only; no automated application backup schedule is installed by this guide. Protect configuration backups and dumps with root-only permissions.

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

## 12. Update an existing installation

Build/test/stage first, then back up. Review whether the new commit changes the schema. If required, run the new catalog binary's `migrate` with the production DSN; it applies schema only. Review compatibility before migrations because binary rollback does not reverse schema changes. Run `admin init` only when needed for admin schema initialization, not for resetting credentials.

**Do not run `catalog import` or `catalog bootstrap` during a normal update.** They replace the entire catalog and can discard admin edits. Keep the live DB as the source of truth. For deliberate complete dataset replacement, validate CSV/metadata/facts together, back up, stop admin writes and use the documented CLI import explicitly.

Activate only changed components. The example below updates the admin service; use `backend` + `hackalem-backend` for the API. `mv -Tf` on the Linux server atomically replaces the symlink. A service restart causes a short interruption; this setup does not promise zero downtime.

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

## Frontend activation

Use the staged frontend release built with real API settings. No Node server is started. Save its previous current target, then switch the symlink as above. If Nginx configuration is unchanged, a reload is unnecessary. Keep hashed assets used by recently served HTML available during the transition (copy missing old assets into the new release, or retain them through a shared asset store) so already-open clients do not receive 404s. Never copy an older index.html over the new build.

## 13. Acceptance checks

Run server checks below and then repeat HTTPS checks from a separate machine. Use GET for health checks: the public API can reject HEAD with 405. An unauthenticated admin catalog request must return 401. A 200 from `/admin/` alone is insufficient: verify the Apex admin login page rather than a React SPA fallback.

In the browser verify real API mode, recommendations, admin login and catalog reading. New admins must change their temporary password before catalog access. Exercise create/edit/import/delete against a test DB; do not alter production records merely for smoke testing. Review returned calendar metadata before using a dated recommendation example. Baseline result IDs may change after legitimate catalog edits.

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

## 14. Roll back application code

For a code-only admin update, switch back to the saved release and restart. The previous-release file belongs to the most recent update; inspect it before using it. Apply the same procedure to backend, and switch only the frontend symlink for a static rollback. Run acceptance checks afterwards.

This restores binaries/assets only. It does not undo catalog mutations, passwords, sessions or migrations. If the old binary is incompatible with the new schema, plan data/schema recovery first. Keep previous release directories until the update is accepted. Do not restore an entire old Nginx site just to roll back one component: it may remove routes added by later deployments.

```bash
PREVIOUS=$(cat /opt/hackalem/admin_panel/previous-release)
test -x "$PREVIOUS/admin"
ln -s "$PREVIOUS" /opt/hackalem/admin_panel/current.rollback
mv -Tf /opt/hackalem/admin_panel/current.rollback /opt/hackalem/admin_panel/current
systemctl restart hackalem-admin
curl --retry 5 --retry-connrefused --retry-delay 1 --fail http://127.0.0.1:18081/readyz
```

## 15. Rehearse database restoration

Set BACKUP_FILE to a real selected dump. Restore into a new database first; these commands do not overwrite production. Use a PostgreSQL client compatible with the dump version. Root opens the protected dump and streams it to pg_restore, avoiding permissive file modes. `--no-owner --role=hackalem` creates restored objects as the application role.

After restoration run catalog integrity checks and integration/browser scenarios using an isolated application configuration pointing to the restored DB and different local ports. Counts alone do not prove readiness. CSV export is not a full backup: it excludes facts, calendar metadata, users, sessions and audit.

```bash
BACKUP_FILE=/opt/hackalem/backups/SELECTED_TIMESTAMP/hackalem.dump
RESTORE_DB="hackalem_restore_$(date -u +%Y%m%d%H%M%S)"
runuser -u postgres -- createdb --owner=hackalem "$RESTORE_DB"
runuser -u postgres -- pg_restore --exit-on-error --single-transaction --no-owner --role=hackalem -d "$RESTORE_DB" < "$BACKUP_FILE"
runuser -u postgres -- psql -d "$RESTORE_DB" -c 'SELECT count(*) FROM vendors;'
runuser -u postgres -- psql -d "$RESTORE_DB" -c 'SELECT count(*) FROM admin_panel_users;' 
```

## Production data recovery

This is a separate maintenance operation after a successful rehearsal. Record the selected backup and the writes that would be lost; take a fresh backup of the current state. Stop both Go services, switch both DATABASE_URL values to the validated restored database, and start compatible releases. Peer authentication must permit the restored database (an HBA rule restricted to database `hackalem` will not match its new name). Keep the original database for investigation; do not drop it during cutover. Recheck all public/admin routes. Revoke restored sessions (`DELETE FROM admin_panel_sessions`) before reopening admin access, since a backup can contain previously valid sessions.

The audit table records actors/actions/IDs and revisions, not complete historical profiles. It is not an undo log. Changing a description invalidates previous curated facts; redeploying old files does not recreate them.

## 16. Lost admin password

Normal password changes happen in the admin UI. For a lost password, an authorized SSH operator can reset only the admin account. Stop admin, back up the DB, then execute the transaction below. Sessions for that username are removed by the foreign-key cascade. Catalog and audit remain intact. Run init to generate a new temporary password, save it privately and restart admin. Running init alone never resets an existing account.

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

## 17. Troubleshooting

| Symptom | Check / action |
|---|---|
| Nginx 502 | Service status/journal, binary architecture and permissions, current symlink, loopback port; curl local readiness. |
| Readiness 503 | PostgreSQL availability, peer identity, environment, catalog integrity and initialized admin tables. Do not run bootstrap as a repair shortcut. |
| Admin login fails after env change | Exact ADMIN_ORIGIN, HTTPS, Secure cookie, canonical domain and the new service environment. |
| 403 ORIGIN_REJECTED / CSRF_REJECTED | Use the correct origin, reload/login and obtain a fresh session token; do not disable protection. |
| 403 PASSWORD_CHANGE_REQUIRED | Change the temporary password and log in again. |
| 409 REVISION_CONFLICT | Reload catalog or repeat CSV preview; another mutation changed the revision. |
| 413 upload | Whole multipart request must fit 5 MiB; Nginx has a separate 6 MiB cap and may return HTML. Split the file. |
| 422 data | Read the validation message; verify column names, types, ID, enum values and calendar window. |
| 429 login/password | Wait 15 minutes; limit is 10 attempts per IP per window in process memory. |
| Admin API returns HTML | Inspect Nginx prefix routing and SPA fallback; preserve the upstream URI. |
| Frontend shows mock data | Rebuild with VITE_API_MODE=http and VITE_API_BASE_URL=/api; env changes do not rewrite built JS. |
| UI edit is missing | Admin needs binary rebuild/restart; React needs a new dist build and current symlink. Reload the browser. |
| Peer authentication fails | OS user and DB role must be hackalem, DSN must use the Unix socket and applicable HBA rule. |
| A recommendation date is rejected | Inspect `/api/catalog/meta`; the initial calendar has a fixed end date. |

Useful diagnostics: `journalctl -u hackalem-admin -u hackalem-backend`, `/var/log/nginx/testrr.shop.error.log`, `pg_lsclusters`, `ss -lnt`. Logs and metrics must not expose passwords or session cookies. `/metrics` and application `/readyz` are local operator endpoints, not public Nginx routes.

## 18. Related documentation and validation scope

- [Project overview](README.md)
- [Backend API](backend/ENDPOINT.MD)
- [Admin API](admin_panel/ENDPOINT.MD)
- [Admin deployment templates](admin_panel/deploy/README.md)
- [Backend test guide](backend/TESTING.md) and [admin test guide](admin_panel/TESTING.md)

This documentation update checked source/configuration consistency, relative links and Bash syntax. Fresh-server provisioning, certificate issuance and destructive recovery were not executed against production to validate prose. For each actual release, run its tests, staging checks and acceptance checks; previous successful deployments are not a substitute.
