package cmd

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/pb33f/doctor/printingpress"
	ppmodel "github.com/pb33f/doctor/printingpress/model"
	"github.com/pb33f/doctor/terminal"
	"github.com/stretchr/testify/assert"
)

func TestPrintSummary_RendersStatsAndWarnings(t *testing.T) {
	app, stdout, _ := newTestApplication(t)
	outputDir := filepath.Join(t.TempDir(), "site")
	site := &ppmodel.Site{
		OutputDir: outputDir,
		Models: map[string][]*ppmodel.ModelPage{
			"schemas": {{Name: "Burger", MermaidDiagram: "graph TD", GraphJSON: "{}"}},
		},
		Operations: []*ppmodel.OperationPage{{OperationID: "listBurgers"}},
		Warnings: []*ppmodel.BuildWarning{{
			Message: "source bundling failed; falling back to single-file parse, multi-file output may be incomplete",
			Context: "/tmp/specs",
			Err:     errors.New("invalid model\ninfinite circular reference detected: payment_intent"),
		}},
	}
	palette := terminal.PaletteForTheme(terminal.ThemeDark)
	htmlStats := &printingpress.PressStatistics{
		Pages:            12,
		Models:           1,
		Operations:       1,
		ClassDiagrams:    1,
		DependencyGraphs: 1,
	}
	llmStats := &printingpress.PressStatistics{Pages: 4}

	app.printSummary(palette, site, htmlStats, llmStats, 992*time.Millisecond, 18, 8192)

	output := stdout.String()
	assert.Contains(t, output, "render complete")
	assert.Contains(t, output, "output")
	assert.Contains(t, output, outputDir)
	assert.Contains(t, output, "pages")
	assert.Contains(t, output, "12")
	assert.Contains(t, output, "operations")
	assert.Contains(t, output, "models")
	assert.Contains(t, output, "class diagrams")
	assert.Contains(t, output, "dependency diagrams")
	assert.Contains(t, output, "runtime")
	assert.Contains(t, output, "992ms")
	assert.Contains(t, output, "disk usage")
	assert.Contains(t, output, "18 files, 8.0 KiB")
	assert.Contains(t, output, "warnings")
	assert.Contains(t, output, "errors")
	assert.Contains(t, output, "warnings (1)")
	assert.Contains(t, output, "WRN")
	assert.Contains(t, output, "context")
	assert.Contains(t, output, "error")
	assert.Contains(t, output, "infinite circular reference detected: payment_intent")
	assert.Contains(t, output, "├─")
	assert.Contains(t, output, "└─")
}
