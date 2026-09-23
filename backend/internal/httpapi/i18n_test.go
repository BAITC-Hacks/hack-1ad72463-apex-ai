package httpapi

import (
	"encoding/json"
	"net/http/httptest"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/BAITC-Hacks/hack-1ad72463-apex-ai/backend/internal/domain"
	"github.com/BAITC-Hacks/hack-1ad72463-apex-ai/backend/internal/i18n"
)

func TestHTTPLocales(t *testing.T) {
	h := handler(t)
	baseline := call(h, "POST", "/api/recommendations", valid, "application/json")
	var original domain.Response
	if err := json.Unmarshal(baseline.Body.Bytes(), &original); err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct{ header, locale, prefix, explanation string }{
		{"", "ru", "Подходят", "Профиль поддерживает"}, {"ru", "ru", "Подходят", "Профиль поддерживает"}, {"ru-RU", "ru", "Подходят", "Профиль поддерживает"},
		{"kk", "kk", "Сәйкес мердігерлер саны:", "Профильде"}, {"kk-KZ", "kk", "Сәйкес мердігерлер саны:", "Профильде"},
		{"en", "en", "Matching contractors: 4", "Supports"}, {"en-US", "en", "Matching contractors: 4", "Supports"}, {"en-GB", "en", "Matching contractors: 4", "Supports"},
		{"de-DE", "ru", "Подходят", "Профиль поддерживает"}, {"de-DE, kk-KZ;q=0.8", "kk", "Сәйкес мердігерлер саны:", "Профильде"},
	} {
		t.Run(tt.header, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/api/recommendations", strings.NewReader(valid))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Accept-Language", tt.header)
			req.Header.Set("Origin", "http://localhost:3000")
			w := httptest.NewRecorder()
			h.ServeHTTP(w, req)
			if w.Code != 200 || w.Header().Get("Content-Language") != tt.locale {
				t.Fatalf("status/locale: %d %v", w.Code, w.Header())
			}
			vary := strings.Join(w.Header().Values("Vary"), ",")
			tokens := strings.Split(vary, ",")
			for i := range tokens {
				tokens[i] = strings.TrimSpace(tokens[i])
			}
			if !slices.Contains(tokens, "Origin") || !slices.Contains(tokens, "Accept-Language") || w.Header().Get("Access-Control-Allow-Origin") != "http://localhost:3000" {
				t.Fatalf("CORS/Vary: %v", w.Header())
			}
			var got domain.Response
			if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			if !strings.HasPrefix(got.Message, tt.prefix) {
				t.Fatal(got.Message)
			}
			if !reflect.DeepEqual(got.Diagnostics, original.Diagnostics) || !reflect.DeepEqual(got.Metadata, original.Metadata) || len(got.Results) != len(original.Results) {
				t.Fatal("locale changed matching")
			}
			for i, c := range got.Results {
				if c.ID != original.Results[i].ID || !reflect.DeepEqual(c.Conditions, original.Results[i].Conditions) {
					t.Fatal("locale changed results/conditions")
				}
				if !strings.HasPrefix(c.Explanation, tt.explanation) {
					t.Fatal(c.Explanation)
				}
			}
			if tt.locale == "ru" && w.Body.String() != baseline.Body.String() {
				t.Fatal("RU response changed")
			}
		})
	}
}

func TestLocalizedHTTPValidationAndAlternatives(t *testing.T) {
	h := handler(t)
	for _, path := range []string{"/api/recommendations", "/api/recommendations/alternatives"} {
		for _, l := range []i18n.Locale{i18n.LocaleRU, i18n.LocaleKK, i18n.LocaleEN} {
			for _, tt := range []struct {
				body, key string
				status    int
			}{
				{`{}`, "required", 422},
				{strings.Replace(valid, "1000000", "0", 1), "budget", 422},
				{strings.Replace(valid, `"city":`, `"extra":true,"city":`, 1), "unknown_field", 422},
				{strings.Replace(valid, `"city":`, `"city":"Астана","city":`, 1), "duplicate_field", 422},
				{strings.Replace(valid, "1000000", `"bad"`, 1), "invalid_type", 422},
				{`{"city":`, "", 400},
			} {
				r := httptest.NewRequest("POST", path, strings.NewReader(tt.body))
				r.Header.Set("Content-Type", "application/json")
				r.Header.Set("Accept-Language", string(l))
				w := httptest.NewRecorder()
				h.ServeHTTP(w, r)
				if w.Code != tt.status || w.Header().Get("Content-Language") != string(l) {
					t.Fatalf("%s: %d %s", path, w.Code, w.Body)
				}
				var out errorBody
				if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
					t.Fatal(err)
				}
				if tt.key != "" {
					found := false
					for _, d := range out.Error.Details {
						if d.Message == i18n.Message(l, tt.key) {
							found = true
						}
					}
					if !found {
						t.Fatalf("missing %s: %s", tt.key, w.Body)
					}
				} else if out.Error.Code != "INVALID_JSON" {
					t.Fatal(w.Body)
				}
			}
		}
	}
	body := strings.Replace(valid, "1000000", "100000", 1)
	expected := call(h, "POST", "/api/recommendations/alternatives", body, "application/json").Body.String()
	for _, locale := range []string{"ru", "kk", "en"} {
		r := httptest.NewRequest("POST", "/api/recommendations/alternatives", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Accept-Language", locale)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 200 || w.Body.String() != expected || w.Header().Get("Content-Language") != locale {
			t.Fatalf("alternatives changed: %s", w.Body)
		}
	}
	// UI locale in JSON remains an unknown field, and contractor language stays canonical.
	for _, body := range []string{strings.Replace(valid, `"city":`, `"locale":"en","city":`, 1), strings.Replace(valid, "русский", "en", 1)} {
		if w := call(h, "POST", "/api/recommendations", body, "application/json"); w.Code != 422 {
			t.Fatal("request contract changed")
		}
	}
}
