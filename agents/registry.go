package agents

import (
	"sort"
	"strings"
)

type Registry struct {
	managers map[string]Manager
	order    []string
}

func NewRegistry(managers ...Manager) *Registry {
	r := &Registry{managers: map[string]Manager{}}
	for _, manager := range managers {
		r.Register(manager)
	}
	return r
}

func (r *Registry) Register(manager Manager) {
	if manager == nil {
		return
	}
	id := strings.TrimSpace(manager.ID())
	if id == "" {
		return
	}
	if _, exists := r.managers[id]; !exists {
		r.order = append(r.order, id)
	}
	r.managers[id] = manager
}

func (r *Registry) Get(id string) (Manager, bool) {
	manager, ok := r.managers[strings.TrimSpace(id)]
	return manager, ok
}

func (r *Registry) IDs() []string {
	if len(r.order) > 0 {
		ids := append([]string(nil), r.order...)
		return ids
	}
	ids := make([]string, 0, len(r.managers))
	for id := range r.managers {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func (r *Registry) Statuses(ctx Context) ([]Status, error) {
	statuses := make([]Status, 0, len(r.managers))
	for _, id := range r.IDs() {
		manager, ok := r.managers[id]
		if !ok {
			continue
		}
		status, err := manager.Status(ctx)
		if err != nil {
			return nil, err
		}
		statuses = append(statuses, *status)
	}
	return statuses, nil
}
