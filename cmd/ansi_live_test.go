package cmd

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/require"
)

func TestSplitViewLines_UsesTeaViewContent(t *testing.T) {
	lines := splitViewLines(tea.NewView("one\ntwo"))
	require.Equal(t, []string{"one", "two"}, lines)
}
