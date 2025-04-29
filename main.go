package main

import (
	"fmt"
	"gw/dispatcher/debugger/runnerwatcher"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/redis/go-redis/v9"
)

var rdb *redis.Client

func main() {
	rdb = redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379"})
	defer rdb.Close()

	if _, err := tea.NewProgram(runnerwatcher.New(rdb, nil), tea.WithAltScreen()).Run(); err != nil {
		fmt.Println(err)
	}
}
