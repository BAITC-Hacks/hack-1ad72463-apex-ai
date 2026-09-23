package catalog

import (
	"bytes"
	"encoding/csv"
	"os"
	"strings"
	"testing"

	"github.com/BAITC-Hacks/hack-1ad72463-apex-ai/backend/internal/domain"
)

func fixture(t *testing.T) []byte {
	t.Helper()
	b, e := os.ReadFile("../../data/catalog.csv")
	if e != nil {
		t.Fatal(e)
	}
	return b
}

var window = domain.Window{From: "2026-09-23", To: "2026-12-31"}

func TestDatasetAudit(t *testing.T) {
	s, e := Load("../../data/catalog.csv", "../../data/catalog.meta.json", "../../data/facts.json")
	if e != nil {
		t.Fatal(e)
	}
	if len(s.Vendors) != 66 || len(s.Facts) != 68 {
		t.Fatalf("unexpected sizes: %d %d", len(s.Vendors), len(s.Facts))
	}
	cities := map[string]int{}
	cats := map[string]bool{}
	synthetic, price, city, null, multi := 0, 0, 0, 0, 0
	for _, v := range s.Vendors {
		cities[v.City]++
		for _, c := range v.Categories {
			cats[c] = true
		}
		if v.Synthetic {
			synthetic++
		}
		if v.PriceImputed {
			price++
		}
		if v.CityImputed {
			city++
		}
		if v.MaxHours == nil {
			null++
		}
		if len(v.Categories) > 1 {
			multi++
		}
	}
	if cities["Алматы"] != 50 || cities["Астана"] != 15 || cities["Зарубежье"] != 1 || len(cats) != 17 || synthetic != 13 || price != 18 || city != 8 || null != 9 || multi != 13 {
		t.Fatalf("audit mismatch: %v %d %d %d %d %d %d", cities, len(cats), synthetic, price, city, null, multi)
	}
	if s.Metadata.SnapshotVersion != domain.Hash(fixture(t)) {
		t.Fatal("hash must identify exact CSV bytes")
	}
}

func mutate(t *testing.T, change func([][]string)) []byte {
	t.Helper()
	rows, e := csv.NewReader(bytes.NewReader(fixture(t))).ReadAll()
	if e != nil {
		t.Fatal(e)
	}
	change(rows)
	var b bytes.Buffer
	w := csv.NewWriter(&b)
	w.WriteAll(rows)
	return b.Bytes()
}

func TestStrictImport(t *testing.T) {
	tests := []struct {
		name  string
		col   int
		value string
	}{
		{"empty id", 0, ""}, {"empty name", 1, " "}, {"empty category", 2, "Ведущий|"}, {"empty city", 3, ""}, {"invalid flag", 4, "maybe"}, {"false must be explicit", 5, "0"}, {"negative price", 6, "-1"}, {"decimal price", 6, "1.5"}, {"invalid price flag", 7, "false"}, {"unknown format", 8, "пикник"}, {"unknown language", 9, "французский"}, {"zero hours", 10, "0"}, {"NaN hours", 10, "NaN"}, {"infinite hours", 10, "Inf"}, {"bad date", 11, "2026-11-31"}, {"outside window", 11, "2027-01-01"}, {"empty description", 12, " "},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := mutate(t, func(rows [][]string) { rows[1][tt.col] = tt.value })
			if _, e := Parse(b, window); e == nil {
				t.Fatal("expected import rejection")
			}
		})
	}
	t.Run("duplicate ID", func(t *testing.T) {
		b := mutate(t, func(rows [][]string) { rows[2][0] = rows[1][0] })
		if _, e := Parse(b, window); e == nil {
			t.Fatal("duplicate accepted")
		}
	})
	t.Run("missing column", func(t *testing.T) {
		b := mutate(t, func(rows [][]string) { rows[0][0] = "other" })
		if _, e := Parse(b, window); e == nil {
			t.Fatal("missing header accepted")
		}
	})
	t.Run("duplicate column", func(t *testing.T) {
		b := mutate(t, func(rows [][]string) { rows[0][0] = rows[0][1] })
		if _, e := Parse(b, window); e == nil {
			t.Fatal("duplicate header accepted")
		}
	})
	t.Run("broken quotes", func(t *testing.T) {
		if _, e := Parse([]byte("id,\"unterminated"), window); e == nil {
			t.Fatal("broken CSV accepted")
		}
	})
	t.Run("invalid UTF8", func(t *testing.T) {
		if _, e := Parse([]byte{0xff}, window); e == nil {
			t.Fatal("invalid UTF8 accepted")
		}
	})
	t.Run("empty catalog", func(t *testing.T) {
		if _, e := Parse([]byte(strings.Join(columns, ",")+"\n"), window); e == nil {
			t.Fatal("empty catalog accepted")
		}
	})
	t.Run("invalid window", func(t *testing.T) {
		if _, e := Parse(fixture(t), domain.Window{}); e == nil {
			t.Fatal("invalid window accepted")
		}
	})
}

func TestBOMQuotesAndNull(t *testing.T) {
	b := mutate(t, func(rows [][]string) {
		rows[1][12] = "Текст, с \"кавычками\"\nи новой строкой"
		rows[1][11] = ""
		rows[1][10] = ""
	})
	s, e := Parse(append([]byte{0xef, 0xbb, 0xbf}, b...), window)
	if e != nil {
		t.Fatal(e)
	}
	for _, v := range s.Vendors {
		if v.ID == "HK-39372" {
			if v.Description != "Текст, с \"кавычками\"\nи новой строкой" || v.MaxHours != nil || v.Synthetic || len(v.BusyDates) != 0 {
				t.Fatalf("wrong parsed values: %+v", v)
			}
			return
		}
	}
	t.Fatal("vendor missing")
}

func TestEvidenceValidation(t *testing.T) {
	for _, kind := range []string{"stale hash", "quote absent", "unknown vendor", "not reviewed", "duplicate"} {
		t.Run(kind, func(t *testing.T) {
			s, e := Load("../../data/catalog.csv", "../../data/catalog.meta.json", "../../data/facts.json")
			if e != nil {
				t.Fatal(e)
			}
			switch kind {
			case "stale hash":
				s.Facts[0].DescriptionHash = "changed"
			case "quote absent":
				s.Facts[0].Quote = "UNSUPPORTED CLAIM"
			case "unknown vendor":
				s.Facts[0].VendorID = "missing"
			case "not reviewed":
				s.Facts[0].ReviewStatus = "pending"
			case "duplicate":
				s.Facts = append(s.Facts, s.Facts[0])
			}
			if Validate(s) == nil {
				t.Fatal("invalid evidence accepted")
			}
		})
	}
}
