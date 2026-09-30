package checker

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/DorsetDigital/sitechecker/internal/config"
	"golang.org/x/net/html"
)

func CheckAll(cfg config.Config) []Result {
	results := make([]Result, len(cfg.Sites))
	workers := cfg.Defaults.Concurrency
	if workers > len(cfg.Sites) {
		workers = len(cfg.Sites)
	}
	if workers < 1 {
		workers = 1
	}

	jobs := make(chan int)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range jobs {
				results[idx] = CheckSite(cfg.Sites[idx], cfg.Defaults)
			}
		}()
	}

	for i := range cfg.Sites {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	return results
}

func CheckSite(site config.Site, defaults config.Defaults) Result {
	result := Result{URL: site.URL}
	start := time.Now()

	target, err := normaliseURL(site.URL)
	if err != nil {
		result.Error = err.Error()
		result.Timing.Total = time.Since(start)
		return result
	}

	var (
		dnsStart, connectStart, tlsStart time.Time
		firstByte                        time.Time
		redirects                        []string
	)

	trace := &httptrace.ClientTrace{
		DNSStart: func(httptrace.DNSStartInfo) { dnsStart = time.Now() },
		DNSDone: func(httptrace.DNSDoneInfo) {
			if !dnsStart.IsZero() {
				result.Timing.DNS = time.Since(dnsStart)
			}
		},
		ConnectStart: func(_, _ string) { connectStart = time.Now() },
		ConnectDone: func(_, _ string, _ error) {
			if !connectStart.IsZero() {
				result.Timing.Connect = time.Since(connectStart)
			}
		},
		TLSHandshakeStart: func() { tlsStart = time.Now() },
		TLSHandshakeDone: func(_ tls.ConnectionState, _ error) {
			if !tlsStart.IsZero() {
				result.Timing.TLS = time.Since(tlsStart)
			}
		},
		GotFirstResponseByte: func() { firstByte = time.Now() },
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	client := &http.Client{
		Transport: transport,
		Timeout:   defaults.Timeout.Duration,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			redirects = append(redirects, req.URL.String())
			if len(via) >= 10 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}

	ctx := httptrace.WithClientTrace(context.Background(), trace)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		result.Error = err.Error()
		result.Timing.Total = time.Since(start)
		return result
	}
	req.Header.Set("User-Agent", defaults.UserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml")

	resp, err := client.Do(req)
	if err != nil {
		result.Error = err.Error()
		result.Redirects = redirects
		result.Timing.Total = time.Since(start)
		return result
	}
	defer resp.Body.Close()

	result.FinalURL = resp.Request.URL.String()
	result.StatusCode = resp.StatusCode
	result.Redirects = redirects
	result.ContentType = resp.Header.Get("Content-Type")
	result.TLS = tlsResult(resp, defaults.CertWarningDays)
	if result.TLS != nil && result.TLS.Warning != "" {
		result.Warnings = append(result.Warnings, result.TLS.Warning)
	}
	if !firstByte.IsZero() {
		result.Timing.TTFB = firstByte.Sub(start)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		result.Error = fmt.Sprintf("unexpected HTTP status %d", resp.StatusCode)
		result.Timing.Total = time.Since(start)
		return result
	}

	mediaType, _, _ := mime.ParseMediaType(result.ContentType)
	if mediaType != "" && mediaType != "text/html" && mediaType != "application/xhtml+xml" {
		result.Error = fmt.Sprintf("unexpected content type %q", mediaType)
		result.Timing.Total = time.Since(start)
		return result
	}

	limited := io.LimitReader(resp.Body, defaults.MaxHTMLBytes+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		result.Error = fmt.Sprintf("could not read HTML: %v", err)
		result.Timing.Total = time.Since(start)
		return result
	}
	if int64(len(body)) > defaults.MaxHTMLBytes {
		result.Error = fmt.Sprintf("HTML exceeds max_html_bytes (%d)", defaults.MaxHTMLBytes)
		result.Timing.Total = time.Since(start)
		return result
	}

	root, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		result.Error = fmt.Sprintf("could not parse HTML: %v", err)
		result.Timing.Total = time.Since(start)
		return result
	}

	assets := extractAssets(resp.Request.URL, root)
	firstParty := make([]assetRef, 0, len(assets))
	for _, asset := range assets {
		if !isFirstParty(resp.Request.URL.Hostname(), asset.URL.Hostname(), site.FirstPartyHosts) {
			continue
		}
		if ignoredAsset(asset.URL, site.IgnoreAssets) {
			continue
		}
		firstParty = append(firstParty, asset)
	}

	if len(firstParty) > defaults.MaxAssets {
		firstParty = firstParty[:defaults.MaxAssets]
		result.AssetsTruncated = true
		result.Warnings = append(result.Warnings, fmt.Sprintf("asset checks limited to %d", defaults.MaxAssets))
	}

	assetResults := checkAssets(firstParty, defaults)
	result.AssetsChecked = len(assetResults)
	for _, ar := range assetResults {
		if ar.OK {
			result.AssetsPassed++
		} else {
			result.AssetFailures = append(result.AssetFailures, ar)
		}
	}

	result.OK = len(result.AssetFailures) == 0
	result.Timing.Total = time.Since(start)
	return result
}

func checkAssets(assets []assetRef, defaults config.Defaults) []AssetResult {
	results := make([]AssetResult, len(assets))
	if len(assets) == 0 {
		return results
	}

	workers := defaults.AssetConcurrency
	if workers > len(assets) {
		workers = len(assets)
	}
	jobs := make(chan int)
	var wg sync.WaitGroup

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			client := assetClient(defaults.AssetTimeout.Duration)
			for idx := range jobs {
				results[idx] = checkAsset(client, assets[idx], defaults)
			}
		}()
	}
	for i := range assets {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	return results
}

func assetClient(timeout time.Duration) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	return &http.Client{Transport: transport, Timeout: timeout}
}

func checkAsset(client *http.Client, asset assetRef, defaults config.Defaults) AssetResult {
	out := AssetResult{URL: asset.URL.String(), Type: asset.Type}

	if asset.URL.Scheme != "https" {
		out.Error = "first-party asset is not using HTTPS"
		return out
	}

	req, err := http.NewRequest(http.MethodGet, asset.URL.String(), nil)
	if err != nil {
		out.Error = err.Error()
		return out
	}
	req.Header.Set("User-Agent", defaults.UserAgent)
	req.Header.Set("Accept", "*/*")

	resp, err := client.Do(req)
	if err != nil {
		out.Error = err.Error()
		return out
	}
	defer resp.Body.Close()
	_, _ = io.CopyN(io.Discard, resp.Body, 32*1024)

	out.StatusCode = resp.StatusCode
	out.TLS = tlsResult(resp, defaults.CertWarningDays)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		out.Error = fmt.Sprintf("unexpected HTTP status %d", resp.StatusCode)
		return out
	}
	if out.TLS == nil || !out.TLS.Valid {
		out.Error = "TLS validation unavailable or failed"
		return out
	}
	out.OK = true
	return out
}

func normaliseURL(raw string) (*url.URL, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("empty URL")
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("unsupported URL scheme %q", u.Scheme)
	}
	if u.Hostname() == "" {
		return nil, fmt.Errorf("URL has no hostname")
	}
	return u, nil
}

func tlsResult(resp *http.Response, warningDays int) *TLSResult {
	if resp.Request == nil || resp.Request.URL == nil || resp.Request.URL.Scheme != "https" {
		return nil
	}
	if resp.TLS == nil || len(resp.TLS.PeerCertificates) == 0 {
		return &TLSResult{Valid: false}
	}
	cert := resp.TLS.PeerCertificates[0]
	days := int(time.Until(cert.NotAfter).Hours() / 24)
	out := &TLSResult{
		Valid:      resp.TLS.HandshakeComplete && len(resp.TLS.VerifiedChains) > 0,
		ServerName: cert.Subject.CommonName,
		Issuer:     cert.Issuer.CommonName,
		NotAfter:   cert.NotAfter,
		DaysLeft:   days,
	}
	if warningDays > 0 && days < warningDays {
		out.Warning = fmt.Sprintf("TLS certificate expires in %d day(s)", days)
	}
	return out
}
