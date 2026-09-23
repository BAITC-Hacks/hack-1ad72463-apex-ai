package app

import (
	"context"
	"crypto/subtle"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	catalog "github.com/BAITC-Hacks/hack-1ad72463-apex-ai/backend/catalogadmin"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

//go:embed web/*
var assets embed.FS

const cookieName = "hackalem_admin_session"

type Config struct {
	Origin       string
	SecureCookie bool
	TrustProxy   bool
}
type attempt struct {
	Count int
	Until time.Time
}
type App struct {
	pool      *pgxpool.Pool
	catalog   *catalog.Service
	config    Config
	logger    *slog.Logger
	dummyHash []byte
	mu        sync.Mutex
	attempts  map[string]attempt
}

func New(pool *pgxpool.Pool, config Config, logger *slog.Logger) *App {
	dummy, _ := bcrypt.GenerateFromPassword([]byte(token()), bcrypt.DefaultCost)
	return &App{pool: pool, catalog: catalog.New(pool), config: config, logger: logger, dummyHash: dummy, attempts: map[string]attempt{}}
}
func write(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
func fail(w http.ResponseWriter, code int, key, message string) {
	write(w, code, map[string]any{"error": map[string]string{"code": key, "message": message}})
}
func (a *App) cookie(w http.ResponseWriter, raw string, expiry time.Time) {
	http.SetCookie(w, &http.Cookie{Name: cookieName, Value: raw, Path: "/admin", HttpOnly: true, Secure: a.config.SecureCookie, SameSite: http.SameSiteStrictMode, Expires: expiry, MaxAge: 28800})
}
func (a *App) clearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: cookieName, Path: "/admin", HttpOnly: true, Secure: a.config.SecureCookie, SameSite: http.SameSiteStrictMode, MaxAge: -1})
}
func allowed(w http.ResponseWriter, r *http.Request, methods ...string) bool {
	if slices.Contains(methods, r.Method) {
		return true
	}
	w.Header().Set("Allow", strings.Join(methods, ", "))
	fail(w, 405, "METHOD_NOT_ALLOWED", "Метод не поддерживается.")
	return false
}
func decode(w http.ResponseWriter, r *http.Request, dst any) bool {
	media, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if media != "application/json" {
		fail(w, 415, "UNSUPPORTED_MEDIA_TYPE", "Ожидается application/json.")
		return false
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 512*1024))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		fail(w, 422, "INVALID_JSON", "Проверьте поля, типы и размер JSON.")
		return false
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		fail(w, 422, "INVALID_JSON", "Ожидается один JSON-объект.")
		return false
	}
	return true
}
func (a *App) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	id := token()[:16]
	w.Header().Set("X-Request-ID", id)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "same-origin")
	w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'")
	defer func() {
		if recover() != nil {
			a.logger.Error("admin request panic", "request_id", id)
			fail(w, 500, "INTERNAL_ERROR", "Внутренняя ошибка.")
		}
	}()
	if r.URL.Path == "/admin" {
		http.Redirect(w, r, "/admin/", http.StatusPermanentRedirect)
		return
	}
	if r.URL.Path == "/admin/" || r.URL.Path == "/admin/app.js" || r.URL.Path == "/admin/style.css" {
		if !allowed(w, r, "GET", "HEAD") {
			return
		}
		file := "index.html"
		contentType := "text/html; charset=utf-8"
		if r.URL.Path == "/admin/app.js" {
			file = "app.js"
			contentType = "text/javascript; charset=utf-8"
		}
		if r.URL.Path == "/admin/style.css" {
			file = "style.css"
			contentType = "text/css; charset=utf-8"
		}
		b, _ := assets.ReadFile("web/" + file)
		w.Header().Set("Content-Type", contentType)
		if r.Method == "GET" {
			_, _ = w.Write(b)
		}
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	r = r.WithContext(ctx)
	if r.URL.Path == "/readyz" {
		if !allowed(w, r, "GET") {
			return
		}
		var exists bool
		err := a.pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM admin_panel_users)").Scan(&exists)
		if err == nil && exists {
			_, err = a.catalog.Snapshot(ctx)
		}
		if err != nil || !exists {
			fail(w, 503, "NOT_READY", "Админка не готова.")
			return
		}
		write(w, 200, map[string]string{"status": "ready"})
		return
	}
	if !strings.HasPrefix(r.URL.Path, "/admin/api/") {
		fail(w, 404, "NOT_FOUND", "Путь не найден.")
		return
	}
	mutating := r.Method != "GET" && r.Method != "HEAD"
	if mutating && r.Header.Get("Origin") != a.config.Origin {
		fail(w, 403, "ORIGIN_REJECTED", "Запрос должен исходить со страницы админки.")
		return
	}
	if r.URL.Path == "/admin/api/login" {
		a.handleLogin(w, r)
		return
	}
	session, err := r.Cookie(cookieName)
	if err != nil {
		fail(w, 401, "UNAUTHORIZED", "Войдите в панель.")
		return
	}
	user, err := a.identity(ctx, session.Value)
	if err != nil {
		if errors.Is(err, errCredentials) {
			a.clearCookie(w)
			fail(w, 401, "UNAUTHORIZED", "Сессия истекла. Войдите снова.")
		} else {
			fail(w, 503, "DATABASE_UNAVAILABLE", "База данных временно недоступна.")
		}
		return
	}
	if mutating && subtle.ConstantTimeCompare([]byte(user.CSRF), []byte(r.Header.Get("X-CSRF-Token"))) != 1 {
		fail(w, 403, "CSRF_REJECTED", "Обновите страницу и повторите запрос.")
		return
	}
	switch r.URL.Path {
	case "/admin/api/session":
		if allowed(w, r, "GET") {
			write(w, 200, user)
		}
		return
	case "/admin/api/logout":
		if !allowed(w, r, "POST") {
			return
		}
		if _, err = a.pool.Exec(ctx, "DELETE FROM admin_panel_sessions WHERE token_hash=$1", digest(session.Value)); err != nil {
			fail(w, 503, "DATABASE_UNAVAILABLE", "Не удалось завершить сессию.")
			return
		}
		a.clearCookie(w)
		write(w, 200, map[string]bool{"ok": true})
		return
	case "/admin/api/password":
		if !allowed(w, r, "POST") {
			return
		}
		if a.limited(a.clientIP(r) + ":password") {
			w.Header().Set("Retry-After", "900")
			fail(w, 429, "RATE_LIMITED", "Повторите через 15 минут.")
			return
		}
		var body struct {
			Current string `json:"current_password"`
			Next    string `json:"new_password"`
		}
		if !decode(w, r, &body) {
			return
		}
		if err = a.changePassword(ctx, user, body.Current, body.Next); err != nil {
			fail(w, 422, "PASSWORD_REJECTED", "Проверьте текущий пароль. Новый: минимум 12 символов, максимум 72 байта; должен отличаться.")
			return
		}
		a.clearCookie(w)
		write(w, 200, map[string]bool{"ok": true})
		return
	}
	if user.MustChange {
		fail(w, 403, "PASSWORD_CHANGE_REQUIRED", "Смените временный пароль перед работой с каталогом.")
		return
	}
	a.catalogRoutes(w, r, user)
}

func (a *App) clientIP(r *http.Request) string {
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	if a.config.TrustProxy && net.ParseIP(host).IsLoopback() {
		if ip := net.ParseIP(r.Header.Get("X-Real-IP")); ip != nil {
			return ip.String()
		}
	}
	return host
}
func (a *App) limited(key string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	now := time.Now()
	for k, v := range a.attempts {
		if now.After(v.Until) {
			delete(a.attempts, k)
		}
	}
	if len(a.attempts) > 10000 {
		return true
	}
	v := a.attempts[key]
	if v.Count == 0 {
		v.Until = now.Add(15 * time.Minute)
	}
	v.Count++
	a.attempts[key] = v
	return v.Count > 10
}
func (a *App) handleLogin(w http.ResponseWriter, r *http.Request) {
	if !allowed(w, r, "POST") {
		return
	}
	if a.limited(a.clientIP(r) + ":login") {
		w.Header().Set("Retry-After", "900")
		fail(w, 429, "RATE_LIMITED", "Слишком много попыток. Повторите через 15 минут.")
		return
	}
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !decode(w, r, &body) {
		return
	}
	if len(body.Password) > 72 || body.Username == "" {
		fail(w, 401, "INVALID_CREDENTIALS", "Неверный логин или пароль.")
		return
	}
	user, raw, err := a.login(r.Context(), body.Username, body.Password)
	if err != nil {
		if errors.Is(err, errCredentials) {
			fail(w, 401, "INVALID_CREDENTIALS", "Неверный логин или пароль.")
		} else {
			fail(w, 503, "DATABASE_UNAVAILABLE", "База данных временно недоступна.")
		}
		return
	}
	a.cookie(w, raw, user.Expires)
	write(w, 200, user)
}

func (a *App) catalogRoutes(w http.ResponseWriter, r *http.Request, user Identity) {
	ctx := r.Context()
	path := r.URL.Path
	if path == "/admin/api/audit" {
		if !allowed(w, r, "GET") {
			return
		}
		rows, err := a.pool.Query(ctx, "SELECT id,actor,action,vendor_ids,created_at FROM admin_panel_audit ORDER BY id DESC LIMIT 50")
		if err != nil {
			fail(w, 503, "DATABASE_UNAVAILABLE", "Не удалось загрузить журнал.")
			return
		}
		defer rows.Close()
		events := []map[string]any{}
		for rows.Next() {
			var id int64
			var actor, action string
			var ids []string
			var at time.Time
			if rows.Scan(&id, &actor, &action, &ids, &at) != nil {
				fail(w, 503, "DATABASE_UNAVAILABLE", "Не удалось загрузить журнал.")
				return
			}
			events = append(events, map[string]any{"id": id, "actor": actor, "action": action, "vendor_ids": ids, "created_at": at})
		}
		if rows.Err() != nil {
			fail(w, 503, "DATABASE_UNAVAILABLE", "Не удалось загрузить журнал.")
			return
		}
		write(w, 200, events)
		return
	}
	known := path == "/admin/api/vendors" || strings.HasPrefix(path, "/admin/api/vendors/") || path == "/admin/api/import" || path == "/admin/api/import/preview" || path == "/admin/api/export" || path == "/admin/api/template"
	if !known {
		fail(w, 404, "NOT_FOUND", "Путь не найден.")
		return
	}
	snapshot, err := a.catalog.Snapshot(ctx)
	if err != nil {
		fail(w, 503, "CATALOG_UNAVAILABLE", "Каталог временно недоступен.")
		return
	}
	revision := catalog.Revision(snapshot)
	switch path {
	case "/admin/api/vendors":
		if r.Method == "GET" {
			w.Header().Set("ETag", `"`+revision+`"`)
			write(w, 200, map[string]any{"vendors": snapshot.Vendors, "revision": revision, "metadata": snapshot.Metadata, "event_formats": catalog.Formats, "languages": catalog.Languages})
			return
		}
		if !allowed(w, r, "POST") {
			return
		}
		var v catalog.Vendor
		if !decode(w, r, &v) {
			return
		}
		a.apply(w, r, user, catalog.Change{Action: "create", Vendors: []catalog.Vendor{v}})
		return
	case "/admin/api/export", "/admin/api/template":
		if !allowed(w, r, "GET") {
			return
		}
		vendors := snapshot.Vendors
		name := "catalog.csv"
		if path == "/admin/api/template" {
			vendors = nil
			name = "catalog-template.csv"
		}
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
		_, _ = w.Write(catalog.CSV(vendors))
		return
	case "/admin/api/import", "/admin/api/import/preview":
		if !allowed(w, r, "POST") {
			return
		}
		w.Header().Set("ETag", `"`+revision+`"`)
		r.Body = http.MaxBytesReader(w, r.Body, 5*1024*1024)
		if err = r.ParseMultipartForm(5 * 1024 * 1024); err != nil {
			fail(w, 413, "INVALID_UPLOAD", "Передайте CSV файлом до 5 МБ.")
			return
		}
		defer r.MultipartForm.RemoveAll()
		file, _, err := r.FormFile("file")
		if err != nil {
			fail(w, 422, "FILE_REQUIRED", "Выберите CSV-файл.")
			return
		}
		defer file.Close()
		raw, err := io.ReadAll(file)
		if err != nil {
			fail(w, 422, "INVALID_UPLOAD", "Не удалось прочитать файл.")
			return
		}
		parsed, err := catalog.ParseCSV(raw, snapshot.Metadata.CalendarWindow)
		if err != nil {
			fail(w, 422, "INVALID_CSV", err.Error())
			return
		}
		if len(parsed.Vendors) > 5000 {
			fail(w, 422, "TOO_MANY_RECORDS", "Не более 5000 строк за одну загрузку.")
			return
		}
		upsert := r.FormValue("update_existing") == "true"
		known := map[string]bool{}
		for _, v := range snapshot.Vendors {
			known[v.ID] = true
		}
		existing := []string{}
		added := 0
		for _, v := range parsed.Vendors {
			if known[v.ID] {
				existing = append(existing, v.ID)
			} else {
				added++
			}
		}
		if path == "/admin/api/import/preview" {
			sample := parsed.Vendors[:min(len(parsed.Vendors), 8)]
			write(w, 200, map[string]any{"total": len(parsed.Vendors), "new_count": added, "existing_count": len(existing), "existing_ids": existing, "sample": sample, "revision": revision})
			return
		}
		a.apply(w, r, user, catalog.Change{Action: "import", Vendors: parsed.Vendors, Upsert: upsert})
		return
	}
	if strings.HasPrefix(path, "/admin/api/vendors/") {
		id := strings.TrimPrefix(path, "/admin/api/vendors/")
		if id == "" || strings.Contains(id, "/") {
			fail(w, 404, "NOT_FOUND", "Запись не найдена.")
			return
		}
		if !allowed(w, r, "PUT", "DELETE") {
			return
		}
		change := catalog.Change{Action: "delete", ID: id}
		if r.Method == "PUT" {
			var v catalog.Vendor
			if !decode(w, r, &v) {
				return
			}
			change.Action = "update"
			change.Vendors = []catalog.Vendor{v}
		}
		a.apply(w, r, user, change)
	}
}
func (a *App) apply(w http.ResponseWriter, r *http.Request, user Identity, c catalog.Change) {
	c.Expected = strings.Trim(r.Header.Get("If-Match"), `"`)
	if c.Expected == "" {
		fail(w, 428, "REVISION_REQUIRED", "Обновите каталог перед изменением.")
		return
	}
	c.Actor = user.Username
	updated, err := a.catalog.Apply(r.Context(), c)
	if err != nil {
		switch {
		case errors.Is(err, catalog.ErrConflict):
			fail(w, 409, "REVISION_CONFLICT", "Каталог уже изменился. Обновите список и повторите действие.")
		case errors.Is(err, catalog.ErrDuplicate):
			fail(w, 409, "DUPLICATE_ID", err.Error())
		case errors.Is(err, catalog.ErrNotFound):
			fail(w, 404, "NOT_FOUND", "Запись не найдена.")
		case errors.Is(err, catalog.ErrLastVendor):
			fail(w, 409, "LAST_VENDOR", "Последнюю запись удалять нельзя: рабочий каталог должен содержать хотя бы один профиль.")
		case errors.Is(err, catalog.ErrInvalid):
			fail(w, 422, "INVALID_VENDOR", err.Error())
		default:
			a.logger.Error("admin mutation failed", "action", c.Action)
			fail(w, 503, "SAVE_FAILED", "Изменения не сохранены. Повторите позже.")
		}
		return
	}
	w.Header().Set("ETag", fmt.Sprintf("%q", catalog.Revision(updated)))
	write(w, 200, map[string]any{"ok": true, "revision": catalog.Revision(updated), "catalog_count": len(updated.Vendors)})
}
