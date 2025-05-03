package backends

import (
	"lb/internal/modules/backends/models"
	"sync"
)

var (
	idCounter uint64
	idMutex   sync.Mutex
)

func NewBackend(url string, health string) *models.Backend {
	idMutex.Lock()
	idCounter++
	id := idCounter
	idMutex.Unlock()
	return &models.Backend{
		Id:     id,
		URL:    url,
		Health: health,
	}
}
