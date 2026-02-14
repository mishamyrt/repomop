package tui

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"

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
		opts:      scanner.ScanOptions{RootPath: root},
		state:     stateList,
		artifacts: artifacts,
		selected:  map[int]bool{0: true},
		cursor:    0,
		width:     120,
		height:    30,
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
	if !strings.Contains(output, "■") {
		t.Fatalf("selected marker must use ■:\n%s", output)
	}
	if !strings.Contains(output, "□") {
		t.Fatalf("unselected marker must use □:\n%s", output)
	}
	if !strings.Contains(output, "▌") {
		t.Fatalf("focused row must include a left block marker:\n%s", output)
	}

	artifactLine := firstLineContaining(output, "2.0 KiB", "workspace/node_modules", "node_modules")
	if artifactLine == "" {
		t.Fatalf("artifact line with expected fields not found:\n%s", output)
	}
	expectedOrder := regexp.MustCompile(`■\s+2\.0 KiB\s+workspace/node_modules\s+node_modules`)
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
		opts:      scanner.ScanOptions{RootPath: root},
		state:     stateList,
		artifacts: artifacts,
		selected:  map[int]bool{},
		cursor:    0,
		width:     43,
		height:    20,
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
		opts:      scanner.ScanOptions{RootPath: root},
		state:     stateList,
		artifacts: artifacts,
		selected:  map[int]bool{},
		cursor:    0,
		width:     120,
		height:    30,
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
	if !strings.Contains(lastLine, "Keys: ↑↓ move, Space select, q quit") {
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
	outputWithSelection := stripANSI(withSelection.renderListView())
	if !strings.Contains(outputWithSelection, "Keys: ↑↓ move, Space select, Enter confirm, q quit") {
		t.Fatalf("help must include Enter confirm when selection exists:\n%s", outputWithSelection)
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
