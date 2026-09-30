package checker

import (
	"fmt"
	"net"
	"net/url"
	"path"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/publicsuffix"
)

type assetRef struct {
	URL  *url.URL
	Type string
}

func extractAssets(base *url.URL, root *html.Node) []assetRef {
	seen := map[string]bool{}
	var out []assetRef

	add := func(raw, kind string) {
		raw = strings.TrimSpace(raw)
		if raw == "" || strings.HasPrefix(raw, "data:") || strings.HasPrefix(raw, "blob:") {
			return
		}
		u, err := url.Parse(raw)
		if err != nil {
			return
		}
		u = base.ResolveReference(u)
		if u.Scheme != "http" && u.Scheme != "https" {
			return
		}
		u.Fragment = ""
		key := u.String()
		if seen[key] {
			return
		}
		seen[key] = true
		out = append(out, assetRef{URL: u, Type: kind})
	}

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			attrs := attrs(n)
			switch n.Data {
			case "script":
				add(attrs["src"], "script")
			case "img":
				add(attrs["src"], "image")
				for _, raw := range parseSrcset(attrs["srcset"]) {
					add(raw, "image")
				}
			case "source":
				add(attrs["src"], "source")
				for _, raw := range parseSrcset(attrs["srcset"]) {
					add(raw, "source")
				}
			case "video":
				add(attrs["src"], "video")
				add(attrs["poster"], "image")
			case "audio":
				add(attrs["src"], "audio")
			case "link":
				rel := strings.ToLower(attrs["rel"])
				if strings.Contains(rel, "stylesheet") {
					add(attrs["href"], "stylesheet")
				} else if strings.Contains(rel, "preload") || strings.Contains(rel, "modulepreload") || strings.Contains(rel, "icon") {
					add(attrs["href"], "link")
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	return out
}

func attrs(n *html.Node) map[string]string {
	out := make(map[string]string, len(n.Attr))
	for _, a := range n.Attr {
		out[strings.ToLower(a.Key)] = a.Val
	}
	return out
}

func parseSrcset(srcset string) []string {
	var out []string
	for _, candidate := range strings.Split(srcset, ",") {
		fields := strings.Fields(strings.TrimSpace(candidate))
		if len(fields) > 0 {
			out = append(out, fields[0])
		}
	}
	return out
}

func isFirstParty(pageHost, assetHost string, additional []string) bool {
	pageHost = hostname(pageHost)
	assetHost = hostname(assetHost)
	if pageHost == "" || assetHost == "" {
		return false
	}
	if pageHost == assetHost {
		return true
	}

	pageRoot, pageErr := publicsuffix.EffectiveTLDPlusOne(pageHost)
	assetRoot, assetErr := publicsuffix.EffectiveTLDPlusOne(assetHost)
	if pageErr == nil && assetErr == nil && strings.EqualFold(pageRoot, assetRoot) {
		return true
	}

	for _, allowed := range additional {
		allowed = hostname(allowed)
		if assetHost == allowed || strings.HasSuffix(assetHost, "."+allowed) {
			return true
		}
	}
	return false
}

func hostname(raw string) string {
	raw = strings.TrimSpace(strings.ToLower(raw))
	if strings.Contains(raw, "://") {
		if u, err := url.Parse(raw); err == nil {
			raw = u.Hostname()
		}
	}
	if host, _, err := net.SplitHostPort(raw); err == nil {
		raw = host
	}
	return strings.TrimSuffix(raw, ".")
}

func ignoredAsset(u *url.URL, patterns []string) bool {
	for _, pattern := range patterns {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}
		if ok, err := path.Match(pattern, u.Path); err == nil && ok {
			return true
		}
		if strings.Contains(u.String(), pattern) {
			return true
		}
	}
	return false
}

func assetLabel(a assetRef) string {
	return fmt.Sprintf("%s %s", a.Type, a.URL.String())
}
