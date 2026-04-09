package cmd

import (
	"fmt"
	"image/color"
	"strings"
	"time"

	lipgloss "charm.land/lipgloss/v2"
	"github.com/pb33f/doctor/printingpress"
	ppmodel "github.com/pb33f/doctor/printingpress/model"
	"github.com/pb33f/doctor/terminal"
)

type summaryRow struct {
	key   string
	value string
}

const (
	summaryTreeBranch = "├─"
	summaryTreeCorner = "└─"
	summaryTreePipe   = "│ "
	summaryTreeSpace  = "  "
)

func (a *application) printSummary(palette terminal.Palette, site *ppmodel.Site, htmlStats, llmStats *printingpress.PressStatistics, totalDuration time.Duration, fileCount int, totalBytes int64) {
	titleStyle := styleWithForeground(palette.Primary).Bold(true)

	fmt.Fprintln(a.stdout, titleStyle.Render("render complete"))
	fmt.Fprintln(a.stdout)
	fmt.Fprintln(a.stdout, renderStatsSummary(palette, buildSummaryRows(site, htmlStats, llmStats, totalDuration, fileCount, totalBytes)))

	if len(site.Warnings) == 0 {
		return
	}

	fmt.Fprintln(a.stdout)
	fmt.Fprintln(a.stdout, renderWarningsSummary(palette, site.Warnings))
}

func buildSummaryRows(site *ppmodel.Site, htmlStats, llmStats *printingpress.PressStatistics, totalDuration time.Duration, fileCount int, totalBytes int64) []summaryRow {
	contentStats := htmlStats
	if contentStats == nil {
		contentStats = llmStats
	}

	pageCount := 0
	if contentStats != nil {
		pageCount = contentStats.Pages
	}

	modelCount := countModels(site)
	operationCount := len(site.Operations) + len(site.Webhooks)
	classDiagramCount := countClassDiagrams(site)
	dependencyDiagramCount := countDependencyGraphs(site)
	if contentStats != nil {
		if contentStats.Models > 0 {
			modelCount = contentStats.Models
		}
		if contentStats.Operations > 0 {
			operationCount = contentStats.Operations
		}
		if contentStats.ClassDiagrams > 0 {
			classDiagramCount = contentStats.ClassDiagrams
		}
		if contentStats.DependencyGraphs > 0 {
			dependencyDiagramCount = contentStats.DependencyGraphs
		}
	}

	warningCount := len(site.Warnings)
	errorCount := countWarningErrors(site.Warnings)

	return []summaryRow{
		{key: "output", value: site.OutputDir},
		{key: "pages", value: fmt.Sprintf("%d", pageCount)},
		{key: "operations", value: fmt.Sprintf("%d", operationCount)},
		{key: "models", value: fmt.Sprintf("%d", modelCount)},
		{key: "class diagrams", value: fmt.Sprintf("%d", classDiagramCount)},
		{key: "dependency diagrams", value: fmt.Sprintf("%d", dependencyDiagramCount)},
		{key: "runtime", value: roundDuration(totalDuration).String()},
		{key: "disk usage", value: fmt.Sprintf("%d files, %s", fileCount, humanBytes(totalBytes))},
		{key: "warnings", value: fmt.Sprintf("%d", warningCount)},
		{key: "errors", value: fmt.Sprintf("%d", errorCount)},
	}
}

func renderStatsSummary(palette terminal.Palette, rows []summaryRow) string {
	keyStyle := styleWithForeground(palette.Muted)
	valueStyle := styleWithForeground(palette.Detail).Bold(true)

	keyWidth := 0
	for _, row := range rows {
		if len(row.key) > keyWidth {
			keyWidth = len(row.key)
		}
	}

	var b strings.Builder
	for i, row := range rows {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(keyStyle.Render(fmt.Sprintf("%-*s", keyWidth, row.key)))
		b.WriteString("  ")
		b.WriteString(valueStyle.Render(row.value))
	}
	return b.String()
}

func countClassDiagrams(site *ppmodel.Site) int {
	total := 0
	for _, pages := range site.Models {
		for _, page := range pages {
			if page.MermaidDiagram != "" {
				total++
			}
		}
	}
	return total
}

func countDependencyGraphs(site *ppmodel.Site) int {
	total := 0
	for _, pages := range site.Models {
		for _, page := range pages {
			if page.GraphJSON != "" {
				total++
			}
		}
	}
	return total
}

func countWarningErrors(warnings []*ppmodel.BuildWarning) int {
	total := 0
	for _, warning := range warnings {
		if warning != nil && warning.Err != nil {
			total++
		}
	}
	return total
}

func renderWarningsSummary(palette terminal.Palette, warnings []*ppmodel.BuildWarning) string {
	titleStyle := styleWithForeground(palette.Modification).Bold(true)
	badgeStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("0")).
		Background(summaryColorValue(palette.Modification, "11")).
		Padding(0, 1)

	blocks := make([]string, 0, len(warnings))
	for _, warning := range warnings {
		if warning == nil {
			continue
		}
		blocks = append(blocks, renderWarningBlock(palette, badgeStyle, warning))
	}
	if len(blocks) == 0 {
		return ""
	}

	return titleStyle.Render(fmt.Sprintf("warnings (%d)", len(blocks))) + "\n\n" + strings.Join(blocks, "\n\n")
}

func renderWarningBlock(palette terminal.Palette, badgeStyle lipgloss.Style, warning *ppmodel.BuildWarning) string {
	messageStyle := styleWithForeground(palette.Modification)
	keyStyle := lipgloss.NewStyle().Bold(true)
	valueStyle := styleWithForeground(palette.Modification)
	treeStyle := styleWithForeground(palette.Muted)

	var b strings.Builder
	b.WriteString(fmt.Sprintf("%s %s\n", badgeStyle.Render("WRN"), messageStyle.Render(warning.Message)))

	type attr struct {
		key   string
		value string
	}
	attrs := make([]attr, 0, 2)
	if warning.Context != "" {
		attrs = append(attrs, attr{key: "context", value: warning.Context})
	}
	if warning.Err != nil {
		attrs = append(attrs, attr{key: "error", value: warning.Err.Error()})
	}

	for i, item := range attrs {
		lines := splitWarningLines(item.value)
		if len(lines) == 0 {
			continue
		}
		isLast := i == len(attrs)-1
		connector := summaryTreeBranch
		childPrefix := summaryTreePipe
		if isLast {
			connector = summaryTreeCorner
			childPrefix = summaryTreeSpace
		}
		b.WriteString(treeStyle.Render(connector))
		b.WriteString(" ")
		b.WriteString(keyStyle.Render(item.key))
		b.WriteString(": ")
		b.WriteString(valueStyle.Render(lines[0]))
		b.WriteByte('\n')
		for _, line := range lines[1:] {
			b.WriteString(treeStyle.Render(childPrefix))
			b.WriteString("  ")
			b.WriteString(valueStyle.Render(line))
			b.WriteByte('\n')
		}
	}

	return strings.TrimRight(b.String(), "\n")
}

func splitWarningLines(value string) []string {
	parts := strings.Split(strings.TrimSpace(value), "\n")
	lines := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		lines = append(lines, trimmed)
	}
	return lines
}

func summaryColorValue(c color.Color, fallback string) color.Color {
	if c != nil {
		return c
	}
	return lipgloss.Color(fallback)
}
