// Package appcache defines the replaceable cache boundary. Values are serialized,
// so a Redis implementation does not need application-specific Go structs.
package appcache

import (
	"context"
	"strings"
	"sync"
	"time"
)

type Cache interface {
	Get(context.Context, string) ([]byte, bool, error)
	Set(context.Context, string, []byte, time.Duration) error
	DeletePrefix(context.Context, string) error
}
type entry struct {
	data    []byte
	expires time.Time
}
type Memory struct {
	mu              sync.Mutex
	items           map[string]entry
	bytes, maxBytes int
}

func NewMemory(maxBytes int) *Memory { return &Memory{items: map[string]entry{}, maxBytes: maxBytes} }
func (m *Memory) Get(_ context.Context, key string) ([]byte, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.items[key]
	if ok && time.Now().Before(v.expires) {
		return append([]byte(nil), v.data...), true, nil
	}
	if ok {
		m.bytes -= len(v.data)
		delete(m.items, key)
	}
	return nil, false, nil
}
func (m *Memory) Set(_ context.Context, key string, data []byte, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(data) > m.maxBytes || ttl <= 0 {
		return nil
	}
	if v, ok := m.items[key]; ok {
		m.bytes -= len(v.data)
		delete(m.items, key)
	}
	for k, v := range m.items {
		if time.Now().After(v.expires) || m.bytes+len(data) > m.maxBytes {
			m.bytes -= len(v.data)
			delete(m.items, k)
		}
	}
	m.items[key] = entry{append([]byte(nil), data...), time.Now().Add(ttl)}
	m.bytes += len(data)
	return nil
}
func (m *Memory) DeletePrefix(_ context.Context, prefix string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for k, v := range m.items {
		if strings.HasPrefix(k, prefix) {
			m.bytes -= len(v.data)
			delete(m.items, k)
		}
	}
	return nil
}
