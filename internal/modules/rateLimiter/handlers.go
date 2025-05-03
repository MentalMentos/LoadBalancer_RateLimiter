package rateLimiter

import (
	"encoding/json"
	"net/http"
)

func (tb *TokenBucketLimiter) ClientsHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		tb.handleGetClients(w)
	case http.MethodPost:
		tb.handleCreateClient(w, r)
	case http.MethodDelete:
		tb.handleDeleteClient(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (tb *TokenBucketLimiter) handleGetClients(w http.ResponseWriter) {
	clients := tb.ListClients()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(clients)
}

func (tb *TokenBucketLimiter) handleCreateClient(w http.ResponseWriter, r *http.Request) {
	var config ClientConfig
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if config.Ip == "" {
		http.Error(w, "client_id is required", http.StatusBadRequest)
		return
	}

	tb.AddClient(&config)
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(config)
}

func (tb *TokenBucketLimiter) handleDeleteClient(w http.ResponseWriter, r *http.Request) {
	clientIp := r.URL.Query().Get("client_ip")
	if clientIp == "" {
		http.Error(w, "client_ip is required", http.StatusBadRequest)
		return
	}

	tb.DeleteClient(clientIp)
	w.WriteHeader(http.StatusNoContent)
}
