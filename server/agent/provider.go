package agent

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"time"
)

type ConnectOptions struct {
	OpenBrowser bool
}

func CachedTokenPath() string {
	return os.ExpandEnv(tokenCachePath)
}

func HasCachedToken() bool {
	_, err := loadCachedToken()
	return err == nil
}

func DeleteCachedToken() error {
	path := CachedTokenPath()
	err := os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// ConnectOpenAI performs the OAuth login flow and caches the token.
// It returns the auth URL (useful for UIs) even if OpenBrowser is true.
func ConnectOpenAI(ctx context.Context, opts ConnectOptions) (string, string, error) {
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

	listener, err := net.Listen("tcp", "127.0.0.1:1455")
	if err != nil {
		return authURL, "", fmt.Errorf("local callback server failed: %w", err)
	}
	defer listener.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("/auth/callback", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code == "" {
			errCh <- fmt.Errorf("no code in callback")
			return
		}
		_, _ = w.Write([]byte("login successful, you can return to the app"))
		codeCh <- code
	})

	server := &http.Server{Handler: mux}
	go func() {
		_ = server.Serve(listener)
	}()

	if opts.OpenBrowser {
		openBrowser(authURL)
	}

	var code string
	select {
	case code = <-codeCh:
	case err := <-errCh:
		_ = server.Shutdown(ctx)
		return authURL, "", err
	case <-ctx.Done():
		_ = server.Shutdown(context.Background())
		return authURL, "", ctx.Err()
	case <-time.After(3 * time.Minute):
		_ = server.Shutdown(context.Background())
		return authURL, "", fmt.Errorf("login timed out")
	}

	_ = server.Shutdown(ctx)
	accessToken, err := exchangeCode(ctx, code, verifier)
	if err != nil {
		return authURL, "", err
	}
	return authURL, accessToken, nil
}
