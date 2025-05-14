package rateLimiter

import (
	"encoding/json"
	"go.uber.org/zap"
	"net/http"
)

// ClientsHandler обрабатывает HTTP запросы для управления клиентами rate limiter'а.
// Поддерживает GET (получение списка), POST (добавление) и DELETE (удаление) методы.
func (tb *TokenBucketLimiter) ClientsHandler(w http.ResponseWriter, r *http.Request) {
	tb.logger.Info("Received request for ClientsHandler", zap.String("method", r.Method), zap.String("url", r.URL.String()))
	switch r.Method {
	case http.MethodGet:
		tb.handleGetClients(w)
	case http.MethodPost:
		tb.handleCreateClient(w, r)
	case http.MethodDelete:
		tb.handleDeleteClient(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		tb.logger.Warn("Method not allowed", zap.String("method", r.Method), zap.String("url", r.URL.String()))
	}
}

// handleGetClients возвращает текущий список клиентов в формате JSON
func (tb *TokenBucketLimiter) handleGetClients(w http.ResponseWriter) {
	tb.logger.Info("Handling GET request for clients")
	clients := tb.ListClients()
	w.Header().Set("Content-Type", "application/json")
	err := json.NewEncoder(w).Encode(clients)
	if err != nil {
		tb.logger.Error("Error encoding clients to JSON", zap.Error(err))
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
	tb.logger.Info("Successfully returned clients list", zap.Int("clients_count", len(clients)))
}

// handleCreateClient добавляет нового клиента на основе переданной конфигурации
// Валидирует обязательное поле IP адреса
func (tb *TokenBucketLimiter) handleCreateClient(w http.ResponseWriter, r *http.Request) {
	tb.logger.Info("Handling POST request to create a client", zap.String("url", r.URL.String()))
	var config ClientConfig
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		tb.logger.Warn("Error decoding client config", zap.Error(err))
		return
	}

	if config.Ip == "" {
		http.Error(w, "client_id is required", http.StatusBadRequest)
		tb.logger.Warn("Client IP is required")
		return
	}

	tb.AddClient(&config)
	w.WriteHeader(http.StatusCreated)
	err := json.NewEncoder(w).Encode(config)
	if err != nil {
		tb.logger.Error("Error encoding client config to JSON", zap.Error(err))
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
	tb.logger.Info("Successfully created new client", zap.String("client_ip", config.Ip))
}

// handleDeleteClient удаляет клиента по его IP адресу
// IP адрес должен быть передан в query параметре client_ip
func (tb *TokenBucketLimiter) handleDeleteClient(w http.ResponseWriter, r *http.Request) {
	clientIp := r.URL.Query().Get("client_ip")
	if clientIp == "" {
		http.Error(w, "client_ip is required", http.StatusBadRequest)
		tb.logger.Warn("Client IP parameter missing in DELETE request")
		return
	}

	tb.DeleteClient(clientIp)
	w.WriteHeader(http.StatusNoContent)
	tb.logger.Info("Successfully deleted client", zap.String("client_ip", clientIp))
}
