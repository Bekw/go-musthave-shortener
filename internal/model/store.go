package model

import (
	"context"
	"errors"
	"sync"
)

var (
	ErrCollision         = errors.New("id already exists")
	ErrDuplicateOriginal = errors.New("original url already exists")
)

type Store interface {
	Save(ctx context.Context, id, url string) error
	Get(ctx context.Context, id string) (string, bool, error)
	FindByOriginal(ctx context.Context, url string) (string, bool, error)
	Ping(ctx context.Context) error
}

type memoryStore struct {
	mu *sync.RWMutex
	mp map[string]string
	ri map[string]string
}

type DuplicateURLError struct {
	ExistingID string
}

func NewMemoryStore() Store {
	return &memoryStore{
		mu: new(sync.RWMutex),
		mp: make(map[string]string),
		ri: make(map[string]string),
	}
}

func (m *memoryStore) Save(ct context.Context, id, url string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.mp[id]; ok {
		return ErrCollision
	}
	if _, ok := m.ri[url]; ok {
		return ErrDuplicateOriginal
	}
	m.mp[id] = url
	m.ri[url] = id
	return nil
}

func (m *memoryStore) Get(ctx context.Context, id string) (string, bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	u, ok := m.mp[id]
	return u, ok, nil
}

func (m *memoryStore) FindByOriginal(ctx context.Context, url string) (string, bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	id, ok := m.ri[url]
	return id, ok, nil
}

func (m *memoryStore) Ping(ctx context.Context) error {
	return nil
}

func (e *DuplicateURLError) Error() string {
	return "this url already exists"
}
