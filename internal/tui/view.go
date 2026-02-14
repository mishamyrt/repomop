package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"repomop/internal/format"
)

var (
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	warnStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	errStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	okStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	dimStyle   = lipgloss.NewStyle().Faint(true)
)

// View renders the current TUI screen.
func (m Model) View() string {
	switch m.state {
	case stateLoading:
		return m.renderLoadingView()
	case stateList:
		return m.renderListView()
	case stateConfirm:
		return m.renderConfirmView()
	case stateDeleting:
		return m.renderDeletingView()
	case stateError:
		return m.renderErrorView()
	case stateDone:
		return m.renderDoneView()
	default:
		return ""
	}
}

func (m Model) renderLoadingView() string {
	body := fmt.Sprintf("%s Scanning directories and calculating sizes...", m.spinner.View())
	return strings.Join([]string{
		titleStyle.Render("repomop"),
		"",
		body,
		dimStyle.Render("Press q to quit."),
	}, "\n")
}

func (m Model) renderListView() string {
	lines := make([]string, 0, len(m.artifacts)+8)
	lines = append(lines, titleStyle.Render("repomop: select artifacts"))
	lines = append(lines, fmt.Sprintf("Found %d artifacts.", len(m.artifacts)))
	lines = append(lines, dimStyle.Render("Keys: ↑/↓ move, Space select, Enter confirm, q quit"))
	lines = append(lines, "")

	start, end := m.visibleRange()
	for idx := start; idx < end; idx++ {
		artifact := m.artifacts[idx]
		cursor := " "
		if idx == m.cursor {
			cursor = ">"
		}
		checkbox := "[ ]"
		if m.selected[idx] {
			checkbox = "[x]"
		}
		relPath := relativePathOrSelf(m.opts.RootPath, artifact.Path)
		relProject := relativePathOrSelf(m.opts.RootPath, artifact.ProjectRoot)

		line := fmt.Sprintf("%s %s %-11s %8s  %s  (project: %s)",
			cursor,
			checkbox,
			artifact.Kind,
			format.Bytes(artifact.SizeBytes),
			relPath,
			relProject,
		)
		lines = append(lines, line)
	}

	selectedCount := m.selectedCount()
	selectedSize := int64(0)
	for idx, chosen := range m.selected {
		if chosen && idx >= 0 && idx < len(m.artifacts) {
			selectedSize += m.artifacts[idx].SizeBytes
		}
	}
	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("Selected: %d, Potential free space: %s", selectedCount, format.Bytes(selectedSize)))
	if m.message != "" {
		lines = append(lines, warnStyle.Render(m.message))
	}

	return strings.Join(lines, "\n")
}

func (m Model) renderConfirmView() string {
	selected := m.selectedArtifacts()
	sum := int64(0)
	for _, artifact := range selected {
		sum += artifact.SizeBytes
	}

	lines := []string{
		titleStyle.Render("Confirm deletion"),
		"",
		fmt.Sprintf("Artifacts selected: %d", len(selected)),
		fmt.Sprintf("Estimated free space: %s", format.Bytes(sum)),
		dimStyle.Render("This action permanently removes directories."),
		"",
		"Proceed? [y/N]",
	}

	if len(selected) > 0 {
		lines = append(lines, "")
		lines = append(lines, "Top selections:")
		limit := len(selected)
		if limit > 5 {
			limit = 5
		}
		for i := 0; i < limit; i++ {
			artifact := selected[i]
			lines = append(lines, fmt.Sprintf("- %s (%s)", relativePathOrSelf(m.opts.RootPath, artifact.Path), format.Bytes(artifact.SizeBytes)))
		}
		if len(selected) > limit {
			lines = append(lines, dimStyle.Render(fmt.Sprintf("... and %d more", len(selected)-limit)))
		}
	}

	return strings.Join(lines, "\n")
}

func (m Model) renderDeletingView() string {
	return strings.Join([]string{
		titleStyle.Render("repomop"),
		"",
		fmt.Sprintf("%s Removing selected artifacts...", m.spinner.View()),
	}, "\n")
}

func (m Model) renderDoneView() string {
	if m.fatalErr != nil {
		return m.renderErrorView()
	}

	deletedCount := len(m.deleteResult.Deleted)
	deletedSize := m.deleteResult.FreedBytes
	if deletedCount == 0 && m.message == "No artifacts found." {
		return strings.Join([]string{
			titleStyle.Render("repomop"),
			"",
			okStyle.Render(m.message),
			dimStyle.Render("Press Enter to exit."),
		}, "\n")
	}

	lines := []string{
		titleStyle.Render("repomop: done"),
		"",
		okStyle.Render(fmt.Sprintf("Deleted artifacts: %d", deletedCount)),
		okStyle.Render(fmt.Sprintf("Freed space: %s", format.Bytes(deletedSize))),
	}

	if len(m.deleteResult.Errors) > 0 {
		lines = append(lines, "")
		lines = append(lines, errStyle.Render("Errors:"))
		for _, item := range m.deleteResult.Errors {
			lines = append(lines, errStyle.Render(fmt.Sprintf("- %s: %v", relativePathOrSelf(m.opts.RootPath, item.Artifact.Path), item.Err)))
		}
	}

	if len(m.scanWarnings) > 0 {
		lines = append(lines, "")
		lines = append(lines, warnStyle.Render(fmt.Sprintf("Size warnings: %d", len(m.scanWarnings))))
	}
	if m.message != "" {
		lines = append(lines, "")
		lines = append(lines, warnStyle.Render(m.message))
	}

	lines = append(lines, "")
	lines = append(lines, dimStyle.Render("Press Enter to exit."))
	return strings.Join(lines, "\n")
}

func (m Model) renderErrorView() string {
	lines := []string{
		titleStyle.Render("repomop: error"),
		"",
		errStyle.Render(m.message),
		"",
		dimStyle.Render("Press Enter to exit."),
	}
	return strings.Join(lines, "\n")
}

func (m Model) visibleRange() (int, int) {
	if len(m.artifacts) == 0 {
		return 0, 0
	}
	listHeight := m.height - 6
	if listHeight < 5 {
		listHeight = 5
	}
	if listHeight > len(m.artifacts) {
		listHeight = len(m.artifacts)
	}

	start := m.cursor - listHeight + 1
	if start < 0 {
		start = 0
	}
	end := start + listHeight
	if end > len(m.artifacts) {
		end = len(m.artifacts)
		start = end - listHeight
		if start < 0 {
			start = 0
		}
	}
	return start, end
}

func relativePathOrSelf(root string, path string) string {
	if root == "" || path == "" {
		return path
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return path
	}
	if rel == "." {
		return path
	}
	return rel
}
