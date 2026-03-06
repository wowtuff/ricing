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
	"runtime"
	"strings"

	"github.com/gorilla/websocket"
	"github.com/wowtuff/ricing/tools"
	"github.com/wowtuff/ricing/utils"
)

const (
	clientID       = "app_EMoamEEZ73f0CkXaXp7hrann"
	openaiAuthURL  = "https://auth.openai.com/oauth/authorize"
	openaiTokenURL = "https://auth.openai.com/oauth/token"
	redirectURI    = "http://localhost:1455/auth/callback"
	scopes         = "openid profile email offline_access api.connectors.read api.connectors.invoke"
	tokenCachePath = "$HOME/.codex/auth.json"
	wsEndpoint     = "wss://chatgpt.com/backend-api/codex/responses"
)

// stores tokens in the openai way
type authDotJson struct {
	Tokens *tokenData `json:"tokens"`
}

// token payload
type tokenData struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	IdToken      string `json:"id_token"`
}

// what we get back from the oauth token endpoint
type oauthResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	IdToken      string `json:"id_token"`
}

// the request we send over websocket
type wsRequest struct {
	Type               string      `json:"type"`
	Model              string      `json:"model"`
	Instructions       string      `json:"instructions"`
	PreviousResponseID *string     `json:"previous_response_id,omitempty"`
	Input              []inputItem `json:"input"`
	Tools              []any       `json:"tools"`
	ToolChoice         string      `json:"tool_choice"`
	ParallelToolCalls  bool        `json:"parallel_tool_calls"`
	Store              bool        `json:"store"`
	Stream             bool        `json:"stream"`
	Include            []string    `json:"include"`
}

// input
type inputItem struct {
	Type    string         `json:"type"`
	Role    string         `json:"role,omitempty"`
	Content []inputContent `json:"content,omitempty"`
	CallID  string         `json:"call_id,omitempty"`
	Output  string         `json:"output,omitempty"`
}

// the actual text content inside an inputItem
type inputContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// events coming in from the websocket
type wsEvent struct {
	Type     string          `json:"type"`
	Delta    string          `json:"delta,omitempty"`
	Item     json.RawMessage `json:"item,omitempty"`
	Response json.RawMessage `json:"response,omitempty"`
}

// represents a function call that the model wants to make
type functionCallItem struct {
	Type      string `json:"type"`
	ID        string `json:"id"`
	CallID    string `json:"call_id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// the payload when a response is done
type responseCompleted struct {
	ID string `json:"id"`
}

// the main event
func Run(ctx context.Context, reg *tools.Registry, userPrompt string) (string, error) {
	// reads from the env
	token, err := getOrRefreshToken(ctx)
	if err != nil {
		return "", utils.LogError("auth error: %s", err)
	}

	originator := os.Getenv("CODEX_ORIGINATOR")
	if originator == "" {
		originator = "codex_cli_rs"
	}

	headers := http.Header{}
	headers.Set("Authorization", "Bearer "+token)
	headers.Set("Origin", "https://chatgpt.com")
	headers.Set("User-Agent", "codex-cli-rs/0.1.0")
	headers.Set("originator", originator)
	if residency := os.Getenv("REQUIREMENTS_RESIDENCY"); residency != "" {
		headers.Set("x-openai-internal-codex-residency", residency)
	}
	// debugging logs
	fmt.Println("connecting to websocket...")
	conn, resp, err := websocket.DefaultDialer.DialContext(ctx, wsEndpoint, headers)
	if err != nil {
		if resp != nil {
			return "", utils.LogError("websocket dial error: %s | status: %d", err, resp.StatusCode)
		}
		return "", utils.LogError("websocket dial error: %s", err)
	}
	fmt.Println("connected!")
	defer conn.Close()

	toolSpecs := buildToolSpecs(reg)
	var previousResponseID *string
	input := []inputItem{
		{
			Type:    "message",
			Role:    "user",
			Content: []inputContent{{Type: "input_text", Text: userPrompt}},
		},
	}
	// agent request
	for {
		req := wsRequest{
			Type:               "response.create",
			Model:              "gpt-5.2-codex",
			Instructions:       "You are a smart agent, you provide solutions to user prompts, with no outside knowledge but from the toolset provided to you",
			PreviousResponseID: previousResponseID,
			Input:              input,
			Tools:              toolSpecs,
			ToolChoice:         "auto",
			ParallelToolCalls:  true,
			Store:              false,
			Stream:             true,
			Include:            []string{},
		}

		if err := conn.WriteJSON(req); err != nil {
			return "", utils.LogError("websocket write error: %s", err)
		}
		fmt.Println("request sent, waiting for response...")
		var outputText strings.Builder
		var pendingToolCalls []functionCallItem
		var responseID string
		done := false

		for !done {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				return "", utils.LogError("websocket read error: %s", err)
			}
			fmt.Printf("raw event: %s\n", string(msg))
			var event wsEvent
			if err := json.Unmarshal(msg, &event); err != nil {
				continue
			}

			switch event.Type {
			case "response.output_text.delta":
				outputText.WriteString(event.Delta)

			case "response.output_item.done":
				var item functionCallItem
				if err := json.Unmarshal(event.Item, &item); err == nil && item.Type == "function_call" {
					pendingToolCalls = append(pendingToolCalls, item)
				}

			case "response.completed":
				var resp responseCompleted
				if err := json.Unmarshal(event.Response, &resp); err == nil {
					responseID = resp.ID
				}
				done = true

			case "response.failed", "response.incomplete":
				return "", utils.LogError("response error event: %s", event.Type)
			}
		}

		if len(pendingToolCalls) == 0 {
			return outputText.String(), nil
		}

		input = []inputItem{}
		for _, tc := range pendingToolCalls {
			result := executeTool(ctx, reg, tc)
			input = append(input, inputItem{
				Type:   "function_call_output",
				CallID: tc.CallID,
				Output: result,
			})
		}
		previousResponseID = &responseID
	}
}

// finds the tool, parses args, runs it, returns the result as json string
func executeTool(ctx context.Context, reg *tools.Registry, tc functionCallItem) string {
	tool, err := reg.Get(tc.Name)
	if err != nil {
		return fmt.Sprintf(`{"error": "%s"}`, err.Error())
	}

	var args map[string]any
	if err := json.Unmarshal([]byte(tc.Arguments), &args); err != nil {
		return fmt.Sprintf(`{"error": "bad arguments: %s"}`, err.Error())
	}

	result, err := tool.Run(ctx, args)
	if err != nil {
		return fmt.Sprintf(`{"error": "%s"}`, err.Error())
	}

	out, _ := json.Marshal(result)
	return string(out)
}

// go openai sdk's version of JSON list of tools
func buildToolSpecs(reg *tools.Registry) []any {
	specs := reg.List()
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

// tries to use cached token first, falls back to full login flow if needed
func getOrRefreshToken(ctx context.Context) (string, error) {
	if token, err := loadCachedToken(); err == nil {
		return token, nil
	}
	return loginFlow(ctx)
}

// the full oauth flow
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

// swaps the auth code for actual access token
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

// generates a random string for pkce
func generateVerifier() string {
	b := make([]byte, 32)
	rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

// hashes the verifier to create the challenge
func generateChallenge(verifier string) string {
	h := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(h[:])
}

// saves oauth token
func cacheToken(oauthResp oauthResponse) error {
	path := os.ExpandEnv(tokenCachePath)
	if err := os.MkdirAll(strings.TrimSuffix(path, "/auth.json"), 0700); err != nil {
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

// loads the saved oauth token
func loadCachedToken() (string, error) {
	path := os.ExpandEnv(tokenCachePath)
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

// what do i even explain here, literally does what the func says
func openBrowser(url string) {
	switch runtime.GOOS {
	case "windows":
		exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		exec.Command("open", url).Start()
	default:
		exec.Command("xdg-open", url).Start()
	}
}
