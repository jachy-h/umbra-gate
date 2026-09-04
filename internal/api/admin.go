package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/jachy-h/llm-gateway-lite/internal/config"
	"github.com/jachy-h/llm-gateway-lite/internal/db"
	"github.com/jachy-h/llm-gateway-lite/internal/models"
	"github.com/jachy-h/llm-gateway-lite/internal/providers"
	"github.com/jachy-h/llm-gateway-lite/internal/proxy"
	"github.com/jachy-h/llm-gateway-lite/internal/stats"
)

type AdminAPI struct {
	DB              *db.DB
	HTTPClient      *http.Client
	Forwarder       *proxy.Forwarder
	StatsService    *stats.Service
	AttributeLimits config.Storage
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
	p.HasAPIKey = p.APIKey != ""
	p.APIKey = ""
	c.JSON(http.StatusCreated, p)
}

func (a *AdminAPI) UpdateProviderAPIKey(c *gin.Context) {
	var payload struct {
		APIKey string `json:"api_key"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := a.DB.UpdateProviderAPIKey(c.Param("id"), payload.APIKey); err != nil {
		if err == db.ErrNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	p, err := a.DB.GetProvider(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	p.APIKey = ""
	c.JSON(http.StatusOK, p)
}

// RefreshProviderModels retrieves an OpenAI-compatible /models catalog,
// persists it on the Provider, and returns the redacted Provider.
func (a *AdminAPI) RefreshProviderModels(c *gin.Context) {
	p, err := a.DB.GetProvider(c.Param("id"))
	if err != nil {
		if err == db.ErrNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	providerModels, err := a.fetchProviderModels(c.Request.Context(), p)
	if err != nil {
		// Do not keep an unverified catalog after a failed refresh: it can be
		// stale and lead users to choose models this Provider no longer serves.
		if clearErr := a.DB.UpdateProviderModels(p.ID, nil); clearErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": clearErr.Error()})
			return
		}
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	if err := a.DB.UpdateProviderModels(p.ID, providerModels); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	p.Models = providerModels
	p.APIKey = ""
	c.JSON(http.StatusOK, p)
}

func (a *AdminAPI) fetchProviderModels(ctx context.Context, p models.Provider) ([]string, error) {
	endpoint, err := providerModelsURL(p.BaseURL, p.Type)
	if err != nil {
		return nil, err
	}
	requestCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(requestCtx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("create model list request: %w", err)
	}
	if p.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.APIKey)
	}
	client := a.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	response, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request provider model list: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("provider model list returned %s", response.Status)
	}
	var payload struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode provider model list: %w", err)
	}
	providerModels := make([]string, 0, len(payload.Data))
	seen := make(map[string]struct{}, len(payload.Data))
	for _, model := range payload.Data {
		id := strings.TrimSpace(model.ID)
		if id == "" {
			continue
		}
		if _, duplicate := seen[id]; duplicate {
			continue
		}
		seen[id] = struct{}{}
		providerModels = append(providerModels, id)
	}
	return providerModels, nil
}

func providerModelsURL(baseURL, providerType string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("provider has an invalid base URL")
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/models"
	parsed.RawQuery = ""
	parsed.Fragment = ""
	// OpenRouter paginates its model catalog; request the full listing up to
	// 500 entries in one page per its documented interface.
	if providerType == "openrouter" {
		parsed.RawQuery = "limit=500"
	}
	return parsed.String(), nil
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
	l, err := a.prepareLink(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := a.DB.SaveLink(l); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	l, err = a.validateSavedLink(c.Request.Context(), l, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, l)
}

// CreateLinkStream saves a link and streams each real provider validation as
// an SSE event. The final "complete" event contains the persisted link.
func (a *AdminAPI) CreateLinkStream(c *gin.Context) {
	l, err := a.prepareLink(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := a.DB.SaveLink(l); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	streamEvent(c, "saved", l)

	l, err = a.validateSavedLink(c.Request.Context(), l, func(progress proxy.ValidationProgress) {
		streamEvent(c, "provider", progress)
	})
	if err != nil {
		streamEvent(c, "error", gin.H{"error": err.Error()})
		return
	}
	streamEvent(c, "complete", l)
}

func streamEvent(c *gin.Context, event string, value any) {
	c.SSEvent(event, value)
	c.Writer.Flush()
}

func (a *AdminAPI) validateSavedLink(ctx context.Context, l models.ProxyLink, onProgress func(proxy.ValidationProgress)) (models.ProxyLink, error) {
	if a.Forwarder == nil {
		return l, nil
	}
	l = a.Forwarder.ValidateChainWithProgress(ctx, l, "", onProgress)
	if err := a.DB.SaveLink(l); err != nil {
		return l, err
	}
	return l, nil
}

func (a *AdminAPI) prepareLink(c *gin.Context) (models.ProxyLink, error) {
	raw, _ := c.GetRawData()
	var l models.ProxyLink
	if err := json.Unmarshal(raw, &l); err != nil {
		return l, err
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
	if err := validateLinkAttributes(l.Attributes, a.AttributeLimits); err != nil {
		return l, err
	}
	if l.CreatedAt.IsZero() {
		l.CreatedAt = time.Now()
	}
	if len(l.Chain) == 0 {
		return l, fmt.Errorf("chain must contain at least one provider")
	}
	var rawPayload struct {
		Chain []map[string]json.RawMessage `json:"chain"`
	}
	if err := json.Unmarshal(raw, &rawPayload); err != nil {
		return l, err
	}
	for i, entry := range rawPayload.Chain {
		if _, hasAPIKey := entry["api_key"]; hasAPIKey {
			return l, fmt.Errorf("chain entry %s: API key must be configured on the provider", itoa(i))
		}
	}
	// The first node fixes the link protocol. Every following node must select
	// a compatible endpoint. The console does not ask users to choose a style;
	// infer it from the primary provider and keep the chain internally safe.
	for i := range l.Chain {
		e := &l.Chain[i]
		if err := validateModelPriorities(e.ModelPriorities); err != nil {
			return l, fmt.Errorf("chain entry %s: %w", itoa(i), err)
		}
		provider, err := a.DB.GetProvider(e.ProviderID)
		if err != nil {
			return l, fmt.Errorf("chain entry %s: provider not found", itoa(i))
		}
		if e.Protocol == "" {
			if i > 0 && l.Protocol != "" && providerSupportsProtocol(provider, l.Protocol) {
				e.Protocol = l.Protocol
			} else if len(provider.Endpoints) > 0 {
				e.Protocol = provider.Endpoints[0].Protocol
			}
			if e.Protocol == "" {
				return l, fmt.Errorf("chain entry %s: provider has no protocol endpoint", itoa(i))
			}
		}
		if !providerSupportsProtocol(provider, e.Protocol) {
			return l, fmt.Errorf("chain entry %s: provider %s does not support protocol %s", itoa(i), provider.Name, e.Protocol)
		}
		if i == 0 {
			if l.Protocol != "" && l.Protocol != e.Protocol {
				return l, fmt.Errorf("link protocol must match the first chain node protocol")
			}
			l.Protocol = e.Protocol
		} else if e.Protocol != l.Protocol {
			return l, fmt.Errorf("protocol mismatch: chain entry %s uses %s, but this link is %s", itoa(i), e.Protocol, l.Protocol)
		}
	}
	return l, nil
}

func validateModelPriorities(priorities []models.ModelPriority) error {
	if len(priorities) == 0 {
		return fmt.Errorf("model_priorities must contain at least one item")
	}
	hasRequestModel := false
	for i := range priorities {
		priority := &priorities[i]
		switch priority.Source {
		case models.ModelPriorityRequestModel:
			if strings.TrimSpace(priority.Model) != "" {
				return fmt.Errorf("model_priorities[%d]: request_model must not include model", i)
			}
			if hasRequestModel {
				return fmt.Errorf("model_priorities[%d]: request_model may appear only once", i)
			}
			hasRequestModel = true
			priority.Model = ""
		case models.ModelPriorityFixedModel:
			priority.Model = strings.TrimSpace(priority.Model)
			if priority.Model == "" {
				return fmt.Errorf("model_priorities[%d]: fixed_model requires model", i)
			}
		default:
			return fmt.Errorf("model_priorities[%d]: unsupported source %q", i, priority.Source)
		}
	}
	return nil
}

func validateLinkAttributes(attributes models.Map, limits config.Storage) error {
	if limits.MaxLinkAttributes > 0 && len(attributes) > limits.MaxLinkAttributes {
		return fmt.Errorf("attributes exceed maximum of %d", limits.MaxLinkAttributes)
	}
	for key, value := range attributes {
		if key == "" || (limits.MaxAttributeKeyLength > 0 && utf8.RuneCountInString(key) > limits.MaxAttributeKeyLength) {
			return fmt.Errorf("invalid attribute key %q", key)
		}
		encoded, err := json.Marshal(value)
		if err != nil {
			return fmt.Errorf("invalid attribute %q: %w", key, err)
		}
		if limits.MaxAttributeValueLength > 0 && utf8.RuneCountInString(string(encoded)) > limits.MaxAttributeValueLength {
			return fmt.Errorf("attribute %q exceeds maximum value length of %d", key, limits.MaxAttributeValueLength)
		}
	}
	return nil
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
	var payload struct {
		Model string `json:"model"`
	}
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&payload); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}
	l = a.Forwarder.ValidateChainWithModel(c.Request.Context(), l, payload.Model)
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
