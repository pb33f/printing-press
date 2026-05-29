<p align="center">
	<img src=".github/assets/printing-press-logo.png" alt="printing-press" height="306px" width="360px"/>
</p>

# printing-press: Agentic-first OpenAPI documentation

![Pipeline](https://github.com/pb33f/printing-press/workflows/CI/badge.svg)
[![Docker Pulls](https://img.shields.io/docker/pulls/pb33f/printing-press?style=flat-square)](https://hub.docker.com/r/pb33f/printing-press)
[![discord](https://img.shields.io/discord/923258363540815912)](https://discord.gg/x7VACVuEGP)
[![Docs](https://img.shields.io/badge/docs-pb33f.io/printing--press-5fafd7)](https://pb33f.io/printing-press/)

`printing-press` turns OpenAPI contracts into beautiful, fast, portable API documentation for humans **and** agents.

- Works **100% offline**. Open the generated docs straight from disk, zip them, or share them as an archive
- Publish to **any static host** (GitHub Pages, S3, Netlify, Cloudflare Pages, a CDN)
- **AI and agent output** built in. `AGENTS.md`, `llms.txt`, per-operation and per-model markdown, plus JSON artifacts
- **API catalogs** built from directories of OpenAPI specs, with multi-version awareness
- **Integrated diagnostics** via [vacuum](https://quobix.com/vacuum/) reports rendered inline with the docs
- **Mermaid / UML class diagrams** auto-generated for complex schemas

---

## Come chat with us

Need help? Have a question? Want to share your work? [Join our discord](https://discord.gg/x7VACVuEGP) and come say hi!

## Documentation

See all the documentation at https://pb33f.io/printing-press/

- [About printing-press](https://pb33f.io/printing-press/about/)
- [Quick start](https://pb33f.io/printing-press/quickstart/)
- [Installing](https://pb33f.io/printing-press/installing/)
- [Rendering modes](https://pb33f.io/printing-press/rendering-modes/)
- [Navigation](https://pb33f.io/printing-press/navigation/)
- [Agentic AI](https://pb33f.io/printing-press/agentic-ai/)
- [UML & class diagrams](https://pb33f.io/printing-press/uml-diagrams/)
- [Diagnostics mode](https://pb33f.io/printing-press/diagnostics-mode/)
- [API catalog](https://pb33f.io/printing-press/api-catalog/)
- [Configuring](https://pb33f.io/printing-press/configuring/)
- [Generated outputs](https://pb33f.io/printing-press/outputs/)

---

## Install

### Install using curl

```bash
curl -fsSL https://pb33f.io/printing-press/install.sh | sh
```

For CI environments, set `GITHUB_TOKEN` to avoid GitHub API rate limits:

```bash
GITHUB_TOKEN="${GITHUB_TOKEN}" curl -fsSL https://pb33f.io/printing-press/install.sh | sh
```

### Install using [npm](https://npmjs.com)

```bash
npm i -g @pb33f/printing-press
```

The npm package is the most convenient install path for Windows. Requires Node `20` or newer.

### Install using [Homebrew](https://brew.sh)

```bash
brew install pb33f/taps/printing-press
```

### Install using Go

```bash
go install github.com/pb33f/printing-press@latest
```

Go names the binary `printing-press` because it derives command names from the module path. For every other install method the binary is `ppress`.

### Install using Docker

Docker images are published to both Docker Hub and GitHub Container Registry:

```bash
docker run --rm pb33f/printing-press:latest version
docker run --rm ghcr.io/pb33f/printing-press:latest version
```

Render docs from the current directory:

```bash
docker run --rm -v "$PWD:/work" -w /work pb33f/printing-press:latest ./openapi.yaml
docker run --rm -v "$PWD:/work" -w /work ghcr.io/pb33f/printing-press:latest ./openapi.yaml
```

See the [installing](https://pb33f.io/printing-press/installing/) docs for the full Docker recipes (read-only mounts, serving, port mapping, Linux user mapping).

---

## Quick start

### Step 1: Install `ppress`

```bash
curl -fsSL https://pb33f.io/printing-press/install.sh | sh
```

### Step 2: Grab the sample train-travel spec

```bash
curl -o train-travel.yaml https://api.pb33f.io/bootstrap/train-travel
```

### Step 3: Print!

```bash
ppress ./train-travel.yaml
```

Open `api-docs/index.html` in your browser and you have static, offline OpenAPI docs with no server running.

To preview the **published** (web-hosted) layout locally, use `--serve`:

```bash
ppress --serve --output ./api-docs ./train-travel.yaml
```

Then open http://127.0.0.1:9090.

> Read the full docs at [https://pb33f.io/printing-press/](https://pb33f.io/printing-press/)

---

`printing-press` is a product of Princess Beef Heavy Industries, LLC.
