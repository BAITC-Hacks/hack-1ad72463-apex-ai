package matching

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/BAITC-Hacks/hack-1ad72463-apex-ai/backend/internal/domain"
	"github.com/BAITC-Hacks/hack-1ad72463-apex-ai/backend/internal/i18n"
)

func withoutLocalizedText(r domain.Response) domain.Response {
	r.Message = ""
	for i := range r.Results {
		r.Results[i].Explanation = ""
	}
	return r
}

func TestLocalePreservesMatching(t *testing.T) {
	s := data(t)
	cases := []domain.Request{base()}
	for _, modify := range []func(*domain.Request){func(r *domain.Request) { r.Budget = 100000 }, func(r *domain.Request) { r.City = "unknown" }, func(r *domain.Request) { r.Date = "2026-12-19" }, func(r *domain.Request) { r.Language = nil; r.Duration = nil }, func(r *domain.Request) {
		r.Category = "Флорист"
		r.EventFormat = "свадьба"
		r.Budget = 300000
	}} {
		r := base()
		modify(&r)
		cases = append(cases, r)
	}
	for _, r := range cases {
		want := Recommend(s, r)
		for _, locale := range []i18n.Locale{i18n.LocaleRU, i18n.LocaleKK, i18n.LocaleEN} {
			got := RecommendLocalized(s, r, locale)
			if locale == i18n.LocaleRU && !reflect.DeepEqual(got, want) {
				t.Fatal("RU wrapper changed")
			}
			if !reflect.DeepEqual(withoutLocalizedText(got), withoutLocalizedText(Recommend(s, r))) {
				t.Fatalf("locale %s changed non-text response", locale)
			}
		}
	}
}

func TestRussianResponseBackwardCompatibility(t *testing.T) {
	b, err := os.ReadFile("../../examples/recommendation-response.json")
	if err != nil {
		t.Fatal(err)
	}
	var want domain.Response
	if err = json.Unmarshal(b, &want); err != nil {
		t.Fatal(err)
	}
	gotJSON, _ := json.Marshal(Recommend(data(t), base()))
	wantJSON, _ := json.Marshal(want)
	if string(gotJSON) != string(wantJSON) {
		t.Fatalf("default RU changed:\ngot %s\nwant %s", gotJSON, wantJSON)
	}
}

func TestLocalizedExplanationsAndEvidence(t *testing.T) {
	s, r := alternativeFixture()
	r.Budget = 300
	v := s.Vendors[0]
	v.PriceImputed = true
	v.Description = "Описание источника. Условие источника."
	facts := []domain.Fact{{VendorID: v.ID, ID: "feature", Kind: "feature", Text: "Исходный факт.  ", Quote: "Описание источника.", DescriptionHash: domain.Hash([]byte(v.Description)), ReviewStatus: "approved"}, {VendorID: v.ID, ID: "condition", Kind: "condition", Text: "Исходное условие", Quote: "Условие источника.", DescriptionHash: domain.Hash([]byte(v.Description)), ReviewStatus: "approved"}}
	for _, tt := range []struct {
		locale         i18n.Locale
		prefix, system string
	}{
		{i18n.LocaleEN, "Original profile note: ", "Supports the corporate event format"},
		{i18n.LocaleKK, "Профильдегі бастапқы мәтін: ", "Профильде «корпоратив» форматы"},
	} {
		text, conditions := explainLocalized(v, r, facts, tt.locale)
		if !strings.Contains(text, tt.system) || !strings.HasSuffix(text, tt.prefix+facts[0].Text) {
			t.Fatalf("bad localized explanation: %s", text)
		}
		if !reflect.DeepEqual(conditions, []domain.Condition{{FactID: "condition", Text: facts[1].Text}}) {
			t.Fatal("condition source changed")
		}
		system := strings.Split(text, tt.prefix)[0]
		for _, ru := range []string{"стартовая цена", "запрошено", "язык «", "не отмечено занятым", "Профиль поддерживает", "оценка из датасета"} {
			if strings.Contains(system, ru) {
				t.Fatalf("Russian template in %s: %s", tt.locale, text)
			}
		}
		v.MaxHours = nil
		fallback, _ := explainLocalized(v, r, nil, tt.locale)
		if strings.Contains(fallback, "В профиле перечислены") || !strings.Contains(fallback, i18n.Message(tt.locale, "hours_na")) {
			t.Fatal(fallback)
		}
		r.Budget = v.Price
		equal, _ := explainLocalized(v, r, nil, tt.locale)
		if !strings.Contains(equal, i18n.Message(tt.locale, "price_equal", v.Price)) {
			t.Fatal(equal)
		}
	}
}

func TestLocalizedValidation(t *testing.T) {
	w := data(t).Metadata.CalendarWindow
	for _, tt := range []struct {
		key    string
		modify func(*domain.Request)
		args   []any
	}{
		{"required", func(r *domain.Request) { r.City = "" }, nil},
		{"invalid_date", func(r *domain.Request) { r.Date = "bad" }, nil},
		{"date_range", func(r *domain.Request) { r.Date = "2027-01-01" }, []any{w.From, w.To}},
		{"invalid_format", func(r *domain.Request) { r.EventFormat = "bad" }, nil},
		{"budget", func(r *domain.Request) { r.Budget = 0 }, nil},
		{"duration", func(r *domain.Request) { r.Duration = ptr(0.0) }, nil},
		{"language", func(r *domain.Request) { r.Language = ptr("en") }, nil},
	} {
		r := base()
		tt.modify(&r)
		original := Validate(r, w)
		for _, l := range []i18n.Locale{i18n.LocaleRU, i18n.LocaleKK, i18n.LocaleEN} {
			got := ValidateLocalized(r, w, l)
			if len(got) != 1 || got[0].Code != original[0].Code || got[0].Field != original[0].Field || got[0].Message != i18n.Message(l, tt.key, tt.args...) {
				t.Fatalf("%s/%s: %+v", l, tt.key, got)
			}
		}
	}
}
