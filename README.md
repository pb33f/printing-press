![printing-press logo](.github/assets/printing-press-logo.png)

# Printing Press

---
> [!WARNING]
> Important: this tool is currently in preview and is not intended for production use. 
---

OpenAPI Documentation that is: 

- High Quality
- Designed for Agents and Humans
- Beautiful & Clean
- Detailed & Rich
- Fast & Offline
- Instant & Complete

## Run directly from source

```bash
go run printing-press.go path/to/openapi.yaml
```

## Building from source

Build the CLI directly with Go:

```bash
go build ./...
go run printing-press.go path/to/openapi.yaml
```

## Usage

```bash
printing-press [flags] <spec-path-or-url>
```

Examples:

```bash
printing-press ./openapi.yaml
printing-press --publish --output ./api-docs ./openapi.yaml
printing-press --serve --output ./api-docs ./openapi.yaml
printing-press --debug ./openapi.yaml
printing-press --theme roger ./openapi.yaml
```

## Build modes

- Default: builds portable HTML assets intended for offline or `file://` usage
- `--publish`: builds hosted/served HTML assets for static hosting, but does not start a server
- `--serve`: builds hosted/served HTML assets and starts a local preview server

For GitHub Pages, S3, Netlify, Cloudflare Pages, or similar static hosting, use `--publish`.

## Outputs

By default, the CLI renders:

- HTML documentation
- JSON artifacts
- LLM output

You can disable any of these with the `--no-*` flags.

## Basic flags

- `--output`, `-o`: Output directory for rendered docs
- `--title`: Override the API title
- `--base-url`: Base URL to use in generated HTML
- `--base-path`: Base path for resolving local file references
- `--theme`: Terminal theme: `dark`, `roger`, or `tektronix`
- `--no-logo`, `-b`: Disable the banner
- `--debug`: Disable the progress bar and stream build and parser logs live
- `--no-html`: Skip HTML output
- `--no-llm`: Skip LLM output
- `--no-json`: Skip JSON artifact output
- `--publish`: Build hosted/served HTML assets without starting a local server
- `--serve`: Serve the rendered output after building
- `--port`: Port to use with `--serve`
- `--help`: Show all options

## Debugging builds

```bash
printing-press --debug ./openapi.yaml
```

This disables the progress bar and streams styled build, activity, and parser logs live. If you only need a plain non-interactive fallback, `TERM=dumb` still forces text-only progress output.

## Local preview

```bash
printing-press --serve --output ./api-docs ./openapi.yaml
```

This starts a local development preview server at `http://127.0.0.1:9090` by default.

## Static hosting

```bash
printing-press --publish --output ./api-docs ./openapi.yaml
```

This produces the hosted asset layout without starting a local server.
