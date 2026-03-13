package tui

import (
	"context"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	deleter "repomop/internal/delete"
	"repomop/internal/scanner"
)

type viewState int

const (
	stateLoading viewState = iota
	stateList
	stateConfirm
	stateDeleting
	stateDone
	stateError
)

type scanFinishedMsg struct {
	artifacts []scanner.Artifact
	warnings  []error
	err       error
}

type deleteFinishedMsg struct {
	result deleter.Result
}

// Model is the Bubble Tea application state.
type Model struct {
	opts   scanner.ScanOptions
	ctx    context.Context
	cancel context.CancelFunc

	state   viewState
	spinner spinner.Model

	artifacts       []scanner.Artifact
	selected        map[int]struct{}
	cachedSelection []scanner.Artifact
	cursor          int
	scanWarnings    []error
	deleteResult    deleter.Result

	selectedCount   int
	selectedSize    int64
	sizeColumnWidth int

	message  string
	fatalErr error

	width  int
	height int
}

// NewModel creates a new TUI model.
func NewModel(opts scanner.ScanOptions) Model {
	spin := spinner.New()
	spin.Spinner = spinner.Dot
	ctx, cancel := context.WithCancel(context.Background())

	return Model{
		opts:      opts,
		ctx:       ctx,
		cancel:    cancel,
		state:     stateLoading,
		spinner:   spin,
		selected:  make(map[int]struct{}),
		artifacts: make([]scanner.Artifact, 0),
	}
}

// Init starts spinner ticks and asynchronous scanning.
func (m Model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, scanArtifactsCmd(m.ctx, m.opts))
}

// Update handles keyboard input and async scan/delete messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmds := make([]tea.Cmd, 0, 2)

	switch typed := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = typed.Width
		m.height = typed.Height
	case scanFinishedMsg:
		if typed.err != nil {
			m.state = stateError
			m.fatalErr = typed.err
			m.message = typed.err.Error()
			return m, nil
		}
		m.artifacts = typed.artifacts
		m.scanWarnings = typed.warnings
		m.selected = make(map[int]struct{}, len(typed.artifacts))
		m.cursor = 0
		m.selectedCount = 0
		m.selectedSize = 0
		m.sizeColumnWidth = computeSizeColumnWidth(typed.artifacts)
		if len(typed.artifacts) == 0 {
			m.state = stateDone
			m.message = "No artifacts found."
		} else {
			m.state = stateList
			if len(typed.warnings) > 0 {
				m.message = "Some artifact sizes could not be fully calculated."
			}
		}
		return m, nil
	case deleteFinishedMsg:
		m.deleteResult = typed.result
		m.state = stateDone
		if len(typed.result.Errors) > 0 {
			m.message = "Some artifacts could not be removed."
		} else {
			m.message = "Selected artifacts were removed."
		}
		return m, nil
	case tea.KeyMsg:
		switch m.state {
		case stateLoading:
			if keyMatches(typed, "q", "esc", "ctrl+c") {
				if m.cancel != nil {
					m.cancel()
				}
				return m, tea.Quit
			}
		case stateList:
			switch {
			case keyMatches(typed, "up", "k"):
				if m.cursor > 0 {
					m.cursor--
				}
			case keyMatches(typed, "down", "j"):
				if m.cursor < len(m.artifacts)-1 {
					m.cursor++
				}
			case keyMatches(typed, " "):
				if len(m.artifacts) > 0 {
					idx := m.cursor
					if _, ok := m.selected[idx]; ok {
						delete(m.selected, idx)
						m.selectedCount--
						m.selectedSize -= m.artifacts[idx].SizeBytes
					} else {
						m.selected[idx] = struct{}{}
						m.selectedCount++
						m.selectedSize += m.artifacts[idx].SizeBytes
					}
				}
			case keyMatches(typed, "enter"):
				if m.selectedCount == 0 {
					break
				}
				m.cachedSelection = m.selectedArtifacts()
				m.state = stateConfirm
				m.message = ""
			case keyMatches(typed, "q", "esc", "ctrl+c"):
				if m.cancel != nil {
					m.cancel()
				}
				return m, tea.Quit
			}
		case stateConfirm:
			switch {
			case keyMatches(typed, "y"):
				selected := m.cachedSelection
				if selected == nil {
					selected = m.selectedArtifacts()
				}
				if len(selected) == 0 {
					m.cachedSelection = nil
					m.state = stateList
					break
				}
				m.cachedSelection = nil
				m.state = stateDeleting
				m.message = ""
				cmds = append(cmds, m.spinner.Tick)
				cmds = append(cmds, deleteArtifactsCmd(m.ctx, selected))
			case keyMatches(typed, "n"):
				m.cachedSelection = nil
				m.state = stateList
			case keyMatches(typed, "q", "esc", "ctrl+c"):
				if m.cancel != nil {
					m.cancel()
				}
				return m, tea.Quit
			}
		case stateDeleting:
			if keyMatches(typed, "q", "esc", "ctrl+c") {
				return m, nil
			}
		case stateDone, stateError:
			if keyMatches(typed, "enter", "q", "esc", "ctrl+c") {
				if m.cancel != nil {
					m.cancel()
				}
				return m, tea.Quit
			}
		}
	}

	if m.state == stateLoading || m.state == stateDeleting {
		var spinnerCmd tea.Cmd
		m.spinner, spinnerCmd = m.spinner.Update(msg)
		if spinnerCmd != nil {
			cmds = append(cmds, spinnerCmd)
		}
	}

	return m, tea.Batch(cmds...)
}

func (m Model) selectedArtifacts() []scanner.Artifact {
	if len(m.selected) == 0 {
		return nil
	}
	selected := make([]scanner.Artifact, 0, len(m.selected))
	for idx := range m.selected {
		if idx < 0 || idx >= len(m.artifacts) {
			continue
		}
		selected = append(selected, m.artifacts[idx])
	}

	scanner.SortBySizeDesc(selected)

	return selected
}

func keyMatches(msg tea.KeyMsg, keys ...string) bool {
	pressed := msg.String()
	for _, key := range keys {
		if pressed == key {
			return true
		}
	}
	return false
}

func scanArtifactsCmd(ctx context.Context, opts scanner.ScanOptions) tea.Cmd {
	return func() tea.Msg {
		artifacts, warnings, err := scanner.ScanAndMeasure(ctx, opts)
		if err != nil {
			return scanFinishedMsg{err: err}
		}
		return scanFinishedMsg{artifacts: artifacts, warnings: warnings}
	}
}

func deleteArtifactsCmd(ctx context.Context, artifacts []scanner.Artifact) tea.Cmd {
	return func() tea.Msg {
		result := deleter.Artifacts(ctx, artifacts)
		return deleteFinishedMsg{result: result}
	}
}

// FatalError returns the terminal error for a failed TUI session.
func (m Model) FatalError() error {
	return m.fatalErr
}

// DeleteResult returns deletion summary after the session completes.
func (m Model) DeleteResult() deleter.Result {
	return m.deleteResult
}
