package runnerwatcher

import (
	"context"
	"fmt"
	"gw/dispatcher/debugger/msgs"
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

func New() Model {
	return Model{
		states: make(map[string]runnerstate.Model),
		height: 0,
		width:  0,
		csr:    0,

		rdb: nil,
		err: nil,
	}
}

type Model struct {
	states map[string]runnerstate.Model

	height int
	width  int

	csr int

	rdb *redis.Client
	err error
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	m.err = nil

	switch msg := msg.(type) {

	case msgs.RedisStateMsg:
		m.rdb = msg.Client
		if m.rdb != nil {
			return m, updateRunneNames(m.rdb)
		} else {
			return m, nil
		}

	case tea.KeyMsg:
		switch msg.String() {
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
		cmd := make([]tea.Cmd, 0)
		newStates := make(map[string]runnerstate.Model)

		for _, name := range []string(msg) {
			s, ok := m.states[name]
			if !ok {
				newState := runnerstate.New(name, m.rdb)
				cmd = append(cmd, newState.Init())
				newStates[name] = newState
			} else {
				newStates[name] = s
			}
		}
		m.states = newStates
		cmd = append(cmd, delayUpdateRunneNames(m.rdb))
		return m, tea.Batch(cmd...)

	case tea.WindowSizeMsg:
		m.height = msg.Height
		m.width = msg.Width
		return m, nil

	case runnerstate.RunnerStateMsg:
		state, ok := m.states[msg.RunnerName()]
		if !ok {
			return m, nil
		}
		s, cmd := state.Update(msg)
		m.states[msg.RunnerName()] = s.(runnerstate.Model)
		return m, cmd

	default:
		return m, nil
	}
}

func (m Model) View() string {
	if m.rdb == nil {
		return "Redis disconnected."
	}

	titleStyle := lipgloss.NewStyle().Height(1).Width(m.width).
		Border(lipgloss.NormalBorder()).BorderTop(false).BorderRight(false).BorderLeft(false)

	counter := fmt.Sprintf("total %d alive %d dead %d idle %d busy %d",
		len(m.states),
		countIf(m.states, func(m *runnerstate.Model) bool { return m.Alive && m.Heartbeat != nil }),
		countIf(m.states, func(m *runnerstate.Model) bool { return !m.Alive || m.Heartbeat == nil }),
		countIf(m.states, func(m *runnerstate.Model) bool { return !m.Busy }),
		countIf(m.states, func(m *runnerstate.Model) bool { return m.Busy }),
	)
	header := titleStyle.Render(counter)

	var builder strings.Builder
	builder.WriteString(header + "\n")
	builder.WriteString(runnerstate.Header() + "\n")

	orderedStates := make([]runnerstate.Model, 0)
	for _, s := range m.states {
		orderedStates = append(orderedStates, s)
	}

	sort.SliceStable(orderedStates, func(i, j int) bool {
		return orderedStates[i].Name < orderedStates[j].Name
	})

	pageSize := m.height - (titleStyle.GetHeight() + 1 + 1)
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

// Use to count state.
func countIf(states map[string]runnerstate.Model, cond func(*runnerstate.Model) bool) int {
	cnt := 0
	for _, v := range states {
		if cond(&v) {
			cnt++
		}
	}
	return cnt
}
