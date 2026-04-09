package cmd

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"charm.land/bubbles/v2/progress"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
	"github.com/pb33f/doctor/printingpress"
	"github.com/pb33f/doctor/terminal"
	"golang.org/x/term"
)

type buildProgressUI struct {
	writer      io.Writer
	interactive bool
	program     *tea.Program
	done        chan struct{}
	closeOnce   sync.Once
}

type progressUpdateMsg struct {
	stage   string
	task    string
	status  string
	percent float64
	elapsed time.Duration
	error   string
}

type progressQuitMsg struct{}

type progressModel struct {
	spinner         spinner.Model
	bar             progress.Model
	titleStyle      lipgloss.Style
	taskStyle       lipgloss.Style
	errorStyle      lipgloss.Style
	mutedStyle      lipgloss.Style
	totalStages     int
	completed       map[string]bool
	stage           string
	stagePercent    float64
	task            string
	percent         float64
	failed          bool
	errorText       string
	quitting        bool
	completedStages int
}

func newBuildProgressUI(writer io.Writer, palette terminal.Palette, totalStages int) *buildProgressUI {
	ui := &buildProgressUI{
		writer: writer,
		done:   make(chan struct{}),
	}
	if totalStages < 1 {
		totalStages = 1
	}
	if !supportsInteractiveProgress(writer) {
		close(ui.done)
		return ui
	}

	model := newProgressModel(palette, totalStages)
	ui.program = tea.NewProgram(model, tea.WithOutput(writer), tea.WithInput(os.Stdin))
	ui.interactive = true
	go func() {
		_, _ = ui.program.Run()
		close(ui.done)
	}()
	return ui
}

func newProgressModel(palette terminal.Palette, totalStages int) progressModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = styleWithForeground(palette.Secondary).Bold(true)

	start, end := progressRamp(palette.Theme)
	bar := progress.New(
		progress.WithWidth(38),
		progress.WithColors(lipgloss.Color(start), lipgloss.Color(end)),
		progress.WithScaled(true),
		progress.WithFillCharacters('█', '░'),
	)

	return progressModel{
		spinner:     s,
		bar:         bar,
		titleStyle:  styleWithForeground(palette.Primary).Bold(true),
		taskStyle:   styleWithForeground(palette.Detail),
		errorStyle:  styleWithForeground(palette.Breaking).Bold(true),
		mutedStyle:  styleWithForeground(palette.Muted),
		totalStages: totalStages,
		completed:   make(map[string]bool),
		task:        "warming up printing press",
	}
}

func progressRamp(theme terminal.ThemeName) (string, string) {
	switch theme {
	case terminal.ThemeLight:
		return "#606060", "#ffffff"
	case terminal.ThemeTektronix:
		return "#33ff33", "#66ff66"
	default:
		return "#62c4ff", "#f83aff"
	}
}

func supportsInteractiveProgress(writer io.Writer) bool {
	file, ok := writer.(*os.File)
	if !ok {
		return false
	}
	if os.Getenv("TERM") == "dumb" {
		return false
	}
	if !term.IsTerminal(int(file.Fd())) {
		return false
	}
	return term.IsTerminal(int(os.Stdin.Fd()))
}

func (ui *buildProgressUI) renderActivity(sub *printingpress.ActivitySubscription) {
	if sub == nil {
		return
	}
	if !ui.interactive {
		renderActivityFallback(ui.writer, sub)
		return
	}
	for update := range sub.Updates() {
		ui.program.Send(progressUpdateMsg{
			stage:   update.JobType,
			task:    update.CurrentTask,
			status:  update.Status,
			percent: stagePercent(update),
			elapsed: update.Elapsed,
			error:   update.Error,
		})
	}
}

func (ui *buildProgressUI) updateManual(stage, task, status string, percent float64, elapsed time.Duration, err error) {
	if !ui.interactive {
		if status == "completed" {
			fmt.Fprintln(ui.writer, formatStatusLine(stage, fmt.Sprintf("completed in %s", roundDuration(elapsed))))
			return
		}
		fmt.Fprintln(ui.writer, formatStatusLine(stage, task))
		return
	}
	msg := progressUpdateMsg{
		stage:   stage,
		task:    task,
		status:  status,
		percent: clampPercent(percent),
		elapsed: elapsed,
	}
	if err != nil {
		msg.error = err.Error()
	}
	ui.program.Send(msg)
}

func (ui *buildProgressUI) Close() {
	ui.closeOnce.Do(func() {
		if ui.interactive && ui.program != nil {
			ui.program.Send(progressQuitMsg{})
		}
		<-ui.done
	})
}

func renderActivityFallback(writer io.Writer, sub *printingpress.ActivitySubscription) {
	printed := false
	lastLen := 0
	for update := range sub.Updates() {
		line := formatActivity(update)
		if line == "" {
			continue
		}
		padding := ""
		if diff := lastLen - len(line); diff > 0 {
			padding = strings.Repeat(" ", diff)
		}
		fmt.Fprintf(writer, "\r%s%s", line, padding)
		printed = true
		lastLen = len(line)
	}
	if printed {
		fmt.Fprintln(writer)
	}
}

func stagePercent(update printingpress.ActivityUpdate) float64 {
	if update.Status == "completed" {
		return 1
	}
	if update.TotalTasks > 0 {
		return clampPercent(update.PercentComplete / 100)
	}
	if update.Status == "running" {
		return 0.08
	}
	return 0
}

func clampPercent(p float64) float64 {
	if p < 0 {
		return 0
	}
	if p > 1 {
		return 1
	}
	return p
}

func buildStageCount(opts *rootOptions) int {
	total := 0
	if !opts.noHTML {
		total++
	}
	if !opts.noLLM {
		total++
	}
	if !opts.noJSON {
		total++
		if opts.noHTML && opts.noLLM {
			total++
		}
	}
	if total == 0 {
		return 1
	}
	return total
}

func (m progressModel) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m progressModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		if m.quitting {
			return m, nil
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case progress.FrameMsg:
		var cmd tea.Cmd
		m.bar, cmd = m.bar.Update(msg)
		return m, cmd
	case progressUpdateMsg:
		stage := strings.ToUpper(msg.stage)
		if stage == "" {
			stage = "BUILD"
		}
		stageChanged := m.stage != "" && m.stage != stage
		m.stage = stage
		m.task = msg.task
		if m.task == "" {
			m.task = strings.ToLower(m.stage)
		}
		if msg.status == "failed" {
			m.failed = true
			m.errorText = msg.error
		}
		stagePercent := clampPercent(msg.percent)
		if msg.status == "completed" {
			if !m.completed[msg.stage] {
				m.completed[msg.stage] = true
				m.completedStages++
			}
			m.task = fmt.Sprintf("completed in %s", roundDuration(msg.elapsed))
			m.stagePercent = 0
		} else if msg.status == "failed" {
			m.stagePercent = 0
		} else {
			if stageChanged {
				m.stagePercent = 0
			}
			if stagePercent > m.stagePercent {
				m.stagePercent = stagePercent
			}
		}
		nextPercent := clampPercent((float64(m.completedStages) + m.stagePercent) / float64(m.totalStages))
		if nextPercent < m.percent {
			nextPercent = m.percent
		}
		m.percent = nextPercent
		cmd := m.bar.SetPercent(m.percent)
		return m, cmd
	case progressQuitMsg:
		m.quitting = true
		return m, tea.Quit
	}
	return m, nil
}

func (m progressModel) View() tea.View {
	if m.quitting {
		return tea.NewView("")
	}
	headline := fmt.Sprintf("%s %s", m.spinner.View(), m.titleStyle.Render(m.stage))
	if m.failed {
		headline = fmt.Sprintf("%s %s", m.spinner.View(), m.errorStyle.Render(m.stage+" failed"))
	}
	task := m.taskStyle.Render(m.task)
	if m.failed && m.errorText != "" {
		task = m.errorStyle.Render(m.errorText)
	}
	meta := m.mutedStyle.Render(fmt.Sprintf("%d/%d stages", m.completedStages, m.totalStages))
	return tea.NewView(fmt.Sprintf("%s\n%s %s\n%s", headline, m.bar.ViewAs(m.percent), meta, task))
}
