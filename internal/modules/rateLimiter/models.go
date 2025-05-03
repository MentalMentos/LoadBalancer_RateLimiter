package rateLimiter

import (
	"time"
)

type ClientConfig struct {
	Ip       string        `json:"client_id"`
	Capacity int           `json:"capacity"`
	Interval time.Duration `json:"interval"`
}

func NewClientStore() *ClientStore {
	return &ClientStore{
		clients: make(map[string]*ClientConfig),
	}
}
