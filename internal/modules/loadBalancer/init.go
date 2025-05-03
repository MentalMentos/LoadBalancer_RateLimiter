package loadBalancer

import (
	"go.uber.org/zap"
	"lb/internal/modules/backends"
	models "lb/internal/modules/backends/models"
	"lb/internal/modules/healthchecker"
)

type RouteConfig struct {
	Path     string
	Backends []models.Backend
}

// создает мапу path -> LoadBalancerHandler
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

func registerBackend(backend *models.Backend, registry *backends.BackendRegistry, healthChecker *healthchecker.HealthChecker) {
	healthChecker.AddBackend(backend)
	registry.AddBackendToRegistry(*backend)
}
