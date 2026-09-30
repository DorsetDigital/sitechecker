package checker

import "time"

type Timing struct {
	DNS     time.Duration `json:"dns,omitempty"`
	Connect time.Duration `json:"connect,omitempty"`
	TLS     time.Duration `json:"tls,omitempty"`
	TTFB    time.Duration `json:"ttfb,omitempty"`
	Total   time.Duration `json:"total"`
}

type TLSResult struct {
	Valid       bool      `json:"valid"`
	ServerName  string    `json:"server_name,omitempty"`
	Issuer      string    `json:"issuer,omitempty"`
	NotAfter    time.Time `json:"not_after,omitempty"`
	DaysLeft    int       `json:"days_left,omitempty"`
	Warning     string    `json:"warning,omitempty"`
}

type AssetResult struct {
	URL        string     `json:"url"`
	Type       string     `json:"type"`
	StatusCode int        `json:"status_code,omitempty"`
	OK         bool       `json:"ok"`
	TLS        *TLSResult `json:"tls,omitempty"`
	Error      string     `json:"error,omitempty"`
}

type Result struct {
	URL              string        `json:"url"`
	FinalURL         string        `json:"final_url,omitempty"`
	StatusCode       int           `json:"status_code,omitempty"`
	OK               bool          `json:"ok"`
	ContentType      string        `json:"content_type,omitempty"`
	Redirects        []string      `json:"redirects,omitempty"`
	TLS              *TLSResult    `json:"tls,omitempty"`
	Timing           Timing        `json:"timing"`
	AssetsChecked    int           `json:"assets_checked"`
	AssetsPassed     int           `json:"assets_passed"`
	AssetsTruncated  bool          `json:"assets_truncated,omitempty"`
	AssetFailures    []AssetResult `json:"asset_failures,omitempty"`
	Error            string        `json:"error,omitempty"`
	Warnings         []string      `json:"warnings,omitempty"`
}
