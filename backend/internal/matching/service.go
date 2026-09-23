package matching

import (
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/BAITC-Hacks/hack-1ad72463-apex-ai/backend/internal/catalog"
	"github.com/BAITC-Hacks/hack-1ad72463-apex-ai/backend/internal/domain"
)

func Validate(r domain.Request, w domain.Window) []domain.FieldError {
	errs := []domain.FieldError{}
	add := func(field, code, msg string) {
		errs = append(errs, domain.FieldError{Field: field, Code: code, Message: msg})
	}
	for _, x := range []struct{ k, v string }{{"city", r.City}, {"category", r.Category}, {"date", r.Date}, {"event_format", r.EventFormat}} {
		if domain.Normalize(x.v) == "" {
			add(x.k, "REQUIRED", "Укажите обязательное поле.")
		}
	}
	if r.Date != "" {
		if !catalog.ValidDate(r.Date) {
			add("date", "INVALID_DATE", "Используйте дату YYYY-MM-DD.")
		} else if r.Date < w.From || r.Date > w.To {
			add("date", "DATE_OUT_OF_RANGE", fmt.Sprintf("Выберите дату с %s по %s включительно.", w.From, w.To))
		}
	}
	if domain.Normalize(r.EventFormat) != "" && !domain.Contains(domain.Formats, r.EventFormat) {
		add("event_format", "INVALID_ENUM", "Неизвестный формат мероприятия.")
	}
	if r.Budget <= 0 {
		add("budget_kzt", "OUT_OF_RANGE", "Бюджет должен быть целым числом больше нуля.")
	}
	if r.Duration != nil && (*r.Duration <= 0 || math.IsNaN(*r.Duration) || math.IsInf(*r.Duration, 0)) {
		add("duration_hours", "OUT_OF_RANGE", "Длительность должна быть конечным числом больше нуля.")
	}
	if r.Language != nil && !domain.Contains(domain.Languages, *r.Language) {
		add("language", "INVALID_ENUM", "Допустимы русский, казахский, английский.")
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
		card.Explanation, card.Conditions = explain(v, r, s.Facts)
		out.Results = append(out.Results, card)
	}
	d.ReturnedCount = len(out.Results)
	switch {
	case d.CatalogCount == 0:
		out.Message = fmt.Sprintf("В каталоге нет категории «%s» в городе «%s». Измените город или категорию.", strings.TrimSpace(r.Category), strings.TrimSpace(r.City))
	case d.EligibleCount == 0:
		out.Status = "NO_MATCH"
		out.Message = "Ни один подрядчик не соответствует условиям. " + exclusions(*d) + " Измените условия запроса."
	default:
		out.Status = "MATCHES_FOUND"
		out.Message = fmt.Sprintf("Подходят %d подрядчика; показаны %d по стартовой цене, затем ID.", d.EligibleCount, d.ReturnedCount)
		if d.ReturnedCount < 3 {
			if d.EligibleCount == d.CatalogCount {
				out.Message += fmt.Sprintf(" В каталоге города и категории всего %d профиля.", d.CatalogCount)
			} else {
				out.Message += " " + exclusions(*d)
			}
		}
	}
	return out
}

func exclusions(d domain.Diagnostics) string {
	labels := []string{"не поддерживают формат", "стартовая цена выше бюджета", "не поддерживают язык", "превышена длительность", "заняты на выбранную дату"}
	parts := []string{}
	for i, k := range domain.Reasons {
		if n := d.ExcludedCounts[k]; n > 0 {
			parts = append(parts, fmt.Sprintf("%s: %d", labels[i], n))
		}
	}
	return "Исключены по первой неподходящей проверке: " + strings.Join(parts, "; ") + "."
}

func explain(v domain.Vendor, r domain.Request, facts []domain.Fact) (string, []domain.Condition) {
	price := fmt.Sprintf("стартовая цена %d ₸ укладывается в бюджет %d ₸", v.Price, r.Budget)
	if v.Price == r.Budget {
		price = fmt.Sprintf("стартовая цена %d ₸ равна бюджету", v.Price)
	}
	if v.PriceImputed {
		price += " (оценка из датасета)"
	}
	parts := []string{fmt.Sprintf("Профиль поддерживает формат «%s» в городе %s", domain.Normalize(r.EventFormat), v.City), price}
	if r.Language != nil {
		parts = append(parts, "язык «"+domain.Normalize(*r.Language)+"» указан в профиле")
	}
	if r.Duration != nil {
		if v.MaxHours == nil {
			parts = append(parts, "ограничение часов для этой услуги неприменимо")
		} else {
			parts = append(parts, fmt.Sprintf("запрошено %g ч при лимите %g ч", *r.Duration, *v.MaxHours))
		}
	}
	parts = append(parts, r.Date+" не отмечено занятым в календаре датасета")
	text := strings.Join(parts, "; ") + "."
	conditions := []domain.Condition{}
	feature := ""
	for _, f := range facts {
		if f.VendorID != v.ID || f.ReviewStatus != "approved" || f.DescriptionHash != domain.Hash([]byte(v.Description)) || !strings.Contains(v.Description, f.Quote) {
			continue
		}
		if f.Kind == "condition" {
			conditions = append(conditions, domain.Condition{FactID: f.ID, Text: f.Text})
		} else if f.Kind == "feature" && feature == "" {
			feature = f.Text
		}
	}
	if feature != "" {
		text += " В описании отмечено: " + strings.TrimRight(feature, ". ") + "."
	} else {
		text += fmt.Sprintf("В профиле перечислены услуги: %s; языки: %s.", strings.Join(v.Categories, ", "), strings.Join(v.Languages, ", "))
	}
	return text, conditions
}
