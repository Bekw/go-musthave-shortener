package service

import (
	"context"
	"errors"
	"math/rand"
	"sync"

	"go.uber.org/zap"

	"github.com/Bekw/go-musthave-shortener/internal/model"
)

// deleteTask represents a batch of URLs to delete for a user.
type deleteTask struct {
	userID string
	ids    []string
}

// URLService handles business logic for URL shortening operations.
type URLService struct {
	store model.Store
	log   *zap.Logger

	deleteCh  chan deleteTask
	closeOnce sync.Once
	workerWG  sync.WaitGroup
}

// NewURLService creates a new URL service with background workers.
func NewURLService(store model.Store, log *zap.Logger) *URLService {
	if log == nil {
		log = zap.NewNop()
	}

	s := &URLService{
		store:    store,
		log:      log,
		deleteCh: make(chan deleteTask, 128),
	}

	// Start background worker for async deletion.
	s.workerWG.Add(1)
	go s.deleteWorker()

	return s
}

// deleteWorker processes deletion tasks from the channel.
// It will finish processing all queued tasks after deleteCh is closed.
func (s *URLService) deleteWorker() {
	defer s.workerWG.Done()

	for task := range s.deleteCh {
		if len(task.ids) == 0 {
			continue
		}

		if err := s.store.DeleteUserURLs(context.Background(), task.userID, task.ids); err != nil {
			s.log.Error("async delete failed", zap.Error(err))
			continue
		}
	}
}

// Shutdown gracefully stops background workers and waits until they finish.
// Pass a context with timeout from main() so shutdown can't hang forever.
func (s *URLService) Shutdown(ctx context.Context) error {
	// Close channel once to signal worker to finish remaining tasks.
	s.closeOnce.Do(func() {
		close(s.deleteCh)
	})

	done := make(chan struct{})
	go func() {
		s.workerWG.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// GenerateID generates a random 6-character alphanumeric ID.
func GenerateID() string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, 6)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

// SaveWithRetries attempts to save a URL with the given number of retries on ID collision.
// Returns the generated ID, whether it already existed, and any error.
func (s *URLService) SaveWithRetries(ctx context.Context, original string, maxTry int) (id string, existed bool, err error) {
	for i := 0; i < maxTry; i++ {
		id = GenerateID()
		err = s.store.Save(ctx, id, original)
		if err == nil {
			return id, false, nil
		}

		if errors.Is(err, model.ErrCollision) {
			continue
		}

		var dup *model.DuplicateURLError
		if errors.As(err, &dup) {
			return dup.ExistingID, true, nil
		}

		if errors.Is(err, model.ErrDuplicateOriginal) {
			if existID, ok, e := s.store.FindByOriginal(ctx, original); e == nil && ok {
				return existID, true, nil
			}
			return "", true, nil
		}

		return "", false, err
	}
	return "", false, model.ErrCollision
}

// ScheduleDelete queues URLs for asynchronous deletion.
// Returns an error if the deletion queue is full.
// If shutdown has begun, returns an error.
func (s *URLService) ScheduleDelete(userID string, ids []string) (err error) {
	if len(ids) == 0 {
		return nil
	}

	task := deleteTask{
		userID: userID,
		ids:    append([]string(nil), ids...),
	}

	// If shutdown closes the channel concurrently with send, a panic is possible.
	// Convert it to a normal error.
	defer func() {
		if r := recover(); r != nil {
			err = errors.New("service is shutting down")
		}
	}()

	select {
	case s.deleteCh <- task:
		return nil
	default:
		return errors.New("delete queue is full")
	}
}

// GetUserURLs retrieves all URLs belonging to a user.
func (s *URLService) GetUserURLs(ctx context.Context, userID string) ([]model.UserURL, error) {
	return s.store.GetUserURLs(ctx, userID)
}

// AddUserURL associates a URL with a user.
func (s *URLService) AddUserURL(ctx context.Context, userID, urlID string) error {
	return s.store.AddUserURL(ctx, userID, urlID)
}

// Get retrieves the original URL for a given short ID.
func (s *URLService) Get(ctx context.Context, id string) (string, bool, error) {
	return s.store.Get(ctx, id)
}
