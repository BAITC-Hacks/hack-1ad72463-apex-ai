package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/BAITC-Hacks/hack-1ad72463-apex-ai/backend/internal/domain"
	"github.com/BAITC-Hacks/hack-1ad72463-apex-ai/backend/internal/i18n"
	"github.com/BAITC-Hacks/hack-1ad72463-apex-ai/backend/internal/matching"
)

type Source interface {
	Snapshot(context.Context) (domain.Snapshot, error)
}
type API struct {
	source    Source
	logger    *slog.Logger
	origins   []string
	mu        sync.Mutex
	counts    map[string]int
	latencies []float64
}
type errorBody struct {
	Error struct {
		Code    string              `json:"code"`
		Details []domain.FieldError `json:"details"`
	} `json:"error"`
}

func New(source Source, logger *slog.Logger, origins []string) http.Handler {
	a := &API{source: source, logger: logger, origins: origins, counts: map[string]int{}}
	return http.HandlerFunc(a.serve)
}
func write(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func fail(w http.ResponseWriter, status int, code string, details []domain.FieldError) {
	if details == nil {
		details = []domain.FieldError{}
	}
	b := errorBody{}
	b.Error.Code = code
	b.Error.Details = details
	write(w, status, b)
}

type recorder struct {
	http.ResponseWriter
	status int
}

func (w *recorder) WriteHeader(code int) { w.status = code; w.ResponseWriter.WriteHeader(code) }

func (a *API) serve(rw http.ResponseWriter, r *http.Request) {
	start := time.Now()
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	id := hex.EncodeToString(b)
	w := &recorder{ResponseWriter: rw, status: 200}
	w.Header().Set("X-Request-ID", id)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "no-store")
	locale := i18n.ParseAcceptLanguage(strings.Join(r.Header.Values("Accept-Language"), ","))
	w.Header().Set("Content-Language", string(locale))
	w.Header().Add("Vary", "Accept-Language")
	defer func() {
		if recover() != nil {
			a.logger.Error("request panic", "request_id", id)
			fail(w, 500, "INTERNAL_ERROR", nil)
		}
		elapsed := time.Since(start).Seconds()
		a.mu.Lock()
		a.counts[fmt.Sprint(w.status)]++
		a.latencies = append(a.latencies, elapsed)
		if len(a.latencies) > 1024 {
			a.latencies = a.latencies[1:]
		}
		a.mu.Unlock()
		a.logger.Info("http_request", "request_id", id, "method", r.Method, "status", w.status, "duration_ms", float64(time.Since(start).Microseconds())/1000)
	}()
	if origin := r.Header.Get("Origin"); origin != "" {
		w.Header().Add("Vary", "Origin")
		if slices.Contains(a.origins, origin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Expose-Headers", "X-Request-ID")
		}
	}
	routes := map[string]string{"/healthz": "GET", "/readyz": "GET", "/metrics": "GET", "/api/catalog/meta": "GET", "/api/recommendations": "POST", "/api/recommendations/alternatives": "POST"}
	method, ok := routes[r.URL.Path]
	if !ok {
		fail(w, 404, "NOT_FOUND", nil)
		return
	}
	if r.Method == http.MethodOptions {
		if !slices.Contains(a.origins, r.Header.Get("Origin")) {
			fail(w, 403, "ORIGIN_NOT_ALLOWED", nil)
			return
		}
		w.Header().Set("Access-Control-Allow-Methods", method+", OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.WriteHeader(204)
		return
	}
	if r.Method != method {
		w.Header().Set("Allow", method+", OPTIONS")
		fail(w, 405, "METHOD_NOT_ALLOWED", nil)
		return
	}
	if r.URL.Path == "/healthz" {
		write(w, 200, map[string]string{"status": "ok"})
		return
	}
	if r.URL.Path == "/metrics" {
		a.metrics(w)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	var req domain.Request
	validationStart := time.Now()
	if r.URL.Path == "/api/recommendations" || r.URL.Path == "/api/recommendations/alternatives" {
		media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || media != "application/json" {
			fail(w, 415, "UNSUPPORTED_MEDIA_TYPE", nil)
			return
		}
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 64*1024))
		if err != nil {
			var max *http.MaxBytesError
			if errors.As(err, &max) {
				fail(w, 413, "PAYLOAD_TOO_LARGE", nil)
			} else {
				fail(w, 400, "INVALID_JSON", nil)
			}
			return
		}
		var fields []domain.FieldError
		req, fields, err = decodeRequestLocalized(body, locale)
		if err != nil {
			fail(w, 400, "INVALID_JSON", nil)
			return
		}
		if len(fields) > 0 {
			fail(w, 422, "VALIDATION_ERROR", fields)
			return
		}
	}
	snapshot, err := a.source.Snapshot(ctx)
	if err != nil {
		fail(w, 503, "CATALOG_UNAVAILABLE", nil)
		return
	}
	switch r.URL.Path {
	case "/readyz":
		write(w, 200, map[string]any{"status": "ready", "catalog_count": len(snapshot.Vendors), "metadata": snapshot.Metadata})
	case "/api/catalog/meta":
		write(w, 200, catalogMeta(snapshot))
	case "/api/recommendations", "/api/recommendations/alternatives":
		if fields := matching.ValidateLocalized(req, snapshot.Metadata.CalendarWindow, locale); len(fields) > 0 {
			fail(w, 422, "VALIDATION_ERROR", fields)
			return
		}
		if r.URL.Path == "/api/recommendations/alternatives" {
			write(w, 200, matching.FindAlternatives(snapshot, req))
			return
		}
		validationMS := float64(time.Since(validationStart).Microseconds()) / 1000
		matchStart := time.Now()
		response := matching.RecommendLocalized(snapshot, req, locale)
		ids := []string{}
		for _, v := range response.Results {
			ids = append(ids, v.ID)
		}
		a.logger.Info("recommendation", "request_id", id, "versions", snapshot.Metadata, "business_status", response.Status, "diagnostics", response.Diagnostics, "selected_ids", ids, "validation_and_load_ms", validationMS, "matching_and_explanation_ms", float64(time.Since(matchStart).Microseconds())/1000, "explanation_source", "template")
		write(w, 200, response)
	}
}

func catalogMeta(s domain.Snapshot) any {
	cities, categories := map[string]bool{}, map[string]bool{}
	byCity := map[string][]string{}
	for _, v := range s.Vendors {
		cities[v.City] = true
		for _, c := range v.Categories {
			categories[c] = true
			if !slices.Contains(byCity[v.City], c) {
				byCity[v.City] = append(byCity[v.City], c)
			}
		}
	}
	keys := func(m map[string]bool) []string {
		out := []string{}
		for k := range m {
			out = append(out, k)
		}
		slices.Sort(out)
		return out
	}
	for city := range byCity {
		slices.Sort(byCity[city])
	}
	return map[string]any{"catalog_count": len(s.Vendors), "cities": keys(cities), "categories": keys(categories), "categories_by_city": byCity, "event_formats": domain.Formats, "languages": domain.Languages, "metadata": s.Metadata}
}

// Bounded rolling quantiles: operational observations, not a global SLO estimate.
func (a *API) metrics(w http.ResponseWriter) {
	a.mu.Lock()
	samples := slices.Clone(a.latencies)
	counts := map[string]int{}
	for k, v := range a.counts {
		counts[k] = v
	}
	a.mu.Unlock()
	slices.Sort(samples)
	p := func(q float64) float64 {
		if len(samples) == 0 {
			return 0
		}
		return samples[int(float64(len(samples)-1)*q)]
	}
	keys := []string{}
	for k := range counts {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	var b strings.Builder
	b.WriteString("# TYPE hackalem_http_requests_total counter\n")
	for _, k := range keys {
		fmt.Fprintf(&b, "hackalem_http_requests_total{status=%q} %d\n", k, counts[k])
	}
	b.WriteString("# HELP hackalem_http_duration_seconds Rolling quantiles of the last 1024 completed HTTP requests.\n# TYPE hackalem_http_duration_seconds gauge\n")
	fmt.Fprintf(&b, "hackalem_http_duration_seconds{quantile=\"0.5\"} %g\nhackalem_http_duration_seconds{quantile=\"0.95\"} %g\n", p(.5), p(.95))
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	w.WriteHeader(200)
	_, _ = io.WriteString(w, b.String())
}
