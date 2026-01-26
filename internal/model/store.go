package model

import (
	"context"
	"errors"
	"sync"
)

var (
	ErrCollision         = errors.New("id already exists")
	ErrDuplicateOriginal = errors.New("original url already exists")
	ErrDeleted           = errors.New("url was deleted")
)

type UserURL struct {
	ID          string
	OriginalURL string
}

// Store defines operations required by the shortener storage layer.
type Store interface {
	Save(ctx context.Context, id, url string) error
	Get(ctx context.Context, id string) (string, bool, error)
	FindByOriginal(ctx context.Context, url string) (string, bool, error)
	Ping(ctx context.Context) error
	MarkDeleted(ctx context.Context, ids []string) error
	AddUserURL(ctx context.Context, userID, urlID string) error
	GetUserURLs(ctx context.Context, userID string) ([]UserURL, error)
	DeleteUserURLs(ctx context.Context, userID string, ids []string) error
}

type memoryStore struct {
	mu       *sync.RWMutex
	mp       map[string]string
	ri       map[string]string
	deleted  map[string]bool
	userURLs map[string]map[string]struct{}
}

type DuplicateURLError struct {
	ExistingID string
}

func NewMemoryStore() Store {
	return &memoryStore{
		mu:       new(sync.RWMutex),
		mp:       make(map[string]string),
		ri:       make(map[string]string),
		deleted:  make(map[string]bool),
		userURLs: make(map[string]map[string]struct{}),
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
	if m.deleted == nil {
		m.deleted = make(map[string]bool)
	}
	m.deleted[id] = false
	return nil
}

func (m *memoryStore) Get(ctx context.Context, id string) (string, bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	u, ok := m.mp[id]
	if !ok {
		return "", false, nil
	}
	if m.deleted != nil && m.deleted[id] {
		return "", false, ErrDeleted
	}
	return u, true, nil
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

func (m *memoryStore) MarkDeleted(ctx context.Context, ids []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.deleted == nil {
		m.deleted = make(map[string]bool)
	}
	for _, id := range ids {
		if _, ok := m.mp[id]; ok {
			m.deleted[id] = true
		}
	}
	return nil
}

func (m *memoryStore) AddUserURL(ctx context.Context, userID, urlID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.userURLs == nil {
		m.userURLs = make(map[string]map[string]struct{})
	}
	set, ok := m.userURLs[userID]
	if !ok {
		set = make(map[string]struct{})
		m.userURLs[userID] = set
	}
	set[urlID] = struct{}{}
	return nil
}

func (m *memoryStore) GetUserURLs(ctx context.Context, userID string) ([]UserURL, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	set := m.userURLs[userID]
	if len(set) == 0 {
		return nil, nil
	}
	res := make([]UserURL, 0, len(set))
	for id := range set {
		orig, ok := m.mp[id]
		if !ok {
			continue
		}
		if m.deleted != nil && m.deleted[id] {
			continue
		}
		res = append(res, UserURL{
			ID:          id,
			OriginalURL: orig,
		})
	}
	return res, nil
}

func (m *memoryStore) DeleteUserURLs(ctx context.Context, userID string, ids []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	set := m.userURLs[userID]
	if set == nil {
		return nil
	}
	if m.deleted == nil {
		m.deleted = make(map[string]bool)
	}
	for _, id := range ids {
		if _, ok := set[id]; !ok {
			continue
		}
		m.deleted[id] = true
		delete(set, id)
	}
	if len(set) == 0 {
		delete(m.userURLs, userID)
	} else {
		m.userURLs[userID] = set
	}
	return nil
}
