package report

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/DorsetDigital/sitechecker/internal/checker"
)

type Summary struct {
	StartedAt time.Time        `json:"started_at"`
	Duration  time.Duration    `json:"duration"`
	Total     int              `json:"total"`
	Passed    int              `json:"passed"`
	Failed    int              `json:"failed"`
	Results   []checker.Result `json:"results"`
}

func Summarise(results []checker.Result, started time.Time) Summary {
	s := Summary{
		StartedAt: started.UTC(),
		Duration:  time.Since(started),
		Total:     len(results),
		Results:   results,
	}
	for _, r := range results {
		if r.OK {
			s.Passed++
		} else {
			s.Failed++
		}
	}
	return s
}

func PrintProgress(w io.Writer, completed, total int, r checker.Result) {
	status := "OK"
	if !r.OK {
		status = "FAIL"
	}
	httpState := "-"
	if r.StatusCode > 0 {
		httpState = fmt.Sprintf("%d", r.StatusCode)
	}
	assets := fmt.Sprintf("%d/%d assets", r.AssetsPassed, r.AssetsChecked)
	if len(r.AssetFailures) > 0 {
		assets += " FAIL"
	}
	fmt.Fprintf(w, "[%d/%d] %-4s %-36s HTTP %-3s  %-16s %s\n",
		completed, total, status, displaySite(r), httpState, assets, roundDuration(r.Timing.Total))
}

func Print(w io.Writer, results []checker.Result, summary Summary) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "SITE\tTLS\tHTTP\tASSETS\tTIME")
	for _, r := range results {
		tlsState := "-"
		if r.TLS != nil {
			if r.TLS.Valid {
				tlsState = "OK"
			} else {
				tlsState = "FAIL"
			}
		}
		httpState := "-"
		if r.StatusCode > 0 {
			httpState = fmt.Sprintf("%d", r.StatusCode)
		}
		assets := fmt.Sprintf("%d/%d OK", r.AssetsPassed, r.AssetsChecked)
		if len(r.AssetFailures) > 0 {
			assets = fmt.Sprintf("%d/%d FAIL", r.AssetsPassed, r.AssetsChecked)
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n",
			displaySite(r), tlsState, httpState, assets, roundDuration(r.Timing.Total))
	}
	_ = tw.Flush()

	fmt.Fprintf(w, "\n%d sites tested, %d passed, %d failed\n", summary.Total, summary.Passed, summary.Failed)

	for _, r := range results {
		if r.OK && len(r.Warnings) == 0 {
			continue
		}
		fmt.Fprintf(w, "\n%s\n", displaySite(r))
		if r.Error != "" {
			fmt.Fprintf(w, "  FAIL %s\n", r.Error)
		}
		for _, warning := range r.Warnings {
			fmt.Fprintf(w, "  WARN %s\n", warning)
		}
		for _, asset := range r.AssetFailures {
			if asset.StatusCode > 0 {
				fmt.Fprintf(w, "  FAIL %d %s (%s)\n", asset.StatusCode, asset.URL, asset.Type)
			} else {
				fmt.Fprintf(w, "  FAIL %s: %s (%s)\n", asset.URL, asset.Error, asset.Type)
			}
		}
	}
}

func displaySite(r checker.Result) string {
	s := strings.TrimPrefix(strings.TrimPrefix(r.URL, "https://"), "http://")
	return strings.TrimSuffix(s, "/")
}

func roundDuration(d time.Duration) time.Duration {
	if d < time.Millisecond {
		return d.Round(time.Microsecond)
	}
	return d.Round(time.Millisecond)
}
