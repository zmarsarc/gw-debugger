package keylist

import (
	"context"
	"gw/dispatcher/debugger/msgs"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/redis/go-redis/v9"
)

func New() Model {
	return Model{
		rdb:  nil,
		keys: nil,
		err:  nil,
	}
}

type Model struct {
	rdb      *redis.Client
	keys     []string
	err      error
	pageSize int
	csr      int
}

func (m Model) View() string {
	if m.rdb == nil {
		return "Redis disconnected."
	}
	if len(m.keys) == 0 {
		return "No keys."
	}
	if m.err != nil {
		return "Err"
	}

	pos, size := m.csr, m.pageSize
	var builder strings.Builder
	for pos < len(m.keys) && size > 0 {
		builder.WriteString(m.keys[pos] + "\n")
		pos++
	}
	return builder.String()
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case msgs.RedisStateMsg:
		m.rdb = msg.Client
		return m, queryKeysCmd(m.rdb)

	case keyUpdateMessage:
		m.keys = msg.Keys
		m.err = msg.Err

	case tea.WindowSizeMsg:
		m.pageSize = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "up":
			if m.csr > 0 {
				m.csr--
			}
		case "down":
			if m.csr < len(m.keys)-1 {
				m.csr++
			}
		}
	}

	return m, nil
}

type keyUpdateMessage struct {
	Keys []string
	Err  error
}

func queryKeysCmd(rdb *redis.Client) tea.Cmd {
	return func() tea.Msg {
		keys, err := rdb.Keys(context.Background(), "*").Result()
		return keyUpdateMessage{Keys: keys, Err: err}
	}
}
