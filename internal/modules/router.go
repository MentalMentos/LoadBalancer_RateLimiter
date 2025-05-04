package routes

import (
	"encoding/json"
	"go.uber.org/zap"
	"lb/internal/modules/loadBalancer"
	rateLimiter2 "lb/internal/modules/rateLimiter"
	"net"
	"net/http"
)

func CreateRouter(lbMap map[string]*loadBalancer.LoadBalancerHandler,
	limiter *rateLimiter2.TokenBucketLimiter, logger *zap.Logger) *http.ServeMux {

	router := http.NewServeMux()

	for path, handler := range lbMap {
		logger.Info("Registering route: ")
		logger.Info(path)
		router.Handle(path, rateLimitMiddleware(handler, limiter))
	}

	router.HandleFunc("/clients", limiter.ClientsHandler)

	return router
}

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

func getClientIP(r *http.Request) string {
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return ""
	}
	return ip
}
