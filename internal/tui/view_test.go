package tui

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	deleter "repomop/internal/delete"
	"repomop/internal/scanner"
)

var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func TestRenderListViewUsesFzfLikeMarkersAndFormat(t *testing.T) {
	root := filepath.Join(string(filepath.Separator), "repo")
	artifacts := []scanner.Artifact{
		{
			Kind:      scanner.ArtifactNodeModule,
			Path:      filepath.Join(root, "workspace", "node_modules"),
			SizeBytes: 2048,
		},
		{
			Kind:      scanner.ArtifactNodeModule,
			Path:      filepath.Join(root, "workspace", "vendor", "node_modules"),
			SizeBytes: 1024,
		},
	}

	model := Model{
		opts:            scanner.ScanOptions{RootPath: root},
		state:           stateList,
		artifacts:       artifacts,
		selected:        map[int]bool{0: true},
		selectedCount:   1,
		selectedSize:    2048,
		sizeColumnWidth: computeSizeColumnWidth(artifacts),
		cursor:          0,
		width:           120,
		height:          30,
	}

	output := stripANSI(model.renderListView())
	if strings.Contains(output, "[ ]") || strings.Contains(output, "[x]") {
		t.Fatalf("legacy checkboxes must not be present:\n%s", output)
	}
	if strings.Contains(output, "(project:") {
		t.Fatalf("project column must not be present:\n%s", output)
	}
	if strings.Contains(output, "\n>") {
		t.Fatalf("legacy cursor marker must not be present:\n%s", output)
	}
	if !strings.Contains(output, "●") {
		t.Fatalf("selected marker must use ●:\n%s", output)
	}
	if !strings.Contains(output, "○") {
		t.Fatalf("unselected marker must use ○:\n%s", output)
	}
	if !strings.Contains(output, "▶") {
		t.Fatalf("focused row must include a cursor marker ▶:\n%s", output)
	}

	artifactLine := firstLineContaining(output, "2.0 KiB", "workspace/node_modules", "node_modules")
	if artifactLine == "" {
		t.Fatalf("artifact line with expected fields not found:\n%s", output)
	}
	expectedOrder := regexp.MustCompile(`●\s+2\.0 KiB\s+workspace/node_modules\s+node-modules`)
	if !expectedOrder.MatchString(artifactLine) {
		t.Fatalf("artifact line must be in 'marker size path type' order, got: %q", artifactLine)
	}
}

func TestRenderListViewTruncatesPathFromLeft(t *testing.T) {
	root := filepath.Join(string(filepath.Separator), "repo")
	artifactPath := filepath.Join(root, "a", "very", "long", "path", "node_modules")
	artifacts := []scanner.Artifact{
		{
			Kind:      scanner.ArtifactNodeModule,
			Path:      artifactPath,
			SizeBytes: 10 * 1024,
		},
	}

	model := Model{
		opts:            scanner.ScanOptions{RootPath: root},
		state:           stateList,
		artifacts:       artifacts,
		selected:        map[int]bool{},
		sizeColumnWidth: computeSizeColumnWidth(artifacts),
		cursor:          0,
		width:           43,
		height:          20,
	}

	output := stripANSI(model.renderListView())
	artifactLine := firstLineContaining(output, "10.0 KiB", "node_modules")
	if artifactLine == "" {
		t.Fatalf("artifact line not found:\n%s", output)
	}
	if !strings.Contains(artifactLine, "…/") {
		t.Fatalf("expected left truncation with ellipsis prefix, got: %q", artifactLine)
	}
	if !strings.Contains(artifactLine, "ode_modules") {
		t.Fatalf("expected preserved tail of path after truncation, got: %q", artifactLine)
	}
	if strings.Contains(artifactLine, "a/very/long/path/node_modules") {
		t.Fatalf("expected full path to be truncated, got: %q", artifactLine)
	}
}

func TestRenderListViewLayoutAndHelpText(t *testing.T) {
	root := filepath.Join(string(filepath.Separator), "repo")
	artifacts := []scanner.Artifact{
		{
			Kind:      scanner.ArtifactNodeModule,
			Path:      filepath.Join(root, "workspace", "node_modules"),
			SizeBytes: 1024,
		},
	}

	base := Model{
		opts:            scanner.ScanOptions{RootPath: root},
		state:           stateList,
		artifacts:       artifacts,
		selected:        map[int]bool{},
		sizeColumnWidth: computeSizeColumnWidth(artifacts),
		cursor:          0,
		width:           120,
		height:          30,
	}

	outputNoSelection := stripANSI(base.renderListView())
	if strings.Contains(outputNoSelection, "\nrepomop\n") || strings.HasPrefix(outputNoSelection, "repomop\n") {
		t.Fatalf("list view must not render app title:\n%s", outputNoSelection)
	}
	if strings.Contains(outputNoSelection, "Enter confirm") {
		t.Fatalf("help must not include Enter confirm when nothing is selected:\n%s", outputNoSelection)
	}

	lines := strings.Split(outputNoSelection, "\n")
	if len(lines) < 3 {
		t.Fatalf("unexpected list view layout:\n%s", outputNoSelection)
	}
	if !strings.Contains(lines[0], "Found 1 artifacts") {
		t.Fatalf("first line must contain artifact count, got: %q", lines[0])
	}
	if !strings.Contains(lines[1], "Selected 0 · Reclaimable 0 B") {
		t.Fatalf("second line must contain selection summary, got: %q", lines[1])
	}
	lastLine := lines[len(lines)-1]
	if !strings.Contains(lastLine, "↑↓ | Space select | q quit") {
		t.Fatalf("last line must contain help text at bottom, got: %q", lastLine)
	}
	if !strings.Contains(outputNoSelection, "▶ ○") {
		t.Fatalf("focused row must start with active marker ▶:\n%s", outputNoSelection)
	}

	artifactLineIdx := -1
	for idx, line := range lines {
		if strings.Contains(line, "workspace/node_modules") {
			artifactLineIdx = idx
			break
		}
	}
	if artifactLineIdx == -1 {
		t.Fatalf("artifact line not found:\n%s", outputNoSelection)
	}
	if artifactLineIdx <= 1 {
		t.Fatalf("selection summary must be above artifact list:\n%s", outputNoSelection)
	}

	withSelection := base
	withSelection.selected = map[int]bool{0: true}
	withSelection.selectedCount = 1
	withSelection.selectedSize = 1024
	outputWithSelection := stripANSI(withSelection.renderListView())
	if !strings.Contains(outputWithSelection, "↑↓ | Space select | Enter confirm | q quit") {
		t.Fatalf("help must include Enter confirm when selection exists:\n%s", outputWithSelection)
	}
}

// --- View() dispatch ---

func TestViewLoading(t *testing.T) {
	m := NewModel(scanner.ScanOptions{RootPath: "/test"})
	m.state = stateLoading
	output := stripANSI(m.View())
	if !strings.Contains(output, "Scanning") {
		t.Fatalf("expected scanning text, got:\n%s", output)
	}
}

func TestViewError(t *testing.T) {
	m := Model{state: stateError, message: "something broke"}
	output := stripANSI(m.View())
	if !strings.Contains(output, "something broke") {
		t.Fatalf("expected error message, got:\n%s", output)
	}
	if !strings.Contains(output, "error") {
		t.Fatalf("expected error title, got:\n%s", output)
	}
}

func TestViewDeleting(t *testing.T) {
	m := NewModel(scanner.ScanOptions{RootPath: "/test"})
	m.state = stateDeleting
	output := stripANSI(m.View())
	if !strings.Contains(output, "Removing") {
		t.Fatalf("expected removing text, got:\n%s", output)
	}
}

func TestViewDoneNoArtifacts(t *testing.T) {
	m := Model{
		state:   stateDone,
		message: "No artifacts found.",
	}
	output := stripANSI(m.View())
	if !strings.Contains(output, "No artifacts found.") {
		t.Fatalf("expected no artifacts message, got:\n%s", output)
	}
}

func TestViewDoneWithDeleted(t *testing.T) {
	m := Model{
		state: stateDone,
		deleteResult: deleter.Result{
			Deleted:    []scanner.Artifact{{Path: "/a", SizeBytes: 1024}},
			FreedBytes: 1024,
			Errors:     []deleter.Error{},
		},
	}
	output := stripANSI(m.View())
	if !strings.Contains(output, "Deleted artifacts: 1") {
		t.Fatalf("expected deleted count, got:\n%s", output)
	}
	if !strings.Contains(output, "1.0 KiB") {
		t.Fatalf("expected freed space, got:\n%s", output)
	}
}

func TestViewDoneWithErrors(t *testing.T) {
	m := Model{
		state: stateDone,
		deleteResult: deleter.Result{
			Deleted: []scanner.Artifact{},
			Errors: []deleter.Error{
				{Artifact: scanner.Artifact{Path: "/fail"}, Err: fmt.Errorf("perm denied")},
			},
		},
	}
	output := stripANSI(m.View())
	if !strings.Contains(output, "Errors:") {
		t.Fatalf("expected errors section, got:\n%s", output)
	}
	if !strings.Contains(output, "perm denied") {
		t.Fatalf("expected error detail, got:\n%s", output)
	}
}

func TestViewDoneWithWarnings(t *testing.T) {
	m := Model{
		state:        stateDone,
		scanWarnings: []error{fmt.Errorf("warn1")},
		deleteResult: deleter.Result{
			Deleted: []scanner.Artifact{{Path: "/x"}},
			Errors:  []deleter.Error{},
		},
	}
	output := stripANSI(m.View())
	if !strings.Contains(output, "Size warnings: 1") {
		t.Fatalf("expected warning count, got:\n%s", output)
	}
}

func TestViewDoneWithMessage(t *testing.T) {
	m := Model{
		state:   stateDone,
		message: "Some artifacts could not be removed.",
		deleteResult: deleter.Result{
			Deleted: []scanner.Artifact{{Path: "/x"}},
			Errors:  []deleter.Error{},
		},
	}
	output := stripANSI(m.View())
	if !strings.Contains(output, "Some artifacts could not be removed.") {
		t.Fatalf("expected message, got:\n%s", output)
	}
}

func TestViewDoneWithFatalError(t *testing.T) {
	m := Model{
		state:    stateDone,
		fatalErr: fmt.Errorf("fatal"),
		message:  "fatal",
	}
	output := stripANSI(m.View())
	if !strings.Contains(output, "error") {
		t.Fatalf("expected error view, got:\n%s", output)
	}
}

func TestViewConfirm(t *testing.T) {
	m := Model{
		opts:  scanner.ScanOptions{RootPath: "/repo"},
		state: stateConfirm,
		artifacts: []scanner.Artifact{
			{Kind: scanner.ArtifactNodeModule, Path: "/repo/a/node_modules", SizeBytes: 1024},
			{Kind: scanner.ArtifactRustTarget, Path: "/repo/b/target", SizeBytes: 2048},
		},
		selected:      map[int]bool{0: true, 1: true},
		selectedCount: 2,
		selectedSize:  3072,
	}
	output := stripANSI(m.View())
	if !strings.Contains(output, "Confirm deletion") {
		t.Fatalf("expected confirm title, got:\n%s", output)
	}
	if !strings.Contains(output, "Artifacts selected: 2") {
		t.Fatalf("expected selection count, got:\n%s", output)
	}
	if !strings.Contains(output, "Proceed? [y/N]") {
		t.Fatalf("expected proceed prompt, got:\n%s", output)
	}
}

func TestViewConfirmMoreThanFive(t *testing.T) {
	arts := make([]scanner.Artifact, 7)
	selected := make(map[int]bool)
	for i := range arts {
		arts[i] = scanner.Artifact{
			Kind:      scanner.ArtifactNodeModule,
			Path:      fmt.Sprintf("/repo/p%d/node_modules", i),
			SizeBytes: int64((i + 1) * 100),
		}
		selected[i] = true
	}
	m := Model{
		opts:          scanner.ScanOptions{RootPath: "/repo"},
		state:         stateConfirm,
		artifacts:     arts,
		selected:      selected,
		selectedCount: 7,
	}
	output := stripANSI(m.View())
	if !strings.Contains(output, "and 2 more") {
		t.Fatalf("expected '... and 2 more', got:\n%s", output)
	}
}

// --- visibleRange ---

func TestVisibleRangeEmpty(t *testing.T) {
	m := Model{artifacts: []scanner.Artifact{}, height: 30}
	start, end := m.visibleRange()
	if start != 0 || end != 0 {
		t.Fatalf("expected 0,0, got %d,%d", start, end)
	}
}

func TestVisibleRangeSmallHeight(t *testing.T) {
	arts := make([]scanner.Artifact, 20)
	for i := range arts {
		arts[i] = scanner.Artifact{Path: fmt.Sprintf("/p%d", i)}
	}
	m := Model{artifacts: arts, height: 5, cursor: 0}
	start, end := m.visibleRange()
	if end-start < 5 {
		t.Fatalf("expected at least 5 visible, got %d", end-start)
	}
}

func TestVisibleRangeCursorAtEnd(t *testing.T) {
	arts := make([]scanner.Artifact, 50)
	for i := range arts {
		arts[i] = scanner.Artifact{Path: fmt.Sprintf("/p%d", i)}
	}
	m := Model{artifacts: arts, height: 20, cursor: 49}
	start, end := m.visibleRange()
	if end != 50 {
		t.Fatalf("expected end 50, got %d", end)
	}
	if start >= end {
		t.Fatalf("expected start < end, got %d >= %d", start, end)
	}
}

// --- computeSizeColumnWidth ---

func TestComputeSizeColumnWidth(t *testing.T) {
	arts := []scanner.Artifact{
		{SizeBytes: 100},
		{SizeBytes: 1024 * 1024 * 100},
	}
	w := computeSizeColumnWidth(arts)
	if w <= 0 {
		t.Fatalf("expected positive width, got %d", w)
	}
}

// --- truncatePathLeft ---

func TestTruncatePathLeftShortPath(t *testing.T) {
	got := truncatePathLeft("short", 20)
	if got != "short" {
		t.Fatalf("expected 'short', got %q", got)
	}
}

func TestTruncatePathLeftZeroWidth(t *testing.T) {
	got := truncatePathLeft("anything", 0)
	if got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
}

func TestTruncatePathLeftWidthOne(t *testing.T) {
	got := truncatePathLeft("long/path/here", 1)
	if got != "…" {
		t.Fatalf("expected ellipsis, got %q", got)
	}
}

func TestTruncatePathLeftNoSeparator(t *testing.T) {
	got := truncatePathLeft("verylongnameindeed", 8)
	if !strings.HasPrefix(got, "…") {
		t.Fatalf("expected ellipsis prefix, got %q", got)
	}
	if !strings.HasSuffix(got, "indeed") {
		t.Fatalf("expected tail preserved, got %q", got)
	}
}

// --- tailByWidth ---

func TestTailByWidthZero(t *testing.T) {
	got := tailByWidth("abc", 0)
	if got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
}

func TestTailByWidthFull(t *testing.T) {
	got := tailByWidth("abc", 10)
	if got != "abc" {
		t.Fatalf("expected 'abc', got %q", got)
	}
}

func TestTailByWidthTruncated(t *testing.T) {
	got := tailByWidth("abcdef", 3)
	if got != "def" {
		t.Fatalf("expected 'def', got %q", got)
	}
}

// --- pathColumnWidth ---

func TestPathColumnWidthDefault(t *testing.T) {
	m := Model{width: 0, height: 20}
	w := m.pathColumnWidth(8, "node-modules", false)
	if w < 1 {
		t.Fatalf("expected positive width, got %d", w)
	}
}

func TestPathColumnWidthNarrow(t *testing.T) {
	m := Model{width: 10, height: 20}
	w := m.pathColumnWidth(8, "node-modules", false)
	if w < 1 {
		t.Fatalf("expected at least 1, got %d", w)
	}
}

func TestViewDefaultReturnsEmpty(t *testing.T) {
	m := Model{state: viewState(99)}
	output := m.View()
	if output != "" {
		t.Fatalf("expected empty for unknown state, got %q", output)
	}
}

// --- visibleRange edge cases ---

func TestVisibleRangeFewArtifacts(t *testing.T) {
	arts := make([]scanner.Artifact, 3)
	for i := range arts {
		arts[i] = scanner.Artifact{Path: fmt.Sprintf("/p%d", i)}
	}
	m := Model{artifacts: arts, height: 100, cursor: 0}
	start, end := m.visibleRange()
	if start != 0 {
		t.Fatalf("expected start 0, got %d", start)
	}
	if end != 3 {
		t.Fatalf("expected end 3, got %d", end)
	}
}

func TestVisibleRangeCursorMiddle(t *testing.T) {
	arts := make([]scanner.Artifact, 30)
	for i := range arts {
		arts[i] = scanner.Artifact{Path: fmt.Sprintf("/p%d", i)}
	}
	m := Model{artifacts: arts, height: 16, cursor: 15}
	start, end := m.visibleRange()
	if start < 0 {
		t.Fatalf("start should be >= 0, got %d", start)
	}
	if end > 30 {
		t.Fatalf("end should be <= 30, got %d", end)
	}
	if m.cursor < start || m.cursor >= end {
		t.Fatalf("cursor %d should be within [%d, %d)", m.cursor, start, end)
	}
}

// --- truncatePathLeft edge cases ---

func TestTruncatePathLeftSmallWidthWithSeparator(t *testing.T) {
	got := truncatePathLeft("a/b/c/d/e", 2)
	if got != "…" {
		t.Fatalf("expected ellipsis for very narrow width, got %q", got)
	}
}

func TestTruncatePathLeftExactFit(t *testing.T) {
	got := truncatePathLeft("abc", 3)
	if got != "abc" {
		t.Fatalf("expected exact fit 'abc', got %q", got)
	}
}

func TestTruncatePathLeftSingleCharPath(t *testing.T) {
	got := truncatePathLeft("x", 5)
	if got != "x" {
		t.Fatalf("expected 'x', got %q", got)
	}
}

// --- renderListView with message ---

func TestVisibleRangeStartNotNegative(t *testing.T) {
	arts := make([]scanner.Artifact, 10)
	for i := range arts {
		arts[i] = scanner.Artifact{Path: fmt.Sprintf("/p%d", i)}
	}
	// height=16, listViewChromeLines=6 => listHeight=10, but capped to len(artifacts)=10
	// cursor=9 => start = 9-10+1 = 0
	m := Model{artifacts: arts, height: 16, cursor: 9}
	start, end := m.visibleRange()
	if start < 0 {
		t.Fatalf("start should be >= 0, got %d", start)
	}
	if end != 10 {
		t.Fatalf("expected end 10, got %d", end)
	}
}

func TestVisibleRangeStartPositive(t *testing.T) {
	arts := make([]scanner.Artifact, 100)
	for i := range arts {
		arts[i] = scanner.Artifact{Path: fmt.Sprintf("/p%d", i)}
	}
	// height=16 => listHeight=10, cursor=50 => start=50-10+1=41
	m := Model{artifacts: arts, height: 16, cursor: 50}
	start, end := m.visibleRange()
	if start != 41 {
		t.Fatalf("expected start 41, got %d", start)
	}
	if end != 51 {
		t.Fatalf("expected end 51, got %d", end)
	}
}

func TestVisibleRangeEndCapped(t *testing.T) {
	arts := make([]scanner.Artifact, 10)
	for i := range arts {
		arts[i] = scanner.Artifact{Path: fmt.Sprintf("/p%d", i)}
	}
	// height=10 => listHeight=max(10-6, 5)=5, cursor=9 => start=9-5+1=5, end=10
	m := Model{artifacts: arts, height: 10, cursor: 9}
	start, end := m.visibleRange()
	if end != 10 {
		t.Fatalf("expected end 10, got %d", end)
	}
	if start != 5 {
		t.Fatalf("expected start 5, got %d", start)
	}
}

// Test truncatePathLeft with a single very long path segment after separator
func TestTruncatePathLeftLongLastSegment(t *testing.T) {
	// Path with separator but last segment is very long and must be truncated
	got := truncatePathLeft("x/verylongsegmentnamehere", 10)
	if !strings.HasPrefix(got, "…") {
		t.Fatalf("expected ellipsis prefix, got %q", got)
	}
}

func TestRenderListViewWithWarningMessage(t *testing.T) {
	root := filepath.Join(string(filepath.Separator), "repo")
	artifacts := []scanner.Artifact{
		{Kind: scanner.ArtifactNodeModule, Path: filepath.Join(root, "nm"), SizeBytes: 100},
	}
	m := Model{
		opts:            scanner.ScanOptions{RootPath: root},
		state:           stateList,
		artifacts:       artifacts,
		selected:        map[int]bool{},
		sizeColumnWidth: computeSizeColumnWidth(artifacts),
		cursor:          0,
		width:           120,
		height:          30,
		message:         "Some artifact sizes could not be fully calculated.",
	}
	output := stripANSI(m.renderListView())
	if !strings.Contains(output, "Some artifact sizes could not be fully calculated.") {
		t.Fatalf("expected warning message in output:\n%s", output)
	}
}

func stripANSI(value string) string {
	return ansiPattern.ReplaceAllString(value, "")
}

func firstLineContaining(value string, fragments ...string) string {
	lines := strings.Split(value, "\n")
	for _, line := range lines {
		matches := true
		for _, fragment := range fragments {
			if !strings.Contains(line, fragment) {
				matches = false
				break
			}
		}
		if matches {
			return line
		}
	}
	return ""
}
