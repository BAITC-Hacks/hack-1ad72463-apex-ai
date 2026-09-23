package domain

// Alternative contains only the fields belonging to its BUDGET or DATE type.
type Alternative struct {
	Type            string `json:"type"`
	CurrentBudget   int64  `json:"current_budget_kzt,omitempty"`
	SuggestedBudget int64  `json:"suggested_budget_kzt,omitempty"`
	CurrentDate     string `json:"current_date,omitempty"`
	SuggestedDate   string `json:"suggested_date,omitempty"`
	DistanceDays    int    `json:"distance_days,omitempty"`
	EligibleCount   int    `json:"eligible_count"`
}

type AlternativesResponse struct {
	Alternatives []Alternative `json:"alternatives"`
}
