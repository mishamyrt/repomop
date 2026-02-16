package tui

import (
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	deleter "repomop/internal/delete"
	"repomop/internal/scanner"
	"repomop/internal/testutil"
)

func newTestModel(state viewState, artifacts []scanner.Artifact, selected map[int]bool) Model {
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
		m.selected = map[int]bool{}
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

// --- Loading state ---

func TestLoadingEscQuits(t *testing.T) {
	m := newTestModel(stateLoading, nil, nil)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("expected quit command")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("expected quit msg")
	}
}

func TestLoadingQKeyQuits(t *testing.T) {
	m := newTestModel(stateLoading, nil, nil)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatal("expected quit command")
	}
}

func TestLoadingCtrlCQuits(t *testing.T) {
	m := newTestModel(stateLoading, nil, nil)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("expected quit command")
	}
}

// --- List state ---

func TestListViewEscQuits(t *testing.T) {
	m := newTestModel(stateList, sampleArtifacts, map[int]bool{})
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("expected quit command on esc key")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("expected esc key to trigger quit")
	}
}

func TestConfirmViewEscQuits(t *testing.T) {
	m := newTestModel(stateConfirm, sampleArtifacts, map[int]bool{0: true})
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("expected quit command on esc key")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("expected esc key to trigger quit from confirm view")
	}
}

func TestListViewEnterWithoutSelectionDoesNotSetHint(t *testing.T) {
	m := newTestModel(stateList, sampleArtifacts, map[int]bool{})
	m.message = "Some artifact sizes could not be fully calculated."

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(Model)
	if got.state != stateList {
		t.Fatalf("expected stateList, got %v", got.state)
	}
	if got.message != m.message {
		t.Fatalf("message should remain unchanged, got: %q", got.message)
	}
}

func TestListViewCursorUp(t *testing.T) {
	m := newTestModel(stateList, sampleArtifacts, map[int]bool{})
	m.cursor = 2
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
	got := updated.(Model)
	if got.cursor != 1 {
		t.Fatalf("expected cursor 1, got %d", got.cursor)
	}
}

func TestListViewCursorUpAtZero(t *testing.T) {
	m := newTestModel(stateList, sampleArtifacts, map[int]bool{})
	m.cursor = 0
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
	got := updated.(Model)
	if got.cursor != 0 {
		t.Fatalf("expected cursor 0, got %d", got.cursor)
	}
}

func TestListViewCursorDown(t *testing.T) {
	m := newTestModel(stateList, sampleArtifacts, map[int]bool{})
	m.cursor = 0
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	got := updated.(Model)
	if got.cursor != 1 {
		t.Fatalf("expected cursor 1, got %d", got.cursor)
	}
}

func TestListViewCursorDownAtEnd(t *testing.T) {
	m := newTestModel(stateList, sampleArtifacts, map[int]bool{})
	m.cursor = len(sampleArtifacts) - 1
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	got := updated.(Model)
	if got.cursor != len(sampleArtifacts)-1 {
		t.Fatalf("expected cursor at end, got %d", got.cursor)
	}
}

func TestListViewCursorK(t *testing.T) {
	m := newTestModel(stateList, sampleArtifacts, map[int]bool{})
	m.cursor = 1
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	got := updated.(Model)
	if got.cursor != 0 {
		t.Fatalf("expected cursor 0, got %d", got.cursor)
	}
}

func TestListViewCursorJ(t *testing.T) {
	m := newTestModel(stateList, sampleArtifacts, map[int]bool{})
	m.cursor = 0
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	got := updated.(Model)
	if got.cursor != 1 {
		t.Fatalf("expected cursor 1, got %d", got.cursor)
	}
}

func TestListViewSpaceTogglesSelection(t *testing.T) {
	m := newTestModel(stateList, sampleArtifacts, map[int]bool{})
	m.cursor = 1

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	got := updated.(Model)
	if !got.selected[1] {
		t.Fatal("expected item 1 to be selected")
	}
	if got.selectedCount != 1 {
		t.Fatalf("expected selectedCount 1, got %d", got.selectedCount)
	}
	if got.selectedSize != sampleArtifacts[1].SizeBytes {
		t.Fatalf("expected selectedSize %d, got %d", sampleArtifacts[1].SizeBytes, got.selectedSize)
	}

	updated2, _ := got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	got2 := updated2.(Model)
	if got2.selected[1] {
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
	m := newTestModel(stateList, sampleArtifacts, map[int]bool{0: true})

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(Model)
	if got.state != stateConfirm {
		t.Fatalf("expected stateConfirm, got %v", got.state)
	}
}

func TestListViewQQuits(t *testing.T) {
	m := newTestModel(stateList, sampleArtifacts, map[int]bool{})
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatal("expected quit command")
	}
}

// --- Confirm state ---

func TestConfirmYProceeds(t *testing.T) {
	m := newTestModel(stateConfirm, sampleArtifacts, map[int]bool{0: true})

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	got := updated.(Model)
	if got.state != stateDeleting {
		t.Fatalf("expected stateDeleting, got %v", got.state)
	}
	if cmd == nil {
		t.Fatal("expected command for delete")
	}
}

func TestConfirmNReturnsToList(t *testing.T) {
	m := newTestModel(stateConfirm, sampleArtifacts, map[int]bool{0: true})

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	got := updated.(Model)
	if got.state != stateList {
		t.Fatalf("expected stateList, got %v", got.state)
	}
}

func TestConfirmCtrlCQuits(t *testing.T) {
	m := newTestModel(stateConfirm, sampleArtifacts, map[int]bool{0: true})
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("expected quit command")
	}
}

// --- Deleting state ---

func TestDeletingIgnoresNonQuitKeys(t *testing.T) {
	m := newTestModel(stateDeleting, sampleArtifacts, map[int]bool{0: true})
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	got := updated.(Model)
	if got.state != stateDeleting {
		t.Fatalf("expected stateDeleting, got %v", got.state)
	}
	_ = cmd
}

func TestDeletingEscDoesNotQuit(t *testing.T) {
	m := newTestModel(stateDeleting, sampleArtifacts, map[int]bool{0: true})
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd != nil {
		if _, ok := cmd().(tea.QuitMsg); ok {
			t.Fatal("deleting state should not quit on esc")
		}
	}
}

// --- Done state ---

func TestDoneEnterQuits(t *testing.T) {
	m := newTestModel(stateDone, nil, nil)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected quit command")
	}
}

func TestDoneQQuits(t *testing.T) {
	m := newTestModel(stateDone, nil, nil)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatal("expected quit command")
	}
}

// --- Error state ---

func TestErrorEnterQuits(t *testing.T) {
	m := newTestModel(stateError, nil, nil)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
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

// --- deleteFinishedMsg ---

func TestDeleteFinishedSuccess(t *testing.T) {
	m := newTestModel(stateDeleting, sampleArtifacts, map[int]bool{0: true})
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
	m := newTestModel(stateDeleting, sampleArtifacts, map[int]bool{0: true})
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
	m := newTestModel(stateList, sampleArtifacts, map[int]bool{})
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
	m := newTestModel(stateList, sampleArtifacts, map[int]bool{0: true, 2: true})
	selected := m.selectedArtifacts()
	if len(selected) != 2 {
		t.Fatalf("expected 2, got %d", len(selected))
	}
	if selected[0].SizeBytes < selected[1].SizeBytes {
		t.Fatal("expected sorted by size descending")
	}
}

func TestSelectedArtifactsEmpty(t *testing.T) {
	m := newTestModel(stateList, sampleArtifacts, map[int]bool{})
	selected := m.selectedArtifacts()
	if selected != nil {
		t.Fatalf("expected nil, got %v", selected)
	}
}

// --- Confirm with empty selected (edge case) ---

func TestConfirmYWithEmptySelectedReturnsToList(t *testing.T) {
	m := newTestModel(stateConfirm, sampleArtifacts, map[int]bool{})

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
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
}

// --- selectedArtifacts with out-of-bounds index ---

func TestSelectedArtifactsSkipsOutOfBounds(t *testing.T) {
	m := newTestModel(stateList, sampleArtifacts, map[int]bool{0: true, 99: true, -1: true})
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

	cmd := scanArtifactsCmd(scanner.ScanOptions{RootPath: root, MaxDepth: -1})
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
	cmd := scanArtifactsCmd(scanner.ScanOptions{RootPath: "", MaxDepth: -1})
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
	cmd := deleteArtifactsCmd(artifacts)
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
