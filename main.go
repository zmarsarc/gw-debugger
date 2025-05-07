package main

import (
	"flag"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	var addr string
	var port int
	var password string
	var db int

	flag.StringVar(&addr, "h", "127.0.0.1", "redis host")
	flag.IntVar(&port, "p", 6379, "redis port")
	flag.StringVar(&password, "pwd", "", "password")
	flag.IntVar(&db, "db", 0, "redis db")
	flag.Parse()

	app := NewApp()
	app.rdbConfig = redisConfig{
		host:     addr,
		port:     port,
		password: password,
		db:       db,
	}

	if _, err := tea.NewProgram(app, tea.WithAltScreen()).Run(); err != nil {
		fmt.Println(err)
	}
}
