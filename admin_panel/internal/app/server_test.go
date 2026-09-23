package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	catalog "github.com/BAITC-Hacks/hack-1ad72463-apex-ai/backend/catalogadmin"
	"github.com/jackc/pgx/v5/pgxpool"
)

const initialPassword = "Initial-admin-test-2026"
const changedPassword = "Changed-admin-test-2026"

type fixture struct {
	app      *App
	pool     *pgxpool.Pool
	catalog  *catalog.Service
	original catalog.Snapshot
	cookie   *http.Cookie
	user     Identity
}

func setup(t *testing.T) *fixture {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL required for PostgreSQL integration")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	schema := fmt.Sprintf("admin_panel_test_%d", time.Now().UnixNano())
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pool.Close()
		_, err := admin.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE")
		if err != nil {
			t.Error(err)
		}
		admin.Close()
	})
	fixtureDir := os.Getenv("TEST_FIXTURE_DIR")
	if fixtureDir == "" {
		fixtureDir = "../../../backend/data"
	}
	original, err := catalog.Load(filepath.Join(fixtureDir, "catalog.csv"), filepath.Join(fixtureDir, "catalog.meta.json"), filepath.Join(fixtureDir, "facts.json"))
	if err != nil {
		t.Fatal(err)
	}
	service := catalog.New(pool)
	if err = service.Bootstrap(ctx, original); err != nil {
		t.Fatal(err)
	}
	password, created, err := Initialize(ctx, pool, initialPassword)
	if err != nil || !created || password != initialPassword {
		t.Fatalf("bootstrap: %t %v", created, err)
	}
	again, created, err := Initialize(ctx, pool, "ignored-different-password")
	if err != nil || created || again != "" {
		t.Fatal("bootstrap reset an existing account")
	}
	return &fixture{pool: pool, catalog: service, original: original, app: New(pool, Config{Origin: "https://admin.example", SecureCookie: true}, slog.New(slog.NewTextHandler(io.Discard, nil)))}
}
func (f *fixture) call(method, path string, data any, revision string, csrf bool, origin bool) *httptest.ResponseRecorder {
	var body io.Reader
	if data != nil {
		b, _ := json.Marshal(data)
		body = bytes.NewReader(b)
	}
	r := httptest.NewRequest(method, path, body)
	r.RemoteAddr = "127.0.0.1:1234"
	r.Header.Set("Content-Type", "application/json")
	if origin {
		r.Header.Set("Origin", f.app.config.Origin)
	}
	if f.cookie != nil {
		r.AddCookie(f.cookie)
	}
	if csrf {
		r.Header.Set("X-CSRF-Token", f.user.CSRF)
	}
	if revision != "" {
		r.Header.Set("If-Match", `"`+revision+`"`)
	}
	w := httptest.NewRecorder()
	f.app.ServeHTTP(w, r)
	return w
}
func requireStatus(t *testing.T, w *httptest.ResponseRecorder, code int) {
	t.Helper()
	if w.Code != code {
		t.Fatalf("HTTP %d instead of %d: %s", w.Code, code, w.Body)
	}
}
func (f *fixture) signin(t *testing.T, password string) {
	t.Helper()
	w := f.call("POST", "/admin/api/login", map[string]string{"username": "admin", "password": password}, "", false, true)
	requireStatus(t, w, 200)
	if err := json.Unmarshal(w.Body.Bytes(), &f.user); err != nil {
		t.Fatal(err)
	}
	f.cookie = w.Result().Cookies()[0]
	if !f.cookie.HttpOnly || !f.cookie.Secure || f.cookie.SameSite != http.SameSiteStrictMode || f.cookie.Path != "/admin" {
		t.Fatal("unsafe session cookie")
	}
}
func (f *fixture) readyUser(t *testing.T) {
	f.signin(t, initialPassword)
	w := f.call("POST", "/admin/api/password", map[string]string{"current_password": initialPassword, "new_password": changedPassword}, "", true, true)
	requireStatus(t, w, 200)
	f.signin(t, changedPassword)
}
func (f *fixture) snapshot(t *testing.T) catalog.Snapshot {
	t.Helper()
	s, e := f.catalog.Snapshot(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	return s
}
func (f *fixture) upload(path string, raw []byte, revision string, upsert bool) *httptest.ResponseRecorder {
	var b bytes.Buffer
	writer := multipart.NewWriter(&b)
	part, _ := writer.CreateFormFile("file", "catalog.csv")
	_, _ = part.Write(raw)
	_ = writer.WriteField("update_existing", fmt.Sprint(upsert))
	_ = writer.Close()
	r := httptest.NewRequest("POST", path, &b)
	r.Header.Set("Content-Type", writer.FormDataContentType())
	r.Header.Set("Origin", f.app.config.Origin)
	r.Header.Set("X-CSRF-Token", f.user.CSRF)
	if revision != "" {
		r.Header.Set("If-Match", revision)
	}
	r.AddCookie(f.cookie)
	w := httptest.NewRecorder()
	f.app.ServeHTTP(w, r)
	return w
}

func TestAuthenticationAndSessionLifecycle(t *testing.T) {
	f := setup(t)
	requireStatus(t, f.call("GET", "/admin/", nil, "", false, false), 200)
	requireStatus(t, f.call("GET", "/admin/api/vendors", nil, "", false, false), 401)
	requireStatus(t, f.call("POST", "/admin/api/login", map[string]string{"username": "admin", "password": initialPassword}, "", false, false), 403)
	requireStatus(t, f.call("POST", "/admin/api/login", map[string]string{"username": "admin", "password": "wrong"}, "", false, true), 401)
	requireStatus(t, f.call("POST", "/admin/api/login", map[string]string{"username": "nobody", "password": "wrong"}, "", false, true), 401)
	f.signin(t, initialPassword)
	if !f.user.MustChange {
		t.Fatal("must require password change")
	}
	requireStatus(t, f.call("GET", "/admin/api/vendors", nil, "", false, false), 403)
	requireStatus(t, f.call("GET", "/admin/api/session", nil, "", false, false), 200)
	requireStatus(t, f.call("POST", "/admin/api/password", map[string]string{"current_password": initialPassword, "new_password": changedPassword}, "", false, true), 403)
	requireStatus(t, f.call("POST", "/admin/api/password", map[string]string{"current_password": initialPassword, "new_password": "short"}, "", true, true), 422)
	requireStatus(t, f.call("POST", "/admin/api/password", map[string]string{"current_password": initialPassword, "new_password": changedPassword}, "", true, true), 200)
	requireStatus(t, f.call("GET", "/admin/api/session", nil, "", false, false), 401)
	f.signin(t, changedPassword)
	if f.user.MustChange {
		t.Fatal("password state not updated")
	}
	requireStatus(t, f.call("GET", "/admin/api/vendors", nil, "", false, false), 200)
	requireStatus(t, f.call("POST", "/admin/api/logout", nil, "", true, true), 200)
	requireStatus(t, f.call("GET", "/admin/api/vendors", nil, "", false, false), 401)
	f.signin(t, changedPassword)
	if _, err := f.pool.Exec(context.Background(), "UPDATE admin_panel_sessions SET expires_at=now()-interval '1 second'"); err != nil {
		t.Fatal(err)
	}
	requireStatus(t, f.call("GET", "/admin/api/session", nil, "", false, false), 401)
	var hash string
	_ = f.pool.QueryRow(context.Background(), "SELECT password_hash FROM admin_panel_users WHERE username='admin'").Scan(&hash)
	if !strings.HasPrefix(hash, "$2a$") || strings.Contains(hash, changedPassword) {
		t.Fatal("password not stored as bcrypt")
	}
}

func TestCatalogCRUDImportAndAudit(t *testing.T) {
	f := setup(t)
	f.readyUser(t)
	revision := catalog.Revision(f.original)
	v := f.original.Vendors[0]
	v.ID = "ADMIN-TEST-1"
	v.Name = "Тестовый подрядчик"
	v.Synthetic = true
	requireStatus(t, f.call("POST", "/admin/api/vendors", v, revision, false, true), 403)
	requireStatus(t, f.call("POST", "/admin/api/vendors", v, revision, true, false), 403)
	requireStatus(t, f.call("POST", "/admin/api/vendors", v, "", true, true), 428)
	requireStatus(t, f.call("POST", "/admin/api/vendors", v, revision, true, true), 200)
	s := f.snapshot(t)
	if len(s.Vendors) != 67 {
		t.Fatal("create did not persist")
	}
	requireStatus(t, f.call("POST", "/admin/api/vendors", v, revision, true, true), 409)
	requireStatus(t, f.call("POST", "/admin/api/vendors", v, catalog.Revision(s), true, true), 409)
	requireStatus(t, f.call("PUT", "/admin/api/vendors/not-the-id", v, catalog.Revision(s), true, true), 422)
	v.Price = -1
	requireStatus(t, f.call("PUT", "/admin/api/vendors/ADMIN-TEST-1", v, catalog.Revision(s), true, true), 422)
	v.Price = 450000
	requireStatus(t, f.call("PUT", "/admin/api/vendors/ADMIN-TEST-1", v, catalog.Revision(s), true, true), 200)
	s = f.snapshot(t)
	edited := f.original.Vendors[0]
	edited.Description = "Новое описание с другими условиями"
	requireStatus(t, f.call("PUT", "/admin/api/vendors/"+edited.ID, edited, catalog.Revision(s), true, true), 200)
	s = f.snapshot(t)
	for _, fact := range s.Facts {
		if fact.VendorID == edited.ID {
			t.Fatal("stale evidence retained")
		}
	}
	revision = catalog.Revision(s)
	newVendor := v
	newVendor.ID = "ADMIN-CSV-2"
	csvData := catalog.CSV([]catalog.Vendor{newVendor, v})
	preview := f.upload("/admin/api/import/preview", csvData, "", false)
	requireStatus(t, preview, 200)
	var p struct {
		New      int `json:"new_count"`
		Existing int `json:"existing_count"`
	}
	_ = json.Unmarshal(preview.Body.Bytes(), &p)
	if p.New != 1 || p.Existing != 1 {
		t.Fatalf("bad preview %s", preview.Body)
	}
	if catalog.Revision(f.snapshot(t)) != revision {
		t.Fatal("preview modified catalog")
	}
	requireStatus(t, f.upload("/admin/api/import", csvData, revision, false), 409)
	if catalog.Revision(f.snapshot(t)) != revision {
		t.Fatal("duplicate import partially saved")
	}
	requireStatus(t, f.upload("/admin/api/import", csvData, revision, true), 200)
	s = f.snapshot(t)
	if len(s.Vendors) != 68 {
		t.Fatal("wrong count after merge")
	}
	malformed := []byte("id,wrong\n123,foo\n")
	requireStatus(t, f.upload("/admin/api/import", malformed, catalog.Revision(s), true), 422)
	if catalog.Revision(f.snapshot(t)) != catalog.Revision(s) {
		t.Fatal("invalid CSV changed catalog")
	}
	exported := f.call("GET", "/admin/api/export", nil, "", false, false)
	requireStatus(t, exported, 200)
	parsed, err := catalog.ParseCSV(exported.Body.Bytes(), s.Metadata.CalendarWindow)
	if err != nil || len(parsed.Vendors) != 68 {
		t.Fatalf("bad export %v", err)
	}
	requireStatus(t, f.call("GET", "/admin/api/template", nil, "", false, false), 200)
	requireStatus(t, f.call("DELETE", "/admin/api/vendors/ADMIN-TEST-1", nil, catalog.Revision(s), true, true), 200)
	s = f.snapshot(t)
	requireStatus(t, f.call("DELETE", "/admin/api/vendors/ADMIN-TEST-1", nil, catalog.Revision(s), true, true), 404)
	audit := f.call("GET", "/admin/api/audit", nil, "", false, false)
	requireStatus(t, audit, 200)
	var events []map[string]any
	_ = json.Unmarshal(audit.Body.Bytes(), &events)
	if len(events) != 6 {
		t.Fatalf("unexpected audit events %s", audit.Body)
	}
	// A storage failure must roll back both catalog and audit changes.
	_, err = f.pool.Exec(context.Background(), `CREATE FUNCTION reject_admin_write() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'test failure'; END $$; CREATE TRIGGER reject_admin_write BEFORE INSERT ON vendors FOR EACH ROW EXECUTE FUNCTION reject_admin_write()`)
	if err != nil {
		t.Fatal(err)
	}
	newVendor.ID = "ADMIN-ROLLBACK"
	requireStatus(t, f.call("POST", "/admin/api/vendors", newVendor, catalog.Revision(s), true, true), 503)
	if catalog.Revision(f.snapshot(t)) != catalog.Revision(s) {
		t.Fatal("storage error changed catalog")
	}
	var count int
	_ = f.pool.QueryRow(context.Background(), "SELECT count(*) FROM admin_panel_audit").Scan(&count)
	if count != 6 {
		t.Fatal("failed mutation wrote audit")
	}
}

func TestConcurrentMutationsAndLastRecord(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	expected := catalog.Revision(f.original)
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			v := f.original.Vendors[0]
			v.ID = fmt.Sprintf("CONCURRENT-%d", i)
			_, err := f.catalog.Apply(ctx, catalog.Change{Action: "create", Actor: "admin", Expected: expected, Vendors: []catalog.Vendor{v}})
			errs <- err
		}(i)
	}
	wg.Wait()
	close(errs)
	passed, conflict := 0, 0
	for err := range errs {
		if err == nil {
			passed++
		} else if errors.Is(err, catalog.ErrConflict) {
			conflict++
		} else {
			t.Fatal(err)
		}
	}
	if passed != 1 || conflict != 1 || len(f.snapshot(t).Vendors) != 67 {
		t.Fatal("concurrent write was lost")
	}
	one := f.original
	one.Vendors = one.Vendors[:1]
	one.Facts = []catalog.Fact{}
	if err := f.catalog.Bootstrap(ctx, one); err != nil {
		t.Fatal(err)
	}
	s := f.snapshot(t)
	_, err := f.catalog.Apply(ctx, catalog.Change{Action: "delete", Actor: "admin", Expected: catalog.Revision(s), ID: s.Vendors[0].ID})
	if !errors.Is(err, catalog.ErrLastVendor) {
		t.Fatal("last record deletion should preserve readiness")
	}
}

func TestRateLimitAndProxy(t *testing.T) {
	a := &App{attempts: map[string]attempt{}}
	for i := 0; i < 10; i++ {
		if a.limited("client") {
			t.Fatal("limited early")
		}
	}
	if !a.limited("client") {
		t.Fatal("unlimited guesses")
	}
	a.attempts["client"] = attempt{Count: 10, Until: time.Now().Add(-time.Second)}
	if a.limited("client") {
		t.Fatal("limit did not expire")
	}
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "203.0.113.5:50"
	r.Header.Set("X-Real-IP", "1.2.3.4")
	a.config.TrustProxy = true
	if a.clientIP(r) != "203.0.113.5" {
		t.Fatal("untrusted proxy accepted")
	}
	r.RemoteAddr = "127.0.0.1:50"
	if a.clientIP(r) != "1.2.3.4" {
		t.Fatal("trusted proxy not recognized")
	}
}
