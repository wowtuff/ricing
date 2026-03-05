package tools

import (
	"sync"

	"github.com/wowtuff/ricing/utils"
)

type Registry struct {
	mu    sync.RWMutex
	tools map[string]Tool
}

func NewRegistry() *Registry {
	return &Registry{
		tools: make(map[string]Tool),
	}
}

func (r *Registry) Register(t Tool) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := t.Specs().Name
	if _, exists := r.tools[name]; exists {
		return utils.LogError("tool already registered: %s", name)
	}
	r.tools[name] = t
	return nil
}

func (r *Registry) Get(name string) (Tool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	t, ok := r.tools[name]
	if !ok {
		return nil, utils.LogError("tool not found: %s", name)
	}
	return t, nil
}

func (r *Registry) List() []ToolSpec {
	r.mu.RLock()
	defer r.mu.RUnlock()

	specs := make([]ToolSpec, 0, len(r.tools))
	for _, t := range r.tools {
		specs = append(specs, t.Specs())
	}
	return specs
}
