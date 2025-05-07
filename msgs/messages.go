package msgs

import "github.com/redis/go-redis/v9"

type RedisStateMsg struct {
	Client *redis.Client
}
