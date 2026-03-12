package agent

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"sync"
	"time"
)

// connectoptions controls how ConnectOpenAI behaves

// set NoWait to return the auth url immediately instead of blocking
type ConnectOptions struct {
	OpenBrowser bool
	NoWait      bool
	OnConnected func(accessToken string)
}

// cachedtokenpath returns the full path of the token cache file, or "" on error
func CachedTokenPath() string {
	path, err := tokenCacheFile()
	if err != nil {
		return ""
	}
	return path
}

// reports whether a usable access token exists on disk
func HasCachedToken() bool {
	_, err := loadCachedToken()
	return err == nil
}

// removes the token file used by the disconnect flow
func DeleteCachedToken() error {
	path := CachedTokenPath()
	if path == "" {
		return nil
	}
	err := os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

var (
	serverMU      sync.Mutex
	serverRunning bool
)

// startoauthserver launches the callback http server on :1455 exactly once
// subsequent calls are no-ops thanks to the serverRunning guard
func startOAuthServer() {
	serverMU.Lock()
	if serverRunning {
		serverMU.Unlock()
		return
	}
	serverRunning = true
	serverMU.Unlock()

	go func() {
		listener, err := net.Listen("tcp", ":1455")
		if err != nil {
			fmt.Printf("Failed to start OAuth server: %v\n", err)
			return
		}
		fmt.Printf("OAuth server listening on :1455\n")

		mux := http.NewServeMux()
		mux.HandleFunc("/auth/callback", handleOAuthCallback)

		server := &http.Server{Handler: mux}
		server.Serve(listener)
	}()

	time.Sleep(100 * time.Millisecond)
}

var (
	// oauthCodeChan receives the raw authorization code from the browser callback
	oauthCodeChan   = make(chan string, 1)
	oauthErrChan    = make(chan error, 1)
	currentVerifier string
	verifierMU      sync.Mutex
)

// handleOAuthCallback receives the ?code= redirect, validates it, and forwards the code to ConnectOpenAI via oauthCodeChan. The token exchange happens there so that the caller (ConnectOpenAI) controls caching and signalling
func handleOAuthCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	fmt.Printf("[OAuth] Callback received! code length: %d\n", len(code))
	if code == "" {
		errMsg := r.URL.Query().Get("error")
		fmt.Printf("[OAuth] Callback error: %s\n", errMsg)
		select {
		case oauthErrChan <- fmt.Errorf("oauth error: %s", errMsg):
		default:
		}
		w.Write([]byte("login failed"))
		return
	}

	fmt.Println("[OAuth] Forwarding code to token exchange...")
	select {
	case oauthCodeChan <- code:
	default:
		// channel already has a value (shouldn't normally happen)
		w.Write([]byte("duplicate callback ignored"))
		return
	}

	w.Write([]byte("<html><body><p>Login successful! You can close this window and return to the app.</p></body></html>"))
}

// connectOpenAI performs OAuth login.

// when NoWait is true it returns the auth URL immediately and, if OnConnected is set, calls it in a background goroutine once the token is cached. when NoWait is false it blocks until the token is received or the context expires
func ConnectOpenAI(ctx context.Context, opts ConnectOptions) (string, string, error) {
	startOAuthServer()

	verifier := generateVerifier()
	verifierMU.Lock()
	currentVerifier = verifier
	verifierMU.Unlock()

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

	if opts.OpenBrowser {
		openBrowser(authURL)
	}

	if opts.NoWait {
		//drain any stale values so the next real callback gets through
		for {
			select {
			case <-oauthCodeChan:
			case <-oauthErrChan:
			default:
				goto drained
			}
		}
	drained:

		if opts.OnConnected != nil {
			go func() {
				select {
				case code := <-oauthCodeChan:
					verifierMU.Lock()
					v := currentVerifier
					verifierMU.Unlock()
					accessToken, err := exchangeCode(context.Background(), code, v)
					if err != nil {
						fmt.Printf("[OAuth] Background token exchange failed: %v\n", err)
						return
					}
					fmt.Printf("[OAuth] Background token exchange successful!\n")
					opts.OnConnected(accessToken)
				case err := <-oauthErrChan:
					fmt.Printf("[OAuth] Background OAuth error: %v\n", err)
				case <-time.After(3 * time.Minute):
					fmt.Printf("[OAuth] Background wait timed out\n")
				}
			}()
		}

		return authURL, "", nil
	}

	// blocking mode: wait for the code, exchange it, return the token
	select {
	case code := <-oauthCodeChan:
		verifierMU.Lock()
		v := currentVerifier
		verifierMU.Unlock()
		accessToken, err := exchangeCode(ctx, code, v)
		if err != nil {
			return authURL, "", err
		}
		return authURL, accessToken, nil
	case err := <-oauthErrChan:
		return authURL, "", err
	case <-ctx.Done():
		return authURL, "", ctx.Err()
	case <-time.After(3 * time.Minute):
		return authURL, "", fmt.Errorf("login timed out")
	}
}
