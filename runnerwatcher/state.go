package runnerwatcher

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/redis/go-redis/v9"
)

// Use when fetch new runner state.
type StateUpdateMsg struct {
	Name      string
	State     map[string]string
	Pending   *redis.XPending
	Heartbeat *time.Time
	Err       error
}

var (
	// Time format use to parse ctime and utime in runner state.
	timeParseFormat string = "2006-01-02T15:04:05.999999999"

	// Format fo print time.
	timePrintFormat string = "2006-01-02 15:04:05"
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

// The model use to storage runner state and display.
type state struct {
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

func newState(name string, rdb *redis.Client) state {
	return state{Name: name, rdb: rdb}
}

func (s state) Init() tea.Cmd {
	return updateRunnerState(s.Name, s.rdb)
}

func (s state) View() string {

	if s.err != nil {
		return fmt.Sprintf("%s Update error last time %s", s.Name, s.err.Error())
	}

	var builder strings.Builder
	builder.WriteString(nameStyle.Render(s.Name))
	builder.WriteString(modelStyle.Render(s.Model))

	switch {
	case s.Alive && s.Heartbeat != nil:
		text := fmt.Sprintf("ALIVE(%ds)", int(math.Ceil(time.Since(*s.Heartbeat).Seconds())))
		builder.WriteString(heartbeatStyle.Inherit(okColor).Render(text))
	case s.Alive && s.Heartbeat == nil:
		builder.WriteString(heartbeatStyle.Inherit(errorColor).Render("ALIVE(-)"))
	case !s.Alive && s.Heartbeat != nil:
		text := fmt.Sprintf("DEAD(%ds)", int(math.Ceil(time.Since(*s.Heartbeat).Seconds())))
		builder.WriteString(heartbeatStyle.Inherit(errorColor).Render(text))
	case !s.Alive && s.Heartbeat == nil:
		builder.WriteString(heartbeatStyle.Inherit(errorColor).Render("DEAD(-)"))
	}

	if s.Busy {
		builder.WriteString(stateStyle.Inherit(warningColor).Render("BUSY"))
	} else {
		builder.WriteString(stateStyle.Inherit(okColor).Render("IDLE"))
	}

	if s.Pending != nil {
		if s.Pending.Count != 0 {
			builder.WriteString(pendingStyle.Inherit(textInverse).Render(fmt.Sprintf("%d", s.Pending.Count)))
		} else {
			builder.WriteString(pendingStyle.Render(fmt.Sprintf("%d", s.Pending.Count)))
		}
	} else {
		builder.WriteString(pendingStyle.Render("-"))
	}

	builder.WriteString(ctimeStyle.Render(puttyTime(s.Ctime)))
	builder.WriteString(utimeStyle.Render(puttyTime(s.Utime)))

	return builder.String()
}

func (s state) Update(msg tea.Msg) (state, tea.Cmd) {
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
