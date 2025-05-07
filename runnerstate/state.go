package runnerstate

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/redis/go-redis/v9"
)

type RunnerStateMsg interface {
	RunnerName() string
}

// Use when fetch new runner state.
type StateUpdateMsg struct {
	Name      string
	State     map[string]string
	Pending   *redis.XPending
	Heartbeat *time.Time
	Err       error
}

func (m StateUpdateMsg) RunnerName() string {
	return m.Name
}

// Message to report runner heartbeat.
type HeartbeatUpdateMsg struct {
	Name     string
	Lasttime string
	Err      error
}

func (m HeartbeatUpdateMsg) RunnerName() string {
	return m.Name
}

var (
	// Time format use to parse ctime and utime in runner state.
	timeParseFormat string = "2006-01-02T15:04:05.999999999"

	// Format fo print time.
	timePrintFormat string = "2006-01-02 15:04:05"
)

// Styles.
var (
	nameStyle  lipgloss.Style = lipgloss.NewStyle().Width(6).Bold(true)
	modelStyle lipgloss.Style = lipgloss.NewStyle().Width(22)
	timeStyle  lipgloss.Style = lipgloss.NewStyle().Width(22)
	busyStyle  lipgloss.Style = lipgloss.NewStyle().Margin(0, 1).
			Padding(0, 1).Background(lipgloss.ANSIColor(11)).Foreground(lipgloss.ANSIColor(0))
	idleStyle lipgloss.Style = lipgloss.NewStyle().Margin(0, 1).
			Padding(0, 1).Background(lipgloss.ANSIColor(10)).Foreground(lipgloss.ANSIColor(0))
	deadStyle lipgloss.Style = lipgloss.NewStyle().Background(lipgloss.ANSIColor(9)).
			Margin(0, 1).Width(22).Align(lipgloss.Center).Foreground(lipgloss.ANSIColor(0))
	aliveStyle lipgloss.Style = lipgloss.NewStyle().Background(lipgloss.ANSIColor(10)).
			Margin(0, 1).Width(22).Align(lipgloss.Center).Foreground(lipgloss.ANSIColor(0))
	pendingTaskStyle = lipgloss.NewStyle().Width(5).Align(lipgloss.Center).Background(lipgloss.ANSIColor(7)).Foreground(lipgloss.ANSIColor(0)).Margin(0, 1)
)

// Command update runner state.
func updateRunnerState(name string, rdb *redis.Client) tea.Cmd {
	return func() tea.Msg {
		state := StateUpdateMsg{Name: name}

		// Read runner state
		state.State, state.Err = rdb.HGetAll(context.Background(),
			fmt.Sprintf("%s::runner::gw", name)).Result()
		if state.Err != nil {
			return state
		}

		// Read heartbeat.
		hb, err := rdb.Get(context.Background(),
			fmt.Sprintf("%s::runner::heartbeat::gw", name)).Result()
		if err != nil {
			if err != redis.Nil {
				state.Err = err
				return state
			}
		} else {
			t, err := time.ParseInLocation(timeParseFormat, hb, time.Local)
			if err == nil {
				state.Heartbeat = &t
			}
		}

		// Read pending msg.
		state.Pending, state.Err = rdb.XPending(context.Background(),
			fmt.Sprintf("%s::runner::stream::gw", name),
			fmt.Sprintf("%s::runner::readgroup::gw", name)).Result()

		return state
	}
}

// Delay run command after dealy_s seconds.
func delayRunCommand(delay_s int, cmd tea.Cmd) tea.Cmd {
	return func() tea.Msg {
		time.Sleep(time.Duration(delay_s) * time.Second)
		return cmd()
	}
}

func puttyTime(t time.Time) string {
	d := time.Since(t)

	if d.Seconds() < 60 {
		return fmt.Sprintf("%02ds before", int(math.Ceil(d.Seconds())))
	}
	if d.Minutes() < 60 {
		min := int(math.Floor(d.Minutes()))
		sec := int(math.Mod(d.Seconds(), 60.0))
		return fmt.Sprintf("%02d:%02d before", min, sec)
	}
	if d.Hours() < 24 {
		hor := int(math.Floor(d.Hours()))
		min := int(math.Mod(d.Minutes(), 60.0))
		sec := int(math.Mod(d.Seconds(), 60.0))
		return fmt.Sprintf("%02d:%02d:%02d before", hor, min, sec)
	}

	return t.Format(timePrintFormat)
}

type Model struct {
	Name      string
	Model     string
	Ctime     time.Time
	Utime     time.Time
	Busy      bool
	Alive     bool
	Heartbeat *time.Time
	Pending   *redis.XPending

	err error
	rdb *redis.Client
}

func New(name string, rdb *redis.Client) Model {
	return Model{Name: name, rdb: rdb}
}

func (s Model) Init() tea.Cmd {
	return tea.Batch(updateRunnerState(s.Name, s.rdb))
}

func (s Model) View() string {

	if s.err != nil {
		return fmt.Sprintf("%s Update error last time %s", s.Name, s.err.Error())
	}

	var builder strings.Builder
	builder.WriteString(nameStyle.Render(s.Name))
	builder.WriteString(modelStyle.Render(s.Model))

	switch {
	case s.Alive && s.Heartbeat != nil:
		text := fmt.Sprintf("ALIVE - hb %ds before", int(math.Ceil(time.Since(*s.Heartbeat).Seconds())))
		builder.WriteString(aliveStyle.Render(text))
	case s.Alive && s.Heartbeat == nil:
		builder.WriteString(deadStyle.Render("ALIVE- no hb"))
	case !s.Alive && s.Heartbeat != nil:
		text := fmt.Sprintf("DEAD - hb %ds before", int(math.Ceil(time.Since(*s.Heartbeat).Seconds())))
		builder.WriteString(deadStyle.Render(text))
	case !s.Alive && s.Heartbeat == nil:
		builder.WriteString(deadStyle.Render("DEAD - no hb"))
	}

	if s.Busy {
		builder.WriteString(busyStyle.Render("BUSY"))
	} else {
		builder.WriteString(idleStyle.Render("IDLE"))
	}

	if s.Pending != nil {
		builder.WriteString(pendingTaskStyle.Render(fmt.Sprintf("%d", s.Pending.Count)))
	} else {
		builder.WriteString(pendingTaskStyle.Render("0"))
	}

	builder.WriteString(timeStyle.Render(puttyTime(s.Ctime)))
	builder.WriteString(timeStyle.Render(puttyTime(s.Utime)))

	return builder.String()
}

func (s Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	s.err = nil

	switch msg := msg.(type) {
	case StateUpdateMsg:

		if s.Name != msg.Name {
			return s, nil
		}

		if msg.Err != nil {
			s.err = msg.Err
			return s, delayRunCommand(1, updateRunnerState(s.Name, s.rdb))
		}

		s.Model = msg.State["model_id"]

		s.Ctime, s.err = time.ParseInLocation(timeParseFormat, msg.State["ctime"], time.Local)
		if s.err != nil {
			return s, nil
		}

		s.Utime, s.err = time.ParseInLocation(timeParseFormat, msg.State["utime"], time.Local)
		if s.err != nil {
			return s, nil
		}

		if msg.State["busy"] == "0" {
			s.Busy = false
		} else {
			s.Busy = true
		}

		if msg.State["is_alive"] == "0" {
			s.Alive = false
		} else {
			s.Alive = true
		}

		s.Pending = msg.Pending
		s.Heartbeat = msg.Heartbeat

		return s, delayRunCommand(1, updateRunnerState(s.Name, s.rdb))

	default:
		return s, nil
	}
}

func Header() string {
	var builder strings.Builder
	liveStyle := lipgloss.NewStyle().Margin(0, 1).Width(22).Align(lipgloss.Center)
	busyStyle := lipgloss.NewStyle().Margin(0, 1).Padding(0, 1)
	pendingStyle := lipgloss.NewStyle().Width(7).Align(lipgloss.Center)

	builder.WriteString(nameStyle.UnsetBold().Render("NAME"))
	builder.WriteString(modelStyle.Align(lipgloss.Center).Render("MODEL"))
	builder.WriteString(liveStyle.Align(lipgloss.Center).Render("LIFE"))

	builder.WriteString(busyStyle.Render("BUSY"))
	builder.WriteString(pendingStyle.Render("PENDING"))

	builder.WriteString(timeStyle.Align(lipgloss.Center).Render("CTIME"))
	builder.WriteString(timeStyle.Align(lipgloss.Center).Render("UTIME"))

	return builder.String()
}
