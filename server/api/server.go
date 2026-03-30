package api

import (
	"net/http"
	"time"

	"github.com/wowtuff/ricing/service"
	"github.com/wowtuff/ricing/tools"
)

type Server struct {
	addr      string
	httpSrv   *http.Server
	reg       *tools.Registry
	providers *service.ProviderService
	sessions  *service.SessionService
	runs      *service.RunService
	uiDir     string
}

func NewServer(addr string, reg *tools.Registry, uiDir string) *Server {
	providers := service.NewProviderService()
	sessions := service.NewSessionService("")
	runs := service.NewRunService(reg, providers, sessions)

	s := &Server{
		addr:      addr,
		reg:       reg,
		providers: providers,
		sessions:  sessions,
		runs:      runs,
		uiDir:     uiDir,
	}

	mux := http.NewServeMux()
	s.registerRoutes(mux)

	if uiDir != "" {
		mux.HandleFunc("/", s.serveUI)
	}

	corsMux := corsMiddleware(mux)

	s.httpSrv = &http.Server{
		Addr:              addr,
		Handler:           corsMux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	return s
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) ListenAndServe() error {
	return s.httpSrv.ListenAndServe()
}

func (s *Server) registerRoutes(mux *http.ServeMux) {
	// core
	mux.HandleFunc("/api/v1/health", s.handleHealth)
	mux.HandleFunc("/api/v1/meta", s.handleMeta)
	mux.HandleFunc("/api/v1/tools", s.handleTools)

	// providers
	mux.HandleFunc("/api/v1/providers/types", s.handleProviderTypes)
	mux.HandleFunc("/api/v1/providers", s.handleProviders)
	mux.HandleFunc("/api/v1/providers/", s.handleProviderActions)

	// runs
	mux.HandleFunc("/api/v1/runs", s.handleRuns)
	mux.HandleFunc("/api/v1/runs/", s.handleRunActions)
	mux.HandleFunc("/api/v1/sessions", s.handleSessions)
	mux.HandleFunc("/api/v1/sessions/", s.handleSessionActions)
	mux.HandleFunc("/api/v1/approvals", s.handleApprovals)
	mux.HandleFunc("/api/v1/approvals/", s.handleApprovalActions)

	// websocket
	mux.HandleFunc("/api/v1/ws", s.handleWS)

	// stubs (reserve shapes)
	mux.HandleFunc("/api/v1/assets", s.handleNotImplemented)
	mux.HandleFunc("/api/v1/assets/", s.handleNotImplemented)
	mux.HandleFunc("/api/v1/snapshots", s.handleNotImplemented)
	mux.HandleFunc("/api/v1/snapshots/", s.handleNotImplemented)
	mux.HandleFunc("/api/v1/queue", s.handleNotImplemented)
	mux.HandleFunc("/api/v1/system", s.handleNotImplemented)
	mux.HandleFunc("/api/v1/skills", s.handleNotImplemented)
	mux.HandleFunc("/api/v1/wallpapers", s.handleNotImplemented)
}

func (s *Server) serveUI(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/" || r.URL.Path == "/index.html" {
		http.ServeFile(w, r, s.uiDir+"/index.html")
		return
	}
	http.FileServer(http.Dir(s.uiDir)).ServeHTTP(w, r)
}
