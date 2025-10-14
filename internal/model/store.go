package model

import "sync"

type Store interface {
	Set(id, url string)
	Get(id string) (string, bool)
	Exists(id string) bool
}

type memoryStore struct {
	mu sync.RWMutex
	mp map[string]string
}

func NewMemoryStore() Store {
	return &memoryStore{mp: make(map[string]string)}
}

func (m *memoryStore) Set(id, url string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.mp[id] = url
}

func (m *memoryStore) Get(id string) (string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	url, ok := m.mp[id]
	return url, ok
}

func (m *memoryStore) Exists(id string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.mp[id]
	return ok
}
