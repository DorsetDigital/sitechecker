package checker

import (
	"net/url"
	"strings"
	"testing"

	"golang.org/x/net/html"
)

func TestFirstParty(t *testing.T) {
	tests := []struct {
		page, asset string
		want        bool
	}{
		{"www.example.co.uk", "assets.example.co.uk", true},
		{"example.com", "static.example.com", true},
		{"example.com", "example.net", false},
		{"localhost", "localhost", true},
	}

	for _, tt := range tests {
		if got := isFirstParty(tt.page, tt.asset, nil); got != tt.want {
			t.Errorf("isFirstParty(%q, %q) = %v, want %v", tt.page, tt.asset, got, tt.want)
		}
	}
}

func TestExtractAssets(t *testing.T) {
	base, _ := url.Parse("https://www.example.com/path/")
	doc, err := html.Parse(strings.NewReader(`<!doctype html>
<html><head>
<link rel="stylesheet" href="/app.css">
<link rel="preload" href="/font.woff2" as="font">
<script src="https://www.example.com/app.js"></script>
</head><body>
<img src="image.jpg" srcset="image-2x.jpg 2x, image-3x.jpg 3x">
<img src="data:image/png;base64,abc">
</body></html>`))
	if err != nil {
		t.Fatal(err)
	}

	got := extractAssets(base, doc)
	if len(got) != 6 {
		t.Fatalf("got %d assets, want 6: %#v", len(got), got)
	}
}
