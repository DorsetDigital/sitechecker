package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/DorsetDigital/sitechecker/internal/checker"
	"github.com/DorsetDigital/sitechecker/internal/config"
	"github.com/DorsetDigital/sitechecker/internal/report"
)

var version = "dev"

func main() {
	os.Exit(run())
}

func run() int {
	var (
		configPath  string
		jsonPath    string
		concurrency int
		showVersion bool
	)

	flag.StringVar(&configPath, "config", "", "YAML configuration file")
	flag.StringVar(&jsonPath, "json", "", "write JSON report to this path (use - for stdout)")
	flag.IntVar(&concurrency, "concurrency", 0, "override site concurrency")
	flag.BoolVar(&showVersion, "version", false, "print version and exit")
	flag.Parse()

	if showVersion {
		fmt.Println(version)
		return 0
	}

	cfg := config.Default()
	if configPath != "" {
		loaded, err := config.Load(configPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "configuration error: %v\n", err)
			return 2
		}
		cfg = loaded
	}

	for _, raw := range flag.Args() {
		cfg.Sites = append(cfg.Sites, config.Site{URL: raw})
	}

	applyEnv(&cfg)
	if concurrency > 0 {
		cfg.Defaults.Concurrency = concurrency
	}

	if len(cfg.Sites) == 0 {
		fmt.Fprintln(os.Stderr, "no sites supplied; pass URLs as arguments or use -config")
		flag.Usage()
		return 2
	}

	started := time.Now()

	var progress checker.ProgressFunc
	if jsonPath != "-" {
		fmt.Fprintf(os.Stdout, "Checking %d site(s)...\n\n", len(cfg.Sites))
		progress = func(completed, total int, result checker.Result) {
			report.PrintProgress(os.Stdout, completed, total, result)
		}
	}

	results := checker.CheckAllWithProgress(cfg, progress)
	summary := report.Summarise(results, started)

	if jsonPath != "-" {
		report.Print(os.Stdout, results, summary)
	}

	if jsonPath != "" {
		data, err := json.MarshalIndent(summary, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "could not encode JSON report: %v\n", err)
			return 2
		}
		data = append(data, '\n')

		if jsonPath == "-" {
			if _, err := os.Stdout.Write(data); err != nil {
				fmt.Fprintf(os.Stderr, "could not write JSON report: %v\n", err)
				return 2
			}
		} else if err := os.WriteFile(jsonPath, data, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "could not write JSON report: %v\n", err)
			return 2
		}
	}

	if summary.Failed > 0 {
		return 1
	}
	return 0
}

func applyEnv(cfg *config.Config) {
	if v := os.Getenv("SITECHECK_CONCURRENCY"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.Defaults.Concurrency = n
		}
	}
	if v := os.Getenv("SITECHECK_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.Defaults.Timeout = config.Duration{Duration: d}
		}
	}
	if v := os.Getenv("SITECHECK_ASSET_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.Defaults.AssetTimeout = config.Duration{Duration: d}
		}
	}
}
