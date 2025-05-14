package rateLimiter

import (
	"context"
	"go.uber.org/zap"
	"strings"
	"sync"
	"time"
)

// TokenBucketLimiter реализует алгоритм ограничения запросов "Token Bucket"
// с поддержкой индивидуальных лимитов для разных клиентов
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

// NewTokenBucketLimiter создает новый экземпляр rate limiter'а
// ctx - контекст для graceful shutdown
// limit - дефолтное количество запросов
// period - период, за который разрешено limit запросов
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

// StartPeriod запускает периодическое пополнение токенов
func (tb *TokenBucketLimiter) StartPeriod(ctx context.Context) {
	tb.logger.Info("Token bucket refill started", zap.Duration("period", tb.Period))
	ticker := time.NewTicker(tb.Period)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			tb.logger.Info("Token bucket refill stopped due to context cancellation")
			return
		case <-ticker.C:
			tb.refillBuckets()
		}
	}
}

// refillBuckets пополняет токены во всех bucket'ах
func (tb *TokenBucketLimiter) refillBuckets() {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	for ip, bucket := range tb.tokenBucket {
		select {
		case bucket <- struct{}{}:
			tb.logger.Debug("Refilled token for IP", zap.String("ip", ip))
		default:
			tb.logger.Debug("Bucket full, skipping refill", zap.String("ip", ip))
		}
	}
}

// Allow проверяет доступность токена для указанного IP
// Возвращает true если запрос разрешен, false если лимит исчерпан
func (tb *TokenBucketLimiter) Allow(ip string) bool {
	tb.mu.RLock()
	defer tb.mu.RUnlock()

	// Проверяем наличие индивидуального bucket'а для IP
	if ipBucket, exists := tb.tokenBucket[getIPFromIdentifier(ip)]; exists {
		select {
		case <-ipBucket:
			tb.logger.Debug("Request allowed", zap.String("ip", ip))
			return true
		default:
			tb.logger.Debug("Request denied - no tokens", zap.String("ip", ip))
			return false
		}
	}

	allowed := tb.allowDefault()
	// Если ничего нет - дефолт значение
	tb.logger.Debug("Request fallback to default", zap.String("ip", ip), zap.Bool("allowed", allowed))
	return allowed
}

// AddClient добавляет нового клиента с индивидуальными настройками лимита
func (tb *TokenBucketLimiter) AddClient(config *ClientConfig) {
	tb.clientStore.mu.Lock()
	defer tb.clientStore.mu.Unlock()

	// Создаем bucket с указанной емкостью
	ch := make(chan struct{}, config.Capacity)
	for i := 0; i < config.Capacity; i++ {
		ch <- struct{}{}
	}

	// Инициализируем хранилище если оно nil (на всякий случай)
	if tb.clientStore == nil {
		tb.clientStore = &ClientStore{
			clients: make(map[string]*ClientConfig),
		}
	}

	// Сохраняем клиента
	tb.clientStore.clients[config.Ip] = config
	tb.tokenBucket[config.Ip] = ch

	tb.logger.Info("Client added to TokenBucketLimiter",
		zap.String("ip", config.Ip),
		zap.Int("capacity", config.Capacity))

}

// GetClient возвращает конфигурацию клиента по IP
func (tb *TokenBucketLimiter) GetClient(clientIp string) (*ClientConfig, bool) {
	tb.clientStore.mu.RLock()
	defer tb.clientStore.mu.RUnlock()
	client, exists := tb.clientStore.clients[clientIp]
	return client, exists
}

// DeleteClient удаляет клиента и его bucket
func (tb *TokenBucketLimiter) DeleteClient(clientIp string) {
	tb.clientStore.mu.Lock()
	defer tb.clientStore.mu.Unlock()

	if client, exists := tb.clientStore.clients[clientIp]; exists {
		if client.Ip != "" {
			delete(tb.tokenBucket, client.Ip)
		}
		delete(tb.clientStore.clients, clientIp)
		delete(tb.tokenBucket, clientIp)
		tb.logger.Info("Client deleted", zap.String("ip", clientIp))
	}
}

// ListClients возвращает список всех клиентов
func (tb *TokenBucketLimiter) ListClients() []*ClientConfig {
	tb.clientStore.mu.RLock()
	defer tb.clientStore.mu.RUnlock()

	clients := make([]*ClientConfig, 0, len(tb.clientStore.clients))
	for _, client := range tb.clientStore.clients {
		clients = append(clients, client)
	}
	return clients
}

// allowDefault проверяет доступность токена в дефолтном bucket'е
func (tb *TokenBucketLimiter) allowDefault() bool {
	select {
	case <-tb.tokenBucket["default"]:
		return true
	default:
		return false
	}
}

// getIPFromIdentifier извлекает IP из идентификатора
func getIPFromIdentifier(identifier string) string {
	if strings.Contains(identifier, ".") || strings.Contains(identifier, ":") {
		return identifier
	}
	return ""
}
