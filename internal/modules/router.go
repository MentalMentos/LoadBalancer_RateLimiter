package routes

import (
	"encoding/json"
	"lb/internal/modules/loadBalancer"
	rateLimiter2 "lb/internal/modules/rateLimiter"
	"net/http"
)

func CreateRouter(lbMap map[string]*loadBalancer.LoadBalancerHandler,
	limiter *rateLimiter2.TokenBucketLimiter) *http.ServeMux {

	router := http.NewServeMux()

	for path, handler := range lbMap {
		router.Handle(path, rateLimitMiddleware(handler, limiter))
	}

	router.HandleFunc("/clients", limiter.ClientsHandler)

	return router
}

func rateLimitMiddleware(next http.Handler, limiter *rateLimiter2.TokenBucketLimiter) http.Handler {
	ip := "127.0.0.1"
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
