![printing-press logo](.github/assets/printing-press-logo.png)

# Printing Press

---
> [!WARNING]
> `printing-press` is currently in preview and is not intended for production use.
---

`printing-press` is the CLI for the `pb33f/doctor` Printing Press engine. It can:

- render a single OpenAPI spec into static documentation
- scan a directory tree, discover many specs, and build an API catalog
- emit HTML, JSON, and LLM-oriented artifacts
- build portable offline docs or hosted docs for static serving

## Quick start

Run a single spec:

```bash
go run printing-press.go ./openapi.yaml
```

Scan a repo tree and build an API catalog:

```bash
go run printing-press.go ./services
```

By default the output is written to `./api-docs` in your current working directory.

## Build from source

```bash
go build ./...
./printing-press ./openapi.yaml
```

## Usage

```bash
printing-press [flags] <spec-path-or-url>
printing-press [flags] <directory>
```

Examples:

```bash
printing-press ./openapi.yaml
printing-press --publish --output ./api-docs ./openapi.yaml
printing-press --serve --output ./api-docs ./openapi.yaml
printing-press --debug ./openapi.yaml
printing-press --theme roger ./openapi.yaml
printing-press ./services
printing-press --serve ./services
printing-press --build-mode fast ./services
printing-press --disable-skipped-rendering ./services
```

## Single spec vs API catalog

### Single spec

If the input is a file or URL, `printing-press` renders one documentation site.

Example:

```bash
printing-press ./openapi.yaml
```

Typical outputs:

- `index.html`
- `operations/*.html`
- `models/**/*.html`
- `bundle.json`
- `llms.txt`
- `AGENTS.md`

### API catalog

If the input is a directory, `printing-press` scans the tree, discovers root OpenAPI documents, groups them into services and versions, and renders one catalog plus one full doc site per discovered spec entry.

Example monorepo:

```text
services/
  banking/specs/banking.yaml
  auditing/src/things/specs/auditing.yaml
  users/src/specs/usersv1.yaml
  users/src/specs/usersv2.yaml
```

Run:

```bash
printing-press ./services
```

That produces:

- a root catalog at `api-docs/index.html`
- grouped service/version docs under `api-docs/services/...`
- per-entry spec docs under `api-docs/services/<service>/versions/<version>/specs/<entry>/...`

## API catalog LLM outputs

Catalog builds also emit an LLM discovery tree so an agent can start at the top and drill down into the exact spec it wants:

- `api-docs/AGENTS.md`
- `api-docs/llms.txt`
- `api-docs/services/<service>/llms.txt`
- `api-docs/services/<service>/versions/<version>/llms.txt`
- `api-docs/services/<service>/versions/<version>/specs/<entry>/AGENTS.md`
- `api-docs/services/<service>/versions/<version>/specs/<entry>/llms.txt`

The intent is:

- root `AGENTS.md` explains the catalog and links to all visible services, versions, and spec-entry indexes
- root `llms.txt` is the compact catalog index
- service and version `llms.txt` files progressively narrow the search space
- each spec entry still carries its own full `AGENTS.md` and `llms.txt`

## Build modes

- default: portable/offline HTML suitable for `file://` use
- `--publish`: hosted/served HTML asset layout for static hosting, but does not start a server
- `--serve`: hosted/served HTML asset layout and starts a local preview server

For GitHub Pages, S3, Netlify, Cloudflare Pages, or similar static hosting, use `--publish`.

## Outputs

By default, `printing-press` renders:

- HTML documentation
- JSON artifacts
- LLM output

You can disable any of these with:

- `--no-html`
- `--no-json`
- `--no-llm`

## Aggregate build modes

Directory/catalog builds support:

- `--build-mode full`: rebuild everything
- `--build-mode fast`: rescan and rebuild changed specs
- `--build-mode watch`: watch-oriented incremental mode

They also support pool tuning:

- `--max-pools`
- `--workers-per-pool`

And skipped-render warning suppression in the generated catalog:

- `--disable-skipped-rendering`

## Config file

You can configure the CLI with `printing-press.yaml` or `printing-press.yml`.

The CLI will look for it:

- next to the input file or directory
- in the current working directory

You can also pass it explicitly:

```bash
printing-press --config ./printing-press.yaml ./services
```

Example:

```yaml
title: Platform Catalog
description: Internal API documentation for all services.
output: ./api-docs
publish: true

scan:
  root: ./services
  ignoreRules:
    - "**/vendor/**"
    - "**/testdata/**"

grouping:
  serviceOverrides:
    - pattern: "services/payments/**"
      value: "billing"
  displayNameOverrides:
    - pattern: "services/payments/**"
      value: "Billing API"

build:
  mode: fast
  maxPools: 3
  workersPerPool: 2
  disableSkippedRendering: true

state:
  namespace: platform-catalog
  sqlite:
    path: ./.printing-press-state.db
```

## Important flags

- `--output`, `-o`: Output directory for rendered docs
- `--config`: Path to a `printing-press.yaml` config file
- `--title`: Override the API or catalog title
- `--base-url`: Base URL to use in generated HTML
- `--base-path`: Base path for resolving local file references
- `--build-mode`: Aggregate build mode: `full`, `fast`, or `watch`
- `--max-pools`: Aggregate max concurrent render pools
- `--workers-per-pool`: Aggregate core budget per render pool
- `--disable-skipped-rendering`: Hide skipped-render warnings from aggregate catalog pages
- `--theme`: Terminal theme: `dark`, `roger`, or `tektronix`
- `--no-logo`, `-b`: Disable the pb33f banner
- `--debug`: Disable progress bars and stream build logs live
- `--no-html`: Skip HTML output
- `--no-llm`: Skip LLM output
- `--no-json`: Skip JSON artifact output
- `--publish`: Build hosted/served HTML assets without starting a local server
- `--serve`: Serve the rendered output after building
- `--port`: Port to use with `--serve`

## Local preview

Preview a single spec:

```bash
printing-press --serve --output ./api-docs ./openapi.yaml
```

Preview an API catalog:

```bash
printing-press --serve --output ./api-docs ./services
```

This starts a local preview server at `http://127.0.0.1:9090` by default.

## Static hosting

Single spec:

```bash
printing-press --publish --output ./api-docs ./openapi.yaml
```

API catalog:

```bash
printing-press --publish --output ./api-docs ./services
```

This produces the hosted asset layout without starting a local server.

## Debugging builds

```bash
printing-press --debug ./openapi.yaml
printing-press --debug ./services
```

This disables interactive progress bars and streams styled build, activity, and parser logs live.
