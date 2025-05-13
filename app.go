package main

import (
	"fmt"
	"gw/dispatcher/debugger/keylist"
	"gw/dispatcher/debugger/msgs"
	"gw/dispatcher/debugger/runnerwatcher"
	"gw/dispatcher/debugger/style"
	"gw/dispatcher/debugger/theme"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/redis/go-redis/v9"
)

// Footer (status bar) height is fixed.
const footerHeight = 2

// Defune app style.
var (
	itemStyle      = style.W().M.Align(lipgloss.Center)
	selectModifier = lipgloss.NewStyle().
			Background(theme.G().PanelLight).
			Foreground(theme.G().TextDark)
	unselectModifier = lipgloss.NewStyle().Background(theme.G().PanelDark)

	headerBox  = lipgloss.NewStyle().Background(theme.G().PanelDark)
	footerBox  = lipgloss.NewStyle().Height(footerHeight).Background(theme.G().PanelDark)
	leftBorder = lipgloss.NewStyle().Border(lipgloss.NormalBorder(), false, true, false, false)
	mainBox    = lipgloss.NewStyle()
)

// Interface for component which can update status bar message.
type Statusbar interface {
	StatusBarView() string
}

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
	var cmds []tea.Cmd
	cmds = append(cmds, connectRedis(&a.rdbConfig))
	for i := range a.models {
		cmds = append(cmds, a.models[i].Init())
	}
	return tea.Batch(cmds...)
}

func (a App) View() string {
	// Build header.
	header := make([]string, len(a.tabs))
	for i := range a.tabs {
		if i == a.csr {
			header[i] = itemStyle.Inherit(selectModifier).Render(a.tabs[i])
		} else {
			header[i] = itemStyle.Inherit(unselectModifier).Render(a.tabs[i])
		}
	}

	// Render header data, fill the rest of the line.
	renderHeader := headerBox.Width(a.width).Render(
		lipgloss.JoinHorizontal(lipgloss.Center, header...),
	)

	// Build footer, fill the rest of line.
	redisStatus := leftBorder.Render(lipgloss.JoinVertical(lipgloss.Center,
		"Redis",
		fmt.Sprintf("%s:%d@%d", a.rdbConfig.host, a.rdbConfig.port, a.rdbConfig.db),
	))
	statusBar := ""
	switch model := a.models[a.csr].(type) {
	case Statusbar:
		statusBar = model.StatusBarView()
	}
	space := a.width - lipgloss.Width(redisStatus)
	renderFooter := footerBox.Width(a.width).Render(
		lipgloss.JoinHorizontal(lipgloss.Top,
			redisStatus,
			lipgloss.PlaceHorizontal(space, lipgloss.Right, statusBar)),
	)

	// Calc main box height.
	mainBoxHeight := a.height - lipgloss.Height(renderHeader) - lipgloss.Height(renderFooter)

	// Render main data.
	main := ""
	if a.models[a.csr] != nil {
		main = a.models[a.csr].View()
	}
	renderMain := mainBox.Height(mainBoxHeight).MaxHeight(mainBoxHeight).Render(main)

	// Render app.
	return lipgloss.JoinVertical(lipgloss.Left,
		renderHeader, renderMain, renderFooter,
	)
}

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		a.height = msg.Height
		a.width = msg.Width
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
