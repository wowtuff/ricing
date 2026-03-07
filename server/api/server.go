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
	runs      *service.RunService
}

func NewServer(addr string, reg *tools.Registry) *Server {
	providers := service.NewProviderService()
	runs := service.NewRunService(reg, providers)

	s := &Server{
		addr:      addr,
		reg:       reg,
		providers: providers,
		runs:      runs,
	}

	mux := http.NewServeMux()
	s.registerRoutes(mux)

	s.httpSrv = &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	return s
}

func (s *Server) ListenAndServe() error {
	return s.httpSrv.ListenAndServe()
}
