package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/ergonix/auditor/backend/internal/models"
	"github.com/ergonix/auditor/backend/internal/settings"
)

// listWebsites returns the configured website registry plus the default
// audit parameters so the UI can prefill the form. AI/schedule state reflects
// the live (settings-overridden) configuration.
func (s *Server) listWebsites(c *gin.Context) {
	eff := s.settings.Effective()
	c.JSON(http.StatusOK, gin.H{
		"websites": s.cfg.Websites,
		"defaults": gin.H{
			"maxPages":          s.cfg.DefaultParams.MaxPages,
			"maxDepth":          s.cfg.DefaultParams.MaxDepth,
			"concurrency":       s.cfg.DefaultParams.Concurrency,
			"requestTimeoutSec": s.cfg.DefaultParams.RequestTimeoutSec,
			"retryCount":        s.cfg.DefaultParams.RetryCount,
			"useAI":             eff.AIEnabled(),
		},
		"aiEnabled":      eff.AIEnabled(),
		"aiModel":        eff.AIModel,
		"browserEnabled": s.cfg.BrowserEnabled,
		"categories":     models.Categories,
		"schedule": gin.H{
			"enabled":       eff.ScheduleEnabled,
			"intervalHours": int(eff.ScheduleInterval.Hours()),
		},
	})
}

func (s *Server) dashboard(c *gin.Context) {
	data, err := s.store.Dashboard()
	if err != nil {
		s.log.Error("dashboard", "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to compute dashboard"})
		return
	}
	c.JSON(http.StatusOK, data)
}

// settingsResponse is the structured, secret-safe view of runtime settings.
func (s *Server) settingsResponse() gin.H {
	eff := s.settings.Effective()
	sites := eff.ScheduleWebsites
	if len(sites) == 0 {
		sites = s.cfg.Websites
	}
	return gin.H{
		"ai": gin.H{
			"enabled":    eff.AIEnabled(),
			"keySet":     eff.AIKey != "",
			"keyPreview": settings.MaskKey(eff.AIKey),
			"baseUrl":    eff.AIBaseURL,
			"model":      eff.AIModel,
		},
		"schedule": gin.H{
			"enabled":       eff.ScheduleEnabled,
			"intervalHours": int(eff.ScheduleInterval.Hours()),
			"websites":      sites,
		},
		"availableWebsites": s.cfg.Websites,
	}
}

func (s *Server) getSettings(c *gin.Context) {
	c.JSON(http.StatusOK, s.settingsResponse())
}

func (s *Server) putSettings(c *gin.Context) {
	var u settings.Update
	if err := c.ShouldBindJSON(&u); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid settings body: " + err.Error()})
		return
	}
	if err := s.settings.Save(u); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, s.settingsResponse())
}
