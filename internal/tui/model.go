package tui

import (
	"sort"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"repomop/internal/delete"
	"repomop/internal/scanner"
	"repomop/internal/size"
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
	result delete.Result
}

// Model is the Bubble Tea application state.
type Model struct {
	opts scanner.ScanOptions

	state   viewState
	spinner spinner.Model

	artifacts    []scanner.Artifact
	selected     map[int]bool
	cursor       int
	scanWarnings []error
	deleteResult delete.Result

	message  string
	fatalErr error

	width  int
	height int
}

// NewModel creates a new TUI model.
func NewModel(opts scanner.ScanOptions) Model {
	spin := spinner.New()
	spin.Spinner = spinner.Dot

	return Model{
		opts:      opts,
		state:     stateLoading,
		spinner:   spin,
		selected:  make(map[int]bool),
		artifacts: make([]scanner.Artifact, 0),
	}
}

// Init starts spinner ticks and asynchronous scanning.
func (m Model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, scanArtifactsCmd(m.opts))
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
		m.selected = make(map[int]bool, len(typed.artifacts))
		m.cursor = 0
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
			if keyMatches(typed, "q", "ctrl+c") {
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
					m.selected[idx] = !m.selected[idx]
				}
			case keyMatches(typed, "enter"):
				if m.selectedCount() == 0 {
					m.message = "Select at least one artifact with Space."
					break
				}
				m.state = stateConfirm
				m.message = ""
			case keyMatches(typed, "q", "ctrl+c"):
				return m, tea.Quit
			}
		case stateConfirm:
			switch {
			case keyMatches(typed, "y"):
				selected := m.selectedArtifacts()
				if len(selected) == 0 {
					m.state = stateList
					break
				}
				m.state = stateDeleting
				m.message = ""
				cmds = append(cmds, m.spinner.Tick)
				cmds = append(cmds, deleteArtifactsCmd(selected))
			case keyMatches(typed, "n", "esc"):
				m.state = stateList
			case keyMatches(typed, "q", "ctrl+c"):
				return m, tea.Quit
			}
		case stateDeleting:
			if keyMatches(typed, "q", "ctrl+c") {
				return m, nil
			}
		case stateDone, stateError:
			if keyMatches(typed, "enter", "q", "ctrl+c") {
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
	for idx, chosen := range m.selected {
		if !chosen {
			continue
		}
		if idx < 0 || idx >= len(m.artifacts) {
			continue
		}
		selected = append(selected, m.artifacts[idx])
	}

	sort.SliceStable(selected, func(i int, j int) bool {
		if selected[i].SizeBytes == selected[j].SizeBytes {
			return selected[i].Path < selected[j].Path
		}
		return selected[i].SizeBytes > selected[j].SizeBytes
	})

	return selected
}

func (m Model) selectedCount() int {
	count := 0
	for _, selected := range m.selected {
		if selected {
			count++
		}
	}
	return count
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

func scanArtifactsCmd(opts scanner.ScanOptions) tea.Cmd {
	return func() tea.Msg {
		artifacts, err := scanner.Scan(opts)
		if err != nil {
			return scanFinishedMsg{err: err}
		}

		paths := make([]string, 0, len(artifacts))
		for _, artifact := range artifacts {
			paths = append(paths, artifact.Path)
		}

		sizes, warnings := size.Directories(paths, size.RecommendedWorkerCount())
		for idx := range artifacts {
			artifacts[idx].SizeBytes = sizes[artifacts[idx].Path]
		}

		sort.SliceStable(artifacts, func(i int, j int) bool {
			if artifacts[i].SizeBytes == artifacts[j].SizeBytes {
				return artifacts[i].Path < artifacts[j].Path
			}
			return artifacts[i].SizeBytes > artifacts[j].SizeBytes
		})

		return scanFinishedMsg{artifacts: artifacts, warnings: warnings}
	}
}

func deleteArtifactsCmd(artifacts []scanner.Artifact) tea.Cmd {
	return func() tea.Msg {
		result := delete.Artifacts(artifacts)
		return deleteFinishedMsg{result: result}
	}
}

// FatalError returns the terminal error for a failed TUI session.
func (m Model) FatalError() error {
	return m.fatalErr
}

// DeleteResult returns deletion summary after the session completes.
func (m Model) DeleteResult() delete.Result {
	return m.deleteResult
}
