package queue

import (
	"context"
	"fmt"
	"gw/dispatcher/debugger/msgs"
	"gw/dispatcher/debugger/style"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/redis/go-redis/v9"
)

var streamNameStyle = style.W().L

const (
	// Name of task create queue.
	taskQueueName = "task_create::stream::gw"

	// Name of task inference complete queue.
	inferCompleteQueueName = "inference_complete::stream::gw"

	// Name of postprocess complete queue.
	postprocessComplelteQueueName = "postprocess_complete::stream::gw"
)

// Check queue status period in second.
const checkPeriod = 1

type Model struct {
	rdb    *redis.Client
	status msgs.StreamUpdateMsg
}

func New() Model {
	return Model{}
}

func (m Model) View() string {
	return lipgloss.JoinVertical(lipgloss.Left,
		buildCol("Task Create", &m.status.TaskCreate),
		buildCol("Infer Down", &m.status.InferDown),
		buildCol("Postprocess Down", &m.status.ProcessDown),
	)
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case msgs.RedisStateMsg:
		m.rdb = msg.Client
		return m, checkQueueStatus(m.rdb)

	case msgs.StreamUpdateMsg:
		m.status = msg
		return m, delayRunCommand(checkPeriod, checkQueueStatus(m.rdb))
	}

	return m, nil
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
		result := msgs.StreamUpdateMsg{}
		result.TaskCreate = inspectStream(rdb, taskQueueName)
		result.InferDown = inspectStream(rdb, inferCompleteQueueName)
		result.ProcessDown = inspectStream(rdb, postprocessComplelteQueueName)

		return result
	}
}

func inspectStream(rdb *redis.Client, key string) msgs.ReadgroupStatus {
	info, err := rdb.XInfoGroups(context.Background(), key).Result()
	if err != nil {
		return msgs.ReadgroupStatus{Err: err}
	}

	// It always one read group bucause all consumer use same group name.
	// Get last delivered id then read messages after that.
	result := msgs.ReadgroupStatus{Err: nil}
	result.LastDeliveredID = info[0].LastDeliveredID
	result.Lag = info[0].Lag
	result.Pending = info[0].Pending

	return result
}

func buildCol(title string, data *msgs.ReadgroupStatus) string {
	var header, info string

	header = streamNameStyle.Render(title + ":")

	if data.Err != nil {
		info = data.Err.Error()
	} else {
		info = fmt.Sprintf("%d waiting, %d processing", data.Lag, data.Pending)
	}

	return lipgloss.JoinHorizontal(lipgloss.Bottom, header, info)
}
