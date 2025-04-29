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

// Use to report redis error.
type ErrorMsg struct {
	Name string
	Err  error
}

// Use when fetch new runner state.
type StateUpdateMsg struct {
	Name  string
	State map[string]string
}

// Error message to report can not find runner heartbeat.
type NoHeartbeatErrorMsg struct {
	Name string
}

// Message to report runner heartbeat.
type HeartbeatUpdateMsg struct {
	Name     string
	Lasttime string
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
)

// Command update runner state.
func updateRunnerState(name string, rdb *redis.Client) tea.Cmd {
	return func() tea.Msg {
		data, err := rdb.HGetAll(context.Background(), fmt.Sprintf("%s::runner::gw", name)).Result()
		if err != nil {
			return ErrorMsg{Name: name, Err: err}
		}
		return StateUpdateMsg{Name: name, State: data}
	}
}

// Command to update runner heartbeat.
func updateHeartbeatState(name string, rdb *redis.Client) tea.Cmd {
	return func() tea.Msg {
		data, err := rdb.Get(context.Background(), fmt.Sprintf("%s::runner::heartbeat::gw", name)).Result()
		if err != nil {
			if err == redis.Nil {
				return NoHeartbeatErrorMsg{Name: name}
			}
			return ErrorMsg{Name: name, Err: err}
		}
		return HeartbeatUpdateMsg{Name: name, Lasttime: data}
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

	err error
	rdb *redis.Client
}

func New(name string, rdb *redis.Client) Model {
	return Model{Name: name, rdb: rdb}
}

func (s Model) Init() tea.Cmd {
	return tea.Batch(updateRunnerState(s.Name, s.rdb), updateHeartbeatState(s.Name, s.rdb))
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

		return s, delayRunCommand(1, updateRunnerState(s.Name, s.rdb))

	case HeartbeatUpdateMsg:
		t, err := time.ParseInLocation(timeParseFormat, msg.Lasttime, time.Local)
		if err != nil {
			return s, nil
		}
		s.Heartbeat = &t
		return s, delayRunCommand(1, updateHeartbeatState(s.Name, s.rdb))

	case NoHeartbeatErrorMsg:
		s.Heartbeat = nil
		return s, delayRunCommand(1, updateHeartbeatState(s.Name, s.rdb))

	case ErrorMsg:
		s.err = msg.Err
		return s, delayRunCommand(1, updateRunnerState(s.Name, s.rdb))

	default:
		return s, nil
	}
}
