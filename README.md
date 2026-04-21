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

## Install

Install the latest tagged release binary with the shell installer:

```bash
curl -fsSL https://raw.githubusercontent.com/pb33f/printing-press/main/scripts/install_printing_press.sh | sh
```

That installs the `pp` executable.

If you prefer `go install`, Go will still name the binary `printing-press` because it derives command names from the module path:

```bash
go install github.com/pb33f/printing-press@latest
```

For CI environments, set `GITHUB_TOKEN` to avoid GitHub API rate limiting:

```bash
GITHUB_TOKEN="${GITHUB_TOKEN}" curl -fsSL https://raw.githubusercontent.com/pb33f/printing-press/main/scripts/install_printing_press.sh | sh
```

Verify the installed release binary:

```bash
pp version
pp version --verbose
```

If you installed via `go install`, use `printing-press version` instead.

## Container image

Each tagged release also publishes a container image to GitHub Container Registry:

```bash
docker run --rm ghcr.io/pb33f/printing-press:latest version
```

To work with local specs and generated docs, bind mount host directories into the container. 
The image already uses `pp` as its entrypoint and `/work` as its default working directory, so mounted files behave like local CLI inputs.

Render docs from your current directory:

```bash
docker run --rm -v "$PWD:/work" -w /work ghcr.io/pb33f/printing-press:latest ./openapi.yaml
```

If you want to keep the input tree read-only and write docs to a separate host directory:

```bash
mkdir -p ./api-docs
docker run --rm \
  --mount type=bind,src="$PWD",target=/src,readonly \
  --mount type=bind,src="$PWD/api-docs",target=/out \
  -w /src \
  ghcr.io/pb33f/printing-press:latest \
  --output /out ./openapi.yaml
```

On Linux, add `--user "$(id -u):$(id -g)"` for bind-mounted runs so the container can read and write host files as your current user instead of hitting permission problems or leaving root-owned output behind:

```bash
docker run --rm \
  --user "$(id -u):$(id -g)" \
  --mount type=bind,src="$PWD",target=/work \
  -w /work \
  ghcr.io/pb33f/printing-press:latest \
  ./openapi.yaml
```

To serve docs from the container and view them in your browser, publish the container port to the host:

```bash
docker run --rm \
  -p 9090:9090 \
  --mount type=bind,src="$PWD",target=/work \
  -w /work \
  ghcr.io/pb33f/printing-press:latest \
  --serve --port 9090 ./openapi.yaml
```

Then open `http://127.0.0.1:9090`.

If you want a different host port, change the left side of `-p`. For example, `-p 8080:9090` still runs `pp` on port `9090` inside the container, but you would visit `http://127.0.0.1:8080` on the host.

Tagged images are also published with the release version, for example `ghcr.io/pb33f/printing-press:0.0.4`.

## Quick start

Run a single spec:

```bash
go run pp.go ./openapi.yaml
```

Scan a repo tree and build an API catalog:

```bash
go run pp.go ./services
```

By default the output is written to `./api-docs` in your current working directory.

## Build from source

```bash
go build -o pp .
./pp ./openapi.yaml
```

## Usage

```bash
pp [flags] <spec-path-or-url>
pp [flags] <directory>
```

Examples:

```bash
pp ./openapi.yaml
pp --publish --output ./api-docs ./openapi.yaml
pp --serve --output ./api-docs ./openapi.yaml
pp --debug ./openapi.yaml
pp --theme roger ./openapi.yaml
pp ./services
pp --serve ./services
pp --build-mode fast ./services
pp --disable-skipped-rendering ./services
```

## Single spec vs API catalog

### Single spec

If the input is a file or URL, `printing-press` renders one documentation site.

Example:

```bash
pp ./openapi.yaml
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
pp ./services
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

## Config file

You can configure the CLI with `printing-press.yaml` or `printing-press.yml`.

The CLI will look for it:

- next to the input file or directory
- in the current working directory

You can also pass it explicitly:

```bash
pp --config ./printing-press.yaml ./services
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
pp --serve --output ./api-docs ./openapi.yaml
```

Preview an API catalog:

```bash
pp --serve --output ./api-docs ./services
```

This starts a local preview server at `http://127.0.0.1:9090` by default.

## Static hosting

Single spec:

```bash
pp --publish --output ./api-docs ./openapi.yaml
```

API catalog:

```bash
pp --publish --output ./api-docs ./services
```

This produces the hosted asset layout without starting a local server.

## Debugging builds

```bash
pp --debug ./openapi.yaml
pp --debug ./services
```

This disables interactive progress bars and streams styled build, activity, and parser logs live.
