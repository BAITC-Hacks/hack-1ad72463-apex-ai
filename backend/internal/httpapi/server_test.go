package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/BAITC-Hacks/hack-1ad72463-apex-ai/backend/internal/catalog"
	"github.com/BAITC-Hacks/hack-1ad72463-apex-ai/backend/internal/domain"
)

type fakeSource struct {
	snapshot domain.Snapshot
	err      error
}

func (s fakeSource) Snapshot(context.Context) (domain.Snapshot, error) { return s.snapshot, s.err }

const valid = `{"city":"Алматы","category":"Ведущий","date":"2026-11-14","event_format":"корпоратив","budget_kzt":1000000,"duration_hours":6,"language":"русский"}`

func handler(t *testing.T) http.Handler {
	t.Helper()
	s, e := catalog.Load("../../data/catalog.csv", "../../data/catalog.meta.json", "../../data/facts.json")
	if e != nil {
		t.Fatal(e)
	}
	return New(fakeSource{snapshot: s}, slog.New(slog.NewTextHandler(io.Discard, nil)), []string{"http://localhost:3000"})
}
func call(h http.Handler, method, path, body, media string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.Header.Set("Content-Type", media)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestHTTPContract(t *testing.T) {
	h := handler(t)
	tests := []struct {
		name, body  string
		status      int
		code, field string
	}{
		{"success", valid, 200, "", ""},
		{"null optionals", strings.ReplaceAll(strings.ReplaceAll(valid, `"duration_hours":6`, `"duration_hours":null`), `"language":"русский"`, `"language":null`), 200, "", ""},
		{"unknown city", strings.Replace(valid, "Алматы", "Несуществующий", 1), 200, "", ""},
		{"null required", strings.Replace(valid, `"budget_kzt":1000000`, `"budget_kzt":null`, 1), 422, "REQUIRED", "budget_kzt"},
		{"missing required", `{"date":"2026-11-14"}`, 422, "REQUIRED", "city"},
		{"unknown field", strings.Replace(valid, `"city":`, `"extra":true,"city":`, 1), 422, "UNKNOWN_FIELD", "extra"},
		{"duplicate field", strings.Replace(valid, `"city":`, `"city":"Астана","city":`, 1), 422, "INVALID_TYPE", "city"},
		{"fractional budget", strings.Replace(valid, `1000000`, `1000000.0`, 1), 422, "INVALID_TYPE", "budget_kzt"},
		{"exponent budget", strings.Replace(valid, `1000000`, `1e6`, 1), 422, "INVALID_TYPE", "budget_kzt"},
		{"string budget", strings.Replace(valid, `1000000`, `"1000000"`, 1), 422, "INVALID_TYPE", "budget_kzt"},
		{"boolean city", strings.Replace(valid, `"Алматы"`, `true`, 1), 422, "INVALID_TYPE", "city"},
		{"zero budget", strings.Replace(valid, `1000000`, `0`, 1), 422, "OUT_OF_RANGE", "budget_kzt"},
		{"zero duration", strings.Replace(valid, `"duration_hours":6`, `"duration_hours":0`, 1), 422, "OUT_OF_RANGE", "duration_hours"},
		{"overflow duration", strings.Replace(valid, `"duration_hours":6`, `"duration_hours":1e999`, 1), 422, "INVALID_TYPE", "duration_hours"},
		{"blank language", strings.Replace(valid, `"русский"`, `" "`, 1), 422, "INVALID_ENUM", "language"},
		{"unknown format", strings.Replace(valid, `"корпоратив"`, `"пикник"`, 1), 422, "INVALID_ENUM", "event_format"},
		{"blank city", strings.Replace(valid, `"Алматы"`, `"  "`, 1), 422, "REQUIRED", "city"},
		{"invalid date", strings.Replace(valid, `2026-11-14`, `2026-11-31`, 1), 422, "INVALID_DATE", "date"},
		{"date before", strings.Replace(valid, `2026-11-14`, `2026-09-22`, 1), 422, "DATE_OUT_OF_RANGE", "date"},
		{"date after", strings.Replace(valid, `2026-11-14`, `2027-01-01`, 1), 422, "DATE_OUT_OF_RANGE", "date"},
		{"invalid json", `{"city":`, 400, "", ""}, {"array", `[]`, 400, "", ""}, {"trailing json", valid + `{}`, 400, "", ""}, {"empty body", "", 400, "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := call(h, "POST", "/api/recommendations", tt.body, "application/json")
			if w.Code != tt.status {
				t.Fatalf("status %d: %s", w.Code, w.Body)
			}
			if w.Header().Get("X-Request-ID") == "" {
				t.Fatal("missing request id")
			}
			if tt.status != 200 {
				var e errorBody
				if err := json.Unmarshal(w.Body.Bytes(), &e); err != nil {
					t.Fatal(err)
				}
				if tt.status == 422 {
					found := false
					for _, d := range e.Error.Details {
						if d.Code == tt.code && d.Field == tt.field {
							found = true
						}
					}
					if !found {
						t.Fatalf("missing field error: %s", w.Body)
					}
				}
				if strings.Contains(w.Body.String(), `"status"`) {
					t.Fatal("technical error must not be a business result")
				}
			}
		})
	}
}

func TestHTTPUtilitiesAndLimits(t *testing.T) {
	h := handler(t)
	for _, tt := range []struct {
		method, path, body, media string
		status                    int
	}{
		{"GET", "/healthz", "", "", 200}, {"GET", "/readyz", "", "", 200}, {"GET", "/api/catalog/meta", "", "", 200}, {"GET", "/metrics", "", "", 200},
		{"GET", "/missing", "", "", 404}, {"GET", "/api/recommendations", "", "", 405}, {"POST", "/api/recommendations", valid, "text/plain", 415}, {"POST", "/api/recommendations", strings.Repeat("x", 65537), "application/json", 413},
	} {
		w := call(h, tt.method, tt.path, tt.body, tt.media)
		if w.Code != tt.status {
			t.Fatalf("%s %s: %d %s", tt.method, tt.path, w.Code, w.Body)
		}
	}
	w := call(h, "GET", "/api/catalog/meta", "", "")
	var meta struct {
		Count      int      `json:"catalog_count"`
		Categories []string `json:"categories"`
	}
	if json.Unmarshal(w.Body.Bytes(), &meta) != nil || meta.Count != 66 || len(meta.Categories) != 17 {
		t.Fatalf("bad metadata: %s", w.Body)
	}
	for _, origin := range []string{"http://localhost:3000", "https://untrusted.example"} {
		r := httptest.NewRequest("OPTIONS", "/api/recommendations", nil)
		r.Header.Set("Origin", origin)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if origin == "http://localhost:3000" {
			if w.Code != 204 || w.Header().Get("Access-Control-Allow-Origin") != origin {
				t.Fatal("preflight failed")
			}
		} else if w.Code != 403 || w.Header().Get("Access-Control-Allow-Origin") != "" {
			t.Fatal("untrusted origin allowed")
		}
	}
}

func TestUnavailableCatalog(t *testing.T) {
	h := New(fakeSource{err: errors.New("secret /local/path password")}, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
	for _, p := range []string{"/readyz", "/api/catalog/meta", "/api/recommendations"} {
		method, body := "GET", ""
		if p == "/api/recommendations" {
			method, body = "POST", valid
		}
		w := call(h, method, p, body, "application/json")
		if w.Code != 503 || !strings.Contains(w.Body.String(), "CATALOG_UNAVAILABLE") || strings.Contains(w.Body.String(), "secret") {
			t.Fatalf("invalid unavailable response: %s", w.Body)
		}
	}
	if w := call(h, "GET", "/healthz", "", ""); w.Code != 200 {
		t.Fatal("liveness should be independent of database")
	}
}
