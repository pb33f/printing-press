![printing-press logo](.github/assets/printing-press-logo.png)

# Printing Press

---
> Important: this tool is currently a preview and is not intended for production use.
---

A CLI for rendering OpenAPI documentation with the pb33f printing press. 

Super high quality, detailed, rich, fast, offline, instant complete OpenAPI documentation.

## Installation

```bash
go install github.com/pb33f/printing-press@latest
```

Or run directly from source:

```bash
go run printing-press.go ./openapi.yaml
```

## Building From Source

For maintainers working in the pb33f workspace, the wrapper build needs fresh templates and browser assets from `doctor/printingpress` before compiling this CLI.
Use the Makefile in this repo for that development workflow:

```bash
make
```

That default build does three things:

- runs `templ` generation for `doctor/printingpress`
- installs UI dependencies and rebuilds the docs UI bundle for `doctor/printingpress`
- builds the `printing-press` wrapper CLI binary in this repo

Useful targets:

```bash
make build
make build-ui
make templ
```

- `make build`: full wrapper build with template generation and UI rebuild first
- `make build-ui`: install UI deps and rebuild the browser bundle only
- `make templ`: regenerate `templ` output only

This is a repository development flow. Normal end-user installation remains:

```bash
go install github.com/pb33f/printing-press@latest
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
printing-press --theme light ./openapi.yaml
```

## Build Modes

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

## Basic Flags

- `--output`, `-o`: Output directory for rendered docs
- `--title`: Override the API title
- `--base-url`: Base URL to use in generated HTML
- `--base-path`: Base path for resolving local file references
- `--theme`: Terminal theme: `dark`, `light`, or `tektronix`
- `--no-logo`, `-b`: Disable the banner
- `--no-html`: Skip HTML output
- `--no-llm`: Skip LLM output
- `--no-json`: Skip JSON artifact output
- `--publish`: Build hosted/served HTML assets without starting a local server
- `--serve`: Serve the rendered output after building
- `--port`: Port to use with `--serve`
- `--help`: Show all options

## Local Preview

```bash
printing-press --serve --output ./api-docs ./openapi.yaml
```

This starts a local development preview server at `http://127.0.0.1:9090` by default.

## Static Hosting

```bash
printing-press --publish --output ./api-docs ./openapi.yaml
```

This produces the hosted asset layout without starting a local server.
