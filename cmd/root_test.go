package cmd

import (
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pb33f/doctor/printingpress"
	"github.com/pb33f/doctor/terminal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRootCommand_NoArgsShowsWelcome(t *testing.T) {
	app, stdout, _ := newTestApplication(t)
	cmd := app.newRootCommand()
	cmd.SetArgs(nil)

	require.NoError(t, cmd.Execute())
	assert.Contains(t, stdout.String(), "https://pb33f.io/printing-press/")
	assert.Contains(t, stdout.String(), "Welcome! To render docs")
	assert.Contains(t, stdout.String(), "printing-press ./openapi.yaml")
}

func TestRootCommand_HelpIncludesDebugFlag(t *testing.T) {
	app, stdout, _ := newTestApplication(t)
	cmd := app.newRootCommand()
	cmd.SetArgs([]string{"--help"})

	require.NoError(t, cmd.Execute())
	assert.Contains(t, stdout.String(), "--debug")
	assert.Contains(t, stdout.String(), "--config")
	assert.Contains(t, stdout.String(), "--build-mode")
	assert.Contains(t, stdout.String(), "--max-pools")
	assert.Contains(t, stdout.String(), "--workers-per-pool")
	assert.Contains(t, stdout.String(), "--disable-skipped-rendering")
	assert.Contains(t, stdout.String(), "stream build logs live")
}

func TestRootCommand_DefaultBuildWritesAllOutputs(t *testing.T) {
	specPath := writeSingleFileSpec(t, t.TempDir())
	outputDir := filepath.Join(t.TempDir(), "site")
	app, stdout, _ := newTestApplication(t)

	cmd := app.newRootCommand()
	cmd.SetArgs([]string{"--no-logo", "--output", outputDir, specPath})

	require.NoError(t, cmd.Execute())
	assert.FileExists(t, filepath.Join(outputDir, "index.html"))
	assert.FileExists(t, filepath.Join(outputDir, "llms.txt"))
	assert.FileExists(t, filepath.Join(outputDir, "bundle.json"))
	assert.FileExists(t, filepath.Join(outputDir, "static", "printing-press-shared.js"))
	assert.NoFileExists(t, filepath.Join(outputDir, "static", "printing-press-shared.json"))
	assert.Contains(t, stdout.String(), "Output")
	assert.Contains(t, stdout.String(), outputDir)
	assert.Contains(t, stdout.String(), "render complete")
}

func TestRootCommand_DebugStreamsActivityLogs(t *testing.T) {
	specPath := writeSingleFileSpec(t, t.TempDir())
	outputDir := filepath.Join(t.TempDir(), "site")
	app, _, stderr := newTestApplication(t)

	cmd := app.newRootCommand()
	cmd.SetArgs([]string{"--no-logo", "--debug", "--output", outputDir, specPath})

	require.NoError(t, cmd.Execute())
	assert.Contains(t, stderr.String(), "building libopenapi document")
	assert.Contains(t, stderr.String(), "building v3 model")
	assert.Contains(t, stderr.String(), "JSON complete")
}

func TestRootCommand_DefaultBasePathUsesSpecDirectory(t *testing.T) {
	specPath := writeSplitSpec(t, t.TempDir())
	outputDir := filepath.Join(t.TempDir(), "site")
	app, _, _ := newTestApplication(t)

	cmd := app.newRootCommand()
	cmd.SetArgs([]string{"--no-logo", "--output", outputDir, specPath})

	require.NoError(t, cmd.Execute())
	assert.FileExists(t, filepath.Join(outputDir, "models", "schemas", "widget.json"))
}

func TestRootCommand_LocalSpecPathIsForwardedToSourceMetadata(t *testing.T) {
	specDir := t.TempDir()
	specPath := filepath.Join(specDir, "sailpoint.yaml")
	require.NoError(t, os.WriteFile(specPath, []byte(singleFileSpecYAML), 0o644))
	outputDir := filepath.Join(t.TempDir(), "site")
	app, _, _ := newTestApplication(t)

	cmd := app.newRootCommand()
	cmd.SetArgs([]string{"--no-logo", "--output", outputDir, specPath})

	require.NoError(t, cmd.Execute())
	llmBytes, err := os.ReadFile(filepath.Join(outputDir, "operations", "list-burgers.md"))
	require.NoError(t, err)
	assert.Contains(t, string(llmBytes), filepath.Base(specPath)+":")
	assert.NotContains(t, string(llmBytes), "openapi.yaml:")
}

func TestRootCommand_SkipFlags(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		present []string
		absent  []string
	}{
		{
			name:    "skip html",
			args:    []string{"--no-logo", "--no-html"},
			present: []string{"llms.txt", "bundle.json"},
			absent:  []string{"index.html"},
		},
		{
			name:    "skip llm",
			args:    []string{"--no-logo", "--no-llm"},
			present: []string{"index.html", "bundle.json"},
			absent:  []string{"llms.txt"},
		},
		{
			name:    "skip json",
			args:    []string{"--no-logo", "--no-json"},
			present: []string{"index.html", "llms.txt"},
			absent:  []string{"bundle.json"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			specPath := writeSingleFileSpec(t, t.TempDir())
			outputDir := filepath.Join(t.TempDir(), "site")
			app, _, _ := newTestApplication(t)

			cmd := app.newRootCommand()
			args := append(append([]string{}, tt.args...), "--output", outputDir, specPath)
			cmd.SetArgs(args)

			require.NoError(t, cmd.Execute())
			for _, path := range tt.present {
				assert.FileExists(t, filepath.Join(outputDir, path))
			}
			for _, path := range tt.absent {
				assert.NoFileExists(t, filepath.Join(outputDir, path))
			}
		})
	}
}

func TestRootCommand_RemoteSpecBuilds(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, singleFileSpecYAML)
	}))
	defer server.Close()

	outputDir := filepath.Join(t.TempDir(), "site")
	app, _, _ := newTestApplication(t)
	cmd := app.newRootCommand()
	cmd.SetArgs([]string{"--no-logo", "--output", outputDir, server.URL + "/openapi.yaml"})

	require.NoError(t, cmd.Execute())
	assert.FileExists(t, filepath.Join(outputDir, "index.html"))
	assert.FileExists(t, filepath.Join(outputDir, "bundle.json"))
}

func TestRootCommand_DirectoryBuildWritesAggregateOutputs(t *testing.T) {
	repoRoot := writeAggregateRepo(t, t.TempDir())
	outputDir := filepath.Join(t.TempDir(), "site")
	app, stdout, _ := newTestApplication(t)

	cmd := app.newRootCommand()
	cmd.SetArgs([]string{"--no-logo", "--output", outputDir, repoRoot})

	require.NoError(t, cmd.Execute())
	assert.FileExists(t, filepath.Join(outputDir, "index.html"))
	assert.NoFileExists(t, filepath.Join(outputDir, "services", "users", "index.html"))
	assert.NoFileExists(t, filepath.Join(outputDir, "services", "users", "versions", "v2", "index.html"))
	matches, err := filepath.Glob(filepath.Join(outputDir, "services", "users", "versions", "v2", "specs", "*", "index.html"))
	require.NoError(t, err)
	require.Len(t, matches, 1)
	assert.FileExists(t, filepath.Join(outputDir, "llms.txt"))
	assert.FileExists(t, filepath.Join(outputDir, "bundle.json"))
	assert.Contains(t, stdout.String(), "services")
	assert.Contains(t, stdout.String(), "specs")
}

func TestRootCommand_DirectoryBuildDefaultsOutputToWorkingDirectory(t *testing.T) {
	repoRoot := writeAggregateRepo(t, t.TempDir())
	workDir := t.TempDir()
	t.Chdir(workDir)

	app, stdout, _ := newTestApplication(t)
	cmd := app.newRootCommand()
	cmd.SetArgs([]string{"--no-logo", repoRoot})

	require.NoError(t, cmd.Execute())

	outputDir := filepath.Join(workDir, "api-docs")
	assert.FileExists(t, filepath.Join(outputDir, "index.html"))
	matches, err := filepath.Glob(filepath.Join(outputDir, "services", "users", "versions", "v2", "specs", "*", "index.html"))
	require.NoError(t, err)
	require.Len(t, matches, 1)
	assert.Contains(t, stdout.String(), outputDir)
}

func TestRootCommand_LoadsAggregateConfigFromTargetDirectory(t *testing.T) {
	repoRoot := t.TempDir()
	writeAggregateSpecFile(t, repoRoot, "services/users/src/specs/usersv1.yaml", "Users API", "v1")
	writeAggregateSpecFile(t, repoRoot, "services/ignored/specs/ignored.yaml", "Ignore Me", "v1")
	configPath := filepath.Join(repoRoot, "printing-press.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(`
title: Repo Catalog
output: ./site-output
scan:
  ignoreRules:
    - services/ignored/**
grouping:
  displayNameOverrides:
    - pattern: services/users/**
      value: Customer API
`), 0o644))

	app, _, _ := newTestApplication(t)
	cmd := app.newRootCommand()
	cmd.SetArgs([]string{"--no-logo", repoRoot})

	require.NoError(t, cmd.Execute())

	outputDir := filepath.Join(repoRoot, "site-output")
	assert.FileExists(t, filepath.Join(outputDir, "index.html"))
	assert.NoFileExists(t, filepath.Join(outputDir, "services", "ignored"))

	indexBytes, err := os.ReadFile(filepath.Join(outputDir, "index.html"))
	require.NoError(t, err)
	assert.Contains(t, string(indexBytes), "Repo Catalog")
	assert.Contains(t, string(indexBytes), "Customer API")
	assert.NotContains(t, string(indexBytes), "Ignore Me")
}

func TestBuildAggregateConfig_PropagatesDisableSkippedRendering(t *testing.T) {
	cfg := buildAggregateConfig("/tmp/repo", "/tmp/site", printingpress.HTMLAssetModePortable, &rootOptions{
		disableSkippedRendering: true,
	}, nil)

	assert.True(t, cfg.DisableSkippedRendering)
}

func TestBuildAggregateConfig_LoadsDisableSkippedRenderingFromConfig(t *testing.T) {
	cfg := buildAggregateConfig("/tmp/repo", "/tmp/site", printingpress.HTMLAssetModePortable, &rootOptions{}, &printingPressConfigFile{
		Build: printingPressBuildConfig{
			DisableSkippedRendering: true,
		},
	})

	assert.True(t, cfg.DisableSkippedRendering)
}

func TestRootCommand_NoArgsUsesConfigScanRoot(t *testing.T) {
	projectRoot := t.TempDir()
	scanRoot := filepath.Join(projectRoot, "apis")
	writeAggregateRepo(t, scanRoot)
	require.NoError(t, os.WriteFile(filepath.Join(projectRoot, "printing-press.yaml"), []byte(`
title: Workspace Catalog
scan:
  root: ./apis
output: ./site
`), 0o644))

	t.Chdir(projectRoot)

	app, _, _ := newTestApplication(t)
	cmd := app.newRootCommand()
	cmd.SetArgs([]string{"--no-logo"})

	require.NoError(t, cmd.Execute())
	assert.FileExists(t, filepath.Join(projectRoot, "site", "index.html"))
}

func TestRootCommand_AggregateDebugLogsPools(t *testing.T) {
	repoRoot := writeAggregateRepo(t, t.TempDir())
	outputDir := filepath.Join(t.TempDir(), "site")
	app, _, stderr := newTestApplication(t)

	cmd := app.newRootCommand()
	cmd.SetArgs([]string{"--no-logo", "--debug", "--output", outputDir, repoRoot})

	require.NoError(t, cmd.Execute())
	assert.Contains(t, stderr.String(), "aggregate pool")
	assert.Contains(t, stderr.String(), "completed_specs")
}

func TestRootCommand_ServeUsesRenderedOutput(t *testing.T) {
	specPath := writeSingleFileSpec(t, t.TempDir())
	outputDir := filepath.Join(t.TempDir(), "site")
	app, stdout, _ := newTestApplication(t)

	var servedAddr string
	var servedDir string
	var servedBaseURL string
	app.serveFn = func(addr, dir, baseURL string) error {
		servedAddr = addr
		servedDir = dir
		servedBaseURL = baseURL
		_, err := os.Stat(filepath.Join(dir, "index.html"))
		return err
	}

	cmd := app.newRootCommand()
	cmd.SetArgs([]string{"--no-logo", "--serve", "--port", "9191", "--output", outputDir, specPath})

	require.NoError(t, cmd.Execute())
	assert.Equal(t, ":9191", servedAddr)
	assert.Equal(t, outputDir, servedDir)
	assert.Empty(t, servedBaseURL)
	assert.FileExists(t, filepath.Join(outputDir, "static", "printing-press-shared.json"))
	assert.NoFileExists(t, filepath.Join(outputDir, "static", "printing-press-shared.js"))
	assert.Contains(t, stdout.String(), "serving http://127.0.0.1:9191")
}

func TestRootCommand_ServeForwardsBaseURL(t *testing.T) {
	specPath := writeSingleFileSpec(t, t.TempDir())
	outputDir := filepath.Join(t.TempDir(), "site")
	app, _, _ := newTestApplication(t)

	var servedBaseURL string
	app.serveFn = func(addr, dir, baseURL string) error {
		servedBaseURL = baseURL
		return nil
	}

	cmd := app.newRootCommand()
	cmd.SetArgs([]string{"--no-logo", "--serve", "--base-url", "/docs/", "--output", outputDir, specPath})

	require.NoError(t, cmd.Execute())
	assert.Equal(t, "/docs/", servedBaseURL)
}

func TestRootCommand_AggregateServeForwardsBaseURL(t *testing.T) {
	repoRoot := writeAggregateRepo(t, t.TempDir())
	outputDir := filepath.Join(t.TempDir(), "site")
	app, _, _ := newTestApplication(t)

	var servedBaseURL string
	app.serveFn = func(addr, dir, baseURL string) error {
		servedBaseURL = baseURL
		return nil
	}

	cmd := app.newRootCommand()
	cmd.SetArgs([]string{"--no-logo", "--serve", "--base-url", "/catalog/", "--output", outputDir, repoRoot})

	require.NoError(t, cmd.Execute())
	assert.Equal(t, "/catalog/", servedBaseURL)
}

func TestRootCommand_PublishBuildsServedAssetsWithoutStartingServer(t *testing.T) {
	specPath := writeSingleFileSpec(t, t.TempDir())
	outputDir := filepath.Join(t.TempDir(), "site")
	app, stdout, _ := newTestApplication(t)

	serverCalled := false
	app.serveFn = func(addr, dir, baseURL string) error {
		serverCalled = true
		return nil
	}

	cmd := app.newRootCommand()
	cmd.SetArgs([]string{"--no-logo", "--publish", "--output", outputDir, specPath})

	require.NoError(t, cmd.Execute())
	assert.False(t, serverCalled)
	assert.FileExists(t, filepath.Join(outputDir, "static", "printing-press-shared.json"))
	assert.NoFileExists(t, filepath.Join(outputDir, "static", "printing-press-shared.js"))
	assert.NotContains(t, stdout.String(), "serving http://127.0.0.1")
}

func TestRootCommand_NoLogoSuppressesBanner(t *testing.T) {
	specPath := writeSingleFileSpec(t, t.TempDir())
	outputDir := filepath.Join(t.TempDir(), "site")
	app, stdout, _ := newTestApplication(t)

	cmd := app.newRootCommand()
	cmd.SetArgs([]string{"--no-logo", "--output", outputDir, specPath})

	require.NoError(t, cmd.Execute())
	assert.NotContains(t, stdout.String(), "@@@@@@@")
	assert.NotContains(t, stdout.String(), "https://pb33f.io/printing-press/")
}

func TestParseTheme_RogerAlias(t *testing.T) {
	theme, err := parseTheme("roger")
	require.NoError(t, err)
	assert.Equal(t, terminal.ThemeLight, theme)

	theme, err = parseTheme("light")
	require.NoError(t, err)
	assert.Equal(t, terminal.ThemeLight, theme)
}

func TestRootCommand_InvalidInputPathFails(t *testing.T) {
	app, _, _ := newTestApplication(t)
	cmd := app.newRootCommand()
	cmd.SetArgs([]string{"--no-logo", "/definitely/not/here.yaml"})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read spec")
}

func TestStaticServer_CompressesJavaScriptAndSetsLongCache(t *testing.T) {
	dir := t.TempDir()
	staticDir := filepath.Join(dir, "static")
	require.NoError(t, os.MkdirAll(staticDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(staticDir, "printing-press.js"), []byte("console.log('burger');"), 0o644))

	server := httptest.NewServer(newStaticServer(dir, ""))
	defer server.Close()

	req, err := http.NewRequest(http.MethodGet, server.URL+"/static/printing-press.js", nil)
	require.NoError(t, err)
	req.Header.Set("Accept-Encoding", "gzip")

	resp, err := server.Client().Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, "no-cache", resp.Header.Get("Cache-Control"))
	assert.Equal(t, "gzip", resp.Header.Get("Content-Encoding"))
	assert.Contains(t, resp.Header.Values("Vary"), "Accept-Encoding")

	gzr, err := gzip.NewReader(resp.Body)
	require.NoError(t, err)
	defer gzr.Close()

	body, err := io.ReadAll(gzr)
	require.NoError(t, err)
	assert.Equal(t, "console.log('burger');", string(body))
}

func TestStaticServer_UsesRevalidatingCacheForHTMLAndPageData(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "index.html"), []byte("<!doctype html><title>burger</title>"), 0o644))
	pageDataDir := filepath.Join(dir, "static", "page-data", "models")
	require.NoError(t, os.MkdirAll(pageDataDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(pageDataDir, "burger.json"), []byte(`{"ok":true}`), 0o644))

	server := httptest.NewServer(newStaticServer(dir, ""))
	defer server.Close()

	htmlResp, err := server.Client().Get(server.URL + "/index.html")
	require.NoError(t, err)
	defer htmlResp.Body.Close()
	assert.Equal(t, "no-cache", htmlResp.Header.Get("Cache-Control"))

	dataResp, err := server.Client().Get(server.URL + "/static/page-data/models/burger.json")
	require.NoError(t, err)
	defer dataResp.Body.Close()
	assert.Equal(t, "no-cache", dataResp.Header.Get("Cache-Control"))
}

func TestStaticServer_MountsUnderBasePath(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "index.html"), []byte("<!doctype html><title>burger</title>"), 0o644))
	staticDir := filepath.Join(dir, "static")
	require.NoError(t, os.MkdirAll(staticDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(staticDir, "printing-press.css"), []byte("body{}"), 0o644))

	server := httptest.NewServer(newStaticServer(dir, "/docs/"))
	defer server.Close()

	redirectClient := *server.Client()
	redirectClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}

	rootResp, err := redirectClient.Get(server.URL + "/")
	require.NoError(t, err)
	defer rootResp.Body.Close()
	assert.Equal(t, http.StatusTemporaryRedirect, rootResp.StatusCode)
	assert.Equal(t, "/docs/", rootResp.Header.Get("Location"))

	indexResp, err := server.Client().Get(server.URL + "/docs/index.html")
	require.NoError(t, err)
	defer indexResp.Body.Close()
	assert.Equal(t, http.StatusOK, indexResp.StatusCode)

	cssResp, err := server.Client().Get(server.URL + "/docs/static/printing-press.css")
	require.NoError(t, err)
	defer cssResp.Body.Close()
	assert.Equal(t, http.StatusOK, cssResp.StatusCode)

	notFoundResp, err := server.Client().Get(server.URL + "/index.html")
	require.NoError(t, err)
	defer notFoundResp.Body.Close()
	assert.Equal(t, http.StatusNotFound, notFoundResp.StatusCode)
}

func TestStaticServer_CompressesMarkdownAndYAML(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "llms-full.txt"), []byte("docs"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "notes.md"), []byte("# burger"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "openapi.yaml"), []byte("openapi: 3.1.0\n"), 0o644))

	server := httptest.NewServer(newStaticServer(dir, ""))
	defer server.Close()

	for _, path := range []string{"/notes.md", "/openapi.yaml"} {
		req, err := http.NewRequest(http.MethodGet, server.URL+path, nil)
		require.NoError(t, err)
		req.Header.Set("Accept-Encoding", "gzip")

		resp, err := server.Client().Do(req)
		require.NoError(t, err)
		assert.Equal(t, "gzip", resp.Header.Get("Content-Encoding"))
		resp.Body.Close()
	}
}

func TestRootCommand_AllOutputsDisabledFails(t *testing.T) {
	specPath := writeSingleFileSpec(t, t.TempDir())
	app, _, _ := newTestApplication(t)
	cmd := app.newRootCommand()
	cmd.SetArgs([]string{"--no-logo", "--no-html", "--no-llm", "--no-json", specPath})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "all output types are disabled")
}

func newTestApplication(t *testing.T) (*application, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	app := newApplication(BuildInfo{Version: "test", Date: "Tue, 08 Apr 2026 12:00:00 EDT"})
	app.stdout = stdout
	app.stderr = stderr
	return app, stdout, stderr
}

func writeSingleFileSpec(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "openapi.yaml")
	require.NoError(t, os.WriteFile(path, []byte(singleFileSpecYAML), 0o644))
	return path
}

func writeSplitSpec(t *testing.T, dir string) string {
	t.Helper()
	main := filepath.Join(dir, "openapi.yaml")
	schemas := filepath.Join(dir, "schemas.yaml")
	require.NoError(t, os.WriteFile(main, []byte(splitSpecYAML), 0o644))
	require.NoError(t, os.WriteFile(schemas, []byte(splitSchemasYAML), 0o644))
	return main
}

func writeAggregateRepo(t *testing.T, root string) string {
	t.Helper()
	writeAggregateSpecFile(t, root, "services/users/src/specs/usersv1.yaml", "Users API", "v1")
	writeAggregateSpecFile(t, root, "services/users/src/specs/usersv2.yaml", "Users API", "v2")
	writeAggregateSpecFile(t, root, "services/auditing/src/specs/auditing.yaml", "Auditing API", "1.0.0")
	return root
}

func writeAggregateSpecFile(t *testing.T, root, relPath, title, version string) string {
	t.Helper()
	absPath := filepath.Join(root, relPath)
	require.NoError(t, os.MkdirAll(filepath.Dir(absPath), 0o755))
	spec := fmt.Sprintf(`openapi: 3.1.0
info:
  title: %s
  version: %s
paths:
  /health:
    get:
      operationId: listHealth
      responses:
        '200':
          description: ok
components:
  schemas:
    Status:
      type: object
      properties:
        ok:
          type: boolean
`, title, version)
	require.NoError(t, os.WriteFile(absPath, []byte(spec), 0o644))
	return absPath
}

const singleFileSpecYAML = `openapi: 3.1.0
info:
  title: Burger API
  version: 1.0.0
paths:
  /burgers:
    get:
      operationId: listBurgers
      summary: List burgers
      responses:
        '200':
          description: ok
          content:
            application/json:
              schema:
                type: array
                items:
                  $ref: '#/components/schemas/Burger'
components:
  schemas:
    Burger:
      type: object
      required:
        - id
      properties:
        id:
          type: string
        name:
          type: string
`

const splitSpecYAML = `openapi: 3.1.0
info:
  title: Widget API
  version: 1.0.0
paths:
  /widgets:
    get:
      operationId: listWidgets
      responses:
        '200':
          description: ok
          content:
            application/json:
              schema:
                type: array
                items:
                  $ref: './schemas.yaml#/components/schemas/Widget'
`

const splitSchemasYAML = `components:
  schemas:
    Widget:
      type: object
      properties:
        id:
          type: string
`

func TestRootCommand_TooManyArgsFails(t *testing.T) {
	app, _, _ := newTestApplication(t)
	cmd := app.newRootCommand()
	cmd.SetArgs([]string{"a.yaml", "b.yaml"})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected exactly one spec path, directory, or URL")
}

func TestRenderCommandError_UsesStyledPrefixAndHint(t *testing.T) {
	app, _, stderr := newTestApplication(t)
	app.renderCommandError(&cliError{
		message: "boom",
		detail:  "something exploded",
		hint:    "try again with one spec",
	}, terminal.PaletteForTheme(terminal.ThemeDark))

	assert.Contains(t, stderr.String(), "✗")
	assert.Contains(t, stderr.String(), "boom")
	assert.Contains(t, stderr.String(), "something exploded")
	assert.Contains(t, stderr.String(), "try again with one spec")
}

func TestConfigureBuildLogger_UsesPrettyLogger(t *testing.T) {
	app, _, stderr := newTestApplication(t)
	session := app.configureBuildLogger(terminal.PaletteForTheme(terminal.ThemeDark), activityRenderModePlain)
	defer session.finish(nil)

	slog.Warn("source bundling failed", slog.String("context", "/tmp/spec.yaml"))

	assert.Contains(t, stderr.String(), "source bundling failed")
	assert.Contains(t, stderr.String(), "context")
	assert.Contains(t, stderr.String(), "└─")
}

func TestConfigureBuildLogger_DebugModeStreamsInfoImmediately(t *testing.T) {
	app, _, stderr := newTestApplication(t)
	session := app.configureBuildLogger(terminal.PaletteForTheme(terminal.ThemeDark), activityRenderModeDebug)
	defer session.finish(nil)

	slog.Info("building libopenapi document", slog.String("stage", "HTML"))

	assert.Contains(t, stderr.String(), "building libopenapi document")
	assert.Contains(t, stderr.String(), "stage")
	assert.False(t, session.buffered)
}

func TestBuildLoggerSession_FlushesBufferedLogsOnError(t *testing.T) {
	var stderr bytes.Buffer
	handler := terminal.NewPrettyHandler(&terminal.PrettyHandlerOptions{
		Level:      slog.LevelWarn,
		TimeFormat: terminal.TimeFormatTimeOnly,
		Writer:     &stderr,
		Palette:    ptr(terminal.PaletteForTheme(terminal.ThemeDark)),
		Buffer:     true,
	})
	previous := slog.Default()
	slog.SetDefault(slog.New(handler))

	slog.Warn("source bundling failed", slog.String("context", "/tmp/spec.yaml"))

	session := &buildLoggerSession{previous: previous, handler: handler, buffered: true}
	session.finish(errors.New("boom"))

	assert.Contains(t, stderr.String(), "source bundling failed")
	assert.Contains(t, stderr.String(), "context")
}

func TestRunWithActivity_TimesOutWhenRendererDoesNotExit(t *testing.T) {
	pp, err := printingpress.CreatePrintingPressFromBytes([]byte(singleFileSpecYAML), nil)
	require.NoError(t, err)

	previousTimeout := activityRenderWaitTimeout
	activityRenderWaitTimeout = 10 * time.Millisecond
	defer func() {
		activityRenderWaitTimeout = previousTimeout
	}()

	release := make(chan struct{})
	renderDone := make(chan struct{})
	start := time.Now()

	result, err := runWithActivity(pp, func(sub *printingpress.ActivitySubscription) {
		defer close(renderDone)
		<-release
	}, func() (int, error) {
		return 42, nil
	})

	elapsed := time.Since(start)
	close(release)
	<-renderDone

	require.NoError(t, err)
	assert.Equal(t, 42, result)
	assert.Less(t, elapsed, 80*time.Millisecond)
}

func ptr[T any](v T) *T {
	return &v
}
