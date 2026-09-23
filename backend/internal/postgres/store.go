package postgres

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/BAITC-Hacks/hack-1ad72463-apex-ai/backend/internal/catalog"
	"github.com/BAITC-Hacks/hack-1ad72463-apex-ai/backend/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrations embed.FS

var ErrUnavailable = errors.New("catalog unavailable")

type Store struct{ Pool *pgxpool.Pool }

func Open(ctx context.Context, url string) (*Store, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, errors.New("invalid DATABASE_URL")
	}
	cfg.MaxConns = 10
	cfg.ConnConfig.ConnectTimeout = 3 * time.Second
	p, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, errors.New("cannot initialize PostgreSQL pool")
	}
	return &Store{Pool: p}, nil
}
func (s *Store) Close() { s.Pool.Close() }

func (s *Store) Migrate(ctx context.Context) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(79479001)"); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, "CREATE TABLE IF NOT EXISTS schema_migrations (version text PRIMARY KEY, applied_at timestamptz NOT NULL DEFAULT now())"); err != nil {
		return err
	}
	entries, err := migrations.ReadDir("migrations")
	if err != nil {
		return err
	}
	for _, entry := range entries {
		var exists bool
		if err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version=$1)", entry.Name()).Scan(&exists); err != nil {
			return err
		}
		if exists {
			continue
		}
		b, e := migrations.ReadFile("migrations/" + entry.Name())
		if e != nil {
			return e
		}
		if _, err = tx.Exec(ctx, string(b)); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, "INSERT INTO schema_migrations(version) VALUES($1)", entry.Name()); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func contentHash(s domain.Snapshot) string { b, _ := json.Marshal(s); return domain.Hash(b) }

// Replace validates before starting a transaction and publishes all rows atomically.
func (s *Store) Replace(ctx context.Context, snapshot domain.Snapshot) error {
	if err := catalog.Validate(snapshot); err != nil {
		return err
	}
	snapshot.Vendors = slices.Clone(snapshot.Vendors)
	slices.SortFunc(snapshot.Vendors, func(a, b domain.Vendor) int { return strings.Compare(a.ID, b.ID) })
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(79479002)"); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, "DELETE FROM vendors"); err != nil {
		return err
	}
	for _, v := range snapshot.Vendors {
		keys := make([]string, len(v.Categories))
		for i, c := range v.Categories {
			keys[i] = domain.Normalize(c)
		}
		dates := make([]time.Time, len(v.BusyDates))
		for i, d := range v.BusyDates {
			dates[i], _ = time.Parse(time.DateOnly, d)
		}
		_, err = tx.Exec(ctx, `INSERT INTO vendors(id,anon_name,city,city_key,categories,category_keys,city_imputed,synthetic,price_from_kzt,price_imputed,event_formats,languages,max_hours,busy_dates,description) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)`, v.ID, v.Name, v.City, domain.Normalize(v.City), v.Categories, keys, v.CityImputed, v.Synthetic, v.Price, v.PriceImputed, v.Formats, v.Languages, v.MaxHours, dates, v.Description)
		if err != nil {
			return err
		}
	}
	meta, _ := json.Marshal(snapshot.Metadata)
	facts, _ := json.Marshal(snapshot.Facts)
	_, err = tx.Exec(ctx, `INSERT INTO catalog_state(id,metadata,facts,vendor_count,content_hash) VALUES(1,$1,$2,$3,$4) ON CONFLICT(id) DO UPDATE SET metadata=excluded.metadata,facts=excluded.facts,vendor_count=excluded.vendor_count,content_hash=excluded.content_hash,loaded_at=now()`, meta, facts, len(snapshot.Vendors), contentHash(snapshot))
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// Snapshot reads a consistent revision even while another process imports a catalog.
func (s *Store) Snapshot(ctx context.Context) (domain.Snapshot, error) {
	var snapshot domain.Snapshot
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return snapshot, ErrUnavailable
	}
	defer tx.Rollback(ctx)
	var meta, facts []byte
	var count int
	var hash string
	err = tx.QueryRow(ctx, "SELECT metadata,facts,vendor_count,content_hash FROM catalog_state WHERE id=1").Scan(&meta, &facts, &count, &hash)
	if err != nil {
		return snapshot, ErrUnavailable
	}
	if json.Unmarshal(meta, &snapshot.Metadata) != nil || json.Unmarshal(facts, &snapshot.Facts) != nil {
		return snapshot, ErrUnavailable
	}
	rows, err := tx.Query(ctx, `SELECT id,anon_name,city,categories,city_imputed,synthetic,price_from_kzt,price_imputed,event_formats,languages,max_hours,busy_dates,description FROM vendors ORDER BY id COLLATE "C"`)
	if err != nil {
		return snapshot, ErrUnavailable
	}
	snapshot.Vendors = []domain.Vendor{}
	for rows.Next() {
		var v domain.Vendor
		var dates []time.Time
		err = rows.Scan(&v.ID, &v.Name, &v.City, &v.Categories, &v.CityImputed, &v.Synthetic, &v.Price, &v.PriceImputed, &v.Formats, &v.Languages, &v.MaxHours, &dates, &v.Description)
		if err != nil {
			rows.Close()
			return snapshot, ErrUnavailable
		}
		v.BusyDates = []string{}
		for _, d := range dates {
			v.BusyDates = append(v.BusyDates, d.Format(time.DateOnly))
		}
		snapshot.Vendors = append(snapshot.Vendors, v)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return snapshot, ErrUnavailable
	}
	if len(snapshot.Vendors) != count || contentHash(snapshot) != hash {
		return snapshot, fmt.Errorf("%w: integrity check failed", ErrUnavailable)
	}
	if err = catalog.Validate(snapshot); err != nil {
		return snapshot, ErrUnavailable
	}
	if err = tx.Commit(ctx); err != nil {
		return snapshot, ErrUnavailable
	}
	return snapshot, nil
}
