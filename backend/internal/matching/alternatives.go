package matching

import (
	"slices"
	"time"

	"github.com/BAITC-Hacks/hack-1ad72463-apex-ai/backend/internal/domain"
)

// FindAlternatives expects a validated request and snapshot. Every candidate is
// checked by Recommend against the same snapshot, changing exactly one field.
func FindAlternatives(s domain.Snapshot, r domain.Request) domain.AlternativesResponse {
	out := domain.AlternativesResponse{Alternatives: []domain.Alternative{}}
	if Recommend(s, r).Status != "NO_MATCH" {
		return out
	}

	// Eligibility can change with budget only at a catalog price threshold.
	prices := []int64{}
	for _, v := range s.Vendors {
		if domain.Normalize(v.City) == domain.Normalize(r.City) && domain.Contains(v.Categories, r.Category) && v.Price > r.Budget {
			prices = append(prices, v.Price)
		}
	}
	slices.Sort(prices)
	for _, price := range slices.Compact(prices) {
		candidate := r
		candidate.Budget = price
		result := Recommend(s, candidate)
		if result.Status == "MATCHES_FOUND" {
			out.Alternatives = append(out.Alternatives, domain.Alternative{
				Type: "BUDGET", CurrentBudget: r.Budget, SuggestedBudget: price,
				EligibleCount: result.Diagnostics.EligibleCount,
			})
			break
		}
	}

	// Parse in UTC: a calendar day is always 24 hours, including across DST.
	current, _ := time.Parse(time.DateOnly, r.Date)
	from, _ := time.Parse(time.DateOnly, s.Metadata.CalendarWindow.From)
	to, _ := time.Parse(time.DateOnly, s.Metadata.CalendarWindow.To)
	maxDistance := max(int(current.Sub(from).Hours()/24), int(to.Sub(current).Hours()/24))
	for distance := 1; distance <= maxDistance; distance++ {
		// Later dates win ties. Each candidate starts from the original request,
		// never from a successful budget alternative.
		for _, offset := range []int{distance, -distance} {
			date := current.AddDate(0, 0, offset)
			if date.Before(from) || date.After(to) {
				continue
			}
			candidate := r
			candidate.Date = date.Format(time.DateOnly)
			result := Recommend(s, candidate)
			if result.Status == "MATCHES_FOUND" {
				out.Alternatives = append(out.Alternatives, domain.Alternative{
					Type: "DATE", CurrentDate: r.Date, SuggestedDate: candidate.Date,
					DistanceDays: distance, EligibleCount: result.Diagnostics.EligibleCount,
				})
				return out
			}
		}
	}
	return out
}
