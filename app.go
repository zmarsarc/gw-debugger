package main

import (
	"fmt"
	"gw/dispatcher/debugger/keylist"
	"gw/dispatcher/debugger/msgs"
	"gw/dispatcher/debugger/runnerwatcher"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/redis/go-redis/v9"
)

var headerItemStyle = lipgloss.NewStyle().Width(10).Align(lipgloss.Center).Background(lipgloss.ANSIColor(4))
var headerItemSelectedStyle = lipgloss.NewStyle().Width(10).Align(lipgloss.Center).
	Background(lipgloss.ANSIColor(7)).Foreground(lipgloss.ANSIColor(0)).Bold(true)

// Redis config use to store redis setup.
type redisConfig struct {
	host     string
	port     int
	password string
	db       int
}

func newRedisConfig() redisConfig {
	return redisConfig{
		host:     "127.0.0.1",
		port:     6379,
		password: "",
		db:       0,
	}
}

func connectRedis(cfg *redisConfig) tea.Cmd {
	return func() tea.Msg {
		rdb := redis.NewClient(&redis.Options{
			Addr:     fmt.Sprintf("%s:%d", cfg.host, cfg.port),
			Password: cfg.password,
			DB:       cfg.db,
		})
		return msgs.RedisStateMsg{Client: rdb}
	}
}

type App struct {
	tabs   []string
	models []tea.Model
	csr    int

	rdb       *redis.Client
	rdbConfig redisConfig

	width  int
	height int
}

func NewApp() App {
	app := App{
		tabs: []string{
			"runner",
			"raw keys",
		},
		models: []tea.Model{
			runnerwatcher.New(),
			keylist.New(),
		},
		csr: 0,

		rdb:       nil,
		rdbConfig: newRedisConfig(),
	}
	return app
}

func (a App) Init() tea.Cmd {
	return connectRedis(&a.rdbConfig)
}

func (a App) View() string {
	// Build header.
	header := make([]string, len(a.tabs))
	for i := range a.tabs {
		if i == a.csr {
			header[i] = headerItemSelectedStyle.Render(a.tabs[i])
		} else {
			header[i] = headerItemStyle.Render(a.tabs[i])
		}
	}

	// Build main data.
	main := ""
	if a.models[a.csr] != nil {
		main = a.models[a.csr].View()
	}

	// Build footer.
	footer := fmt.Sprintf("Redis %s:%d@%d", a.rdbConfig.host, a.rdbConfig.port, a.rdbConfig.db)

	headerBox := lipgloss.NewStyle().Background(lipgloss.ANSIColor(4)).Width(a.width).Height(1)
	footerBox := lipgloss.NewStyle().Height(1).
		Background(lipgloss.ANSIColor(7)).
		Width(a.width).Foreground(lipgloss.ANSIColor(0))
	mainBox := lipgloss.NewStyle().Height(a.height - headerBox.GetHeight() - footerBox.GetHeight()).MaxHeight(a.height - 2)

	return lipgloss.JoinVertical(lipgloss.Left,
		headerBox.Render(lipgloss.JoinHorizontal(lipgloss.Center, header...)),
		mainBox.Render(main),
		footerBox.Render(footer),
	)
}

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		a.height = msg.Height
		a.width = msg.Width

		msg.Height -= 2
		return a.Broadcast(msg)

	case msgs.RedisStateMsg:
		a.rdb = msg.Client
		return a.Broadcast(msg)

	case tea.QuitMsg:
		if a.rdb != nil {
			a.rdb.Close()
		}

	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "q":
			return a, tea.Quit
		case "tab":
			a.csr = (a.csr + 1) % len(a.tabs)
		case "shift+tab":
			a.csr--
			if a.csr < 0 {
				a.csr = len(a.tabs) - 1
			}
		default:
			return a.SendToFocused(msg)
		}

	default:
		return a.Broadcast(msg)
	}

	return a, nil
}

func (a App) Broadcast(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	for i := range a.models {
		var c tea.Cmd
		a.models[i], c = a.models[i].Update(msg)
		cmds = append(cmds, c)
	}
	return a, tea.Batch(cmds...)
}

func (a App) SendToFocused(msg tea.Msg) (tea.Model, tea.Cmd) {
	var c tea.Cmd
	a.models[a.csr], c = a.models[a.csr].Update(msg)
	return a, c
}
