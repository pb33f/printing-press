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

func TestRootCommand_DefaultBasePathUsesSpecDirectory(t *testing.T) {
	specPath := writeSplitSpec(t, t.TempDir())
	outputDir := filepath.Join(t.TempDir(), "site")
	app, _, _ := newTestApplication(t)

	cmd := app.newRootCommand()
	cmd.SetArgs([]string{"--no-logo", "--output", outputDir, specPath})

	require.NoError(t, cmd.Execute())
	assert.FileExists(t, filepath.Join(outputDir, "models", "schemas", "widget.json"))
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

func TestRootCommand_ServeUsesRenderedOutput(t *testing.T) {
	specPath := writeSingleFileSpec(t, t.TempDir())
	outputDir := filepath.Join(t.TempDir(), "site")
	app, stdout, _ := newTestApplication(t)

	var servedAddr string
	var servedDir string
	app.serveFn = func(addr, dir string) error {
		servedAddr = addr
		servedDir = dir
		_, err := os.Stat(filepath.Join(dir, "index.html"))
		return err
	}

	cmd := app.newRootCommand()
	cmd.SetArgs([]string{"--no-logo", "--serve", "--port", "9191", "--output", outputDir, specPath})

	require.NoError(t, cmd.Execute())
	assert.Equal(t, ":9191", servedAddr)
	assert.Equal(t, outputDir, servedDir)
	assert.FileExists(t, filepath.Join(outputDir, "static", "printing-press-shared.json"))
	assert.NoFileExists(t, filepath.Join(outputDir, "static", "printing-press-shared.js"))
	assert.Contains(t, stdout.String(), "serving http://127.0.0.1:9191")
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

	server := httptest.NewServer(newStaticServer(dir))
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

	server := httptest.NewServer(newStaticServer(dir))
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

func TestStaticServer_CompressesMarkdownAndYAML(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "llms-full.txt"), []byte("docs"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "notes.md"), []byte("# burger"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "openapi.yaml"), []byte("openapi: 3.1.0\n"), 0o644))

	server := httptest.NewServer(newStaticServer(dir))
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
	assert.Contains(t, err.Error(), "expected exactly one spec path or URL")
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
	session := app.configureBuildLogger(terminal.PaletteForTheme(terminal.ThemeDark))
	defer session.finish(nil)

	slog.Warn("source bundling failed", slog.String("context", "/tmp/spec.yaml"))

	assert.Contains(t, stderr.String(), "source bundling failed")
	assert.Contains(t, stderr.String(), "context")
	assert.Contains(t, stderr.String(), "└─")
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

func ptr[T any](v T) *T {
	return &v
}
