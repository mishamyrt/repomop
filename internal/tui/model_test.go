package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"repomop/internal/scanner"
)

func TestListViewEscQuits(t *testing.T) {
	model := Model{
		state: stateList,
		artifacts: []scanner.Artifact{
			{
				Kind: scanner.ArtifactNodeModule,
				Path: "/repo/node_modules",
			},
		},
		selected: map[int]bool{},
	}

	_, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("expected quit command on esc key")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("expected esc key to trigger quit")
	}
}

func TestConfirmViewEscQuits(t *testing.T) {
	model := Model{
		state: stateConfirm,
		selected: map[int]bool{
			0: true,
		},
		artifacts: []scanner.Artifact{
			{
				Kind: scanner.ArtifactNodeModule,
				Path: "/repo/node_modules",
			},
		},
	}

	_, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("expected quit command on esc key")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("expected esc key to trigger quit from confirm view")
	}
}

func TestListViewEnterWithoutSelectionDoesNotSetHint(t *testing.T) {
	model := Model{
		state: stateList,
		artifacts: []scanner.Artifact{
			{
				Kind: scanner.ArtifactNodeModule,
				Path: "/repo/node_modules",
			},
		},
		selected: map[int]bool{},
		message:  "Some artifact sizes could not be fully calculated.",
	}

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got, ok := updated.(Model)
	if !ok {
		t.Fatal("expected updated model")
	}
	if got.state != stateList {
		t.Fatalf("expected stateList, got %v", got.state)
	}
	if got.message != model.message {
		t.Fatalf("message should remain unchanged, got: %q", got.message)
	}
}
