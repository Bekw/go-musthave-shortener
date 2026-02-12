package pool

import (
	"reflect"
	"sync"
)

type Resettable interface {
	Reset()
}

type Pool[T Resettable] struct {
	p sync.Pool
}

func New[T Resettable](newFn func() T) *Pool[T] {
	if newFn == nil {
		panic("pool.New: newFn is nil")
	}

	pl := &Pool[T]{}
	pl.p.New = func() any { return newFn() }
	return pl
}

func (pl *Pool[T]) Get() T {
	v := pl.p.Get()
	if v == nil {
		var zero T
		return zero
	}
	return v.(T)
}

func (pl *Pool[T]) Put(v T) {
	if isNil(v) {
		return
	}

	v.Reset()
	pl.p.Put(v)
}

func isNil[T any](v T) bool {
	rv := reflect.ValueOf(v)
	if !rv.IsValid() {
		return true
	}
	switch rv.Kind() {
	case reflect.Pointer, reflect.Interface, reflect.Slice, reflect.Map, reflect.Func, reflect.Chan:
		return rv.IsNil()
	default:
		return false
	}
}
