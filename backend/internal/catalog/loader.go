package catalog

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/BAITC-Hacks/hack-1ad72463-apex-ai/backend/internal/domain"
)

var columns = []string{"id", "anon_name", "categories", "city", "city_imputed", "synthetic", "price_from_kzt", "price_imputed", "event_formats", "languages", "max_hours", "busy_dates", "description"}

func ValidDate(s string) bool {
	t, e := time.Parse(time.DateOnly, s)
	return e == nil && t.Format(time.DateOnly) == s
}
func ValidateWindow(w domain.Window) error {
	if !ValidDate(w.From) || !ValidDate(w.To) || w.From > w.To {
		return fmt.Errorf("invalid calendar window")
	}
	return nil
}

func Load(csvPath, metaPath, factsPath string) (domain.Snapshot, error) {
	var s domain.Snapshot
	b, err := os.ReadFile(csvPath)
	if err != nil {
		return s, err
	}
	m, err := os.ReadFile(metaPath)
	if err != nil {
		return s, err
	}
	var meta struct {
		CalendarWindow domain.Window `json:"calendar_window"`
	}
	if err = json.Unmarshal(m, &meta); err != nil {
		return s, err
	}
	s, err = Parse(b, meta.CalendarWindow)
	if err != nil {
		return s, err
	}
	if factsPath != "" {
		f, e := os.ReadFile(factsPath)
		if e != nil {
			return s, e
		}
		if err = json.Unmarshal(f, &s.Facts); err != nil {
			return s, err
		}
		s.Metadata.FactsVersion = domain.Hash(f)
	}
	return s, Validate(s)
}

func Parse(raw []byte, w domain.Window) (domain.Snapshot, error) {
	s := domain.Snapshot{Metadata: domain.Metadata{SnapshotVersion: domain.Hash(raw), LoaderVersion: domain.LoaderVersion, PolicyVersion: domain.PolicyVersion, FactsVersion: "structured-only-v1", CalendarWindow: w}, Vendors: []domain.Vendor{}, Facts: []domain.Fact{}}
	if err := ValidateWindow(w); err != nil {
		return s, err
	}
	if !utf8.Valid(raw) {
		return s, fmt.Errorf("CSV must be UTF-8")
	}
	r := csv.NewReader(bytes.NewReader(bytes.TrimPrefix(raw, []byte{0xef, 0xbb, 0xbf})))
	head, err := r.Read()
	if err != nil {
		return s, err
	}
	index := map[string]int{}
	for i, k := range head {
		if _, ok := index[k]; ok {
			return s, fmt.Errorf("duplicate column %q", k)
		}
		index[k] = i
	}
	for _, k := range columns {
		if _, ok := index[k]; !ok {
			return s, fmt.Errorf("missing column %q", k)
		}
	}
	if len(head) != len(columns) {
		return s, fmt.Errorf("unexpected CSV columns")
	}
	for line := 2; ; line++ {
		row, e := r.Read()
		if e == io.EOF {
			break
		}
		if e != nil {
			return s, fmt.Errorf("row %d: %w", line, e)
		}
		get := func(k string) string { return row[index[k]] }
		v := domain.Vendor{ID: strings.TrimSpace(get("id")), Name: strings.TrimSpace(get("anon_name")), City: strings.TrimSpace(get("city")), Description: get("description")}
		fail := func(e error) (domain.Snapshot, error) { return s, fmt.Errorf("row %d (%s): %w", line, v.ID, e) }
		for k, dst := range map[string]*bool{"city_imputed": &v.CityImputed, "synthetic": &v.Synthetic, "price_imputed": &v.PriceImputed} {
			switch get(k) {
			case "True":
				*dst = true
			case "False":
				*dst = false
			default:
				return fail(fmt.Errorf("invalid %s flag", k))
			}
		}
		v.Price, e = strconv.ParseInt(get("price_from_kzt"), 10, 64)
		if e != nil {
			return fail(fmt.Errorf("invalid price"))
		}
		for k, dst := range map[string]*[]string{"categories": &v.Categories, "event_formats": &v.Formats, "languages": &v.Languages, "busy_dates": &v.BusyDates} {
			*dst = []string{}
			if k == "busy_dates" && get(k) == "" {
				continue
			}
			seen := map[string]bool{}
			for _, item := range strings.Split(get(k), "|") {
				item = strings.TrimSpace(item)
				key := domain.Normalize(item)
				if key == "" {
					return fail(fmt.Errorf("empty %s element", k))
				}
				if !seen[key] {
					*dst = append(*dst, item)
					seen[key] = true
				}
			}
			slices.Sort(*dst)
		}
		if h := get("max_hours"); h != "" {
			n, e := strconv.ParseFloat(h, 64)
			if e != nil {
				return fail(fmt.Errorf("invalid max_hours"))
			}
			v.MaxHours = &n
		}
		s.Vendors = append(s.Vendors, v)
	}
	slices.SortFunc(s.Vendors, func(a, b domain.Vendor) int { return strings.Compare(a.ID, b.ID) })
	return s, Validate(s)
}

// Validate also protects the HTTP service against incomplete or tampered DB snapshots.
func Validate(s domain.Snapshot) error {
	if err := ValidateWindow(s.Metadata.CalendarWindow); err != nil {
		return err
	}
	if len(s.Vendors) == 0 {
		return fmt.Errorf("empty catalog")
	}
	if s.Metadata.SnapshotVersion == "" || s.Metadata.FactsVersion == "" || s.Metadata.LoaderVersion != domain.LoaderVersion || s.Metadata.PolicyVersion != domain.PolicyVersion {
		return fmt.Errorf("unsupported catalog versions")
	}
	seen := map[string]domain.Vendor{}
	for _, v := range s.Vendors {
		if domain.Normalize(v.ID) == "" || domain.Normalize(v.Name) == "" || domain.Normalize(v.City) == "" || strings.TrimSpace(v.Description) == "" || v.Price <= 0 {
			return fmt.Errorf("invalid required fields for %s", v.ID)
		}
		if _, ok := seen[v.ID]; ok {
			return fmt.Errorf("duplicate vendor ID %s", v.ID)
		}
		seen[v.ID] = v
		if v.MaxHours != nil && (*v.MaxHours <= 0 || math.IsNaN(*v.MaxHours) || math.IsInf(*v.MaxHours, 0)) {
			return fmt.Errorf("invalid max_hours for %s", v.ID)
		}
		for _, xs := range [][]string{v.Categories, v.Formats, v.Languages} {
			if len(xs) == 0 {
				return fmt.Errorf("empty list for %s", v.ID)
			}
			for _, x := range xs {
				if domain.Normalize(x) == "" {
					return fmt.Errorf("empty list element")
				}
			}
		}
		for _, f := range v.Formats {
			if !domain.Contains(domain.Formats, f) {
				return fmt.Errorf("invalid event format for %s", v.ID)
			}
		}
		for _, l := range v.Languages {
			if !domain.Contains(domain.Languages, l) {
				return fmt.Errorf("invalid language for %s", v.ID)
			}
		}
		for _, d := range v.BusyDates {
			if !ValidDate(d) || d < s.Metadata.CalendarWindow.From || d > s.Metadata.CalendarWindow.To {
				return fmt.Errorf("invalid busy date for %s", v.ID)
			}
		}
	}
	facts := map[string]bool{}
	for _, f := range s.Facts {
		v, ok := seen[f.VendorID]
		if !ok || f.ID == "" || facts[f.ID] || strings.TrimSpace(f.Text) == "" || strings.TrimSpace(f.Quote) == "" || f.DescriptionHash != domain.Hash([]byte(v.Description)) || !strings.Contains(v.Description, f.Quote) || f.ReviewStatus != "approved" || (f.Kind != "feature" && f.Kind != "condition") {
			return fmt.Errorf("invalid or stale evidence %s", f.ID)
		}
		facts[f.ID] = true
	}
	return nil
}
