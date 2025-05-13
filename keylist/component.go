package keylist

import (
	"context"
	"fmt"
	"gw/dispatcher/debugger/msgs"
	"gw/dispatcher/debugger/style"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/redis/go-redis/v9"
)

const textInputHeght = 1

var statusbarStyle = style.W().M.Padding(0, 1)

func New() Model {
	ipt := textinput.New()
	ipt.Focus()

	m := Model{
		rdb:   nil,
		keys:  nil,
		err:   nil,
		input: ipt,
	}
	return m
}

type Model struct {
	rdb       *redis.Client
	keys      []string
	err       error
	pageSize  int
	csr       int
	input     textinput.Model
	lastValue string
}

func (m Model) View() string {
	var builder strings.Builder
	builder.WriteString(m.input.View() + "\n")

	if m.rdb == nil {
		builder.WriteString("Redis disconnected.")
		return builder.String()
	}
	if len(m.keys) == 0 {
		builder.WriteString("No keys.")
		return builder.String()
	}
	if m.err != nil {
		builder.WriteString(m.err.Error())
		return builder.String()
	}

	pos, size := m.csr, m.pageSize

	for pos < len(m.keys) && size > 0 {
		builder.WriteString(m.keys[pos] + "\n")
		pos++
	}
	return builder.String()
}

func (m Model) StatusBarView() string {
	return statusbarStyle.Render(fmt.Sprintf("%d results", len(m.keys)))
}

func (m Model) Init() tea.Cmd {
	return textinput.Blink
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case msgs.RedisStateMsg:
		m.rdb = msg.Client
		return m, nil

	case keyUpdateMessage:
		m.keys = msg.Keys
		m.err = msg.Err
		return m, nil

	case tea.WindowSizeMsg:
		m.pageSize = msg.Height - textInputHeght
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "up":
			if m.csr > 0 {
				m.csr--
			}
			return m, nil
		case "down":
			if m.csr < len(m.keys)-(m.pageSize/2) {
				m.csr++
			}
			return m, nil
		case "enter":
			return m, queryKeysCmd(m.rdb, m.input.Value())
		}
	}

	var c tea.Cmd
	m.input, c = m.input.Update(msg)

	if m.lastValue != m.input.Value() {
		if m.input.Value() == "" {
			m.keys = []string{}
			m.err = nil
			m.csr = 0
			return m, c
		}
		if m.input.Value() == "*" {
			m.lastValue = "*"
		} else {
			m.lastValue = fmt.Sprintf("*%s*", m.input.Value())
		}

		return m, tea.Batch(c, queryKeysCmd(m.rdb, m.lastValue))
	}

	return m, c
}

type keyUpdateMessage struct {
	Keys []string
	Err  error
}

func queryKeysCmd(rdb *redis.Client, patten string) tea.Cmd {
	return func() tea.Msg {
		keys, err := rdb.Keys(context.Background(), patten).Result()
		return keyUpdateMessage{Keys: keys, Err: err}
	}
}
