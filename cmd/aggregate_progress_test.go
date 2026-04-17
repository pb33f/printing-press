package cmd

import (
	"strings"
	"testing"

	"github.com/pb33f/doctor/printingpress"
	"github.com/pb33f/doctor/terminal"
	"github.com/stretchr/testify/require"
)

func TestAggregatePoolModelView_UsesGradientProgressBar(t *testing.T) {
	model := newAggregatePoolModel(terminal.PaletteForTheme(terminal.ThemeDark))

	updated, _ := model.Update(aggregatePoolUpdateMsg{update: printingpress.AggregateProgressUpdate{
		PoolID:         1,
		Status:         printingpress.AggregateProgressStatusRunning,
		CompletedSpecs: 2,
		TotalSpecs:     5,
		CompletedBytes: 256,
		TotalBytes:     1024,
		CurrentSpec:    "APIs/example.com/v1/openapi.yaml",
		CurrentStage:   "building model",
		OverallPercent: 0.42,
	}})
	model = updated.(aggregatePoolModel)

	view := model.View().Content
	require.Contains(t, view, "POOL 1")
	require.Contains(t, view, "42%")
	require.Contains(t, view, "APIs/example.com/v1/openapi.yaml")
	require.True(t, strings.Contains(view, "\x1b["), "expected ANSI-styled gradient bar output")
}
