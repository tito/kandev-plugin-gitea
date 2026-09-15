package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/kandev/kandev/pkg/pluginsdk"
)

type fakeHost struct {
	pluginsdk.UnimplementedHostData
	mu      sync.Mutex
	state   map[string]map[string]any
	secrets map[string]string
	repos   []pluginsdk.Repository
	tasks   []pluginsdk.Task
}

func newFakeHost() *fakeHost {
	return &fakeHost{
		state:   make(map[string]map[string]any),
		secrets: make(map[string]string),
	}
}

func stateKey(scope, scopeID, key string) string {
	return scope + "/" + scopeID + "/" + key
}

func (h *fakeHost) GetState(_ context.Context, scope, scopeID, key string) (map[string]any, bool, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	v, ok := h.state[stateKey(scope, scopeID, key)]
	return v, ok, nil
}

func (h *fakeHost) SetState(_ context.Context, scope, scopeID, key string, value map[string]any) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.state[stateKey(scope, scopeID, key)] = jsonRoundTrip(value)
	return nil
}

func (h *fakeHost) DeleteState(_ context.Context, scope, scopeID, key string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.state, stateKey(scope, scopeID, key))
	return nil
}

func (h *fakeHost) ListState(_ context.Context, scope, scopeID string) ([]pluginsdk.StateEntry, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	prefix := scope + "/" + scopeID + "/"
	var entries []pluginsdk.StateEntry
	for k, v := range h.state {
		if strings.HasPrefix(k, prefix) {
			entries = append(entries, pluginsdk.StateEntry{Key: strings.TrimPrefix(k, prefix), Value: v})
		}
	}
	return entries, nil
}

func (h *fakeHost) GetConfig(context.Context) (map[string]any, error) {
	return map[string]any{}, nil
}

func (h *fakeHost) RevealSecret(context.Context, string) (string, error) { return "", nil }

func (h *fakeHost) GetSecret(_ context.Context, key string) (string, bool, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	v, ok := h.secrets[key]
	return v, ok, nil
}

func (h *fakeHost) SetSecret(_ context.Context, key, value string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.secrets[key] = value
	return nil
}

func (h *fakeHost) DeleteSecret(_ context.Context, key string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.secrets, key)
	return nil
}

func (h *fakeHost) EmitEvent(context.Context, string, map[string]any) error { return nil }

func (h *fakeHost) Repositories() pluginsdk.RepositoryReader {
	return &fakeRepoReader{repos: h.repos}
}

func (h *fakeHost) Tasks() pluginsdk.TaskReader {
	return &fakeTaskReader{tasks: h.tasks}
}

func (h *fakeHost) setState(scope, scopeID, key string, value map[string]any) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.state[stateKey(scope, scopeID, key)] = jsonRoundTrip(value)
}

func (h *fakeHost) setSecret(key, value string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.secrets[key] = value
}

type fakeRepoReader struct {
	repos []pluginsdk.Repository
}

func (r *fakeRepoReader) List(_ context.Context, _ string, _ pluginsdk.Page) ([]pluginsdk.Repository, *pluginsdk.PageInfo, error) {
	return r.repos, nil, nil
}

func jsonRoundTrip(v map[string]any) map[string]any {
	data, _ := json.Marshal(v)
	var out map[string]any
	json.Unmarshal(data, &out)
	return out
}

type fakeTaskReader struct {
	tasks []pluginsdk.Task
}

func (r *fakeTaskReader) List(_ context.Context, _ pluginsdk.TaskFilter, _ pluginsdk.Page) ([]pluginsdk.Task, *pluginsdk.PageInfo, error) {
	return r.tasks, nil, nil
}

func (r *fakeTaskReader) Get(_ context.Context, id string) (*pluginsdk.Task, error) {
	for _, t := range r.tasks {
		if t.ID == id {
			return &t, nil
		}
	}
	return nil, fmt.Errorf("not found")
}

func (r *fakeTaskReader) Create(context.Context, pluginsdk.CreateTaskInput) (*pluginsdk.Task, error) {
	return nil, fmt.Errorf("not implemented")
}

func (r *fakeTaskReader) Update(context.Context, pluginsdk.UpdateTaskInput) (*pluginsdk.Task, error) {
	return nil, fmt.Errorf("not implemented")
}

func (r *fakeTaskReader) Move(context.Context, pluginsdk.MoveTaskInput) (*pluginsdk.MoveTaskOutcome, error) {
	return nil, fmt.Errorf("not implemented")
}

var _ pluginsdk.Host = (*fakeHost)(nil)
