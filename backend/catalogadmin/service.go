// Package catalogadmin exposes the existing catalog rules to the separate admin
// application. Public recommendation handlers never expose mutation endpoints.
package catalogadmin

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/BAITC-Hacks/hack-1ad72463-apex-ai/backend/internal/catalog"
	"github.com/BAITC-Hacks/hack-1ad72463-apex-ai/backend/internal/domain"
	"github.com/BAITC-Hacks/hack-1ad72463-apex-ai/backend/internal/postgres"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Vendor = domain.Vendor
type Snapshot = domain.Snapshot
type Metadata = domain.Metadata
type Window = domain.Window
type Fact = domain.Fact

var Formats = slices.Clone(domain.Formats)
var Languages = slices.Clone(domain.Languages)
var ErrConflict = postgres.ErrConflict
var ErrNotFound = errors.New("record not found")
var ErrDuplicate = errors.New("ID already exists")
var ErrLastVendor = errors.New("cannot delete the last catalog record")
var ErrInvalid = errors.New("invalid catalog data")

type Service struct{ store *postgres.Store }

func New(pool *pgxpool.Pool) *Service                             { return &Service{store: &postgres.Store{Pool: pool}} }
func (s *Service) Snapshot(ctx context.Context) (Snapshot, error) { return s.store.Snapshot(ctx) }
func Revision(s Snapshot) string                                  { return postgres.Revision(s) }

var validID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,79}$`)

func ParseCSV(b []byte, w Window) (Snapshot, error) {
	s, err := catalog.Parse(b, w)
	if err != nil {
		return s, err
	}
	for _, v := range s.Vendors {
		if !validID.MatchString(v.ID) {
			return s, fmt.Errorf("invalid ID %q: use 1–80 Latin letters, digits, underscore or hyphen", v.ID)
		}
	}
	return s, nil
}
func Load(csvPath, metaPath, factsPath string) (Snapshot, error) {
	return catalog.Load(csvPath, metaPath, factsPath)
}

// Bootstrap is used by local setup and tests, never by the admin HTTP server.
func (s *Service) Bootstrap(ctx context.Context, snapshot Snapshot) error {
	if err := s.store.Migrate(ctx); err != nil {
		return err
	}
	return s.store.Replace(ctx, snapshot)
}

var Headers = []string{"id", "anon_name", "categories", "city", "city_imputed", "synthetic", "price_from_kzt", "price_imputed", "event_formats", "languages", "max_hours", "busy_dates", "description"}

func CSV(vendors []Vendor) []byte {
	var b bytes.Buffer
	w := csv.NewWriter(&b)
	_ = w.Write(Headers)
	sorted := slices.Clone(vendors)
	slices.SortFunc(sorted, func(a, b Vendor) int { return strings.Compare(a.ID, b.ID) })
	flag := func(v bool) string {
		if v {
			return "True"
		}
		return "False"
	}
	for _, v := range sorted {
		hours := ""
		if v.MaxHours != nil {
			hours = strconv.FormatFloat(*v.MaxHours, 'f', -1, 64)
		}
		_ = w.Write([]string{v.ID, v.Name, strings.Join(v.Categories, "|"), v.City, flag(v.CityImputed), flag(v.Synthetic), strconv.FormatInt(v.Price, 10), flag(v.PriceImputed), strings.Join(v.Formats, "|"), strings.Join(v.Languages, "|"), hours, strings.Join(v.BusyDates, "|"), v.Description})
	}
	w.Flush()
	return b.Bytes()
}

type Change struct {
	Action   string
	Actor    string
	Expected string
	ID       string
	Vendors  []Vendor
	Upsert   bool
}

func (s *Service) Apply(ctx context.Context, c Change) (Snapshot, error) {
	return s.store.Update(ctx, c.Expected, func(snapshot *Snapshot, tx pgx.Tx) error {
		before := Revision(*snapshot)
		ids := []string{}
		switch c.Action {
		case "delete":
			at := slices.IndexFunc(snapshot.Vendors, func(v Vendor) bool { return v.ID == c.ID })
			if at < 0 {
				return ErrNotFound
			}
			if len(snapshot.Vendors) == 1 {
				return ErrLastVendor
			}
			ids = append(ids, c.ID)
			snapshot.Vendors = slices.Delete(snapshot.Vendors, at, at+1)
		case "create", "update", "import":
			if len(c.Vendors) == 0 {
				return fmt.Errorf("%w: no records", ErrInvalid)
			}
			// The same CSV parser normalizes sets and checks all typed fields for manual input.
			normalized, err := ParseCSV(CSV(c.Vendors), snapshot.Metadata.CalendarWindow)
			if err != nil {
				return fmt.Errorf("%w: %s", ErrInvalid, err)
			}
			for _, v := range normalized.Vendors {
				if c.Action == "update" && (len(normalized.Vendors) != 1 || v.ID != c.ID) {
					return fmt.Errorf("%w: ID is immutable", ErrInvalid)
				}
				at := slices.IndexFunc(snapshot.Vendors, func(old Vendor) bool { return old.ID == v.ID })
				if at >= 0 && (c.Action == "create" || (c.Action == "import" && !c.Upsert)) {
					return fmt.Errorf("%w: %s", ErrDuplicate, v.ID)
				}
				if at < 0 && c.Action == "update" {
					return ErrNotFound
				}
				if at >= 0 {
					snapshot.Vendors[at] = v
				} else {
					snapshot.Vendors = append(snapshot.Vendors, v)
				}
				ids = append(ids, v.ID)
			}
		default:
			return fmt.Errorf("%w: unknown action", ErrInvalid)
		}
		// Retain evidence only when its vendor and exact source description still exist.
		current := map[string]Vendor{}
		for _, v := range snapshot.Vendors {
			current[v.ID] = v
		}
		snapshot.Facts = slices.DeleteFunc(snapshot.Facts, func(f Fact) bool {
			v, ok := current[f.VendorID]
			return !ok || domain.Hash([]byte(v.Description)) != f.DescriptionHash
		})
		snapshot.Metadata.SnapshotVersion = domain.Hash(CSV(snapshot.Vendors))
		facts, _ := json.Marshal(snapshot.Facts)
		snapshot.Metadata.FactsVersion = domain.Hash(facts)
		slices.SortFunc(snapshot.Vendors, func(a, b Vendor) int { return strings.Compare(a.ID, b.ID) })
		if err := catalog.Validate(*snapshot); err != nil {
			return fmt.Errorf("%w: %s", ErrInvalid, err)
		}
		_, err := tx.Exec(ctx, `INSERT INTO admin_panel_audit(actor,action,vendor_ids,before_revision,after_revision) VALUES($1,$2,$3,$4,$5)`, c.Actor, c.Action, ids, before, Revision(*snapshot))
		return err
	})
}
