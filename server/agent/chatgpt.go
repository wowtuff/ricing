package agent

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/gorilla/websocket"
	"github.com/wowtuff/ricing/tools"
)

const (
	clientID       = "app_EMoamEEZ73f0CkXaXp7hrann"
	openaiAuthURL  = "https://auth.openai.com/oauth/authorize"
	openaiTokenURL = "https://auth.openai.com/oauth/token"
	redirectURI    = "http://localhost:1455/auth/callback"
	scopes         = "openid profile email offline_access api.connectors.read api.connectors.invoke"
	wsEndpoint     = "wss://chatgpt.com/backend-api/codex/responses"
)

// returns the path where we stash the oauth tokens (~/.codex/auth.json)
func tokenCacheFile() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, ".codex", "auth.json"), nil
}

// mirrors the structure of ~/.codex/auth.json on disk
type authDotJson struct {
	Tokens *tokenData `json:"tokens"`
}

// holds the three tokens returned by the openai oauth endpoint
type tokenData struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	IdToken      string `json:"id_token"`
}

// raw json body from the token endpoint
type oauthResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	IdToken      string `json:"id_token"`
}

// the payload sent over the websocket to kick off a new response
type wsRequest struct {
	Type               string       `json:"type"`
	Model              string       `json:"model"`
	Instructions       string       `json:"instructions"`
	Reasoning          *wsReasoning `json:"reasoning,omitempty"`
	PreviousResponseID *string      `json:"previous_response_id,omitempty"`
	Input              []wsInput    `json:"input"`
	Tools              []any        `json:"tools"`
	ToolChoice         string       `json:"tool_choice"`
	ParallelToolCalls  bool         `json:"parallel_tool_calls"`
	Store              bool         `json:"store"`
	Stream             bool         `json:"stream"`
	Include            []string     `json:"include"`
}

type wsReasoning struct {
	Effort string `json:"effort"`
}

// one item in the input array — either a user message or a tool result
type wsInput struct {
	Type    string      `json:"type"`
	Role    string      `json:"role,omitempty"`
	Content []wsContent `json:"content,omitempty"`
	CallID  string      `json:"call_id,omitempty"`
	Output  string      `json:"output,omitempty"`
}

// typed text block inside a message
type wsContent struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	ImageURL string `json:"image_url,omitempty"`
	Detail   string `json:"detail,omitempty"`
}

// any event pushed from the server over the websocket
type wsEvent struct {
	Type     string          `json:"type"`
	Delta    string          `json:"delta,omitempty"`
	Item     json.RawMessage `json:"item,omitempty"`
	Response json.RawMessage `json:"response,omitempty"`
}

// the parsed shape of a function_call output item.
type wsFunctionCallItem struct {
	Type      string `json:"type"`
	ID        string `json:"id"`
	CallID    string `json:"call_id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// the response id when a turn finishes
type wsResponseCompleted struct {
	ID string `json:"id"`
}

// talks to the codex websocket api using a cached oauth token
type chatGPTBackend struct {
	token           string
	model           string
	reasoningEffort string
}

// loads the cached token and returns a ready-to-use chatGPTBackend
func chatGPT(model, reasoningEffort string) (*chatGPTBackend, error) {
	token, err := getOrRefreshToken(context.Background())
	if err != nil {
		return nil, fmt.Errorf("auth error: %w", err)
	}
	if model == "" {
		model = "gpt-5.2-codex"
	}
	return &chatGPTBackend{
		token:           token,
		model:           model,
		reasoningEffort: normalizeReasoningEffort(reasoningEffort),
	}, nil
}

// complete dials the codex websocket, sends the conversation, and streamsevents until the response is complete or an error occurs
func (b *chatGPTBackend) Complete(ctx context.Context, messages []Message, specs []tools.ToolSpec) (*CompletionResult, error) {
	originator := os.Getenv("CODEX_ORIGINATOR")
	if originator == "" {
		originator = "codex_cli_rs"
	}

	accountID := extractAccountID(b.token)
	sessionID := generateVerifier()

	headers := http.Header{}
	headers.Set("Authorization", "Bearer "+b.token)
	headers.Set("Origin", "https://chatgpt.com")
	headers.Set("User-Agent", "codex-cli-rs/0.1.0")
	headers.Set("originator", originator)
	headers.Set("openai-beta", "responses_websockets=2026-02-06")
	headers.Set("version", "0.111.0")
	headers.Set("session_id", sessionID)
	headers.Set("x-codex-turn-metadata", `{"turn_id":"","sandbox":"none"}`)
	if accountID != "" {
		headers.Set("chatgpt-account-id", accountID)
	}
	if residency := os.Getenv("REQUIREMENTS_RESIDENCY"); residency != "" {
		headers.Set("x-openai-internal-codex-residency", residency)
	}

	conn, _, err := websocket.DefaultDialer.DialContext(ctx, wsEndpoint, headers)
	if err != nil {
		return nil, fmt.Errorf("websocket dial error: %w", err)
	}
	defer conn.Close()

	toolSpecs := buildWSToolSpecs(specs)
	input := messagesToWSInput(messages)

	req := wsRequest{
		Type:              "response.create",
		Model:             b.model,
		Instructions:      defaultInstructions(messageSetHasImages(messages)),
		Input:             input,
		Tools:             toolSpecs,
		ToolChoice:        "auto",
		ParallelToolCalls: true,
		Store:             false,
		Stream:            true,
		Include:           []string{},
	}
	if b.reasoningEffort != "" && reasoningEffortSupported(b.model) {
		req.Reasoning = &wsReasoning{Effort: b.reasoningEffort}
	}

	if err := conn.WriteJSON(req); err != nil {
		return nil, fmt.Errorf("websocket write error: %w", err)
	}

	var outputText strings.Builder
	var toolCalls []ToolCall

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			return nil, fmt.Errorf("websocket read error: %w", err)
		}

		var event wsEvent
		if err := json.Unmarshal(msg, &event); err != nil {
			continue
		}

		switch event.Type {
		case "response.output_text.delta":
			outputText.WriteString(event.Delta)

		case "response.output_item.done":
			var item wsFunctionCallItem
			if err := json.Unmarshal(event.Item, &item); err == nil && item.Type == "function_call" {
				toolCalls = append(toolCalls, ToolCall{
					ID:        item.CallID,
					Name:      item.Name,
					Arguments: item.Arguments,
				})
			}

		case "response.completed":
			return &CompletionResult{
				Content:   outputText.String(),
				ToolCalls: toolCalls,
			}, nil

		case "response.failed", "response.incomplete":
			return nil, fmt.Errorf("response error: %s", event.Type)
		}
	}
}

// converts internal messages to the websocket input format.
func messagesToWSInput(messages []Message) []wsInput {
	var input []wsInput
	for _, m := range messages {
		switch m.Role {
		case "user":
			content := []wsContent{}
			if m.Content != "" {
				content = append(content, wsContent{Type: "input_text", Text: m.Content})
			}
			for _, image := range m.Images {
				content = append(content, wsContent{
					Type:     "input_image",
					ImageURL: image.URL,
					Detail:   image.Detail,
				})
			}
			input = append(input, wsInput{
				Type:    "message",
				Role:    "user",
				Content: content,
			})
		case "tool":
			input = append(input, wsInput{
				Type:   "function_call_output",
				CallID: m.ToolCallID,
				Output: m.Content,
			})
		}
	}
	return input
}

// converts our generic tool specs into the shape the ws api expects.
func buildWSToolSpecs(specs []tools.ToolSpec) []any {
	out := make([]any, 0, len(specs))
	for _, spec := range specs {
		out = append(out, map[string]any{
			"type":        "function",
			"name":        spec.Name,
			"description": spec.Description,
			"parameters":  spec.ParamSchema,
		})
	}
	return out
}

// getorrefreshtoken returns a valid access token from cache if possible, otherwise kicks off the browser login flow
func getOrRefreshToken(ctx context.Context) (string, error) {
	if token, err := loadCachedToken(); err == nil {
		return token, nil
	}
	return loginFlow(ctx)
}

// loginflow starts a local http server, opens the browser to openai's oauth page, waits for the redirect with the code, then exchanges it for tokens
func loginFlow(ctx context.Context) (string, error) {
	verifier := generateVerifier()
	challenge := generateChallenge(verifier)
	state := generateVerifier()

	authURL := fmt.Sprintf(
		"%s?response_type=code&client_id=%s&redirect_uri=%s&scope=%s&code_challenge=%s&code_challenge_method=S256&id_token_add_organizations=true&codex_cli_simplified_flow=true&state=%s",
		openaiAuthURL,
		clientID,
		url.QueryEscape(redirectURI),
		url.QueryEscape(scopes),
		challenge,
		state,
	)

	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)
	listener, err := net.Listen("tcp", ":1455")
	if err != nil {
		return "", fmt.Errorf("local server failed: %w", err)
	}
	server := &http.Server{}
	http.HandleFunc("/auth/callback", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code == "" {
			errCh <- fmt.Errorf("no code in callback")
			return
		}
		fmt.Fprintf(w, "<html><body><p>login successful, you can return to agent now</p></body></html>")
		codeCh <- code
	})
	go func() {
		server.Serve(listener)
	}()

	fmt.Println("opening browser for login...")
	fmt.Println(authURL)
	openBrowser(authURL)

	var code string
	select {
	case code = <-codeCh:
	case err := <-errCh:
		return "", err
	case <-ctx.Done():
		return "", ctx.Err()
	}

	server.Shutdown(ctx)
	return exchangeCode(ctx, code, verifier)
}

// trades the authorization code + pkce verifier for an access token, then writes it to the cache file so future runs don't need the browser
func exchangeCode(ctx context.Context, code, verifier string) (string, error) {
	resp, err := http.PostForm(openaiTokenURL, url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {clientID},
		"code":          {code},
		"redirect_uri":  {redirectURI},
		"code_verifier": {verifier},
	})
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var oauthResp oauthResponse
	if err := json.NewDecoder(resp.Body).Decode(&oauthResp); err != nil {
		return "", fmt.Errorf("failed to decode token response: %w", err)
	}

	if err := cacheToken(oauthResp); err != nil {
		return "", fmt.Errorf("failed to cache token: %w", err)
	}

	return oauthResp.AccessToken, nil
}

// writes tokens to ~/.codex/auth.json, creating the directory if needed
func cacheToken(oauthResp oauthResponse) error {
	path, err := tokenCacheFile()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	auth := authDotJson{
		Tokens: &tokenData{
			AccessToken:  oauthResp.AccessToken,
			RefreshToken: oauthResp.RefreshToken,
			IdToken:      oauthResp.IdToken,
		},
	}
	data, err := json.MarshalIndent(auth, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

// reads the access token from disk, returning an error if it's missing or empty (which means the user needs to log in again).
func loadCachedToken() (string, error) {
	path, err := tokenCacheFile()
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	var auth authDotJson
	if err := json.Unmarshal(data, &auth); err != nil {
		return "", err
	}
	if auth.Tokens == nil || auth.Tokens.AccessToken == "" {
		return "", fmt.Errorf("no access token found")
	}
	return auth.Tokens.AccessToken, nil
}

// makes a random pkce code verifier
func generateVerifier() string {
	b := make([]byte, 32)
	rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

// hashes the verifier to produce the pkce code challenge
func generateChallenge(verifier string) string {
	h := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(h[:])
}

// pulls the chatgpt_account_id out of the jwt payload, returning empty string if the token is malformed or the claim is absent
func extractAccountID(token string) string {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return ""
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return ""
	}
	var claims struct {
		Auth struct {
			ChatGPTAccountID string `json:"chatgpt_account_id"`
		} `json:"https://api.openai.com/auth"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return ""
	}
	return claims.Auth.ChatGPTAccountID
}

// launches the given url in the system's default browser
func openBrowser(u string) {
	switch runtime.GOOS {
	case "windows":
		exec.Command("rundll32", "url.dll,FileProtocolHandler", u).Start()
	case "darwin":
		exec.Command("open", u).Start()
	default:
		exec.Command("xdg-open", u).Start()
	}
}
