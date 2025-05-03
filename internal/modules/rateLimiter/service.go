package rateLimiter

import (
	"context"
	"go.uber.org/zap"
	"strings"
	"sync"
	"time"
)

type TokenBucketLimiter struct {
	tokenBucket map[string]chan struct{} //IP -> канал
	clientStore *ClientStore
	defaultCap  int
	Period      time.Duration
	mu          sync.RWMutex
	logger      *zap.Logger
}

type ClientStore struct {
	clients map[string]*ClientConfig
	mu      sync.RWMutex
}

func NewTokenBucketLimiter(ctx context.Context, limit int, period time.Duration, log *zap.Logger) *TokenBucketLimiter {
	interval := period.Nanoseconds() / int64(limit)
	tb := &TokenBucketLimiter{
		tokenBucket: make(map[string]chan struct{}),
		defaultCap:  limit,
		clientStore: &ClientStore{
			clients: make(map[string]*ClientConfig),
		},
		Period: time.Duration(interval),
		logger: log,
	}

	tb.tokenBucket["default"] = make(chan struct{}, limit)
	for i := 0; i < limit; i++ {
		tb.tokenBucket["default"] <- struct{}{}
	}

	go tb.StartPeriod(ctx)
	return tb
}

func (tb *TokenBucketLimiter) StartPeriod(ctx context.Context) {
	ticker := time.NewTicker(tb.Period)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			tb.refillBuckets()
		}
	}
}

func (tb *TokenBucketLimiter) refillBuckets() {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	for ip, bucket := range tb.tokenBucket {
		select {
		case bucket <- struct{}{}:
			tb.logger.Debug("Refilled token for IP", zap.String("ip", ip))
		default:
		}
	}
}

func (tb *TokenBucketLimiter) Allow(ip string) bool {
	tb.mu.RLock()
	defer tb.mu.RUnlock()

	if ipBucket, exists := tb.tokenBucket[getIPFromIdentifier(ip)]; exists {
		select {
		case <-ipBucket:
			return true
		default:
			return false
		}
	}

	// Если ничего нет - дефолт значение
	return tb.allowDefault()
}

func (tb *TokenBucketLimiter) AddClient(config *ClientConfig) {
	tb.clientStore.mu.Lock()
	defer tb.clientStore.mu.Unlock()

	ch := make(chan struct{}, config.Capacity)
	for i := 0; i < config.Capacity; i++ {
		ch <- struct{}{}
	}

	if tb.clientStore == nil {
		tb.clientStore = &ClientStore{
			clients: make(map[string]*ClientConfig),
		}
	}
	
	tb.clientStore.clients[config.Ip] = config
	tb.tokenBucket[config.Ip] = ch

}

func (tb *TokenBucketLimiter) GetClient(clientIp string) (*ClientConfig, bool) {
	tb.clientStore.mu.RLock()
	defer tb.clientStore.mu.RUnlock()
	client, exists := tb.clientStore.clients[clientIp]
	return client, exists
}

func (tb *TokenBucketLimiter) DeleteClient(clientIp string) {
	tb.clientStore.mu.Lock()
	defer tb.clientStore.mu.Unlock()

	if client, exists := tb.clientStore.clients[clientIp]; exists {
		if client.Ip != "" {
			delete(tb.tokenBucket, client.Ip)
		}
		delete(tb.clientStore.clients, clientIp)
		delete(tb.tokenBucket, clientIp)
	}
}

func (tb *TokenBucketLimiter) ListClients() []*ClientConfig {
	tb.clientStore.mu.RLock()
	defer tb.clientStore.mu.RUnlock()

	clients := make([]*ClientConfig, 0, len(tb.clientStore.clients))
	for _, client := range tb.clientStore.clients {
		clients = append(clients, client)
	}
	return clients
}

func (tb *TokenBucketLimiter) allowDefault() bool {
	select {
	case <-tb.tokenBucket["default"]:
		return true
	default:
		return false
	}
}

func getIPFromIdentifier(identifier string) string {
	if strings.Contains(identifier, ".") || strings.Contains(identifier, ":") {
		return identifier
	}
	return ""
}
