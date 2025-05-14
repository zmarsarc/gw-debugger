package queue

import (
	"context"
	"fmt"
	"gw/dispatcher/debugger/msgs"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/redis/go-redis/v9"
)

const (
	// Name of task queue.
	taskQueueName = "task_create::stream::gw"

	// Name of task queue readgroup name.
	taskQueueReadGroupName = "task_create::readgroup::gw"
)

// Check queue status period in second.
const checkPeriod = 1

type Model struct {
	rdb              *redis.Client
	taskQueuePending *redis.XPending
	lastErr          error
}

func New() Model {
	return Model{}
}

func (m Model) View() string {
	if m.lastErr != nil {
		return m.lastErr.Error()
	}

	return fmt.Sprintf("pending task count %d", m.taskQueuePending.Count)
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case msgs.RedisStateMsg:
		m.rdb = msg.Client
		return m, checkQueueStatus(m.rdb)

	case checkQueueResultMessage:
		m.lastErr = msg.Err
		m.taskQueuePending = msg.TaskQueuePending
		return m, delayRunCommand(checkPeriod, checkQueueStatus(m.rdb))
	}

	return m, nil
}

type checkQueueResultMessage struct {
	TaskQueuePending *redis.XPending
	Err              error
}

// Run command after a delay, unit is seconds.
func delayRunCommand(sec time.Duration, cmd tea.Cmd) tea.Cmd {
	return func() tea.Msg {
		time.Sleep(sec * time.Second)
		return cmd()
	}
}

func checkQueueStatus(rdb *redis.Client) tea.Cmd {
	return func() tea.Msg {
		res, err := rdb.XPending(context.Background(), taskQueueName, taskQueueReadGroupName).Result()
		return checkQueueResultMessage{TaskQueuePending: res, Err: err}
	}
}
