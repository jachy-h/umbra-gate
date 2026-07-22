package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/jachy-h/llm-gateway-lite/internal/db"
	"github.com/jachy-h/llm-gateway-lite/internal/proxy"
)

type ProxyAPI struct {
	DB        *db.DB
	Forwarder *proxy.Forwarder
}

// ChatCompletions handles POST /llm-gateway-lite/:path/v1/chat/completions and forwards
// to the link's provider chain with fallback.
func (p *ProxyAPI) ChatCompletions(c *gin.Context) {
	link, err := p.DB.GetLinkByPath(c.Param("path"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "proxy link not found"})
		return
	}
	if !link.Enabled {
		c.JSON(http.StatusForbidden, gin.H{"error": "proxy link disabled"})
		return
	}
	p.Forwarder.Handle(c.Writer, c.Request, link)
}
