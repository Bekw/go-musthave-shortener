package model

import (
	"errors"
	"sync"
)

var ErrCollision = errors.New("id already exists")

type Store interface {
	Save(id, url string) error
	Get(id string) (string, bool)
}

type memoryStore struct {
	mu *sync.RWMutex
	mp map[string]string
}

type DuplicateURLError struct {
	ExistingID string
}

func NewMemoryStore() Store {
	return &memoryStore{
		mu: new(sync.RWMutex),
		mp: make(map[string]string),
	}
}

func (m *memoryStore) Save(id, url string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.mp[id]; ok {
		return ErrCollision
	}
	m.mp[id] = url
	return nil
}

func (m *memoryStore) Get(id string) (string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	u, ok := m.mp[id]
	return u, ok
}

func (e *DuplicateURLError) Error() string {
	return "this url already exists"
}
