package loadBalancer

import (
	"errors"
	"go.uber.org/zap"
	"lb/internal/modules/backends"
	modelsBackend "lb/internal/modules/backends/models"
	"sync"
	"sync/atomic"
)

// ------------------ROUND-ROBIN ------------------
// RoundRobinAlgorithm реализует классический алгоритм кругового распределения
// Использует атомарный счетчик
type RoundRobinAlgorithm struct {
	current uint32
}

func NewRoundRobinStrategy() *RoundRobinAlgorithm {
	return &RoundRobinAlgorithm{}
}

// GetNextBackend выбирает следующий бэкенд в ротации
// Возвращает ошибку если нет доступных бэкендов (принцип fail-fast)
func (rr *RoundRobinAlgorithm) GetNextBackend(backends []*modelsBackend.Backend) (*modelsBackend.Backend, error) {
	if len(backends) == 0 {
		return nil, errors.New("no backends available")
	}
	// Атомарное увеличение счетчика с защитой от переполнения
	index := atomic.AddUint32(&rr.current, 1) - 1
	return backends[index%uint32(len(backends))], nil
}

type LoadBalancingStrategy interface {
	GetNextBackend([]*modelsBackend.Backend) (*modelsBackend.Backend, error)
}

//--------------------------------------------------

type Loadbalancer struct {
	BackendRegistry      *backends.BackendRegistry
	logger               *zap.Logger
	Algorithm            LoadBalancingStrategy
	healthUpdateChannels []<-chan modelsBackend.BackendStatus
	healthyBackends      []*modelsBackend.Backend
	mu                   sync.RWMutex
}

// NewLoadBalancer конструктор балансировщика
// registry: источник конфигурации бэкендов
// healthChannels: каналы обновления статусов
// logger: настроенный экземпляр логгера
func NewLoadBalancer(registry *backends.BackendRegistry, healthChannels []<-chan modelsBackend.BackendStatus, logger *zap.Logger) *Loadbalancer {
	lb := &Loadbalancer{
		BackendRegistry:      registry,
		Algorithm:            NewRoundRobinStrategy(),
		healthUpdateChannels: healthChannels,
		logger:               logger,
	}
	var wg sync.WaitGroup
	wg.Add(len(lb.healthUpdateChannels) + 1)
	go lb.listenToHealthUpdates(&wg)
	return lb
}

// listenToHealthUpdates запускает мониторинг здоровья бэкендов
// Запускает по горутине на каждый канал обновлений
func (lb *Loadbalancer) listenToHealthUpdates(wg *sync.WaitGroup) {
	lb.logger.Info("Listening for health updates in loadbalancer")
	for _, ch := range lb.healthUpdateChannels {
		go func(c <-chan modelsBackend.BackendStatus) {
			defer wg.Done()
			for update := range c {
				lb.updateProcess(update)
			}
		}(ch)
	}

	wg.Wait()
}

// updateProcess обрабатывает обновления статусов бэкендов
// Разделяет обработку статусов и управление каналами
func (lb *Loadbalancer) updateProcess(update modelsBackend.BackendStatus) {
	if update.IsHealthy {
		lb.addToHealthyBacks(update.Id)
		lb.logger.Info("Proccessing to update healthy backend")
	} else {
		lb.removeFromHealthyBackends(update.Id)
	}
}

// addToHealthyBacks поддерживает актуальный список здоровых нод
// Использует мьютекс для защиты от конкурентного доступа
func (lb *Loadbalancer) addToHealthyBacks(id uint64) {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	for _, backend := range lb.healthyBackends {
		if backend.Id == id {
			lb.healthyBackends = append(lb.healthyBackends, backend)
		}
	}

	backend, ok := lb.BackendRegistry.GetBackendById(id)
	if !ok {
		lb.logger.Info("No backend found for id", zap.Uint64("id", id))
		return
	}

	lb.healthyBackends = append(lb.healthyBackends, &backend)
	lb.logger.Info("Added healthy backend! Backend id: ", zap.Uint64("id", id))
}

// removeFromHealthyBackends удаляет нездоровые ноды
// Использует работу со срезами для эффективного удаления (порядок не сохраняется)
func (lb *Loadbalancer) removeFromHealthyBackends(backendId uint64) {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	for i := 0; i < len(lb.healthyBackends); i++ {
		if lb.healthyBackends[i].Id == backendId {
			copy(lb.healthyBackends[i:], lb.healthyBackends[i+1:])
			lb.healthyBackends = lb.healthyBackends[:len(lb.healthyBackends)-1]
			return
		}
	}
}

// getHealthyBackends возвращает текущий список здоровых бэкендов
func (lb *Loadbalancer) getHealthyBackends() []*modelsBackend.Backend {
	lb.mu.RLock()
	defer lb.mu.RUnlock()
	return lb.healthyBackends
}
