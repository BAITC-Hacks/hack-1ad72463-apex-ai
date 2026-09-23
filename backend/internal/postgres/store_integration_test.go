package postgres_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/BAITC-Hacks/hack-1ad72463-apex-ai/backend/internal/catalog"
	"github.com/BAITC-Hacks/hack-1ad72463-apex-ai/backend/internal/domain"
	"github.com/BAITC-Hacks/hack-1ad72463-apex-ai/backend/internal/httpapi"
	"github.com/BAITC-Hacks/hack-1ad72463-apex-ai/backend/internal/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPostgresIntegration(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set TEST_DATABASE_URL to run real PostgreSQL tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	admin, e := pgxpool.New(ctx, url)
	if e != nil {
		t.Fatal(e)
	}
	defer admin.Close()
	schema := fmt.Sprintf("hackalem_test_%d", time.Now().UnixNano())
	if _, e = admin.Exec(ctx, "CREATE SCHEMA "+schema); e != nil {
		t.Fatal(e)
	}
	defer func() {
		if _, e := admin.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE"); e != nil {
			t.Error(e)
		}
	}()
	cfg, e := pgxpool.ParseConfig(url)
	if e != nil {
		t.Fatal(e)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, e := pgxpool.NewWithConfig(ctx, cfg)
	if e != nil {
		t.Fatal(e)
	}
	store := &postgres.Store{Pool: pool}
	defer store.Close()
	for i := 0; i < 2; i++ {
		if e = store.Migrate(ctx); e != nil {
			t.Fatal(e)
		}
	}
	if _, e = store.Snapshot(ctx); e == nil {
		t.Fatal("empty DB must not be a NO_CATALOG")
	}
	s, e := catalog.Load("../../data/catalog.csv", "../../data/catalog.meta.json", "../../data/facts.json")
	if e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 2; i++ {
		if e = store.Replace(ctx, s); e != nil {
			t.Fatal(e)
		}
	}
	got, e := store.Snapshot(ctx)
	if e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(got, s) {
		t.Fatal("PostgreSQL round trip changed catalog")
	}
	bad := s
	bad.Vendors = append([]domain.Vendor{}, s.Vendors...)
	bad.Vendors[0].Price = -1
	if e = store.Replace(ctx, bad); e == nil {
		t.Fatal("invalid catalog accepted")
	}
	got, e = store.Snapshot(ctx)
	if e != nil || !reflect.DeepEqual(got, s) {
		t.Fatal("rejected import modified active catalog")
	}
	// An error AFTER DELETE and during inserts must roll back the entire transaction.
	_, e = pool.Exec(ctx, `CREATE FUNCTION reject_import() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.id='HK-44923' THEN RAISE EXCEPTION 'test insert failure'; END IF; RETURN NEW; END $$; CREATE TRIGGER reject_import BEFORE INSERT ON vendors FOR EACH ROW EXECUTE FUNCTION reject_import()`)
	if e != nil {
		t.Fatal(e)
	}
	if e = store.Replace(ctx, s); e == nil {
		t.Fatal("expected database failure")
	}
	got, e = store.Snapshot(ctx)
	if e != nil || !reflect.DeepEqual(got, s) {
		t.Fatal("failed transaction damaged snapshot")
	}
	if _, e = pool.Exec(ctx, "DROP TRIGGER reject_import ON vendors; DROP FUNCTION reject_import()"); e != nil {
		t.Fatal(e)
	}
	server := httptest.NewServer(httpapi.New(store, slog.New(slog.NewTextHandler(io.Discard, nil)), nil))
	defer server.Close()
	body := `{"city":"Алматы","category":"Ведущий","date":"2026-11-14","event_format":"корпоратив","budget_kzt":1000000,"language":"русский","duration_hours":6}`
	request := func() error {
		resp, e := http.Post(server.URL+"/api/recommendations", "application/json", strings.NewReader(body))
		if e != nil {
			return e
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			return fmt.Errorf("HTTP %d", resp.StatusCode)
		}
		var out domain.Response
		if e = json.NewDecoder(resp.Body).Decode(&out); e != nil {
			return e
		}
		if len(out.Results) != 3 || out.Results[0].ID != "HK-44923" || out.Results[1].ID != "HK-29829" || out.Results[2].ID != "HK-27222" || out.Diagnostics.EligibleCount != 4 {
			return fmt.Errorf("unexpected E2E response: %+v", out)
		}
		return nil
	}
	if e = request(); e != nil {
		t.Fatal(e)
	}
	for _, tt := range []struct{ body, want string }{
		{strings.Replace(body, "1000000", "100000", 1), `{"alternatives":[{"type":"BUDGET","current_budget_kzt":100000,"suggested_budget_kzt":650000,"eligible_count":1}]}`},
		{strings.Replace(strings.Replace(body, "1000000", "650000", 1), "2026-11-14", "2026-09-24", 1), `{"alternatives":[{"type":"BUDGET","current_budget_kzt":650000,"suggested_budget_kzt":900000,"eligible_count":1},{"type":"DATE","current_date":"2026-09-24","suggested_date":"2026-09-25","distance_days":1,"eligible_count":1}]}`},
	} {
		resp, err := http.Post(server.URL+"/api/recommendations/alternatives", "application/json", strings.NewReader(tt.body))
		if err != nil {
			t.Fatal(err)
		}
		got, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil || resp.StatusCode != 200 || strings.TrimSpace(string(got)) != tt.want {
			t.Fatalf("imported catalog alternatives: HTTP %d %s (%v)", resp.StatusCode, got, err)
		}
	}
	var wg sync.WaitGroup
	errs := make(chan error, 5)
	for worker := 0; worker < 4; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for n := 0; n < 15; n++ {
				if e := request(); e != nil {
					errs <- e
					return
				}
			}
		}()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for n := 0; n < 5; n++ {
			if e := store.Replace(ctx, s); e != nil {
				errs <- e
				return
			}
		}
	}()
	wg.Wait()
	close(errs)
	for e := range errs {
		t.Error(e)
	}
	if _, e = pool.Exec(ctx, "UPDATE vendors SET price_from_kzt=price_from_kzt+1 WHERE id='HK-44923'"); e != nil {
		t.Fatal(e)
	}
	if _, e = store.Snapshot(ctx); e == nil {
		t.Fatal("tampered data must fail integrity check")
	}
	resp, e := http.Get(server.URL + "/readyz")
	if e != nil {
		t.Fatal(e)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 503 {
		t.Fatal("corrupt catalog reported ready")
	}
	if e = store.Replace(ctx, s); e != nil {
		t.Fatal(e)
	}
	if e = request(); e != nil {
		t.Fatal(e)
	}
	store.Close()
	alternativeResp, err := http.Post(server.URL+"/api/recommendations/alternatives", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	outageBody, err := io.ReadAll(alternativeResp.Body)
	alternativeResp.Body.Close()
	if err != nil || alternativeResp.StatusCode != 503 || strings.TrimSpace(string(outageBody)) != `{"error":{"code":"CATALOG_UNAVAILABLE","details":[]}}` {
		t.Fatalf("alternatives database outage: HTTP %d %s (%v)", alternativeResp.StatusCode, outageBody, err)
	}
	resp, e = http.Get(server.URL + "/readyz")
	if e != nil {
		t.Fatal(e)
	}
	resp.Body.Close()
	if resp.StatusCode != 503 {
		t.Fatal("database outage not reported")
	}
}
