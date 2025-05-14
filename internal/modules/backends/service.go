package backends

import (
	"lb/internal/modules/backends/models"
	"log"
	"sync"
)

// healthUpdateChannel - канал для рассылки обновлений статуса бэкендов
type healthUpdateChannel chan models.BackendStatus

// BackendRegistry реализует потокобезопасное хранилище бэкендов
// с механизмом подписки на изменения их состояния
type BackendRegistry struct {
	mu          sync.RWMutex
	backendId   map[uint64]models.Backend
	backends    map[uint64]models.BackendStatus
	subscribers map[uint64][]healthUpdateChannel
}

// NewBackendRegistry создает новый экземпляр реестра бэкендов
func NewBackendRegistry() *BackendRegistry {
	return &BackendRegistry{
		backendId:   make(map[uint64]models.Backend),
		backends:    make(map[uint64]models.BackendStatus),
		subscribers: make(map[uint64][]healthUpdateChannel),
	}
}

// UpdateHealth обновляет статус бэкенда и уведомляет подписчиков
// Возвращает ошибку если передан пустой статус
func (r *BackendRegistry) UpdateHealth(status models.BackendStatus) error {
	if status == (models.BackendStatus{}) {
		log.Fatal("status is empty")
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	// Сохраняем новый статус
	r.backends[status.Id] = status

	// Уведомляем всех подписчиков этого бэкенда
	if subs, ok := r.subscribers[status.Id]; ok {
		for _, ch := range subs {
			ch <- status
		}
	}
	return nil
}

// Subscribe добавляет подписку на обновления статуса бэкенда
// Возвращает канал для получения обновлений
func (r *BackendRegistry) Subscribe(backendId uint64) <-chan models.BackendStatus {
	r.mu.Lock()
	defer r.mu.Unlock()

	chForUpdate := make(chan models.BackendStatus, 10)

	// Добавляем новый канал в список подписчиков
	r.subscribers[backendId] = append(r.subscribers[backendId], chForUpdate)

	return chForUpdate
}

// GetBackendById возвращает бэкенд по его ID
func (r *BackendRegistry) GetBackendById(backendId uint64) (models.Backend, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	backend, exists := r.backendId[backendId]
	return backend, exists
}

// AddBackendToRegistry добавляет новый бэкенд в реестр
func (r *BackendRegistry) AddBackendToRegistry(backend models.Backend) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.backendId[backend.Id] = backend
}
