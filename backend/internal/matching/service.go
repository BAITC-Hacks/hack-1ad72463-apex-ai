package matching

import (
	"math"
	"slices"
	"strings"

	"github.com/BAITC-Hacks/hack-1ad72463-apex-ai/backend/internal/catalog"
	"github.com/BAITC-Hacks/hack-1ad72463-apex-ai/backend/internal/domain"
	"github.com/BAITC-Hacks/hack-1ad72463-apex-ai/backend/internal/i18n"
)

func Validate(r domain.Request, w domain.Window) []domain.FieldError {
	return ValidateLocalized(r, w, i18n.LocaleRU)
}

func ValidateLocalized(r domain.Request, w domain.Window, locale i18n.Locale) []domain.FieldError {
	errs := []domain.FieldError{}
	add := func(field, code, msg string) {
		errs = append(errs, domain.FieldError{Field: field, Code: code, Message: msg})
	}
	for _, x := range []struct{ k, v string }{{"city", r.City}, {"category", r.Category}, {"date", r.Date}, {"event_format", r.EventFormat}} {
		if domain.Normalize(x.v) == "" {
			add(x.k, "REQUIRED", i18n.Message(locale, "required"))
		}
	}
	if r.Date != "" {
		if !catalog.ValidDate(r.Date) {
			add("date", "INVALID_DATE", i18n.Message(locale, "invalid_date"))
		} else if r.Date < w.From || r.Date > w.To {
			add("date", "DATE_OUT_OF_RANGE", i18n.Message(locale, "date_range", w.From, w.To))
		}
	}
	if domain.Normalize(r.EventFormat) != "" && !domain.Contains(domain.Formats, r.EventFormat) {
		add("event_format", "INVALID_ENUM", i18n.Message(locale, "invalid_format"))
	}
	if r.Budget <= 0 {
		add("budget_kzt", "OUT_OF_RANGE", i18n.Message(locale, "budget"))
	}
	if r.Duration != nil && (*r.Duration <= 0 || math.IsNaN(*r.Duration) || math.IsInf(*r.Duration, 0)) {
		add("duration_hours", "OUT_OF_RANGE", i18n.Message(locale, "duration"))
	}
	if r.Language != nil && !domain.Contains(domain.Languages, *r.Language) {
		add("language", "INVALID_ENUM", i18n.Message(locale, "language"))
	}
	return errs
}

// ExclusionReasons keeps every violation; the public funnel counts only the first.
func ExclusionReasons(v domain.Vendor, r domain.Request) []string {
	checks := []bool{!domain.Contains(v.Formats, r.EventFormat), v.Price > r.Budget, r.Language != nil && !domain.Contains(v.Languages, value(r.Language)), r.Duration != nil && v.MaxHours != nil && *r.Duration > *v.MaxHours, slices.Contains(v.BusyDates, r.Date)}
	out := []string{}
	for i, failed := range checks {
		if failed {
			out = append(out, domain.Reasons[i])
		}
	}
	return out
}
func value(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func Recommend(s domain.Snapshot, r domain.Request) domain.Response {
	return RecommendLocalized(s, r, i18n.LocaleRU)
}

// RecommendLocalized changes presentation only; selection and ordering share one pipeline.
func RecommendLocalized(s domain.Snapshot, r domain.Request, locale i18n.Locale) domain.Response {
	out := domain.Response{Status: "NO_CATALOG", Results: []domain.Card{}, Metadata: s.Metadata, Diagnostics: domain.Diagnostics{ExcludedCounts: map[string]int{}, OmittedChecks: []string{}, AppliedOrder: []string{"price_from_kzt:asc", "id:asc"}}}
	d := &out.Diagnostics
	for _, code := range domain.Reasons {
		d.ExcludedCounts[code] = 0
	}
	if r.Language == nil {
		d.OmittedChecks = append(d.OmittedChecks, "language")
	}
	if r.Duration == nil {
		d.OmittedChecks = append(d.OmittedChecks, "duration_hours")
	}
	eligible := []domain.Vendor{}
	for _, v := range s.Vendors {
		if domain.Normalize(v.City) != domain.Normalize(r.City) || !domain.Contains(v.Categories, r.Category) {
			continue
		}
		d.CatalogCount++
		reasons := ExclusionReasons(v, r)
		if len(reasons) > 0 {
			d.ExcludedCounts[reasons[0]]++
			continue
		}
		eligible = append(eligible, v)
	}
	slices.SortFunc(eligible, func(a, b domain.Vendor) int {
		if a.Price < b.Price {
			return -1
		}
		if a.Price > b.Price {
			return 1
		}
		return strings.Compare(a.ID, b.ID)
	})
	d.EligibleCount = len(eligible)
	for _, v := range eligible[:min(len(eligible), 3)] {
		category := r.Category
		for _, c := range v.Categories {
			if domain.Normalize(c) == domain.Normalize(r.Category) {
				category = c
				break
			}
		}
		card := domain.Card{ID: v.ID, Name: v.Name, City: v.City, Category: category, Price: v.Price, MaxHours: v.MaxHours, Synthetic: v.Synthetic, PriceImputed: v.PriceImputed, CityImputed: v.CityImputed}
		card.Explanation, card.Conditions = explainLocalized(v, r, s.Facts, locale)
		out.Results = append(out.Results, card)
	}
	d.ReturnedCount = len(out.Results)
	switch {
	case d.CatalogCount == 0:
		out.Message = i18n.Message(locale, "no_catalog", strings.TrimSpace(r.Category), strings.TrimSpace(r.City))
	case d.EligibleCount == 0:
		out.Status = "NO_MATCH"
		out.Message = i18n.Message(locale, "no_match") + exclusionsLocalized(*d, locale) + i18n.Message(locale, "change_request")
	default:
		out.Status = "MATCHES_FOUND"
		out.Message = i18n.Message(locale, "matches", d.EligibleCount, d.ReturnedCount)
		if d.ReturnedCount < 3 {
			if d.EligibleCount == d.CatalogCount {
				out.Message += i18n.Message(locale, "small_catalog", d.CatalogCount)
			} else {
				out.Message += " " + exclusionsLocalized(*d, locale)
			}
		}
	}
	return out
}
