package matching

import (
	"encoding/json"
	"math"
	"math/rand"
	"reflect"
	"slices"
	"testing"

	"github.com/BAITC-Hacks/hack-1ad72463-apex-ai/backend/internal/domain"
)

func alternativeFixture() (domain.Snapshot, domain.Request) {
	r := base()
	r.Budget = 100
	s := domain.Snapshot{Metadata: domain.Metadata{CalendarWindow: domain.Window{From: "2026-11-10", To: "2026-11-18"}}}
	s.Vendors = []domain.Vendor{{ID: "eligible", City: r.City, Categories: []string{r.Category}, Price: 200, Formats: []string{r.EventFormat}, Languages: []string{*r.Language}, MaxHours: ptr(6.0)}}
	return s, r
}

// Apply exactly the field advertised by the response and ask the real engine.
// Also verify input immutability, including optional values and snapshot slices.
func checkAlternatives(t *testing.T, s domain.Snapshot, r domain.Request) domain.AlternativesResponse {
	t.Helper()
	before, _ := json.Marshal(struct {
		S domain.Snapshot
		R domain.Request
	}{s, r})
	out := FindAlternatives(s, r)
	after, _ := json.Marshal(struct {
		S domain.Snapshot
		R domain.Request
	}{s, r})
	if string(before) != string(after) {
		t.Fatal("input mutated")
	}
	if out.Alternatives == nil || len(out.Alternatives) > 2 {
		t.Fatalf("invalid array: %+v", out)
	}
	for i, a := range out.Alternatives {
		candidate := r
		switch a.Type {
		case "BUDGET":
			if i != 0 || a.CurrentBudget != r.Budget || a.SuggestedBudget <= r.Budget || a.CurrentDate != "" || a.SuggestedDate != "" || a.DistanceDays != 0 {
				t.Fatalf("invalid budget: %+v", a)
			}
			candidate.Budget = a.SuggestedBudget
			below := r
			below.Budget = a.SuggestedBudget - 1
			if Recommend(s, below).Status == "MATCHES_FOUND" {
				t.Fatal("budget is not minimal")
			}
		case "DATE":
			if a.CurrentDate != r.Date || a.DistanceDays <= 0 || a.SuggestedDate == r.Date || a.SuggestedDate < s.Metadata.CalendarWindow.From || a.SuggestedDate > s.Metadata.CalendarWindow.To || a.CurrentBudget != 0 || a.SuggestedBudget != 0 {
				t.Fatalf("invalid date: %+v", a)
			}
			candidate.Date = a.SuggestedDate
		default:
			t.Fatalf("unexpected type: %s", a.Type)
		}
		result := Recommend(s, candidate)
		if result.Status != "MATCHES_FOUND" || result.Diagnostics.EligibleCount != a.EligibleCount {
			t.Fatalf("single-field change does not produce advertised matches: %+v", a)
		}
	}
	return out
}

func TestBudgetAlternative(t *testing.T) {
	s, r := alternativeFixture()
	// Cheaper prices must not bypass any other constraint, even when the public
	// funnel counts BUDGET_TOO_LOW before language, duration, and busy checks.
	for i, modify := range []func(*domain.Vendor){
		func(v *domain.Vendor) { v.City = "Астана" },
		func(v *domain.Vendor) { v.Categories = []string{"Фотограф"} },
		func(v *domain.Vendor) { v.Formats = []string{"свадьба"} },
		func(v *domain.Vendor) { v.Languages = []string{"английский"} },
		func(v *domain.Vendor) { v.MaxHours = ptr(5.0) },
		func(v *domain.Vendor) { v.BusyDates = []string{r.Date} },
	} {
		v := s.Vendors[0]
		v.ID = string(rune('a' + i))
		v.Price = int64(110 + i)
		modify(&v)
		s.Vendors = append(s.Vendors, v)
	}
	for i := 0; i < 4; i++ {
		v := s.Vendors[0]
		v.ID = string(rune('k' + i))
		s.Vendors = append(s.Vendors, v)
	}
	if Recommend(s, r).Status != "NO_MATCH" {
		t.Fatal("invalid fixture")
	}
	got := checkAlternatives(t, s, r)
	want := domain.AlternativesResponse{Alternatives: []domain.Alternative{{Type: "BUDGET", CurrentBudget: 100, SuggestedBudget: 200, EligibleCount: 5}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v want %+v", got, want)
	}
}

func TestDateAlternative(t *testing.T) {
	for _, tt := range []struct {
		name, date string
		busy       []string
		want       string
		distance   int
	}{
		{"tie prefers later", "2026-11-14", []string{"2026-11-14"}, "2026-11-15", 1},
		{"nearest earlier", "2026-11-14", []string{"2026-11-14", "2026-11-15"}, "2026-11-13", 1},
		{"skip busy neighbors", "2026-11-14", []string{"2026-11-13", "2026-11-14", "2026-11-15"}, "2026-11-16", 2},
		{"from boundary", "2026-11-10", []string{"2026-11-10"}, "2026-11-11", 1},
		{"to boundary", "2026-11-18", []string{"2026-11-18"}, "2026-11-17", 1},
		{"full window", "2026-11-10", []string{"2026-11-10", "2026-11-11", "2026-11-12", "2026-11-13", "2026-11-14", "2026-11-15", "2026-11-16", "2026-11-17"}, "2026-11-18", 8},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s, r := alternativeFixture()
			r.Budget = 200
			r.Date = tt.date
			s.Vendors[0].BusyDates = tt.busy
			got := checkAlternatives(t, s, r)
			want := domain.AlternativesResponse{Alternatives: []domain.Alternative{{Type: "DATE", CurrentDate: r.Date, SuggestedDate: tt.want, DistanceDays: tt.distance, EligibleCount: 1}}}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("got %+v want %+v", got, want)
			}
		})
	}
}

func TestAlternativesEmpty(t *testing.T) {
	for _, tt := range []struct {
		name, status string
		modify       func(*domain.Snapshot, *domain.Request)
	}{
		{"no catalog", "NO_CATALOG", func(s *domain.Snapshot, r *domain.Request) { r.City = "missing" }},
		{"already matches", "MATCHES_FOUND", func(s *domain.Snapshot, r *domain.Request) { r.Budget = 200 }},
		{"requires two changes", "NO_MATCH", func(s *domain.Snapshot, r *domain.Request) { s.Vendors[0].BusyDates = []string{r.Date} }},
		{"language cannot change", "NO_MATCH", func(s *domain.Snapshot, r *domain.Request) { r.Language = ptr("английский") }},
		{"duration cannot change", "NO_MATCH", func(s *domain.Snapshot, r *domain.Request) { r.Duration = ptr(7.0) }},
		{"format cannot change", "NO_MATCH", func(s *domain.Snapshot, r *domain.Request) { r.EventFormat = "свадьба" }},
		{"one day window", "NO_MATCH", func(s *domain.Snapshot, r *domain.Request) {
			r.Budget = 200
			s.Metadata.CalendarWindow = domain.Window{From: r.Date, To: r.Date}
			s.Vendors[0].BusyDates = []string{r.Date}
		}},
		{"max budget", "NO_MATCH", func(s *domain.Snapshot, r *domain.Request) {
			r.Budget = math.MaxInt64
			r.Language = ptr("английский")
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s, r := alternativeFixture()
			tt.modify(&s, &r)
			if Recommend(s, r).Status != tt.status {
				t.Fatal("invalid fixture")
			}
			got := checkAlternatives(t, s, r)
			b, _ := json.Marshal(got)
			if string(b) != `{"alternatives":[]}` {
				t.Fatalf("expected empty array, got %s", b)
			}
		})
	}
}

func TestAlternativesDeterminismAndIndependentChanges(t *testing.T) {
	s, r := alternativeFixture()
	r.Budget = 200
	s.Vendors[0].BusyDates = []string{r.Date}
	v := s.Vendors[0]
	v.ID = "expensive"
	v.Price = 300
	v.BusyDates = nil
	s.Vendors = append(s.Vendors, v)
	want := checkAlternatives(t, s, r)
	if len(want.Alternatives) != 2 || want.Alternatives[0].Type != "BUDGET" || want.Alternatives[1].Type != "DATE" || want.Alternatives[1].EligibleCount != 1 {
		t.Fatalf("alternatives were combined: %+v", want)
	}
	rng := rand.New(rand.NewSource(42))
	for i := 0; i < 20; i++ {
		rng.Shuffle(len(s.Vendors), func(i, j int) { s.Vendors[i], s.Vendors[j] = s.Vendors[j], s.Vendors[i] })
		if got := FindAlternatives(s, r); !reflect.DeepEqual(got, want) {
			t.Fatal("nondeterministic alternatives")
		}
	}
	r.Language = nil
	r.Duration = nil
	checkAlternatives(t, s, r)
}

func TestAlternativesActualDataset(t *testing.T) {
	s := data(t)
	r := base()
	r.Budget = 100000
	got := checkAlternatives(t, s, r)
	if len(got.Alternatives) != 1 || got.Alternatives[0].SuggestedBudget != 650000 || got.Alternatives[0].EligibleCount != 1 {
		t.Fatalf("AT04 changed: %+v", got)
	}
	// Discovered by scanning the imported catalog, not assumed in the algorithm.
	r.Budget = 650000
	r.Date = "2026-09-24"
	original := Recommend(s, r)
	if original.Status != "NO_MATCH" || original.Diagnostics.ExcludedCounts["BUSY_ON_DATE"] == 0 {
		t.Fatal("fixture no longer demonstrates busy date")
	}
	got = checkAlternatives(t, s, r)
	at := slices.IndexFunc(got.Alternatives, func(a domain.Alternative) bool { return a.Type == "DATE" })
	if at < 0 || got.Alternatives[at].SuggestedDate != "2026-09-25" || got.Alternatives[at].DistanceDays != 1 || got.Alternatives[at].EligibleCount != 1 {
		t.Fatalf("date scenario changed: %+v", got)
	}
}
