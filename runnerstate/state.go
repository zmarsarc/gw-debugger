package runnerstate

import (
	"context"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
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

// Time format use to parse ctime and utime in runner state.
var timeParseFormat string = "2006-01-02T15:04:05.999999999"

// Time print format use to build view.
var timePrintFormat string = "2006-01-02 15:04:05"

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

// Command update runner state but have one second delay.
func delayUpdateRunnerState(name string, rdb *redis.Client) tea.Cmd {
	return func() tea.Msg {
		time.Sleep(1 * time.Second)
		data, err := rdb.HGetAll(context.Background(), fmt.Sprintf("%s::runner::gw", name)).Result()
		if err != nil {
			return ErrorMsg{Name: name, Err: err}
		}
		return StateUpdateMsg{Name: name, State: data}
	}
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
	return updateRunnerState(s.Name, s.rdb)
}

func (s Model) View() string {
	if s.err != nil {
		return fmt.Sprintf("%s Update error last time %s", s.Name, s.err.Error())
	}
	return fmt.Sprintf("%s %s %s %s", s.Name, s.Model, s.Ctime.Format(timePrintFormat), s.Utime.Format(timePrintFormat))
}

func (s Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	s.err = nil

	switch msg := msg.(type) {
	case StateUpdateMsg:

		if s.Name != msg.Name {
			return s, nil
		}

		s.Model = msg.State["model_id"]

		s.Ctime, s.err = time.Parse(timeParseFormat, msg.State["ctime"])
		if s.err != nil {
			return s, nil
		}

		s.Utime, s.err = time.Parse(timeParseFormat, msg.State["utime"])
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

		return s, delayUpdateRunnerState(s.Name, s.rdb)

	case ErrorMsg:
		s.err = msg.Err
		return s, delayUpdateRunnerState(s.Name, s.rdb)

	default:
		return s, nil
	}
}
