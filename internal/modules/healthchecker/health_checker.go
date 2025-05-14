package healthchecker

import (
	"go.uber.org/zap"
	"lb/internal/modules/backends"
	"lb/internal/modules/backends/models"
	"net/http"
	"sync"
	"time"
)

// HealthChecker реализует систему мониторинга состояния бэкендов.
// Использует пул воркеров для асинхронных проверок и поддерживает
// разные интервалы для здоровых/нездоровых сервисов.
type HealthChecker struct {
	serverChan         chan *models.Backend
	healthyFrequency   time.Duration
	unhealthyFrequency time.Duration
	registry           *backends.BackendRegistry
	healthySet         sync.Map
	httpClient         *http.Client
	logger             *zap.Logger
}

// NewHealthChecker создает экземпляр HealthChecker с настраиваемыми параметрами.
// healthyFreq: интервал проверок для работающих бэкендов (например, 30s)
// unhealthyFreq: интервал для повторных проверок упавших сервисов (например, 5s)
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

// Start запускает пул воркеров для параллельных проверок.
// Оптимальное количество воркеров зависит от нагрузки и сетевых задержек.
func (hc *HealthChecker) Start() {
	numWorkers := 3
	for i := 0; i < numWorkers; i++ {
		go hc.worker(i)
	}
}

// AddBackend добавляет бэкенд в систему мониторинга.
// Гарантирует thread-safe добавление через буферизованный канал.
func (hc *HealthChecker) AddBackend(backend *models.Backend) {
	hc.logger.Info("Backend added to health checker", zap.String("url", backend.URL))
	hc.serverChan <- backend
}

// worker - основной цикл обработки проверок для одного воркера.
// Каждый воркер независимо обрабатывает бэкенды из общего канала.
func (hc *HealthChecker) worker(id int) {
	hc.logger.Info("Health check worker started", zap.Int("worker_id", id))
	for backend := range hc.serverChan {
		hc.checkBackend(backend)
	}
}

// checkBackend выполняет HTTP-проверку состояния бэкенда.
// Логика проверки может быть расширена для поддержки разных протоколов.
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

	// Динамическое планирование следующей проверки
	var nextCheck time.Duration
	if healthy {
		nextCheck = hc.healthyFrequency
	} else {
		nextCheck = hc.unhealthyFrequency
	}

	time.AfterFunc(nextCheck, func() {
		hc.serverChan <- backend // Регистрируем следующую проверку
	})
}

// updateStatus атомарно обновляет состояние бэкенда в registry и кэше.
// Оптимизирует запросы к registry, обновляя только при изменении статуса.
func (hc *HealthChecker) updateStatus(backend *models.Backend, isHealthy bool) {
	_, exists := hc.healthySet.Load(backend.Id)

	// Обновляем только при изменении состояния
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
