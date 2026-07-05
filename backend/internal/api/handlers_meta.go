package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/ergonix/auditor/backend/internal/models"
)

// listWebsites returns the configured website registry plus the default
// audit parameters so the UI can prefill the form.
func (s *Server) listWebsites(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"websites": s.cfg.Websites,
		"defaults": gin.H{
			"maxPages":          s.cfg.DefaultParams.MaxPages,
			"maxDepth":          s.cfg.DefaultParams.MaxDepth,
			"concurrency":       s.cfg.DefaultParams.Concurrency,
			"requestTimeoutSec": s.cfg.DefaultParams.RequestTimeoutSec,
			"retryCount":        s.cfg.DefaultParams.RetryCount,
			"useAI":             s.cfg.AIEnabled(),
		},
		"aiEnabled":      s.cfg.AIEnabled(),
		"aiModel":        s.cfg.AIModel,
		"browserEnabled": s.cfg.BrowserEnabled,
		"categories":     models.Categories,
		"schedule": gin.H{
			"enabled":      s.cfg.ScheduleEnabled,
			"intervalHours": int(s.cfg.ScheduleInterval.Hours()),
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

func (s *Server) getSettings(c *gin.Context) {
	settings, err := s.store.GetSettings()
	if err != nil {
		s.log.Error("get settings", "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load settings"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"settings": settings})
}

func (s *Server) putSettings(c *gin.Context) {
	var body struct {
		Settings map[string]string `json:"settings"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.Settings == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "expected {\"settings\": {…}}"})
		return
	}
	if err := s.store.SaveSettings(body.Settings); err != nil {
		s.log.Error("save settings", "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save settings"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"settings": body.Settings})
}
