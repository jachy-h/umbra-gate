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
)

type AdminAPI struct {
	DB *db.DB
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
	// validate chain providers exist
	for i, e := range l.Chain {
		if _, err := a.DB.GetProvider(e.ProviderID); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "chain entry " + itoa(i) + ": provider not found"})
			return
		}
	}
	if err := a.DB.SaveLink(l); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, l)
}

func (a *AdminAPI) GetLink(c *gin.Context) {
	l, err := a.DB.GetLink(c.Param("id"))
	send(c, l, err)
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
