package healthchecker

import (
	"go.uber.org/zap"
	"lb/internal/modules/backends"
	"lb/internal/modules/backends/models"
	"net/http"
	"sync"
	"time"
)

type HealthChecker struct {
	serverChan         chan *models.Backend
	healthyFrequency   time.Duration
	unhealthyFrequency time.Duration
	registry           *backends.BackendRegistry
	healthySet         sync.Map
	httpClient         *http.Client
	logger             *zap.Logger
}

func NewHealthChecker(
	healthyFreq time.Duration,
	unhealthyFreq time.Duration,
	registry *backends.BackendRegistry,
	httpClient *http.Client,
	logger *zap.Logger,
) *HealthChecker {
	return &HealthChecker{
		serverChan:         make(chan *models.Backend, 1000),
		healthyFrequency:   healthyFreq,
		unhealthyFrequency: unhealthyFreq,
		registry:           registry,
		httpClient:         httpClient,
		logger:             logger,
	}
}

func (hc *HealthChecker) Start() {
	numWorkers := 3
	for i := 0; i < numWorkers; i++ {
		go hc.worker(i)
	}
}

func (hc *HealthChecker) AddBackend(backend *models.Backend) {
	hc.logger.Debug("Backend added to health checker", zap.String("url", backend.URL))
	hc.serverChan <- backend
}

func (hc *HealthChecker) worker(id int) {
	hc.logger.Info("Health check worker started", zap.Int("worker_id", id))
	for backend := range hc.serverChan {
		hc.checkBackend(backend)
	}
}

func (hc *HealthChecker) checkBackend(backend *models.Backend) {
	healthy := false

	resp, err := hc.httpClient.Get(backend.URL + backend.Health)
	if err == nil && resp.StatusCode == http.StatusOK {
		healthy = true
		hc.logger.Debug("Backend is healthy", zap.String("url", backend.URL))
	} else {
		hc.logger.Debug("Backend is unhealthy", zap.String("url", backend.URL), zap.Error(err))
	}

	hc.updateStatus(backend, healthy)

	var nextCheck time.Duration
	if healthy {
		nextCheck = hc.healthyFrequency
	} else {
		nextCheck = hc.unhealthyFrequency
	}

	time.AfterFunc(nextCheck, func() {
		hc.serverChan <- backend
	})
}

func (hc *HealthChecker) updateStatus(backend *models.Backend, isHealthy bool) {
	_, exists := hc.healthySet.Load(backend.Id)

	if isHealthy && !exists {
		hc.healthySet.Store(backend.Id, true)
		hc.registry.UpdateHealth(models.BackendStatus{Id: backend.Id, IsHealthy: true})
		hc.logger.Info("Marked backend healthy", zap.Uint64("id", backend.Id))
	} else if !isHealthy && exists {
		hc.healthySet.Delete(backend.Id)
		hc.registry.UpdateHealth(models.BackendStatus{Id: backend.Id, IsHealthy: false})
		hc.logger.Info("Marked backend unhealthy", zap.Uint64("id", backend.Id))
	}
}
