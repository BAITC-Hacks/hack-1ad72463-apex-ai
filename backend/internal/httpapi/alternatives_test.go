package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/BAITC-Hacks/hack-1ad72463-apex-ai/backend/internal/catalog"
	"github.com/BAITC-Hacks/hack-1ad72463-apex-ai/backend/internal/domain"
)

const alternativesPath = "/api/recommendations/alternatives"

type countedSource struct {
	fakeSource
	calls int
}

func (s *countedSource) Snapshot(ctx context.Context) (domain.Snapshot, error) {
	s.calls++
	return s.fakeSource.Snapshot(ctx)
}

func TestAlternativesHTTP(t *testing.T) {
	s, err := catalog.Load("../../data/catalog.csv", "../../data/catalog.meta.json", "../../data/facts.json")
	if err != nil {
		t.Fatal(err)
	}
	source := &countedSource{fakeSource: fakeSource{snapshot: s}}
	h := New(source, slog.New(slog.NewTextHandler(io.Discard, nil)), []string{"http://localhost:3000"})
	for _, tt := range []struct{ name, body, want string }{
		{"matches", valid, `{"alternatives":[]}`},
		{"no catalog", strings.Replace(valid, "Алматы", "missing", 1), `{"alternatives":[]}`},
		{"budget", strings.Replace(valid, "1000000", "100000", 1), `{"alternatives":[{"type":"BUDGET","current_budget_kzt":100000,"suggested_budget_kzt":650000,"eligible_count":1}]}`},
		{"date", strings.Replace(strings.Replace(valid, "1000000", "650000", 1), "2026-11-14", "2026-09-24", 1), `{"alternatives":[{"type":"BUDGET","current_budget_kzt":650000,"suggested_budget_kzt":900000,"eligible_count":1},{"type":"DATE","current_date":"2026-09-24","suggested_date":"2026-09-25","distance_days":1,"eligible_count":1}]}`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			before := source.calls
			w := call(h, "POST", alternativesPath, tt.body, "application/json")
			if w.Code != 200 || strings.TrimSpace(w.Body.String()) != tt.want {
				t.Fatalf("%d %s", w.Code, w.Body)
			}
			if source.calls-before != 1 {
				t.Fatal("must load exactly one snapshot per request")
			}
		})
	}
	for _, tt := range []struct {
		body, media string
		status      int
	}{
		{`{"city":`, "application/json", 400},
		{valid + `{}`, "application/json", 400},
		{`{}`, "application/json", 422},
		{strings.Replace(valid, "1000000", "0", 1), "application/json", 422},
		{strings.Replace(valid, "1000000", "1e6", 1), "application/json", 422},
		{strings.Replace(valid, "2026-11-14", "2027-01-01", 1), "application/json", 422},
		{strings.Replace(valid, `"city":`, `"city":"Астана","city":`, 1), "application/json", 422},
		{strings.Replace(valid, "русский", "missing", 1), "application/json", 422},
		{valid, "text/plain", 415},
		{strings.Repeat("x", 65537), "application/json", 413},
	} {
		w := call(h, "POST", alternativesPath, tt.body, tt.media)
		original := call(h, "POST", "/api/recommendations", tt.body, tt.media)
		if w.Code != tt.status || w.Body.String() != original.Body.String() {
			t.Fatalf("error semantics differ: %d %s vs %d %s", w.Code, w.Body, original.Code, original.Body)
		}
	}
	source.err = errors.New("private details")
	w := call(h, "POST", alternativesPath, valid, "application/json")
	if w.Code != 503 || strings.TrimSpace(w.Body.String()) != `{"error":{"code":"CATALOG_UNAVAILABLE","details":[]}}` {
		t.Fatalf("unavailable: %d %s", w.Code, w.Body)
	}
}

func TestAlternativesRoutingAndCORS(t *testing.T) {
	h := handler(t)
	w := call(h, "GET", alternativesPath, "", "")
	if w.Code != 405 || w.Header().Get("Allow") != "POST, OPTIONS" {
		t.Fatal("method guard")
	}
	for _, origin := range []string{"http://localhost:3000", "https://untrusted.example"} {
		r := httptest.NewRequest("OPTIONS", alternativesPath, nil)
		r.Header.Set("Origin", origin)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if origin == "http://localhost:3000" {
			if w.Code != 204 || w.Header().Get("Access-Control-Allow-Origin") != origin || w.Header().Get("Access-Control-Allow-Methods") != "POST, OPTIONS" {
				t.Fatal("local preflight broken")
			}
		} else if w.Code != 403 {
			t.Fatal("untrusted origin allowed")
		}
	}
	r := httptest.NewRequest("POST", "https://testrr.shop"+alternativesPath, strings.NewReader(valid))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Origin", "https://testrr.shop")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatal("same-origin request failed")
	}
	// Explicitly protect the existing endpoint's top-level schema.
	w = call(h, "POST", "/api/recommendations", valid, "application/json")
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(w.Body.Bytes(), &fields); err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"status": true, "message": true, "results": true, "diagnostics": true, "metadata": true}
	got := map[string]bool{}
	for k := range fields {
		got[k] = true
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("existing schema changed: %v", got)
	}
}
