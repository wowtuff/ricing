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

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
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
)

type authDotJson struct {
	Tokens *tokenData `json:"tokens"`
}

type tokenData struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	IdToken      string `json:"id_token"`
}

type oauthResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	IdToken      string `json:"id_token"`
}

// https://pkg.go.dev/github.com/openai/openai-go/v3#section-readme
func Run(ctx context.Context, reg *tools.Registry, userPrompt string) (string, error) {
	token, err := getOrRefreshToken(ctx)
	if err != nil {
		return "", utils.LogError("auth error: %s", err)
	}

	// reads from the env
	client := openai.NewClient(
		option.WithAPIKey(token),
		option.WithBaseURL("https://openrouter.ai/api/v1"), // comment when actual api key to be used
	)
	Tools := buildToolSet(reg)

	messages := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage("You are a smart agent, you provide solutions to user prompts, with no outside knowledge but from the toolset provided to you"),
		openai.UserMessage(userPrompt),
	}

	for {
		completion, err := client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
			Model:    openai.ChatModelGPT4o,
			Messages: messages,
			Tools:    Tools,
			// MaxTokens: openai.Int(512), //if you using free 4o from openrouter uncomment this, can be removed later once we use our api key
		})
		if err != nil {
			return "", utils.LogError("openai error: %s", err)
		}

		choice := completion.Choices[0]

		if len(choice.Message.ToolCalls) == 0 {
			return choice.Message.Content, nil
		}

		messages = append(messages, choice.Message.ToParam())

		for _, tc := range choice.Message.ToolCalls {
			result := executeTool(ctx, reg, tc)
			messages = append(messages, openai.ToolMessage(result, tc.ID))
		}
	}
}

func executeTool(ctx context.Context, reg *tools.Registry, tc openai.ChatCompletionMessageToolCallUnion) string {
	tool, err := reg.Get(tc.Function.Name)
	if err != nil {
		return fmt.Sprintf(`{"error": "%s"}`, err.Error())
	}

	var args map[string]any
	if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
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
func buildToolSet(reg *tools.Registry) []openai.ChatCompletionToolUnionParam {
	specs := reg.List()
	out := make([]openai.ChatCompletionToolUnionParam, 0, len(specs))

	for _, spec := range specs {
		out = append(out, openai.ChatCompletionFunctionTool(openai.FunctionDefinitionParam{
			Name:        spec.Name,
			Description: openai.String(spec.Description),
			Parameters:  openai.FunctionParameters(spec.ParamSchema),
		}))
	}

	return out
}

func getOrRefreshToken(ctx context.Context) (string, error) {
	if token, err := loadCachedToken(); err == nil {
		return token, nil
	}
	return loginFlow(ctx)
}

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

	fmt.Println("opening browser for login")
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

func generateVerifier() string {
	b := make([]byte, 32)
	rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

func generateChallenge(verifier string) string {
	h := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(h[:])
}
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