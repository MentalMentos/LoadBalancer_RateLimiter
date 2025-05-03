package backends

import (
	"lb/internal/modules/backends/models"
	"log"
	"sync"
)

type healthUpdateChannel chan models.BackendStatus

type BackendRegistry struct {
	mu          sync.RWMutex
	backendId   map[uint64]models.Backend
	backends    map[uint64]models.BackendStatus
	subscribers map[uint64][]healthUpdateChannel
}

func NewBackendRegistry() *BackendRegistry {
	return &BackendRegistry{
		backendId:   make(map[uint64]models.Backend),
		backends:    make(map[uint64]models.BackendStatus),
		subscribers: make(map[uint64][]healthUpdateChannel),
	}
}

func (r *BackendRegistry) UpdateHealth(status models.BackendStatus) error {
	if status == (models.BackendStatus{}) {
		log.Fatal("status is empty")
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	r.backends[status.Id] = status

	if subs, ok := r.subscribers[status.Id]; ok {
		for _, ch := range subs {
			ch <- status
		}
	}
	return nil
}

func (r *BackendRegistry) Subscribe(backendId uint64) <-chan models.BackendStatus {
	r.mu.Lock()
	defer r.mu.Unlock()

	chForUpdate := make(chan models.BackendStatus, 10)

	r.subscribers[backendId] = append(r.subscribers[backendId], chForUpdate)

	return chForUpdate
}

func (r *BackendRegistry) GetBackendById(backendId uint64) (models.Backend, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	backend, exists := r.backendId[backendId]
	return backend, exists
}

func (r *BackendRegistry) AddBackendToRegistry(backend models.Backend) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.backendId[backend.Id] = backend
}
