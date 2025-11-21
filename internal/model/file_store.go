package model

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

type fileRecord struct {
	ID       string `json:"id"`
	Original string `json:"original_url"`
}

type fileStore struct {
	*memoryStore
	path string
}

func NewFileStore(path string) (Store, error) {
	fs := &fileStore{
		memoryStore: &memoryStore{
			mu: new(sync.RWMutex),
			mp: make(map[string]string),
			ri: make(map[string]string),
		},
		path: path,
	}

	if err := fs.load(); err != nil {
		return nil, fmt.Errorf("load file store: %w", err)
	}

	return fs, nil
}

func (f *fileStore) Save(ctx context.Context, id, url string) error {
	if err := f.memoryStore.Save(ctx, id, url); err != nil {
		return err
	}

	if err := f.flush(); err != nil {
		return fmt.Errorf("flush file store: %w", err)
	}
	return nil
}

func (f *fileStore) load() error {
	if err := os.MkdirAll(filepath.Dir(f.path), 0o755); err != nil {
		return fmt.Errorf("mkdir %q: %w", filepath.Dir(f.path), err)
	}
	b, err := os.ReadFile(f.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read %q: %w", f.path, err)
	}
	if len(b) == 0 {
		return nil
	}

	var items []fileRecord
	if err := json.Unmarshal(b, &items); err != nil {
		return fmt.Errorf("unmarshal %q: %w", f.path, err)
	}
	f.mu.Lock()
	for _, it := range items {
		f.mp[it.ID] = it.Original
	}
	f.mu.Unlock()
	return nil
}

func (f *fileStore) flush() error {
	f.mu.RLock()
	items := make([]fileRecord, 0, len(f.mp))
	for id, orig := range f.mp {
		items = append(items, fileRecord{ID: id, Original: orig})
	}
	f.mu.RUnlock()

	b, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	tmp := f.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return fmt.Errorf("write tmp %q: %w", tmp, err)
	}
	if err := os.Rename(tmp, f.path); err != nil {
		return fmt.Errorf("rename to %q: %w", f.path, err)
	}
	return nil
}
