package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/jachy-h/llm-gateway-lite/internal/db"
	"github.com/jachy-h/llm-gateway-lite/internal/models"
	"github.com/jachy-h/llm-gateway-lite/internal/proxy"
)

type ProxyAPI struct {
	DB        *db.DB
	Forwarder *proxy.Forwarder
}

// Info makes a copied proxy URL useful when opened directly in a browser and
// gives clients a lightweight way to check that the link exists.
func (p *ProxyAPI) Info(c *gin.Context) {
	link, err := p.DB.GetLinkByPath(c.Param("path"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "proxy link not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"ok": link.Enabled, "name": link.Name, "path": link.Path,
		"chat_completions":  "/llm-gateway-lite/" + link.Path + "/v1/chat/completions",
		"responses":         "/llm-gateway-lite/" + link.Path + "/v1/responses",
		"messages":          "/llm-gateway-lite/" + link.Path + "/v1/messages",
		"protocol":          link.Protocol,
		"supported_formats": link.SupportedFormats,
		"checked_at":        time.Now().UTC(),
	})
}

// Responses handles the OpenAI Responses API. Endpoint-specific adaptation is
// only applied when a provider explicitly declares an asymmetric request shape.
func (p *ProxyAPI) Responses(c *gin.Context) {
	// Some OpenAI SDKs append the operation name to their configured base URL.
	// Accept a full /responses operation URL as the base without exposing the
	// resulting /responses/responses duplication to routing or request logs.
	if strings.HasSuffix(c.Request.URL.Path, "/responses/responses") {
		c.Request.URL.Path = strings.TrimSuffix(c.Request.URL.Path, "/responses")
	}
	if strings.HasSuffix(c.Request.URL.RawPath, "/responses/responses") {
		c.Request.URL.RawPath = strings.TrimSuffix(c.Request.URL.RawPath, "/responses")
	}
	link, err := p.DB.GetLinkByPath(c.Param("path"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "proxy link not found"})
		return
	}
	if !link.Enabled {
		c.JSON(http.StatusForbidden, gin.H{"error": "proxy link disabled"})
		return
	}
	p.Forwarder.HandleRequest(c.Writer, c.Request, link, models.ProtocolOpenAI, models.FormatResponses)
}

// Messages exposes an Anthropic-native link without converting it through an
// OpenAI request or response schema.
func (p *ProxyAPI) Messages(c *gin.Context) {
	link, err := p.DB.GetLinkByPath(c.Param("path"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "proxy link not found"})
		return
	}
	if !link.Enabled {
		c.JSON(http.StatusForbidden, gin.H{"error": "proxy link disabled"})
		return
	}
	p.Forwarder.HandleRequest(c.Writer, c.Request, link, models.ProtocolAnthropic, models.FormatMessages)
}

func (p *ProxyAPI) Models(c *gin.Context) {
	link, err := p.DB.GetLinkByPath(c.Param("path"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "proxy link not found"})
		return
	}
	seen := map[string]bool{}
	data := make([]gin.H, 0)
	for _, entry := range link.Chain {
		provider, err := p.DB.GetProvider(entry.ProviderID)
		if err != nil || !provider.Enabled {
			continue
		}
		for _, model := range provider.Models {
			if !seen[model] {
				seen[model] = true
				data = append(data, gin.H{"id": model, "object": "model", "owned_by": provider.Name})
			}
		}
	}
	c.JSON(http.StatusOK, gin.H{"object": "list", "data": data})
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
