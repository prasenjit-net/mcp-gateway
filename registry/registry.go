package registry

import (
	"sync"

	"github.com/prasenjit-net/mcp-gateway/spec"
)

type Registry struct {
	mu          sync.RWMutex
	tools       map[string]*spec.ToolDefinition
	subscribers []chan struct{}
}

func NewRegistry() *Registry {
	return &Registry{
		tools: make(map[string]*spec.ToolDefinition),
	}
}

func (r *Registry) RebuildAll(tools []*spec.ToolDefinition) {
	r.mu.Lock()
	newMap := make(map[string]*spec.ToolDefinition, len(tools))
	for _, t := range tools {
		newMap[t.Name] = t
	}
	r.tools = newMap
	subs := r.subscribers
	r.mu.Unlock()

	r.notifySubscribers(subs)
}

func (r *Registry) Get(name string) (*spec.ToolDefinition, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.tools[name]
	return t, ok
}

func (r *Registry) List() []*spec.ToolDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()
	list := make([]*spec.ToolDefinition, 0, len(r.tools))
	for _, t := range r.tools {
		list = append(list, t)
	}
	return list
}

func (r *Registry) Subscribe() <-chan struct{} {
	r.mu.Lock()
	defer r.mu.Unlock()
	ch := make(chan struct{}, 1)
	r.subscribers = append(r.subscribers, ch)
	return ch
}

func (r *Registry) Unsubscribe(ch <-chan struct{}) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i, sub := range r.subscribers {
		if sub == ch {
			r.subscribers = append(r.subscribers[:i], r.subscribers[i+1:]...)
			return
		}
	}
}

func (r *Registry) notifySubscribers(subs []chan struct{}) {
	for _, ch := range subs {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}
