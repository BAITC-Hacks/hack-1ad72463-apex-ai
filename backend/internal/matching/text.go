package matching

import (
	"fmt"
	"strings"

	"github.com/BAITC-Hacks/hack-1ad72463-apex-ai/backend/internal/domain"
	"github.com/BAITC-Hacks/hack-1ad72463-apex-ai/backend/internal/i18n"
)

func exclusionsLocalized(d domain.Diagnostics, locale i18n.Locale) string {
	parts := []string{}
	for _, code := range domain.Reasons {
		if n := d.ExcludedCounts[code]; n > 0 {
			parts = append(parts, fmt.Sprintf("%s: %d", i18n.ReasonLabel(locale, code), n))
		}
	}
	return i18n.Message(locale, "excluded", strings.Join(parts, "; "))
}

func explain(v domain.Vendor, r domain.Request, facts []domain.Fact) (string, []domain.Condition) {
	return explainLocalized(v, r, facts, i18n.LocaleRU)
}

func explainLocalized(v domain.Vendor, r domain.Request, facts []domain.Fact, locale i18n.Locale) (string, []domain.Condition) {
	price := i18n.Message(locale, "price", v.Price, r.Budget)
	if v.Price == r.Budget {
		price = i18n.Message(locale, "price_equal", v.Price)
	}
	if v.PriceImputed {
		price += i18n.Message(locale, "price_imputed")
	}
	parts := []string{i18n.Message(locale, "supports", i18n.FormatLabel(locale, r.EventFormat), v.City), price}
	if r.Language != nil {
		parts = append(parts, i18n.Message(locale, "profile_language", i18n.LanguageLabel(locale, *r.Language)))
	}
	if r.Duration != nil {
		if v.MaxHours == nil {
			parts = append(parts, i18n.Message(locale, "hours_na"))
		} else {
			parts = append(parts, i18n.Message(locale, "hours", *r.Duration, *v.MaxHours))
		}
	}
	parts = append(parts, i18n.Message(locale, "available", r.Date))
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
		text += " " + i18n.ProfileNote(locale, feature)
	} else {
		languages := make([]string, len(v.Languages))
		for i, language := range v.Languages {
			languages[i] = i18n.LanguageLabel(locale, language)
		}
		text += i18n.Message(locale, "profile_fallback", strings.Join(v.Categories, ", "), strings.Join(languages, ", "))
	}
	return text, conditions
}
