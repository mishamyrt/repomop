package tui

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"repomop/internal/format"
	"repomop/internal/pathutil"
	"repomop/internal/scanner"
)

var (
	titleStyle          = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	subtitleStyle       = lipgloss.NewStyle().Faint(true)
	warnStyle           = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	errStyle            = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	okStyle             = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	dimStyle            = lipgloss.NewStyle().Faint(true)
	helpStyle           = lipgloss.NewStyle().Faint(true)
	selectedMarkerStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("10"))
	idleMarkerStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	focusBarStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Bold(true)
	idleBarStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("236"))
	sizeStyle           = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	pathStyle           = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{
		Light: "0",
		Dark:  "7",
	})
	kindStyle       = lipgloss.NewStyle().Faint(true)
	focusedRowStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Bold(true)
	summaryStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Bold(true)
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

	helpText := "↑↓ | Space select | q quit"
	if m.selectedCount > 0 {
		helpText = "↑↓ | Space select | Enter confirm | q quit"
	}

	lines = append(lines, subtitleStyle.Render(fmt.Sprintf("Found %d artifacts", len(m.artifacts))))
	lines = append(lines, summaryStyle.Render(fmt.Sprintf("Selected %d · Reclaimable %s", m.selectedCount, format.Bytes(m.selectedSize))))
	lines = append(lines, "")

	start, end := m.visibleRange()
	for idx := start; idx < end; idx++ {
		lines = append(lines, m.renderArtifactLine(idx, m.sizeColumnWidth))
	}

	if m.message != "" {
		lines = append(lines, "")
		lines = append(lines, warnStyle.Render(m.message))
	}
	lines = append(lines, "")
	lines = append(lines, helpStyle.Render(helpText))

	return strings.Join(lines, "\n")
}

func (m Model) renderArtifactLine(idx int, sizeWidth int) string {
	artifact := m.artifacts[idx]
	focused := idx == m.cursor

	marker := "○"
	markerRenderer := idleMarkerStyle
	if _, ok := m.selected[idx]; ok {
		marker = "●"
		markerRenderer = selectedMarkerStyle
	}

	path := pathutil.RelativePathOrSelf(m.opts.RootPath, artifact.Path)
	pathWidth := m.pathColumnWidth(sizeWidth, artifact.Kind.String(), focused)
	pathTextRaw := truncatePathLeft(path, pathWidth)

	barTextRaw := "  "
	barRenderer := idleBarStyle
	sizeRenderer := sizeStyle
	pathRenderer := pathStyle
	kindRenderer := kindStyle
	separator := "  "

	if focused {
		barTextRaw = "▶ "
		barRenderer = focusBarStyle
		barRenderer = barRenderer.Background(focusedRowStyle.GetBackground())
		markerRenderer = markerRenderer.Background(focusedRowStyle.GetBackground()).Bold(true)
		sizeRenderer = sizeRenderer.Background(focusedRowStyle.GetBackground()).Bold(true)
		pathRenderer = pathRenderer.Background(focusedRowStyle.GetBackground()).Bold(true)
		kindRenderer = kindRenderer.Background(focusedRowStyle.GetBackground()).Bold(true)
		separator = focusedRowStyle.Render("  ")
	}

	barText := barRenderer.Render(barTextRaw)
	sizeText := sizeRenderer.Width(sizeWidth).Align(lipgloss.Right).Render(format.Bytes(artifact.SizeBytes))
	pathText := pathRenderer.Render(pathTextRaw)
	kindText := kindRenderer.Render(artifact.Kind.String())
	markerText := markerRenderer.Render(marker)

	return barText + markerText + separator + sizeText + separator + pathText + separator + kindText
}

func (m Model) renderConfirmView() string {
	selected := m.cachedSelection
	if selected == nil {
		selected = m.selectedArtifacts()
	}
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
			lines = append(lines, fmt.Sprintf("- %s (%s)", pathutil.RelativePathOrSelf(m.opts.RootPath, artifact.Path), format.Bytes(artifact.SizeBytes)))
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
			lines = append(lines, errStyle.Render(fmt.Sprintf("- %s: %v", pathutil.RelativePathOrSelf(m.opts.RootPath, item.Artifact.Path), item.Err)))
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

// listViewChromeLines is the number of non-list lines in list view:
// subtitle + summary + blank + (warning + blank) + help.
const listViewChromeLines = 6

func (m Model) visibleRange() (int, int) {
	if len(m.artifacts) == 0 {
		return 0, 0
	}
	listHeight := m.height - listViewChromeLines
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

func computeSizeColumnWidth(artifacts []scanner.Artifact) int {
	width := lipgloss.Width(format.Bytes(0))
	for _, artifact := range artifacts {
		current := lipgloss.Width(format.Bytes(artifact.SizeBytes))
		if current > width {
			width = current
		}
	}
	return width
}

func (m Model) pathColumnWidth(sizeWidth int, kind string, focused bool) int {
	totalWidth := m.width
	if totalWidth <= 0 {
		totalWidth = 80
	}

	const (
		barWidth       = 2
		markerWidth    = 1
		separatorWidth = 6
	)
	kindWidth := lipgloss.Width(kind) + kindStyle.GetHorizontalFrameSize()
	pathWidth := totalWidth - barWidth - markerWidth - separatorWidth - sizeWidth - kindWidth
	if focused {
		pathWidth -= focusedRowStyle.GetHorizontalFrameSize()
	}
	if pathWidth < 1 {
		return 1
	}
	return pathWidth
}

func truncatePathLeft(path string, maxWidth int) string {
	const ellipsis = "…"
	if maxWidth <= 0 {
		return ""
	}
	if lipgloss.Width(path) <= maxWidth {
		return path
	}
	if maxWidth == 1 {
		return ellipsis
	}

	separator := string(filepath.Separator)
	hasSeparator := strings.Contains(path, separator)

	prefix := ellipsis
	if hasSeparator {
		prefix = ellipsis + separator
	}
	prefixWidth := lipgloss.Width(prefix)
	if maxWidth <= prefixWidth {
		return ellipsis
	}
	remainingWidth := maxWidth - prefixWidth
	if remainingWidth <= 0 {
		return ellipsis
	}

	if !hasSeparator {
		tail := tailByWidth(path, remainingWidth)
		if tail == "" {
			return ellipsis
		}
		return prefix + tail
	}

	parts := strings.Split(path, separator)
	suffixParts := make([]string, 0, len(parts))
	width := 0
	for idx := len(parts) - 1; idx >= 0; idx-- {
		part := parts[idx]
		if part == "" {
			continue
		}

		partWidth := lipgloss.Width(part)
		if len(suffixParts) > 0 {
			partWidth += lipgloss.Width(separator)
		}
		if width+partWidth > remainingWidth {
			if len(suffixParts) == 0 {
				tail := tailByWidth(part, remainingWidth)
				if tail == "" {
					return ellipsis
				}
				return prefix + tail
			}
			break
		}

		suffixParts = append(suffixParts, part)
		width += partWidth
	}

	if len(suffixParts) == 0 {
		return ellipsis
	}

	slices.Reverse(suffixParts)
	return prefix + strings.Join(suffixParts, separator)
}

func tailByWidth(value string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	if lipgloss.Width(value) <= maxWidth {
		return value
	}

	runes := []rune(value)
	tail := make([]rune, 0, len(runes))
	width := 0
	for idx := len(runes) - 1; idx >= 0; idx-- {
		runeWidth := lipgloss.Width(string(runes[idx]))
		if width+runeWidth > maxWidth {
			break
		}
		tail = append(tail, runes[idx])
		width += runeWidth
	}

	slices.Reverse(tail)
	return string(tail)
}

