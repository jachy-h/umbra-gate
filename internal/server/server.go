package server

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/jachy-h/llm-gateway-lite/internal/api"
	"github.com/jachy-h/llm-gateway-lite/internal/config"
	"github.com/jachy-h/llm-gateway-lite/internal/db"
	"github.com/jachy-h/llm-gateway-lite/internal/proxy"
	"github.com/jachy-h/llm-gateway-lite/internal/stats"
	"github.com/jachy-h/llm-gateway-lite/internal/web"
)

func New(cfg config.Config, d *db.DB) (*gin.Engine, *stats.Service) {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery(), gin.Logger())

	statSvc := stats.New(d)
	fwd := &proxy.Forwarder{DB: d, Stats: statSvc}
	admin := &api.AdminAPI{DB: d, Forwarder: fwd, StatsService: statSvc}
	prox := &api.ProxyAPI{DB: d, Forwarder: fwd}

	adminAuth := func(c *gin.Context) {
		if cfg.Admin.Token == "" {
			c.Next()
			return
		}
		tok := c.GetHeader("X-Admin-Token")
		if tok == "" {
			tok = strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
		}
		if tok != cfg.Admin.Token {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		c.Next()
	}

	ag := r.Group("/admin", adminAuth)
	{
		ag.GET("/types", admin.ListTypes)

		ag.GET("/providers", admin.ListProviders)
		ag.POST("/providers", admin.CreateProvider)
		ag.GET("/providers/:id", admin.GetProvider)
		ag.DELETE("/providers/:id", admin.DeleteProvider)

		ag.GET("/links", admin.ListLinks)
		ag.POST("/links", admin.CreateLink)
		ag.GET("/links/:id", admin.GetLink)
		ag.POST("/links/:id/test", admin.TestLink)
		ag.DELETE("/links/:id", admin.DeleteLink)

		ag.GET("/stats", admin.Stats)
		ag.GET("/requests", admin.RecentRequests)
		ag.GET("/validation-requests", admin.LatestValidationRequests)
	}

	// OpenAI-compatible proxy endpoints per link. Support both common base URL
	// conventions: clients may append /chat/completions or /v1/chat/completions.
	r.GET("/llm-gateway-lite/:path", prox.Info)
	r.GET("/llm-gateway-lite/:path/models", prox.Models)
	r.GET("/llm-gateway-lite/:path/v1/models", prox.Models)
	r.POST("/llm-gateway-lite/:path", prox.ChatCompletions)
	r.POST("/llm-gateway-lite/:path/chat/completions", prox.ChatCompletions)
	r.POST("/llm-gateway-lite/:path/v1/chat/completions", prox.ChatCompletions)
	r.POST("/llm-gateway-lite/:path/responses", prox.Responses)
	r.POST("/llm-gateway-lite/:path/v1/responses", prox.Responses)
	r.POST("/llm-gateway-lite/:path/messages", prox.Messages)
	r.POST("/llm-gateway-lite/:path/v1/messages", prox.Messages)

	r.GET("/healthz", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })

	// Serve the embedded frontend SPA for any unmatched, non-API route so
	// client-side routing works without a separate static host.
	spa := web.Handler()
	r.NoRoute(func(c *gin.Context) {
		p := c.Request.URL.Path
		if strings.HasPrefix(p, "/admin") ||
			strings.HasPrefix(p, "/llm-gateway-lite") ||
			p == "/healthz" {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		spa.ServeHTTP(c.Writer, c.Request)
	})

	return r, statSvc
}
