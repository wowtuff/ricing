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
		{Type: "openrouter", Label: "OpenRouter", Auth: "api_key", Streaming: true},
		{Type: "anthropic", Label: "Anthropic", Auth: "api_key", Streaming: true},
		{Type: "gemini", Label: "Google Gemini", Auth: "api_key", Streaming: true},
		{Type: "ollama", Label: "Ollama", Auth: "none", Streaming: true},
		{Type: "lmstudio", Label: "LM Studio", Auth: "none", Streaming: true},
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
		fmt.Printf("[DEBUG] Provider %s not found\n", id)
		return false
	}
	s.refreshLocked(p)
	fmt.Printf("[DEBUG] Provider %s state: %s\n", id, p.State)
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

type ConnectResult struct {
	Provider  Provider
	AuthURL   string
	PingReply string
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
	if p.Type != "openai_oauth_codex" {
		s.mu.Unlock()
		return Provider{}, "", fmt.Errorf("this provider uses client-side credentials")
	}
	s.busy[id] = true
	p.State = "connecting"
	p.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	s.mu.Unlock()

	// Get auth URL immediately (non-blocking). Register a callback so that
	// when the browser completes the OAuth flow we mark the provider connected.
	authURL, _, err := agent.ConnectOpenAI(ctx, agent.ConnectOptions{
		OpenBrowser: openBrowser,
		NoWait:      true,
		OnConnected: func(accessToken string) {
			s.mu.Lock()
			defer s.mu.Unlock()
			s.busy[id] = false
			if pp, exists := s.providers[id]; exists {
				pp.State = "connected"
				pp.HasCredentials = true
				pp.LastError = ""
				pp.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
				fmt.Printf("[Provider] %s marked as connected\n", id)
			}
		},
	})
	if err != nil {
		s.mu.Lock()
		s.busy[id] = false
		p.LastError = err.Error()
		p.State = "error"
		s.mu.Unlock()
		return *p, authURL, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	p.LastError = ""
	p.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	return *p, authURL, nil
}

// Ping fires a test prompt against a provider and returns the AI's reply.
func (s *ProviderService) Ping(ctx context.Context, id string, model string, apiKey string, url string, reasoningEffort string) (string, error) {
	return agent.Ping(ctx, agent.RunOptions{
		Backend:         id,
		Model:           model,
		ReasoningEffort: reasoningEffort,
		APIKey:          apiKey,
		URL:             url,
	})
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

// BackendForProvider maps a provider ID to the backend string that agent.RunStream understands.
func (s *ProviderService) BackendForProvider(id string) string {
	s.mu.Lock()
	p, ok := s.providers[id]
	s.mu.Unlock()
	if !ok {
		return ""
	}
	switch p.Type {
	case "openai_oauth_codex":
		return "chatgpt"
	case "openai_api_key":
		return "openai"
	case "openrouter":
		return "openrouter"
	case "anthropic":
		return "anthropic"
	case "gemini":
		return "gemini"
	case "ollama":
		return "ollama"
	case "lmstudio":
		return "lmstudio"
	default:
		return p.Type
	}
}

func (s *ProviderService) refreshLocked(p *Provider) {
	switch p.Type {
	case "openai_oauth_codex":
		if agent.HasCachedToken() {
			p.State = "connected"
			p.HasCredentials = true
			return
		}
		if p.State != "connecting" {
			p.State = "disconnected"
		}
		p.HasCredentials = false
	default:
		if p.State != "connecting" {
			p.State = "disconnected"
		}
		p.HasCredentials = false
	}
}
