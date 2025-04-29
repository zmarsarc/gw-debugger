package runnerwatcher

import (
	"context"
	"fmt"
	"gw/dispatcher/debugger/runnerstate"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/redis/go-redis/v9"
)

// ErrorMsg use to report error happened when fetch runner keys.
type ErrorMsg error

// Use to update runner keys.
type UpdateRunnerNamesMsg []string

// Command to fetch runner names.
func updateRunneNames(rdb *redis.Client) tea.Cmd {
	return func() tea.Msg {
		keys, err := rdb.Keys(context.Background(), "*::runner::gw").Result()
		if err != nil {
			return ErrorMsg(err)
		}

		result := make([]string, len(keys))
		for i := range keys {
			name, _ := strings.CutSuffix(keys[i], "::runner::gw")
			result[i] = name
		}
		return UpdateRunnerNamesMsg(result)
	}
}

// Command to fetch runner names but have one second delay.
func delayUpdateRunneNames(rdb *redis.Client) tea.Cmd {
	return func() tea.Msg {
		time.Sleep(1 * time.Second)
		keys, err := rdb.Keys(context.Background(), "*::runner::gw").Result()
		if err != nil {
			return ErrorMsg(err)
		}

		result := make([]string, len(keys))
		for i := range keys {
			name, _ := strings.CutSuffix(keys[i], "::runner::gw")
			result[i] = name
		}
		return UpdateRunnerNamesMsg(result)
	}
}

func New(rdb *redis.Client, parent tea.Model) Model {
	return Model{states: make(map[string]runnerstate.Model), rdb: rdb, parent: parent}
}

type Model struct {
	states map[string]runnerstate.Model

	height int
	width  int

	csr int
	cnt int

	rdb    *redis.Client
	parent tea.Model
	err    error
}

func (m Model) Init() tea.Cmd {
	return updateRunneNames(m.rdb)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	m.err = nil

	switch msg := msg.(type) {

	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			if m.parent != nil {
				return m.parent, nil
			}
			return m, tea.Quit
		case "up":
			if m.csr > 0 {
				m.csr--
			}
		case "down":
			if m.csr <= len(m.states) {
				m.csr++
			}
		}
		return m, nil

	case ErrorMsg:
		m.err = error(msg)
		return m, delayUpdateRunneNames(m.rdb)

	case UpdateRunnerNamesMsg:
		m.cnt++

		cmd := make([]tea.Cmd, 0)

		for _, name := range []string(msg) {
			_, ok := m.states[name]
			if !ok {
				newState := runnerstate.New(name, m.rdb)
				cmd = append(cmd, newState.Init())
				m.states[name] = newState
			}
		}
		cmd = append(cmd, delayUpdateRunneNames(m.rdb))

		return m, tea.Batch(cmd...)

	case tea.WindowSizeMsg:
		m.height = msg.Height
		m.width = msg.Width
		return m, nil

	case runnerstate.StateUpdateMsg:
		state, ok := m.states[msg.Name]
		if !ok {
			return m, nil
		}
		s, cmd := state.Update(msg)
		m.states[msg.Name] = s.(runnerstate.Model)
		return m, cmd

	case runnerstate.ErrorMsg:
		state, ok := m.states[msg.Name]
		if !ok {
			return m, nil
		}
		s, cmd := state.Update(msg)
		m.states[msg.Name] = s.(runnerstate.Model)
		return m, cmd

	default:
		return m, nil
	}
}

func (m Model) View() string {
	titleStyle := lipgloss.NewStyle().Height(1).Width(m.width).Border(lipgloss.NormalBorder()).BorderTop(false).BorderRight(false).BorderLeft(false)

	header := titleStyle.Render(fmt.Sprintf("cnt: %d", m.cnt))

	var builder strings.Builder
	builder.WriteString(header + "\n")

	orderedStates := make([]runnerstate.Model, 0)
	for _, s := range m.states {
		orderedStates = append(orderedStates, s)
	}

	sort.SliceStable(orderedStates, func(i, j int) bool {
		return orderedStates[i].Name < orderedStates[j].Name
	})

	pageSize := m.height - (titleStyle.GetHeight() + 1)
	pos := m.csr
	end := pos + pageSize - 1
	if len(orderedStates) < end {
		end = len(orderedStates)
	}

	for pos < end {
		builder.WriteString(orderedStates[pos].View() + "\n")
		pos++
	}

	return builder.String()
}
