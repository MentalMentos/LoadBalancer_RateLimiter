package routes

import (
	"encoding/json"
	"go.uber.org/zap"
	"lb/internal/modules/loadBalancer"
	rateLimiter2 "lb/internal/modules/rateLimiter"
	"net"
	"net/http"
)

// CreateRouter инициализирует маршрутизатор с обработчиками балансировщика нагрузки
// и middleware для ограничения запросов. Также добавляет endpoint для мониторинга клиентов.
func CreateRouter(lbMap map[string]*loadBalancer.LoadBalancerHandler,
	limiter *rateLimiter2.TokenBucketLimiter, logger *zap.Logger) *http.ServeMux {

	router := http.NewServeMux()

	// Регистрируем все пути из конфигурации балансировщика
	// с middleware для rate limiting'а
	for path, handler := range lbMap {
		router.Handle(path, rateLimitMiddleware(handler, limiter))
	}

	// Специальный endpoint для получения списка клиентов
	router.HandleFunc("/clients", limiter.ClientsHandler)

	return router
}

// rateLimitMiddleware проверяет не превысил ли клиент лимит запросов.
// В случае превышения возвращает 429 статус с JSON ошибкой.
func rateLimitMiddleware(next http.Handler, limiter *rateLimiter2.TokenBucketLimiter) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := getClientIP(r)
		if !limiter.Allow(ip) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "rate limit exceeded",
			})
			return
		}
		next.ServeHTTP(w, r)
	})
}

// getClientIP извлекает IP адрес клиента из запроса,
// обрабатывая случай когда RemoteAddr содержит порт
func getClientIP(r *http.Request) string {
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return ""
	}
	return ip
}
