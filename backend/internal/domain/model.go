package domain

import (
	"crypto/sha256"
	"fmt"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/unicode/norm"
)

const (
	LoaderVersion = "loader-v1"
	PolicyVersion = "eligibility-price-v1"
)

var Formats = []string{"день рождения", "конференция", "корпоратив", "свадьба", "той", "юбилей"}
var Languages = []string{"английский", "казахский", "русский"}
var Reasons = []string{"EVENT_FORMAT_UNSUPPORTED", "BUDGET_TOO_LOW", "LANGUAGE_UNSUPPORTED", "DURATION_EXCEEDED", "BUSY_ON_DATE"}

func Normalize(s string) string {
	return cases.Fold().String(norm.NFC.String(strings.Join(strings.Fields(s), " ")))
}
func Hash(b []byte) string { return fmt.Sprintf("sha256:%x", sha256.Sum256(b)) }
func Contains(xs []string, s string) bool {
	for _, x := range xs {
		if Normalize(x) == Normalize(s) {
			return true
		}
	}
	return false
}

type Window struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type Metadata struct {
	SnapshotVersion string `json:"snapshot_version"`
	LoaderVersion   string `json:"loader_version"`
	PolicyVersion   string `json:"policy_version"`
	FactsVersion    string `json:"facts_version"`
	CalendarWindow  Window `json:"calendar_window"`
}

type Vendor struct {
	ID           string   `json:"id"`
	Name         string   `json:"anon_name"`
	Categories   []string `json:"categories"`
	City         string   `json:"city"`
	CityImputed  bool     `json:"city_imputed"`
	Synthetic    bool     `json:"synthetic"`
	Price        int64    `json:"price_from_kzt"`
	PriceImputed bool     `json:"price_imputed"`
	Formats      []string `json:"event_formats"`
	Languages    []string `json:"languages"`
	MaxHours     *float64 `json:"max_hours"`
	BusyDates    []string `json:"busy_dates"`
	Description  string   `json:"description"`
}

type Fact struct {
	VendorID        string `json:"vendor_id"`
	ID              string `json:"fact_id"`
	Text            string `json:"text"`
	Quote           string `json:"quote"`
	DescriptionHash string `json:"description_hash"`
	ReviewStatus    string `json:"review_status"`
	Kind            string `json:"kind"` // feature or condition
}

type Snapshot struct {
	Metadata Metadata `json:"metadata"`
	Vendors  []Vendor `json:"vendors"`
	Facts    []Fact   `json:"facts"`
}

type Request struct {
	City        string   `json:"city"`
	Date        string   `json:"date"`
	EventFormat string   `json:"event_format"`
	Category    string   `json:"category"`
	Budget      int64    `json:"budget_kzt"`
	Duration    *float64 `json:"duration_hours"`
	Language    *string  `json:"language"`
}

type Condition struct {
	FactID string `json:"fact_id"`
	Text   string `json:"text"`
}

type Card struct {
	ID           string      `json:"id"`
	Name         string      `json:"anon_name"`
	City         string      `json:"city"`
	Category     string      `json:"category"`
	Price        int64       `json:"price_from_kzt"`
	MaxHours     *float64    `json:"max_hours"`
	Synthetic    bool        `json:"synthetic"`
	PriceImputed bool        `json:"price_imputed"`
	CityImputed  bool        `json:"city_imputed"`
	Explanation  string      `json:"explanation"`
	Conditions   []Condition `json:"conditions_to_confirm,omitempty"`
}

type Diagnostics struct {
	CatalogCount   int            `json:"catalog_count"`
	EligibleCount  int            `json:"eligible_count"`
	ReturnedCount  int            `json:"returned_count"`
	ExcludedCounts map[string]int `json:"excluded_counts"`
	OmittedChecks  []string       `json:"omitted_optional_checks"`
	AppliedOrder   []string       `json:"applied_order"`
}

type Response struct {
	Status      string      `json:"status"`
	Message     string      `json:"message"`
	Results     []Card      `json:"results"`
	Diagnostics Diagnostics `json:"diagnostics"`
	Metadata    Metadata    `json:"metadata"`
}

type FieldError struct {
	Field   string `json:"field"`
	Code    string `json:"code"`
	Message string `json:"message"`
}
