# Sitechecker

A lightweight first-stage website smoke tester for deployment and hosting-platform checks.

Sitechecker visits a list of sites, validates their HTTPS connection, checks the final HTTP response, discovers first-party assets in the returned HTML, and verifies that those assets can also be fetched successfully over valid TLS.

It is designed to answer a simple post-deployment question quickly: **are the sites up, serving valid HTTPS, and returning their own assets successfully?**

## Checks

For each site Sitechecker checks:

- URL resolution and HTTP connectivity
- redirects, with the final URL recorded
- HTTPS/TLS certificate validation
- certificate expiry, with a configurable warning threshold
- final page response is 2xx
- response is HTML
- first-party scripts
- first-party stylesheets
- first-party images and `srcset` candidates
- first-party preload/modulepreload/icon resources
- first-party video/audio/source resources
- first-party assets use HTTPS
- first-party asset responses are 2xx
- first-party asset TLS is valid
- DNS, connect, TLS, TTFB and total page-check timings

First-party detection uses the registrable domain, so hosts such as `www.example.co.uk` and `assets.example.co.uk` are treated as belonging to the same site. Additional first-party hosts can be configured per site.

This is intentionally an HTTP-level smoke test rather than a browser test. It does not execute JavaScript or attempt to reproduce a full browser rendering.

## Docker

Check one or more URLs directly:

```bash
docker run --rm ghcr.io/dorsetdigital/sitechecker:latest \
  https://www.example.com \
  https://www.example.org
```

Or use a configuration file:

```bash
docker run --rm \
  -v "$PWD/sites.yml:/config/sites.yml:ro" \
  ghcr.io/dorsetdigital/sitechecker:latest \
  -config /config/sites.yml
```

To write a JSON report to the current directory:

```bash
docker run --rm \
  -v "$PWD:/work" \
  ghcr.io/dorsetdigital/sitechecker:latest \
  -config /work/sites.yml \
  -json /work/report.json
```

The container is published for `linux/amd64` and `linux/arm64`.

## Local usage

Requires Go 1.27 or newer.

```bash
go run ./cmd/sitechecker https://www.example.com
```

Build a standalone binary:

```bash
go build -o sitechecker ./cmd/sitechecker
./sitechecker https://www.example.com
```

## Configuration

See [examples/sites.yml](examples/sites.yml).

```yaml
defaults:
  timeout: 10s
  asset_timeout: 5s
  concurrency: 10
  asset_concurrency: 6
  max_assets: 100
  cert_warning_days: 14
  max_html_bytes: 10485760
  user_agent: "DorsetDigital-Sitechecker/dev"

sites:
  - https://www.example.com

  - url: https://www.example.org
    first_party_hosts:
      - static.example.net
    ignore_assets:
      - "/optional/*"
```

A site can therefore be either a simple URL or an object with site-specific options.

### Environment overrides

The following environment variables are supported:

- `SITECHECK_CONCURRENCY`
- `SITECHECK_TIMEOUT`
- `SITECHECK_ASSET_TIMEOUT`

Command-line `-concurrency` takes precedence over configuration and environment values.

## Output

Normal terminal mode reports each site as soon as it completes so long runs visibly make progress:

```text
Checking 2 site(s)...

[1/2] OK   www.example.com                      HTTP 200  4/4 assets       241ms
[2/2] FAIL www.example.org                      HTTP 200  6/7 assets FAIL  318ms

SITE             TLS  HTTP  ASSETS    TIME
www.example.com  OK   200   4/4 OK    241ms
www.example.org  OK   200   6/7 FAIL  318ms

2 sites tested, 1 passed, 1 failed

www.example.org
  FAIL 404 https://www.example.org/assets/missing.css (stylesheet)
```

Detailed machine-readable output can be written with:

```bash
sitechecker -config sites.yml -json report.json
```

Use `-json -` to output only JSON to stdout. Live progress output is suppressed in this mode so stdout remains valid machine-readable JSON.

## Exit codes

- `0` — every site passed
- `1` — one or more sites failed
- `2` — invalid configuration or a Sitechecker execution/output error

This makes Sitechecker suitable for use as an early CI/CD deployment gate.

## Notes

Sitechecker performs real `GET` requests rather than `HEAD` requests because servers and CDNs can behave differently for `HEAD`. Asset response bodies are only partially drained; the smoke test is concerned with successful HTTP/TLS delivery rather than downloading every asset in full.

TLS connections require TLS 1.2 or newer.

External third-party resources are not tested by default.

## Development

```bash
go test ./...
go vet ./...
go build ./cmd/sitechecker
```

## License

MIT.
