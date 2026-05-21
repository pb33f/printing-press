package cmd

import (
	"bytes"
	"strings"
	"testing"
	"time"

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

func TestAggregatePoolModelView_RendersRuntimeMetrics(t *testing.T) {
	model := newAggregatePoolModel(terminal.PaletteForTheme(terminal.ThemeDark))

	updated, _ := model.Update(aggregateRuntimeMetricsMsg{snapshot: runtimeMetricsSnapshot{
		Elapsed:    2 * time.Second,
		HeapAlloc:  64 * 1024,
		TotalAlloc: 256 * 1024,
		Sys:        128 * 1024,
		NumGC:      3,
		Goroutines: 9,
	}})
	model = updated.(aggregatePoolModel)

	view := model.View().Content
	require.Contains(t, view, "elapsed 2s")
	require.Contains(t, view, "heap 64.0 KiB")
	require.Contains(t, view, "reserved 128.0 KiB")
	require.Contains(t, view, "collections 3")
	require.Contains(t, view, "threads 9")
}

func TestAggregatePoolPlainRendererReportsRuntimeMetrics(t *testing.T) {
	var out bytes.Buffer
	renderer := &aggregatePoolPlainRenderer{
		writer: &out,
		last:   make(map[int]aggregatePoolView),
	}

	renderer.reportRuntimeMetrics(runtimeMetricsSnapshot{
		Elapsed:    1500 * time.Millisecond,
		HeapAlloc:  1024,
		TotalAlloc: 2048,
		Sys:        4096,
		NumGC:      1,
		Goroutines: 4,
	})

	output := out.String()
	require.Contains(t, output, "METRICS")
	require.Contains(t, output, "elapsed 1.5s")
	require.Contains(t, output, "heap 1.0 KiB")
	require.Contains(t, output, "threads 4")
}
