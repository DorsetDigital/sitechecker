package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Duration struct {
	time.Duration
}

func (d *Duration) UnmarshalYAML(node *yaml.Node) error {
	parsed, err := time.ParseDuration(node.Value)
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", node.Value, err)
	}
	d.Duration = parsed
	return nil
}

func (d Duration) MarshalYAML() (interface{}, error) {
	return d.String(), nil
}

type Defaults struct {
	Timeout          Duration `yaml:"timeout"`
	AssetTimeout     Duration `yaml:"asset_timeout"`
	Concurrency      int      `yaml:"concurrency"`
	AssetConcurrency int      `yaml:"asset_concurrency"`
	MaxAssets        int      `yaml:"max_assets"`
	CertWarningDays  int      `yaml:"cert_warning_days"`
	MaxHTMLBytes     int64    `yaml:"max_html_bytes"`
	UserAgent        string   `yaml:"user_agent"`
}

type Site struct {
	URL             string   `yaml:"url"`
	FirstPartyHosts []string `yaml:"first_party_hosts,omitempty"`
	IgnoreAssets    []string `yaml:"ignore_assets,omitempty"`
}

func (s *Site) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.ScalarNode {
		s.URL = node.Value
		return nil
	}
	type plain Site
	var p plain
	if err := node.Decode(&p); err != nil {
		return err
	}
	*s = Site(p)
	return nil
}

type Config struct {
	Defaults Defaults `yaml:"defaults"`
	Sites    []Site   `yaml:"sites"`
}

func Default() Config {
	return Config{
		Defaults: Defaults{
			Timeout:          Duration{Duration: 10 * time.Second},
			AssetTimeout:     Duration{Duration: 5 * time.Second},
			Concurrency:      10,
			AssetConcurrency: 6,
			MaxAssets:        100,
			CertWarningDays:  14,
			MaxHTMLBytes:     10 << 20,
			UserAgent:        "DorsetDigital-Sitechecker/dev",
		},
	}
}

func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	cfg := Default()
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) Validate() error {
	if c.Defaults.Timeout.Duration <= 0 {
		return fmt.Errorf("timeout must be greater than zero")
	}
	if c.Defaults.AssetTimeout.Duration <= 0 {
		return fmt.Errorf("asset_timeout must be greater than zero")
	}
	if c.Defaults.Concurrency < 1 {
		return fmt.Errorf("concurrency must be at least 1")
	}
	if c.Defaults.AssetConcurrency < 1 {
		return fmt.Errorf("asset_concurrency must be at least 1")
	}
	if c.Defaults.MaxAssets < 1 {
		return fmt.Errorf("max_assets must be at least 1")
	}
	if c.Defaults.MaxHTMLBytes < 1024 {
		return fmt.Errorf("max_html_bytes must be at least 1024")
	}
	for i, site := range c.Sites {
		if site.URL == "" {
			return fmt.Errorf("site %d has no URL", i+1)
		}
	}
	return nil
}
