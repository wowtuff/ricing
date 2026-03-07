package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/wowtuff/ricing/agent"
)

type ProviderType struct {
	Type      string `json:"type"`
	Label     string `json:"label"`
	Auth      string `json:"auth"`
	Streaming bool   `json:"streaming"`
}

type Provider struct {
	ID             string `json:"id"`
	Type           string `json:"type"`
	Label          string `json:"label"`
	State          string `json:"state"`
	HasCredentials bool   `json:"has_credentials"`
	LastError      string `json:"last_error,omitempty"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`
}

type ProviderService struct {
	mu        sync.Mutex
	providers map[string]*Provider
	defaultID string
	busy      map[string]bool
}

func NewProviderService() *ProviderService {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	def := &Provider{
		ID:        "prov_openai_oauth_codex",
		Type:      "openai_oauth_codex",
		Label:     "OpenAI (OAuth/Codex)",
		CreatedAt: now,
		UpdatedAt: now,
	}
	s := &ProviderService{
		providers: map[string]*Provider{def.ID: def},
		defaultID: def.ID,
		busy:      make(map[string]bool),
	}
	s.refreshLocked(def)
	return s
}

func (s *ProviderService) DefaultID() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.defaultID
}

func (s *ProviderService) Types() []ProviderType {
	return []ProviderType{
		{Type: "openai_oauth_codex", Label: "OpenAI OAuth (Codex WS)", Auth: "oauth_pkce_local_callback", Streaming: true},
		{Type: "openai_api_key", Label: "OpenAI API Key", Auth: "api_key", Streaming: true},
		{Type: "ollama", Label: "Ollama", Auth: "none", Streaming: true},
	}
}

func (s *ProviderService) Create(typ, label string) (Provider, error) {
	typ = strings.TrimSpace(typ)
	label = strings.TrimSpace(label)
	if typ == "" {
		return Provider{}, fmt.Errorf("type is required")
	}
	known := false
	for _, t := range s.Types() {
		if t.Type == typ {
			known = true
			break
		}
	}
	if !known {
		return Provider{}, fmt.Errorf("unknown provider type: %s", typ)
	}
	if label == "" {
		label = typ
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	id := newID("prov")
	now := time.Now().UTC().Format(time.RFC3339Nano)
	p := &Provider{ID: id, Type: typ, Label: label, CreatedAt: now, UpdatedAt: now}
	s.refreshLocked(p)
	s.providers[id] = p
	return *p, nil
}

func (s *ProviderService) List() []Provider {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]Provider, 0, len(s.providers))
	for _, p := range s.providers {
		s.refreshLocked(p)
		out = append(out, *p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (s *ProviderService) IsConnected(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.providers[id]
	if !ok {
		return false
	}
	s.refreshLocked(p)
	return p.State == "connected"
}

func (s *ProviderService) Get(id string) (Provider, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.providers[id]
	if !ok {
		return Provider{}, false
	}
	s.refreshLocked(p)
	return *p, true
}

func (s *ProviderService) Connect(ctx context.Context, id string, openBrowser bool) (Provider, string, error) {
	s.mu.Lock()
	p, ok := s.providers[id]
	if !ok {
		s.mu.Unlock()
		return Provider{}, "", fmt.Errorf("provider not found")
	}
	if s.busy[id] {
		s.mu.Unlock()
		return Provider{}, "", fmt.Errorf("provider is busy")
	}
	s.busy[id] = true
	p.State = "connecting"
	p.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	s.mu.Unlock()

	authURL, _, err := agent.ConnectOpenAI(ctx, agent.ConnectOptions{OpenBrowser: openBrowser})

	s.mu.Lock()
	defer s.mu.Unlock()
	defer func() { s.busy[id] = false }()
	if err != nil {
		p.LastError = err.Error()
		p.State = "error"
		p.HasCredentials = false
		p.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
		return *p, authURL, err
	}
	// refresh state based on cache
	s.refreshLocked(p)
	p.LastError = ""
	p.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	return *p, authURL, nil
}

func (s *ProviderService) Disconnect(id string) (Provider, error) {
	s.mu.Lock()
	p, ok := s.providers[id]
	if !ok {
		s.mu.Unlock()
		return Provider{}, fmt.Errorf("provider not found")
	}
	s.mu.Unlock()

	if err := agent.DeleteCachedToken(); err != nil {
		return Provider{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.refreshLocked(p)
	p.LastError = ""
	p.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	return *p, nil
}

func (s *ProviderService) refreshLocked(p *Provider) {
	connected := agent.HasCachedToken()
	if connected {
		p.State = "connected"
		p.HasCredentials = true
		return
	}
	if p.State != "connecting" {
		p.State = "disconnected"
	}
	p.HasCredentials = false
}
