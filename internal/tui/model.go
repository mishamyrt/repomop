package tui

import (
	"context"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	deleter "repomop/internal/delete"
	"repomop/internal/scanner"
)

type keyMap struct {
	Up     key.Binding
	Down   key.Binding
	Toggle key.Binding
	Enter  key.Binding
	Yes    key.Binding
	No     key.Binding
	Quit   key.Binding
}

var keys = keyMap{
	Up:     key.NewBinding(key.WithKeys("up", "k")),
	Down:   key.NewBinding(key.WithKeys("down", "j")),
	Toggle: key.NewBinding(key.WithKeys(" ")),
	Enter:  key.NewBinding(key.WithKeys("enter")),
	Yes:    key.NewBinding(key.WithKeys("y")),
	No:     key.NewBinding(key.WithKeys("n")),
	Quit:   key.NewBinding(key.WithKeys("q", "esc", "ctrl+c")),
}

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

type titleAnimationTickMsg struct{}

const (
	titleText              = "repomop"
	titleAnimationPadding  = 2
	titleAnimationInterval = 90 * time.Millisecond
)

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
	confirmOffset   int
	scanWarnings    []error
	deleteResult    deleter.Result

	selectedCount   int
	selectedSize    int64
	sizeColumnWidth int

	message  string
	fatalErr error

	titleAnimationFrame int

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
	return tea.Batch(
		m.spinner.Tick,
		scanArtifactsCmd(m.ctx, m.opts),
		tickTitleAnimation(),
	)
}

// Update handles keyboard input and async scan/delete messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmds := make([]tea.Cmd, 0, 2)

	switch typed := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = typed.Width
		m.height = typed.Height
	case titleAnimationTickMsg:
		if m.state != stateLoading {
			return m, nil
		}
		m.titleAnimationFrame = (m.titleAnimationFrame + 1) % titleAnimationCycleLength()
		cmds = append(cmds, tickTitleAnimation())
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
			if key.Matches(typed, keys.Quit) {
				if m.cancel != nil {
					m.cancel()
				}
				return m, tea.Quit
			}
		case stateList:
			switch {
			case key.Matches(typed, keys.Up):
				if m.cursor > 0 {
					m.cursor--
				}
			case key.Matches(typed, keys.Down):
				if m.cursor < len(m.artifacts)-1 {
					m.cursor++
				}
			case key.Matches(typed, keys.Toggle):
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
			case key.Matches(typed, keys.Enter):
				if m.selectedCount == 0 {
					break
				}
				m.cachedSelection = m.selectedArtifacts()
				m.confirmOffset = 0
				m.state = stateConfirm
				m.message = ""
			case key.Matches(typed, keys.Quit):
				if m.cancel != nil {
					m.cancel()
				}
				return m, tea.Quit
			}
		case stateConfirm:
			selected := m.confirmArtifacts()
			switch {
			case typed.Type == tea.KeyEsc:
				m.cachedSelection = nil
				m.confirmOffset = 0
				m.state = stateList
			case key.Matches(typed, keys.Up):
				if m.confirmOffset > 0 {
					m.confirmOffset--
				}
			case key.Matches(typed, keys.Down):
				if m.confirmOffset < m.maxConfirmOffset(len(selected)) {
					m.confirmOffset++
				}
			case key.Matches(typed, keys.Yes):
				if len(selected) == 0 {
					m.cachedSelection = nil
					m.confirmOffset = 0
					m.state = stateList
					break
				}
				m.cachedSelection = nil
				m.confirmOffset = 0
				m.state = stateDeleting
				m.message = ""
				cmds = append(cmds, m.spinner.Tick)
				cmds = append(cmds, deleteArtifactsCmd(m.ctx, selected))
			case key.Matches(typed, keys.Enter):
				// Ignore Enter in confirm view to avoid accidental cancellations.
			case key.Matches(typed, keys.No):
				m.cachedSelection = nil
				m.confirmOffset = 0
				m.state = stateList
			case key.Matches(typed, keys.Quit):
				if m.cancel != nil {
					m.cancel()
				}
				return m, tea.Quit
			}
		case stateDeleting:
			if key.Matches(typed, keys.Quit) {
				return m, nil
			}
		case stateDone, stateError:
			if key.Matches(typed, keys.Enter) || key.Matches(typed, keys.Quit) {
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

func (m Model) confirmArtifacts() []scanner.Artifact {
	if m.cachedSelection != nil {
		return m.cachedSelection
	}
	return m.selectedArtifacts()
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

func tickTitleAnimation() tea.Cmd {
	return tea.Tick(titleAnimationInterval, func(time.Time) tea.Msg {
		return titleAnimationTickMsg{}
	})
}

func titleAnimationCycleLength() int {
	return len([]rune(titleText)) + (titleAnimationPadding * 2)
}

// FatalError returns the terminal error for a failed TUI session.
func (m Model) FatalError() error {
	return m.fatalErr
}

// DeleteResult returns deletion summary after the session completes.
func (m Model) DeleteResult() deleter.Result {
	return m.deleteResult
}
