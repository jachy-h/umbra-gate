package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/jachy-h/llm-gateway-lite/internal/db"
	"github.com/jachy-h/llm-gateway-lite/internal/models"
	"github.com/jachy-h/llm-gateway-lite/internal/providers"
	"github.com/jachy-h/llm-gateway-lite/internal/proxy"
	"github.com/jachy-h/llm-gateway-lite/internal/stats"
)

type AdminAPI struct {
	DB           *db.DB
	Forwarder    *proxy.Forwarder
	StatsService *stats.Service
}

func newPathToken() string {
	return strings.ReplaceAll(uuid.NewString()[:12], "-", "")
}

func (a *AdminAPI) ListProviders(c *gin.Context) {
	ps, err := a.DB.ListProviders()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	for i := range ps {
		ps[i].APIKey = ""
	}
	c.JSON(http.StatusOK, ps)
}

func (a *AdminAPI) CreateProvider(c *gin.Context) {
	raw, _ := c.GetRawData()
	var p models.Provider
	if err := json.Unmarshal(raw, &p); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if p.ID == "" {
		p.ID = uuid.NewString()
	} else if !strings.Contains(string(raw), "\"api_key\"") {
		if existing, err := a.DB.GetProvider(p.ID); err == nil {
			p.APIKey = existing.APIKey
		}
	}
	if p.Type == "" {
		p.Type = "custom"
	}
	if !strings.Contains(string(raw), "\"enabled\"") {
		p.Enabled = true
	}
	if _, ok := providers.AdapterFor(p.Type); !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported provider type: " + p.Type})
		return
	}
	if len(p.Endpoints) == 0 && p.BaseURL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "at least one protocol endpoint is required"})
		return
	}
	seenEndpoints := map[string]bool{}
	for i, endpoint := range p.Endpoints {
		if !supportedProtocol(endpoint.Protocol) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "endpoint " + itoa(i) + ": unsupported protocol " + endpoint.Protocol})
			return
		}
		if !supportedFormat(endpoint.RequestFormat) || !supportedFormat(endpoint.ResponseFormat) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "endpoint " + itoa(i) + ": unsupported request/response format"})
			return
		}
		if endpoint.Protocol == models.ProtocolAnthropic &&
			(endpoint.RequestFormat != models.FormatMessages || endpoint.ResponseFormat != models.FormatMessages) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "endpoint " + itoa(i) + ": Anthropic Style endpoints must use Messages format"})
			return
		}
		if endpoint.Protocol == models.ProtocolOpenAI &&
			(endpoint.RequestFormat == models.FormatMessages || endpoint.ResponseFormat == models.FormatMessages) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "endpoint " + itoa(i) + ": OpenAI Style endpoints cannot use Messages format"})
			return
		}
		if strings.TrimSpace(endpoint.BaseURL) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "endpoint " + itoa(i) + ": base URL is required"})
			return
		}
		key := endpoint.Protocol + "\x00" + endpoint.ResponseFormat
		if seenEndpoints[key] {
			c.JSON(http.StatusBadRequest, gin.H{"error": "provider has more than one endpoint for protocol/response format " + endpoint.Protocol + "/" + endpoint.ResponseFormat})
			return
		}
		seenEndpoints[key] = true
	}
	if p.CreatedAt.IsZero() {
		p.CreatedAt = time.Now()
	}
	if err := a.DB.UpsertProvider(p); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	p.APIKey = ""
	c.JSON(http.StatusCreated, p)
}

func (a *AdminAPI) GetProvider(c *gin.Context) {
	p, err := a.DB.GetProvider(c.Param("id"))
	if err != nil {
		if err == db.ErrNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	p.APIKey = ""
	c.JSON(http.StatusOK, p)
}

func (a *AdminAPI) DeleteProvider(c *gin.Context) {
	if err := a.DB.DeleteProvider(c.Param("id")); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": c.Param("id")})
}

func (a *AdminAPI) ListLinks(c *gin.Context) {
	ls, err := a.DB.ListLinks()
	send(c, ls, err)
}

func (a *AdminAPI) CreateLink(c *gin.Context) {
	raw, _ := c.GetRawData()
	var l models.ProxyLink
	if err := json.Unmarshal(raw, &l); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if l.ID == "" {
		l.ID = uuid.NewString()
	}
	if !strings.Contains(string(raw), "\"enabled\"") {
		l.Enabled = true
	}
	if l.Path == "" {
		l.Path = newPathToken()
	}
	if l.Attributes == nil {
		l.Attributes = models.Map{}
	}
	if l.CreatedAt.IsZero() {
		l.CreatedAt = time.Now()
	}
	if len(l.Chain) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "chain must contain at least one provider"})
		return
	}
	// The first node fixes the link protocol. Every following node must select
	// an endpoint for that same protocol.
	for i := range l.Chain {
		e := &l.Chain[i]
		provider, err := a.DB.GetProvider(e.ProviderID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "chain entry " + itoa(i) + ": provider not found"})
			return
		}
		if e.Protocol == "" {
			for _, endpoint := range provider.Endpoints {
				if e.Protocol == "" {
					e.Protocol = endpoint.Protocol
				} else if e.Protocol != endpoint.Protocol {
					e.Protocol = ""
					break
				}
			}
			if e.Protocol == "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "chain entry " + itoa(i) + ": select a provider protocol"})
				return
			}
		}
		if !providerSupportsProtocol(provider, e.Protocol) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "chain entry " + itoa(i) + ": provider " + provider.Name + " does not support protocol " + e.Protocol})
			return
		}
		if i == 0 {
			if l.Protocol != "" && l.Protocol != e.Protocol {
				c.JSON(http.StatusBadRequest, gin.H{"error": "link protocol must match the first chain node protocol"})
				return
			}
			l.Protocol = e.Protocol
		} else if e.Protocol != l.Protocol {
			c.JSON(http.StatusBadRequest, gin.H{"error": "protocol mismatch: chain entry " + itoa(i) + " uses " + e.Protocol + ", but this link is " + l.Protocol})
			return
		}
	}
	if err := a.DB.SaveLink(l); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// Probe every node once after create/update and persist the result. A failed
	// probe does not prevent saving the chain: it is shown as a gray node and
	// can still recover before the next real request.
	if a.Forwarder != nil {
		l = a.Forwarder.ValidateChain(c.Request.Context(), l)
		if err := a.DB.SaveLink(l); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}
	c.JSON(http.StatusCreated, l)
}

func supportedProtocol(protocol string) bool {
	return protocol == models.ProtocolOpenAI || protocol == models.ProtocolAnthropic
}

func supportedFormat(format string) bool {
	return format == models.FormatChatCompletions || format == models.FormatResponses || format == models.FormatMessages
}

func providerSupportsProtocol(provider models.Provider, protocol string) bool {
	for _, endpoint := range provider.Endpoints {
		if endpoint.Protocol == protocol && strings.TrimSpace(endpoint.BaseURL) != "" {
			return true
		}
	}
	return false
}

func (a *AdminAPI) GetLink(c *gin.Context) {
	l, err := a.DB.GetLink(c.Param("id"))
	send(c, l, err)
}

// TestLink runs one validation request for every provider in a link's chain.
// It is intentionally separate from saving so operators can recheck a chain
// without changing its configuration.
func (a *AdminAPI) TestLink(c *gin.Context) {
	l, err := a.DB.GetLink(c.Param("id"))
	if err != nil {
		send(c, nil, err)
		return
	}
	if a.Forwarder == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "link testing is unavailable"})
		return
	}
	for i, entry := range l.Chain {
		if entry.Protocol != l.Protocol {
			c.JSON(http.StatusBadRequest, gin.H{"error": "protocol mismatch: chain entry " + itoa(i) + " uses " + entry.Protocol + ", but this link is " + l.Protocol})
			return
		}
	}
	l = a.Forwarder.ValidateChain(c.Request.Context(), l)
	if err := a.DB.SaveLink(l); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, l)
}

func (a *AdminAPI) DeleteLink(c *gin.Context) {
	if err := a.DB.DeleteLink(c.Param("id")); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": c.Param("id")})
}

func (a *AdminAPI) ListTypes(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"types": providers.RegisteredTypes()})
}

func (a *AdminAPI) Stats(c *gin.Context) {
	// Fold the latest logs before reading aggregates so dashboard cards do not
	// depend on the background aggregation timer having fired already.
	if a.StatsService != nil {
		if err := a.StatsService.Aggregate(c.Request.Context()); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}
	linkID := c.Query("link_id")
	from := c.Query("from")
	to := c.Query("to")
	q := `SELECT link_id, provider_id, attr_key, attr_value, period,
			SUM(total) total, SUM(success) success, SUM(failure) failure,
			SUM(total_latency_ms) lat
		FROM stats_hourly WHERE 1=1`
	args := []any{}
	if linkID != "" {
		q += " AND link_id=?"
		args = append(args, linkID)
	}
	if from != "" {
		q += " AND period >= ?"
		args = append(args, from)
	}
	if to != "" {
		q += " AND period <= ?"
		args = append(args, to)
	}
	q += " GROUP BY link_id, provider_id, attr_key, attr_value, period ORDER BY period DESC, link_id"
	rows, err := a.DB.Query(q, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()
	type row struct {
		LinkID, ProviderID, AttrKey, AttrValue, Period string
		Total, Success, Failure, Lat                   int64
	}
	out := make([]row, 0)
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.LinkID, &r.ProviderID, &r.AttrKey, &r.AttrValue, &r.Period, &r.Total, &r.Success, &r.Failure, &r.Lat); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		out = append(out, r)
	}
	c.JSON(http.StatusOK, gin.H{"stats": out})
}

func (a *AdminAPI) RecentRequests(c *gin.Context) {
	logs, err := a.DB.ListRecentLogs(100)
	send(c, logs, err)
}

func (a *AdminAPI) LatestValidationRequests(c *gin.Context) {
	logs, err := a.DB.ListLatestValidationLogs()
	send(c, logs, err)
}

func send(c *gin.Context, v any, err error) {
	if err != nil {
		if err == db.ErrNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, v)
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b []byte
	for i > 0 {
		b = append([]byte{byte('0' + i%10)}, b...)
		i /= 10
	}
	return string(b)
}
