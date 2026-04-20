# Printing Press Agent Notes

## Purpose

`pb33f/printing-press` is the CLI wrapper around the Printing Press renderer.
It is responsible for:

- loading a local or remote OpenAPI source
- choosing build and output modes
- rendering HTML, JSON artifacts, and LLM output
- optionally serving the generated docs locally for preview

## Repo Boundaries

- CLI and preview-server behavior live in `pb33f/printing-press`
- Core rendering logic, generated templates, and docs UI live in `pb33f/doctor`

When changing browser-rendered docs behavior, expect the main implementation to live in `pb33f/doctor`, not here.

## Building From Source

Build this repo directly with Go:

```bash
go build ./...
go run pp.go ./openapi.yaml
```

There is no wrapper-owned UI rebuild or template-generation step here. Those are the responsibility of `pb33f/doctor`, not this CLI repo.

## Build Modes

- Default build: portable HTML assets for offline or `file://` usage
- `--publish`: hosted/served HTML assets without starting a local server
- `--serve`: hosted/served HTML assets and starts the local preview server

`--serve` is for local development only. It is not intended as a production server.

## Static Hosting

For GitHub Pages, S3, Netlify, Cloudflare Pages, or similar static hosting, use:

```bash
go run pp.go --publish --output ./api-docs ./openapi.yaml
```

## Generated Output

This repo writes generated documentation to an output directory such as `./api-docs`.
That output is generated content, not hand-maintained project source.

## Testing

Typical validation:

```bash
go test ./...
```

If a change touches rendering behavior in `pb33f/doctor`, run the relevant tests there as well.
