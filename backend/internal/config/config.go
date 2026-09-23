package config

import (
	"errors"
	"os"
	"strings"
)

type Config struct {
	DatabaseURL, Address, CSVPath, MetaPath, FactsPath string
	Origins                                            []string
}

func Load() (Config, error) {
	c := Config{DatabaseURL: os.Getenv("DATABASE_URL"), Address: get("HTTP_ADDR", ":8080"), CSVPath: get("CATALOG_CSV", "data/catalog.csv"), MetaPath: get("CATALOG_META", "data/catalog.meta.json"), FactsPath: get("CATALOG_FACTS", "data/facts.json"), Origins: []string{}}
	if c.DatabaseURL == "" {
		return c, errors.New("DATABASE_URL is required")
	}
	if c.FactsPath == "-" {
		c.FactsPath = ""
	}
	for _, origin := range strings.Split(os.Getenv("CORS_ORIGINS"), ",") {
		if s := strings.TrimSpace(origin); s != "" {
			c.Origins = append(c.Origins, s)
		}
	}
	return c, nil
}
func get(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
