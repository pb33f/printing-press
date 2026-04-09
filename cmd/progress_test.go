package cmd

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/pb33f/doctor/terminal"
	"github.com/stretchr/testify/require"
)

func TestProgressModelDoesNotMoveBackwardWithinStage(t *testing.T) {
	model := newProgressModel(terminal.PaletteForTheme(terminal.ThemeDark), 3)

	updated, _ := model.Update(progressUpdateMsg{stage: "html", status: "running", percent: 0.8, task: "rendering"})
	model = updated.(progressModel)
	firstPercent := model.percent

	updated, _ = model.Update(progressUpdateMsg{stage: "html", status: "running", percent: 0.3, task: "rendering"})
	model = updated.(progressModel)

	require.GreaterOrEqual(t, model.percent, firstPercent)
	require.InDelta(t, firstPercent, model.percent, 0.0001)
}

func TestProgressModelAdvancesAcrossCompletedStages(t *testing.T) {
	model := newProgressModel(terminal.PaletteForTheme(terminal.ThemeDark), 3)

	updated, _ := model.Update(progressUpdateMsg{stage: "html", status: "running", percent: 0.9, task: "rendering"})
	model = updated.(progressModel)
	runningPercent := model.percent

	updated, _ = model.Update(progressUpdateMsg{stage: "html", status: "completed", elapsed: 850 * time.Millisecond})
	model = updated.(progressModel)
	completedPercent := model.percent

	updated, _ = model.Update(progressUpdateMsg{stage: "llm", status: "running", percent: 0.08, task: "writing llm docs"})
	model = updated.(progressModel)

	require.Greater(t, completedPercent, runningPercent)
	require.Greater(t, model.percent, completedPercent)
	require.Equal(t, 1, model.completedStages)
	require.Equal(t, "LLM", model.stage)
}

func TestProgressModelDefaultsBlankStageToBuild(t *testing.T) {
	model := newProgressModel(terminal.PaletteForTheme(terminal.ThemeDark), 1)

	updated, cmd := model.Update(progressUpdateMsg{status: "running", percent: 0.2, task: "warming up"})
	model = updated.(progressModel)

	require.Equal(t, "BUILD", model.stage)
	require.NotNil(t, cmd)
	_, ok := cmd().(tea.Msg)
	require.True(t, ok)
}

func TestProgressModelFinalStageReachesFullPercent(t *testing.T) {
	model := newProgressModel(terminal.PaletteForTheme(terminal.ThemeDark), 2)

	updated, _ := model.Update(progressUpdateMsg{stage: "html", status: "completed", elapsed: 200 * time.Millisecond})
	model = updated.(progressModel)
	updated, _ = model.Update(progressUpdateMsg{stage: "json", status: "completed", elapsed: 50 * time.Millisecond})
	model = updated.(progressModel)

	require.InDelta(t, 1.0, model.percent, 0.0001)
}
