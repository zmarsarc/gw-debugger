package runnerwatcher

import (
	"context"
	"gw/dispatcher/debugger/msgs"
	"gw/dispatcher/debugger/style"
	"gw/dispatcher/debugger/theme"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/redis/go-redis/v9"
)

// Define runner table column width and align.
var (
	nameStyle      = style.W().S
	modelStyle     = style.W().L
	heartbeatStyle = style.W().M.Align(lipgloss.Center)
	stateStyle     = style.W().S.Align(lipgloss.Center)
	pendingStyle   = style.W().S.Align(lipgloss.Center)
	ctimeStyle     = style.W().L.Align(lipgloss.Center)
	utimeStyle     = style.W().L.Align(lipgloss.Center)
)

// Define column color modifier.
var (
	okColor            = lipgloss.NewStyle().Background(theme.G().Success).Foreground(theme.G().TextDark)
	errorColor         = lipgloss.NewStyle().Background(theme.G().Error).Foreground(theme.G().TextDark)
	warningColor       = lipgloss.NewStyle().Background(theme.G().Warning).Foreground(theme.G().TextDark)
	textInverse        = lipgloss.NewStyle().Background(theme.G().BackgroundInverse).Foreground(theme.G().TextDark)
	textInverseAndBold = textInverse.Bold(true)
)

// Use to update runner keys.
type UpdateRunnerNamesMsg struct {
	Names []string
	Err   error
}

// Command to fetch runner names.
func updateRunneNames(rdb *redis.Client) tea.Cmd {
	return func() tea.Msg {
		keys, err := rdb.Keys(context.Background(), "*::runner::gw").Result()
		if err != nil {
			return UpdateRunnerNamesMsg{Err: err}
		}

		result := make([]string, len(keys))
		for i := range keys {
			name, _ := strings.CutSuffix(keys[i], "::runner::gw")
			result[i] = name
		}
		return UpdateRunnerNamesMsg{Names: result}
	}
}

// Delay run command after dealy_s seconds.
func delayRunCommand(delay_s int, cmd tea.Cmd) tea.Cmd {
	return func() tea.Msg {
		time.Sleep(time.Duration(delay_s) * time.Second)
		return cmd()
	}
}

func New() Model {
	return Model{
		states: make(map[string]state),
		height: 0,
		width:  0,
		csr:    0,

		rdb: nil,
		err: nil,
	}
}

type Model struct {
	states map[string]state

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
			if m.csr < len(m.states)-1 {
				m.csr++
			}
		}
		return m, nil

	case UpdateRunnerNamesMsg:
		if msg.Err != nil {
			m.err = msg.Err
			return m, delayRunCommand(1, updateRunneNames(m.rdb))
		}

		cmd := make([]tea.Cmd, 0)
		newStates := make(map[string]state)

		for _, name := range msg.Names {
			s, ok := m.states[name]
			if !ok {
				newState := newState(name, m.rdb)
				cmd = append(cmd, newState.Init())
				newStates[name] = newState
			} else {
				newStates[name] = s
			}
		}
		m.states = newStates
		cmd = append(cmd, delayRunCommand(1, updateRunneNames(m.rdb)))
		return m, tea.Batch(cmd...)

	case tea.WindowSizeMsg:
		m.height = msg.Height
		m.width = msg.Width
		return m, nil

	case StateUpdateMsg:
		state, ok := m.states[msg.Name]
		if !ok {
			return m, nil
		}
		s, cmd := state.Update(msg)
		m.states[msg.Name] = s
		return m, cmd

	default:
		return m, nil
	}
}

func (m Model) View() string {
	if m.rdb == nil {
		return "Redis disconnected."
	}

	var builder strings.Builder
	builder.WriteString(stateTableHeader(m.width) + "\n")

	orderedStates := make([]state, 0)
	for _, s := range m.states {
		orderedStates = append(orderedStates, s)
	}

	sort.SliceStable(orderedStates, func(i, j int) bool {
		return orderedStates[i].Name < orderedStates[j].Name
	})

	const headerHeight = 1
	pageSize := m.height - headerHeight
	pos := m.csr
	end := min(pos+pageSize, len(orderedStates))

	for pos < end {
		builder.WriteString(orderedStates[pos].View() + "\n")
		pos++
	}

	return builder.String()
}

// Use to count state.
func countIf(states map[string]state, cond func(*state) bool) int {
	cnt := 0
	for _, v := range states {
		if cond(&v) {
			cnt++
		}
	}
	return cnt
}

func stateTableHeader(width int) string {
	var builder strings.Builder

	builder.WriteString(nameStyle.Inherit(textInverseAndBold).Render("NAME"))
	builder.WriteString(modelStyle.Inherit(textInverseAndBold).Render("MODEL"))
	builder.WriteString(heartbeatStyle.Inherit(textInverseAndBold).Render("LIFE"))

	builder.WriteString(stateStyle.Inherit(textInverseAndBold).Render("BUSY"))
	builder.WriteString(pendingStyle.Inherit(textInverseAndBold).Render("PEND"))

	builder.WriteString(ctimeStyle.Inherit(textInverseAndBold).Render("CTIME"))
	builder.WriteString(utimeStyle.Inherit(textInverseAndBold).Render("UTIME"))

	// Fill the rest of this line.
	return textInverse.Width(width).Render(builder.String())
}
