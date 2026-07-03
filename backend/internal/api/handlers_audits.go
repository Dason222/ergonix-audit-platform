package api

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/ergonix/auditor/backend/internal/database"
	"github.com/ergonix/auditor/backend/internal/models"
	"github.com/ergonix/auditor/backend/internal/report"
)

// createAudit validates the request, persists a pending audit, and starts
// the pipeline asynchronously. Responds 201 with the audit immediately.
func (s *Server) createAudit(c *gin.Context) {
	var params models.AuditParams
	if err := c.ShouldBindJSON(&params); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body: " + err.Error()})
		return
	}
	if len(params.Websites) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "websites must not be empty"})
		return
	}
	seen := map[string]bool{}
	for i, w := range params.Websites {
		w = strings.TrimSpace(w)
		if !strings.HasPrefix(w, "http://") && !strings.HasPrefix(w, "https://") {
			w = "https://" + w
		}
		u, err := url.Parse(w)
		if err != nil || u.Host == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid website URL: %q", params.Websites[i])})
			return
		}
		if seen[w] {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("duplicate website: %q", w)})
			return
		}
		seen[w] = true
		params.Websites[i] = w
	}
	params.Normalize()

	a := &models.Audit{
		Status:    models.AuditPending,
		Stage:     models.StageQueued,
		Params:    params,
		Stats:     models.NewAuditStats(),
		CreatedAt: time.Now(),
	}
	for _, w := range params.Websites {
		a.Sites = append(a.Sites, models.AuditSite{Website: w, Status: "pending"})
	}
	if err := s.store.CreateAudit(a); err != nil {
		s.log.Error("create audit", "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create audit"})
		return
	}
	// The orchestrator goroutine mutates the audit as it progresses; give it
	// its own copy so serializing the 201 response does not race.
	run := *a
	s.orchestrator.Start(&run)
	c.JSON(http.StatusCreated, a)
}

func (s *Server) listAudits(c *gin.Context) {
	limit := intQuery(c, "limit", 50)
	offset := intQuery(c, "offset", 0)
	audits, total, err := s.store.ListAudits(limit, offset)
	if err != nil {
		s.log.Error("list audits", "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list audits"})
		return
	}
	if audits == nil {
		audits = []*models.Audit{}
	}
	c.JSON(http.StatusOK, gin.H{"audits": audits, "total": total})
}

func (s *Server) getAudit(c *gin.Context) {
	a, ok := s.auditFromPath(c)
	if !ok {
		return
	}
	c.JSON(http.StatusOK, a)
}

func (s *Server) deleteAudit(c *gin.Context) {
	id, ok := idFromPath(c)
	if !ok {
		return
	}
	s.orchestrator.Cancel(id)
	if err := s.store.DeleteAudit(id); err != nil {
		if errors.Is(err, database.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "audit not found"})
			return
		}
		s.log.Error("delete audit", "id", id, "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete audit"})
		return
	}
	c.Status(http.StatusNoContent)
}

func (s *Server) cancelAudit(c *gin.Context) {
	id, ok := idFromPath(c)
	if !ok {
		return
	}
	if !s.orchestrator.Cancel(id) {
		c.JSON(http.StatusConflict, gin.H{"error": "audit is not running"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "cancelling"})
}

func (s *Server) listIssues(c *gin.Context) {
	id, ok := idFromPath(c)
	if !ok {
		return
	}
	f := database.IssueFilter{
		AuditID:  id,
		Website:  c.Query("website"),
		Severity: c.Query("severity"),
		Category: c.Query("category"),
		Source:   c.Query("source"),
		Search:   c.Query("search"),
		Limit:    intQuery(c, "limit", 0),
		Offset:   intQuery(c, "offset", 0),
	}
	issues, total, err := s.store.ListIssues(f)
	if err != nil {
		s.log.Error("list issues", "audit", id, "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list issues"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"issues": issues, "total": total})
}

func (s *Server) listPages(c *gin.Context) {
	id, ok := idFromPath(c)
	if !ok {
		return
	}
	pages, err := s.store.ListPages(id)
	if err != nil {
		s.log.Error("list pages", "audit", id, "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list pages"})
		return
	}
	if pages == nil {
		pages = []*models.Page{}
	}
	c.JSON(http.StatusOK, gin.H{"pages": pages})
}

func (s *Server) exportAudit(c *gin.Context) {
	a, ok := s.auditFromPath(c)
	if !ok {
		return
	}
	format := strings.ToLower(c.Param("format"))
	exporter := report.ForFormat(format)
	if exporter == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported format: " + format +
			" (supported: json, csv, html, pdf)"})
		return
	}

	issues, _, err := s.store.ListIssues(database.IssueFilter{AuditID: a.ID})
	if err != nil {
		s.log.Error("export: list issues", "audit", a.ID, "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load issues"})
		return
	}
	fa := &report.FullAudit{Audit: a, Issues: issues}
	if format == "json" {
		// JSON export includes the crawled page inventory.
		if pages, err := s.store.ListPages(a.ID); err == nil {
			fa.Pages = pages
		}
	}

	filename := fmt.Sprintf("ergonix-audit-%d.%s", a.ID, exporter.Ext())
	c.Header("Content-Type", exporter.ContentType())
	c.Header("Content-Disposition", `attachment; filename="`+filename+`"`)
	if err := exporter.Export(c.Writer, fa); err != nil {
		s.log.Error("export failed", "audit", a.ID, "format", format, "err", err)
	}
}

// --- helpers ---

func idFromPath(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid audit id"})
		return 0, false
	}
	return id, true
}

func (s *Server) auditFromPath(c *gin.Context) (*models.Audit, bool) {
	id, ok := idFromPath(c)
	if !ok {
		return nil, false
	}
	a, err := s.store.GetAudit(id)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "audit not found"})
		} else {
			s.log.Error("get audit", "id", id, "err", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load audit"})
		}
		return nil, false
	}
	return a, true
}

func intQuery(c *gin.Context, key string, def int) int {
	if v := c.Query(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			return n
		}
	}
	return def
}
