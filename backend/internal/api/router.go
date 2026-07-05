// Package api exposes the REST interface consumed by the frontend.
package api

import (
	"log/slog"

	"github.com/gin-gonic/gin"

	"github.com/ergonix/auditor/backend/internal/audit"
	"github.com/ergonix/auditor/backend/internal/config"
	"github.com/ergonix/auditor/backend/internal/database"
	"github.com/ergonix/auditor/backend/internal/settings"
)

// Server bundles the API dependencies.
type Server struct {
	store        database.Store
	orchestrator *audit.Orchestrator
	settings     *settings.Manager
	cfg          *config.Config
	log          *slog.Logger
}

// NewServer builds the API server.
func NewServer(store database.Store, orch *audit.Orchestrator,
	sm *settings.Manager, cfg *config.Config, log *slog.Logger) *Server {
	return &Server{store: store, orchestrator: orch, settings: sm, cfg: cfg, log: log}
}

// Router constructs the Gin engine with all routes registered.
func (s *Server) Router() *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery(), s.requestLogger(), s.cors())

	r.GET("/healthz", func(c *gin.Context) { c.JSON(200, gin.H{"status": "ok"}) })

	api := r.Group("/api")
	{
		api.POST("/audits", s.createAudit)
		api.GET("/audits", s.listAudits)
		api.GET("/audits/:id", s.getAudit)
		api.DELETE("/audits/:id", s.deleteAudit)
		api.POST("/audits/:id/cancel", s.cancelAudit)
		api.GET("/audits/:id/issues", s.listIssues)
		api.GET("/audits/:id/pages", s.listPages)
		api.GET("/audits/:id/export/:format", s.exportAudit)

		api.GET("/websites", s.listWebsites)
		api.GET("/dashboard", s.dashboard)
		api.GET("/settings", s.getSettings)
		api.PUT("/settings", s.putSettings)
	}
	return r
}
