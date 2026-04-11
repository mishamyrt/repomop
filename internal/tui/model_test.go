package tui

import (
	"context"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"

	deleter "repomop/internal/delete"
	"repomop/internal/scanner"
	"repomop/internal/testutil"
)

func newTestModel(state viewState, artifacts []scanner.Artifact, selected map[int]struct{}) Model {
	m := Model{
		state:           state,
		artifacts:       artifacts,
		selected:        selected,
		selectedCount:   len(selected),
		sizeColumnWidth: 8,
		width:           120,
		height:          30,
	}
	if selected == nil {
		m.selected = map[int]struct{}{}
		m.selectedCount = 0
	}
	var sz int64
	for idx := range m.selected {
		if idx >= 0 && idx < len(artifacts) {
			sz += artifacts[idx].SizeBytes
		}
	}
	m.selectedSize = sz
	return m
}

var sampleArtifacts = []scanner.Artifact{
	{Kind: scanner.ArtifactNodeModule, Path: "/repo/node_modules", SizeBytes: 1024},
	{Kind: scanner.ArtifactRustTarget, Path: "/repo/target", SizeBytes: 2048},
	{Kind: scanner.ArtifactPythonVenv, Path: "/repo/.venv", SizeBytes: 512},
}

func press(key string) tea.KeyPressMsg {
	switch key {
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEsc}
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "up":
		return tea.KeyPressMsg{Code: tea.KeyUp}
	case "down":
		return tea.KeyPressMsg{Code: tea.KeyDown}
	case "ctrl+c":
		return tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}
	case "space":
		return tea.KeyPressMsg{Code: ' ', Text: " "}
	default:
		runes := []rune(key)
		if len(runes) != 1 {
			panic("unsupported test key: " + key)
		}
		return tea.KeyPressMsg{Code: runes[0], Text: key}
	}
}

// --- Loading state ---

func TestLoadingEscQuits(t *testing.T) {
	m := newTestModel(stateLoading, nil, nil)
	_, cmd := m.Update(press("esc"))
	if cmd == nil {
		t.Fatal("expected quit command")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("expected quit msg")
	}
}

func TestLoadingQKeyQuits(t *testing.T) {
	m := newTestModel(stateLoading, nil, nil)
	_, cmd := m.Update(press("q"))
	if cmd == nil {
		t.Fatal("expected quit command")
	}
}

func TestLoadingCtrlCQuits(t *testing.T) {
	m := newTestModel(stateLoading, nil, nil)
	_, cmd := m.Update(press("ctrl+c"))
	if cmd == nil {
		t.Fatal("expected quit command")
	}
}

// --- List state ---

func TestListViewEscQuits(t *testing.T) {
	m := newTestModel(stateList, sampleArtifacts, map[int]struct{}{})
	_, cmd := m.Update(press("esc"))
	if cmd == nil {
		t.Fatal("expected quit command on esc key")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("expected esc key to trigger quit")
	}
}

func TestConfirmViewEscReturnsToList(t *testing.T) {
	m := newTestModel(stateConfirm, sampleArtifacts, map[int]struct{}{0: {}})
	updated, cmd := m.Update(press("esc"))
	got := updated.(Model)
	if got.state != stateList {
		t.Fatalf("expected stateList, got %v", got.state)
	}
	if cmd != nil {
		t.Fatal("expected esc key to cancel confirm without quitting")
	}
}

func TestListViewEnterWithoutSelectionDoesNotSetHint(t *testing.T) {
	m := newTestModel(stateList, sampleArtifacts, map[int]struct{}{})
	m.message = "Some artifact sizes could not be fully calculated."

	updated, _ := m.Update(press("enter"))
	got := updated.(Model)
	if got.state != stateList {
		t.Fatalf("expected stateList, got %v", got.state)
	}
	if got.message != m.message {
		t.Fatalf("message should remain unchanged, got: %q", got.message)
	}
}

func TestListViewCursorUp(t *testing.T) {
	m := newTestModel(stateList, sampleArtifacts, map[int]struct{}{})
	m.cursor = 2
	updated, _ := m.Update(press("up"))
	got := updated.(Model)
	if got.cursor != 1 {
		t.Fatalf("expected cursor 1, got %d", got.cursor)
	}
}

func TestListViewCursorUpAtZero(t *testing.T) {
	m := newTestModel(stateList, sampleArtifacts, map[int]struct{}{})
	m.cursor = 0
	updated, _ := m.Update(press("up"))
	got := updated.(Model)
	if got.cursor != 0 {
		t.Fatalf("expected cursor 0, got %d", got.cursor)
	}
}

func TestListViewCursorDown(t *testing.T) {
	m := newTestModel(stateList, sampleArtifacts, map[int]struct{}{})
	m.cursor = 0
	updated, _ := m.Update(press("down"))
	got := updated.(Model)
	if got.cursor != 1 {
		t.Fatalf("expected cursor 1, got %d", got.cursor)
	}
}

func TestListViewCursorDownAtEnd(t *testing.T) {
	m := newTestModel(stateList, sampleArtifacts, map[int]struct{}{})
	m.cursor = len(sampleArtifacts) - 1
	updated, _ := m.Update(press("down"))
	got := updated.(Model)
	if got.cursor != len(sampleArtifacts)-1 {
		t.Fatalf("expected cursor at end, got %d", got.cursor)
	}
}

func TestListViewCursorK(t *testing.T) {
	m := newTestModel(stateList, sampleArtifacts, map[int]struct{}{})
	m.cursor = 1
	updated, _ := m.Update(press("k"))
	got := updated.(Model)
	if got.cursor != 0 {
		t.Fatalf("expected cursor 0, got %d", got.cursor)
	}
}

func TestListViewCursorJ(t *testing.T) {
	m := newTestModel(stateList, sampleArtifacts, map[int]struct{}{})
	m.cursor = 0
	updated, _ := m.Update(press("j"))
	got := updated.(Model)
	if got.cursor != 1 {
		t.Fatalf("expected cursor 1, got %d", got.cursor)
	}
}

func TestListViewSpaceTogglesSelection(t *testing.T) {
	m := newTestModel(stateList, sampleArtifacts, map[int]struct{}{})
	m.cursor = 1

	updated, _ := m.Update(press("space"))
	got := updated.(Model)
	if _, ok := got.selected[1]; !ok {
		t.Fatal("expected item 1 to be selected")
	}
	if got.selectedCount != 1 {
		t.Fatalf("expected selectedCount 1, got %d", got.selectedCount)
	}
	if got.selectedSize != sampleArtifacts[1].SizeBytes {
		t.Fatalf("expected selectedSize %d, got %d", sampleArtifacts[1].SizeBytes, got.selectedSize)
	}

	updated2, _ := got.Update(press("space"))
	got2 := updated2.(Model)
	if _, ok := got2.selected[1]; ok {
		t.Fatal("expected item 1 to be deselected")
	}
	if got2.selectedCount != 0 {
		t.Fatalf("expected selectedCount 0, got %d", got2.selectedCount)
	}
	if got2.selectedSize != 0 {
		t.Fatalf("expected selectedSize 0, got %d", got2.selectedSize)
	}
}

func TestListViewEnterWithSelectionGoesToConfirm(t *testing.T) {
	m := newTestModel(stateList, sampleArtifacts, map[int]struct{}{0: {}})

	updated, _ := m.Update(press("enter"))
	got := updated.(Model)
	if got.state != stateConfirm {
		t.Fatalf("expected stateConfirm, got %v", got.state)
	}
}

func TestListViewQQuits(t *testing.T) {
	m := newTestModel(stateList, sampleArtifacts, map[int]struct{}{})
	_, cmd := m.Update(press("q"))
	if cmd == nil {
		t.Fatal("expected quit command")
	}
}

// --- Confirm state ---

func TestConfirmYProceeds(t *testing.T) {
	m := newTestModel(stateConfirm, sampleArtifacts, map[int]struct{}{0: {}})

	updated, cmd := m.Update(press("y"))
	got := updated.(Model)
	if got.state != stateDeleting {
		t.Fatalf("expected stateDeleting, got %v", got.state)
	}
	if cmd == nil {
		t.Fatal("expected command for delete")
	}
}

func TestConfirmNReturnsToList(t *testing.T) {
	m := newTestModel(stateConfirm, sampleArtifacts, map[int]struct{}{0: {}})

	updated, _ := m.Update(press("n"))
	got := updated.(Model)
	if got.state != stateList {
		t.Fatalf("expected stateList, got %v", got.state)
	}
}

func TestConfirmEnterDoesNothing(t *testing.T) {
	m := newTestModel(stateConfirm, sampleArtifacts, map[int]struct{}{0: {}})

	updated, cmd := m.Update(press("enter"))
	got := updated.(Model)
	if got.state != stateConfirm {
		t.Fatalf("expected stateConfirm, got %v", got.state)
	}
	if cmd != nil {
		t.Fatal("expected no command for enter in confirm view")
	}
}

func TestConfirmCursorDown(t *testing.T) {
	artifacts := []scanner.Artifact{
		{Path: "/repo/a", SizeBytes: 1},
		{Path: "/repo/b", SizeBytes: 2},
		{Path: "/repo/c", SizeBytes: 3},
		{Path: "/repo/d", SizeBytes: 4},
		{Path: "/repo/e", SizeBytes: 5},
		{Path: "/repo/f", SizeBytes: 6},
	}
	m := newTestModel(stateConfirm, artifacts, map[int]struct{}{0: {}, 1: {}, 2: {}, 3: {}, 4: {}, 5: {}})
	m.height = 13

	updated, _ := m.Update(press("down"))
	got := updated.(Model)
	if got.confirmOffset != 1 {
		t.Fatalf("expected confirmOffset 1, got %d", got.confirmOffset)
	}
}

func TestConfirmCursorUpAtZero(t *testing.T) {
	artifacts := []scanner.Artifact{
		{Path: "/repo/a", SizeBytes: 1},
		{Path: "/repo/b", SizeBytes: 2},
		{Path: "/repo/c", SizeBytes: 3},
		{Path: "/repo/d", SizeBytes: 4},
	}
	m := newTestModel(stateConfirm, artifacts, map[int]struct{}{0: {}, 1: {}, 2: {}, 3: {}})
	m.height = 13

	updated, _ := m.Update(press("up"))
	got := updated.(Model)
	if got.confirmOffset != 0 {
		t.Fatalf("expected confirmOffset 0, got %d", got.confirmOffset)
	}
}

func TestConfirmCursorJAndK(t *testing.T) {
	artifacts := []scanner.Artifact{
		{Path: "/repo/a", SizeBytes: 1},
		{Path: "/repo/b", SizeBytes: 2},
		{Path: "/repo/c", SizeBytes: 3},
		{Path: "/repo/d", SizeBytes: 4},
		{Path: "/repo/e", SizeBytes: 5},
		{Path: "/repo/f", SizeBytes: 6},
	}
	m := newTestModel(stateConfirm, artifacts, map[int]struct{}{0: {}, 1: {}, 2: {}, 3: {}, 4: {}, 5: {}})
	m.height = 13

	updated, _ := m.Update(press("j"))
	got := updated.(Model)
	if got.confirmOffset != 1 {
		t.Fatalf("expected confirmOffset 1 after j, got %d", got.confirmOffset)
	}

	updated, _ = got.Update(press("k"))
	got = updated.(Model)
	if got.confirmOffset != 0 {
		t.Fatalf("expected confirmOffset 0 after k, got %d", got.confirmOffset)
	}
}

func TestConfirmCtrlCQuits(t *testing.T) {
	m := newTestModel(stateConfirm, sampleArtifacts, map[int]struct{}{0: {}})
	_, cmd := m.Update(press("ctrl+c"))
	if cmd == nil {
		t.Fatal("expected quit command")
	}
}

// --- Deleting state ---

func TestDeletingIgnoresNonQuitKeys(t *testing.T) {
	m := newTestModel(stateDeleting, sampleArtifacts, map[int]struct{}{0: {}})
	updated, cmd := m.Update(press("x"))
	got := updated.(Model)
	if got.state != stateDeleting {
		t.Fatalf("expected stateDeleting, got %v", got.state)
	}
	_ = cmd
}

func TestDeletingEscDoesNotQuit(t *testing.T) {
	m := newTestModel(stateDeleting, sampleArtifacts, map[int]struct{}{0: {}})
	_, cmd := m.Update(press("esc"))
	if cmd != nil {
		if _, ok := cmd().(tea.QuitMsg); ok {
			t.Fatal("deleting state should not quit on esc")
		}
	}
}

// --- Done state ---

func TestDoneEnterQuits(t *testing.T) {
	m := newTestModel(stateDone, nil, nil)
	_, cmd := m.Update(press("enter"))
	if cmd == nil {
		t.Fatal("expected quit command")
	}
}

func TestDoneQQuits(t *testing.T) {
	m := newTestModel(stateDone, nil, nil)
	_, cmd := m.Update(press("q"))
	if cmd == nil {
		t.Fatal("expected quit command")
	}
}

// --- Error state ---

func TestErrorEnterQuits(t *testing.T) {
	m := newTestModel(stateError, nil, nil)
	_, cmd := m.Update(press("enter"))
	if cmd == nil {
		t.Fatal("expected quit command")
	}
}

// --- scanFinishedMsg ---

func TestScanFinishedWithError(t *testing.T) {
	m := newTestModel(stateLoading, nil, nil)
	updated, _ := m.Update(scanFinishedMsg{err: errTest("scan failed")})
	got := updated.(Model)
	if got.state != stateError {
		t.Fatalf("expected stateError, got %v", got.state)
	}
	if got.message != "scan failed" {
		t.Fatalf("expected error message, got %q", got.message)
	}
	if got.fatalErr == nil {
		t.Fatal("expected fatalErr to be set")
	}
}

func TestScanFinishedNoArtifacts(t *testing.T) {
	m := newTestModel(stateLoading, nil, nil)
	updated, _ := m.Update(scanFinishedMsg{artifacts: []scanner.Artifact{}})
	got := updated.(Model)
	if got.state != stateDone {
		t.Fatalf("expected stateDone, got %v", got.state)
	}
	if got.message != "No artifacts found." {
		t.Fatalf("expected 'No artifacts found.', got %q", got.message)
	}
}

func TestScanFinishedWithArtifacts(t *testing.T) {
	m := newTestModel(stateLoading, nil, nil)
	updated, _ := m.Update(scanFinishedMsg{artifacts: sampleArtifacts})
	got := updated.(Model)
	if got.state != stateList {
		t.Fatalf("expected stateList, got %v", got.state)
	}
	if len(got.artifacts) != 3 {
		t.Fatalf("expected 3 artifacts, got %d", len(got.artifacts))
	}
}

func TestScanFinishedWithWarnings(t *testing.T) {
	m := newTestModel(stateLoading, nil, nil)
	updated, _ := m.Update(scanFinishedMsg{
		artifacts: sampleArtifacts,
		warnings:  []error{errTest("warn")},
	})
	got := updated.(Model)
	if got.state != stateList {
		t.Fatalf("expected stateList, got %v", got.state)
	}
	if got.message != "Some artifact sizes could not be fully calculated." {
		t.Fatalf("expected warning message, got %q", got.message)
	}
}

func TestTitleAnimationTickAdvancesInLoadingState(t *testing.T) {
	m := newTestModel(stateLoading, nil, nil)

	updated, cmd := m.Update(titleAnimationTickMsg{})
	got := updated.(Model)
	if got.titleAnimationFrame != 1 {
		t.Fatalf("expected title animation frame 1, got %d", got.titleAnimationFrame)
	}
	if cmd == nil {
		t.Fatal("expected next animation tick command")
	}
}

func TestTitleAnimationTickWrapsCycle(t *testing.T) {
	m := newTestModel(stateLoading, nil, nil)
	m.titleAnimationFrame = titleAnimationCycleLength() - 1

	updated, _ := m.Update(titleAnimationTickMsg{})
	got := updated.(Model)
	if got.titleAnimationFrame != 0 {
		t.Fatalf("expected wrapped animation frame 0, got %d", got.titleAnimationFrame)
	}
}

func TestTitleAnimationTickStopsOutsideLoadingState(t *testing.T) {
	m := newTestModel(stateList, sampleArtifacts, map[int]struct{}{})
	m.titleAnimationFrame = 3

	updated, cmd := m.Update(titleAnimationTickMsg{})
	got := updated.(Model)
	if got.titleAnimationFrame != 3 {
		t.Fatalf("expected title animation frame to remain 3, got %d", got.titleAnimationFrame)
	}
	if cmd != nil {
		t.Fatal("expected no animation command outside loading state")
	}
}

// --- deleteFinishedMsg ---

func TestDeleteFinishedSuccess(t *testing.T) {
	m := newTestModel(stateDeleting, sampleArtifacts, map[int]struct{}{0: {}})
	updated, _ := m.Update(deleteFinishedMsg{result: deleter.Result{
		Deleted:    []scanner.Artifact{sampleArtifacts[0]},
		Errors:     []deleter.Error{},
		FreedBytes: 1024,
	}})
	got := updated.(Model)
	if got.state != stateDone {
		t.Fatalf("expected stateDone, got %v", got.state)
	}
	if got.message != "Selected artifacts were removed." {
		t.Fatalf("unexpected message: %q", got.message)
	}
}

func TestDeleteFinishedWithErrors(t *testing.T) {
	m := newTestModel(stateDeleting, sampleArtifacts, map[int]struct{}{0: {}})
	updated, _ := m.Update(deleteFinishedMsg{result: deleter.Result{
		Deleted: []scanner.Artifact{},
		Errors: []deleter.Error{
			{Artifact: sampleArtifacts[0], Err: errTest("perm denied")},
		},
	}})
	got := updated.(Model)
	if got.state != stateDone {
		t.Fatalf("expected stateDone, got %v", got.state)
	}
	if got.message != "Some artifacts could not be removed." {
		t.Fatalf("unexpected message: %q", got.message)
	}
}

// --- WindowSizeMsg ---

func TestWindowSizeMsg(t *testing.T) {
	m := newTestModel(stateList, sampleArtifacts, map[int]struct{}{})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 200, Height: 50})
	got := updated.(Model)
	if got.width != 200 || got.height != 50 {
		t.Fatalf("expected 200x50, got %dx%d", got.width, got.height)
	}
}

// --- NewModel ---

func TestNewModel(t *testing.T) {
	opts := scanner.ScanOptions{RootPath: "/test", MaxDepth: 5}
	m := NewModel(opts)
	if m.state != stateLoading {
		t.Fatalf("expected stateLoading, got %v", m.state)
	}
	if m.opts.RootPath != "/test" {
		t.Fatalf("expected root path /test, got %s", m.opts.RootPath)
	}
}

// --- FatalError / DeleteResult ---

func TestFatalError(t *testing.T) {
	m := Model{fatalErr: errTest("fatal")}
	if m.FatalError() == nil {
		t.Fatal("expected fatal error")
	}
	if m.FatalError().Error() != "fatal" {
		t.Fatalf("unexpected error: %v", m.FatalError())
	}
}

func TestDeleteResult(t *testing.T) {
	m := Model{deleteResult: deleter.Result{FreedBytes: 999}}
	if m.DeleteResult().FreedBytes != 999 {
		t.Fatalf("expected 999, got %d", m.DeleteResult().FreedBytes)
	}
}

// --- selectedArtifacts ---

func TestSelectedArtifactsSort(t *testing.T) {
	m := newTestModel(stateList, sampleArtifacts, map[int]struct{}{0: {}, 2: {}})
	selected := m.selectedArtifacts()
	if len(selected) != 2 {
		t.Fatalf("expected 2, got %d", len(selected))
	}
	if selected[0].SizeBytes < selected[1].SizeBytes {
		t.Fatal("expected sorted by size descending")
	}
}

func TestSelectedArtifactsEmpty(t *testing.T) {
	m := newTestModel(stateList, sampleArtifacts, map[int]struct{}{})
	selected := m.selectedArtifacts()
	if selected != nil {
		t.Fatalf("expected nil, got %v", selected)
	}
}

// --- Confirm with empty selected (edge case) ---

func TestConfirmYWithEmptySelectedReturnsToList(t *testing.T) {
	m := newTestModel(stateConfirm, sampleArtifacts, map[int]struct{}{})

	updated, _ := m.Update(press("y"))
	got := updated.(Model)
	if got.state != stateList {
		t.Fatalf("expected stateList when no actual artifacts selected, got %v", got.state)
	}
}

// --- Init ---

func TestInitReturnsCmd(t *testing.T) {
	m := NewModel(scanner.ScanOptions{RootPath: "/test"})
	cmd := m.Init()
	if cmd == nil {
		t.Fatal("expected Init to return a command")
	}

	msg := cmd()
	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		t.Fatalf("expected tea.BatchMsg, got %T", msg)
	}
	if len(batch) != 3 {
		t.Fatalf("expected 3 init commands, got %d", len(batch))
	}
}

// --- selectedArtifacts with out-of-bounds index ---

func TestSelectedArtifactsSkipsOutOfBounds(t *testing.T) {
	m := newTestModel(stateList, sampleArtifacts, map[int]struct{}{0: {}, 99: {}, -1: {}})
	selected := m.selectedArtifacts()
	if len(selected) != 1 {
		t.Fatalf("expected 1 valid selection, got %d", len(selected))
	}
}

// --- scanArtifactsCmd / deleteArtifactsCmd closures ---

func TestScanArtifactsCmdSuccess(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "js")
	testutil.MkdirAll(t, filepath.Join(project, "node_modules"))
	testutil.WriteFile(t, filepath.Join(project, "package.json"), "{}")
	testutil.WriteFileSized(t, filepath.Join(project, "node_modules", "x.js"), 10)

	cmd := scanArtifactsCmd(context.Background(), scanner.ScanOptions{RootPath: root, MaxDepth: -1})
	msg := cmd()
	result, ok := msg.(scanFinishedMsg)
	if !ok {
		t.Fatalf("expected scanFinishedMsg, got %T", msg)
	}
	if result.err != nil {
		t.Fatalf("unexpected error: %v", result.err)
	}
	if len(result.artifacts) != 1 {
		t.Fatalf("expected 1 artifact, got %d", len(result.artifacts))
	}
}

func TestScanArtifactsCmdError(t *testing.T) {
	cmd := scanArtifactsCmd(context.Background(), scanner.ScanOptions{RootPath: "", MaxDepth: -1})
	msg := cmd()
	result, ok := msg.(scanFinishedMsg)
	if !ok {
		t.Fatalf("expected scanFinishedMsg, got %T", msg)
	}
	if result.err == nil {
		t.Fatal("expected error for empty root")
	}
}

func TestDeleteArtifactsCmdSuccess(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "nm")
	testutil.MkdirAll(t, dir)
	testutil.WriteFileSized(t, filepath.Join(dir, "x.js"), 10)

	artifacts := []scanner.Artifact{
		{Kind: scanner.ArtifactNodeModule, Path: dir, ProjectRoot: root, SizeBytes: 10},
	}
	cmd := deleteArtifactsCmd(context.Background(), artifacts)
	msg := cmd()
	result, ok := msg.(deleteFinishedMsg)
	if !ok {
		t.Fatalf("expected deleteFinishedMsg, got %T", msg)
	}
	if len(result.result.Deleted) != 1 {
		t.Fatalf("expected 1 deleted, got %d", len(result.result.Deleted))
	}
}

type errTest string

func (e errTest) Error() string { return string(e) }
