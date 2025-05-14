package loadBalancer

import (
	"go.uber.org/zap"
	"lb/internal/modules/backends"
	models "lb/internal/modules/backends/models"
	"lb/internal/modules/healthchecker"
)

// RouteConfig определяет конфигурацию маршрута для балансировщика нагрузки.
// Содержит путь (endpoint) и список бэкендов, которые могут его обслуживать.
type RouteConfig struct {
	Path     string
	Backends []models.Backend
}

// CreateLoadBalancers инициализирует набор балансировщиков нагрузки для каждого маршрута.
// Возвращает:
//
//	map[string]*LoadBalancerHandler: готовые к использованию обработчики,
//	где ключ - это путь, а значение - соответствующий LoadBalancerHandler.
func CreateLoadBalancers(routes []RouteConfig,
	registry *backends.BackendRegistry,
	healthChecker *healthchecker.HealthChecker,
	logger *zap.Logger) map[string]*LoadBalancerHandler {

	lbMap := make(map[string]*LoadBalancerHandler)

	for _, route := range routes {
		healthChannels := setupHealthAndRegister(route.Backends, registry, healthChecker)

		lbHandler := NewLBHandler(registry, healthChannels, logger)
		lbMap[route.Path] = lbHandler
		logger.Debug("Load balancer created for route", zap.String("path", route.Path))
	}

	return lbMap
}

// setupHealthAndRegister регистрирует бэкенды в системе и настраивает подписку на их статусы.
// Для каждого бэкенда:
// 1. Добавляет его в health checker для мониторинга
// 2. Регистрирует в общем реестре
// 3. Создает подписку на изменения состояния
//
// Возвращает список каналов для получения обновлений о состоянии бэкендов.
func setupHealthAndRegister(backendsConfig []models.Backend, registry *backends.BackendRegistry, healthChecker *healthchecker.HealthChecker) []<-chan models.BackendStatus {
	var healthChannels []<-chan models.BackendStatus

	for _, backend := range backendsConfig {
		backendCopy := backend
		registerBackend(&backendCopy, registry, healthChecker)

		ch := registry.Subscribe(backendCopy.Id)
		healthChannels = append(healthChannels, ch)
	}

	return healthChannels
}

// registerBackend выполняет полную регистрацию бэкенда в системе:
// 1. Добавляет в health checker для регулярных проверок
// 2. Регистрирует в общем реестре бэкендов
func registerBackend(backend *models.Backend, registry *backends.BackendRegistry, healthChecker *healthchecker.HealthChecker) {
	healthChecker.AddBackend(backend)
	registry.AddBackendToRegistry(*backend)
}
