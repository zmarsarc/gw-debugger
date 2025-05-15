package msgs

import "github.com/redis/go-redis/v9"

type RedisStateMsg struct {
	Client *redis.Client
}

type ReadgroupStatus struct {
	LastDeliveredID string
	Lag             int64
	Pending         int64
	Err             error
}

type StreamUpdateMsg struct {
	TaskCreate  ReadgroupStatus
	InferDown   ReadgroupStatus
	ProcessDown ReadgroupStatus
}
