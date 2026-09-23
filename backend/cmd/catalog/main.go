package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/BAITC-Hacks/hack-1ad72463-apex-ai/backend/internal/catalog"
	"github.com/BAITC-Hacks/hack-1ad72463-apex-ai/backend/internal/config"
	"github.com/BAITC-Hacks/hack-1ad72463-apex-ai/backend/internal/postgres"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	if len(os.Args) != 2 || (os.Args[1] != "migrate" && os.Args[1] != "import" && os.Args[1] != "check" && os.Args[1] != "bootstrap") {
		return fmt.Errorf("usage: catalog migrate|import|check|bootstrap")
	}
	cfg, err := config.Load()
	if err != nil && os.Args[1] != "check" {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if os.Args[1] == "check" {
		s, e := catalog.Load(cfg.CSVPath, cfg.MetaPath, cfg.FactsPath)
		if e != nil {
			return e
		}
		return json.NewEncoder(os.Stdout).Encode(map[string]any{"vendors": len(s.Vendors), "facts": len(s.Facts), "metadata": s.Metadata})
	}
	store, err := postgres.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer store.Close()
	if os.Args[1] == "migrate" {
		if err = store.Migrate(ctx); err != nil {
			return fmt.Errorf("migration failed: %w", err)
		}
		fmt.Println("migrations applied")
		return nil
	}
	s, err := catalog.Load(cfg.CSVPath, cfg.MetaPath, cfg.FactsPath)
	if err != nil {
		return err
	}
	if os.Args[1] == "bootstrap" {
		if err = store.Migrate(ctx); err != nil {
			return fmt.Errorf("migration failed: %w", err)
		}
	}
	if err = store.Replace(ctx, s); err != nil {
		return fmt.Errorf("catalog import failed: %w", err)
	}
	fmt.Printf("imported %d vendors, %d facts, %s\n", len(s.Vendors), len(s.Facts), s.Metadata.SnapshotVersion)
	return nil
}
