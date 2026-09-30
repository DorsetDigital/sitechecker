package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadScalarAndObjectSites(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sites.yml")
	data := []byte(`defaults:
  concurrency: 3
sites:
  - https://www.example.com
  - url: https://www.example.org
    first_party_hosts:
      - static.example.net
`)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Sites) != 2 {
		t.Fatalf("got %d sites, want 2", len(cfg.Sites))
	}
	if cfg.Defaults.Concurrency != 3 {
		t.Fatalf("got concurrency %d, want 3", cfg.Defaults.Concurrency)
	}
	if cfg.Defaults.Timeout.Duration == 0 {
		t.Fatal("default timeout was lost during YAML merge")
	}
}
