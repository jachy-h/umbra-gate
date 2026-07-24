package db

import (
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jachy-h/llm-gateway-lite/internal/models"
)

var ErrNotFound = errors.New("not found")

func normalizeProviderEndpoints(p models.Provider) []models.ProviderEndpoint {
	seen := map[string]bool{}
	out := make([]models.ProviderEndpoint, 0, len(p.Endpoints)+1)
	for _, endpoint := range p.Endpoints {
		legacyProtocol := strings.TrimSpace(endpoint.Protocol)
		endpoint.Protocol = normalizeProtocol(legacyProtocol, p.Type)
		if endpoint.RequestFormat == "" {
			endpoint.RequestFormat = inferEndpointFormat(legacyProtocol, endpoint.BaseURL, p.Type)
		}
		if endpoint.ResponseFormat == "" {
			endpoint.ResponseFormat = endpoint.RequestFormat
		}
		endpoint.BaseURL = strings.TrimRight(strings.TrimSpace(endpoint.BaseURL), "/")
		key := endpoint.Protocol + "\x00" + endpoint.ResponseFormat
		if endpoint.Protocol == "" || endpoint.RequestFormat == "" || endpoint.ResponseFormat == "" || endpoint.BaseURL == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, endpoint)
	}
	if len(out) == 0 && strings.TrimSpace(p.BaseURL) != "" {
		protocol := normalizeProtocol("", p.Type)
		format := inferEndpointFormat("", p.BaseURL, p.Type)
		responseFormat := format
		out = append(out, models.ProviderEndpoint{
			Protocol: protocol, RequestFormat: format, ResponseFormat: responseFormat,
			BaseURL: strings.TrimRight(strings.TrimSpace(p.BaseURL), "/"),
		})
	}
	return out
}

func normalizeProtocol(protocol, providerType string) string {
	if providerType == "anthropic" {
		return models.ProtocolAnthropic
	}
	switch strings.TrimSpace(protocol) {
	case models.ProtocolOpenAI, "openai_chat_completions", "openai_responses":
		return models.ProtocolOpenAI
	case models.ProtocolAnthropic:
		return models.ProtocolAnthropic
	}
	return models.ProtocolOpenAI
}

func inferEndpointFormat(legacyProtocol, baseURL, providerType string) string {
	if providerType == "anthropic" {
		return models.FormatMessages
	}
	switch strings.TrimSpace(legacyProtocol) {
	case "openai_responses":
		return models.FormatResponses
	case "openai_chat_completions":
		return models.FormatChatCompletions
	case models.ProtocolAnthropic:
		return models.FormatMessages
	}
	if providerType == "opencode" || strings.HasSuffix(strings.TrimRight(baseURL, "/"), "/responses") {
		return models.FormatResponses
	}
	return models.FormatChatCompletions
}

func (d *DB) UpsertProvider(p models.Provider) error {
	p.Endpoints = normalizeProviderEndpoints(p)
	if len(p.Endpoints) > 0 {
		p.BaseURL = p.Endpoints[0].BaseURL
	}
	_, err := d.Exec(`INSERT INTO providers(id,name,type,base_url,endpoints_json,api_key,models_json,extra_json,enabled,created_at)
		VALUES(?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(id) DO UPDATE SET name=excluded.name,type=excluded.type,base_url=excluded.base_url,
			endpoints_json=excluded.endpoints_json,api_key=excluded.api_key,models_json=excluded.models_json,extra_json=excluded.extra_json,enabled=excluded.enabled`,
		p.ID, p.Name, p.Type, p.BaseURL, enc(p.Endpoints), p.APIKey, enc(p.Models), enc(p.Extra), btoi(p.Enabled), p.CreatedAt.UTC())
	return err
}

func (d *DB) ListProviders() ([]models.Provider, error) {
	rows, err := d.Query(`SELECT id,name,type,base_url,endpoints_json,api_key,models_json,extra_json,enabled,builtin,created_at FROM providers ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]models.Provider, 0)
	for rows.Next() {
		var p models.Provider
		var endpointsJSON, modelsJSON, extraJSON string
		var enabled, builtin int
		var created string
		if err := rows.Scan(&p.ID, &p.Name, &p.Type, &p.BaseURL, &endpointsJSON, &p.APIKey, &modelsJSON, &extraJSON, &enabled, &builtin, &created); err != nil {
			return nil, err
		}
		p.Models = decModels(modelsJSON)
		_ = json.Unmarshal([]byte(endpointsJSON), &p.Endpoints)
		p.Endpoints = normalizeProviderEndpoints(p)
		p.Extra = decMap(extraJSON)
		p.Enabled = enabled == 1
		p.Builtin = builtin == 1
		p.HasAPIKey = p.APIKey != ""
		p.CreatedAt = parseTime(created)
		out = append(out, p)
	}
	return out, rows.Err()
}

func (d *DB) GetProvider(id string) (models.Provider, error) {
	var p models.Provider
	var endpointsJSON, modelsJSON, extraJSON string
	var enabled, builtin int
	var created string
	err := d.QueryRow(`SELECT id,name,type,base_url,endpoints_json,api_key,models_json,extra_json,enabled,builtin,created_at FROM providers WHERE id=?`, id).
		Scan(&p.ID, &p.Name, &p.Type, &p.BaseURL, &endpointsJSON, &p.APIKey, &modelsJSON, &extraJSON, &enabled, &builtin, &created)
	if err == sql.ErrNoRows {
		return p, ErrNotFound
	}
	if err != nil {
		return p, err
	}
	p.Models = decModels(modelsJSON)
	_ = json.Unmarshal([]byte(endpointsJSON), &p.Endpoints)
	p.Endpoints = normalizeProviderEndpoints(p)
	p.Extra = decMap(extraJSON)
	p.Enabled = enabled == 1
	p.Builtin = builtin == 1
	p.HasAPIKey = p.APIKey != ""
	p.CreatedAt = parseTime(created)
	return p, nil
}

func (d *DB) DeleteProvider(id string) error {
	_, err := d.Exec(`DELETE FROM providers WHERE id=?`, id)
	return err
}

func (d *DB) SaveLink(l models.ProxyLink) error {
	tx, err := d.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`INSERT INTO proxy_links(id,name,path,protocol,supported_formats_json,attributes_json,enabled,created_at)
		VALUES(?,?,?,?,?,?,?,?)
		ON CONFLICT(id) DO UPDATE SET name=excluded.name,path=excluded.path,protocol=excluded.protocol,supported_formats_json=excluded.supported_formats_json,attributes_json=excluded.attributes_json,enabled=excluded.enabled`,
		l.ID, l.Name, l.Path, l.Protocol, enc(l.SupportedFormats), enc(l.Attributes), btoi(l.Enabled), l.CreatedAt.UTC()); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM proxy_link_providers WHERE link_id=?`, l.ID); err != nil {
		return err
	}
	for i, e := range l.Chain {
		rules := enc(e.Rules)
		var validationOK any
		if e.ValidationOK != nil {
			validationOK = btoi(*e.ValidationOK)
		}
		var validatedAt any
		if !e.ValidatedAt.IsZero() {
			validatedAt = e.ValidatedAt.UTC()
		}
		if _, err := tx.Exec(`INSERT INTO proxy_link_providers(link_id,position,provider_id,protocol,retry_count,fallback_model,api_key,rules_json,validation_ok,validation_error,validated_at,supported_formats_json)
			VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`, l.ID, i, e.ProviderID, e.Protocol, e.RetryCount, e.FallbackModel, e.ApiKey, rules, validationOK, e.ValidationError, validatedAt, enc(e.SupportedFormats)); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (d *DB) ListLinks() ([]models.ProxyLink, error) {
	rows, err := d.Query(`SELECT id,name,path,protocol,supported_formats_json,attributes_json,enabled,created_at FROM proxy_links ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]models.ProxyLink, 0)
	for rows.Next() {
		var l models.ProxyLink
		var attrJSON, formatsJSON string
		var enabled int
		var created string
		if err := rows.Scan(&l.ID, &l.Name, &l.Path, &l.Protocol, &formatsJSON, &attrJSON, &enabled, &created); err != nil {
			return nil, err
		}
		l.Attributes = decMap(attrJSON)
		_ = json.Unmarshal([]byte(formatsJSON), &l.SupportedFormats)
		l.Enabled = enabled == 1
		l.CreatedAt = parseTime(created)
		out = append(out, l)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range out {
		chain, err := d.loadChain(out[i].ID)
		if err != nil {
			return nil, err
		}
		out[i].Chain = chain
	}
	return out, nil
}

func (d *DB) GetLinkByPath(path string) (models.ProxyLink, error) {
	var l models.ProxyLink
	var attrJSON, formatsJSON string
	var enabled int
	var created string
	err := d.QueryRow(`SELECT id,name,path,protocol,supported_formats_json,attributes_json,enabled,created_at FROM proxy_links WHERE path=?`, path).
		Scan(&l.ID, &l.Name, &l.Path, &l.Protocol, &formatsJSON, &attrJSON, &enabled, &created)
	if err == sql.ErrNoRows {
		return l, ErrNotFound
	}
	if err != nil {
		return l, err
	}
	l.Attributes = decMap(attrJSON)
	_ = json.Unmarshal([]byte(formatsJSON), &l.SupportedFormats)
	l.Enabled = enabled == 1
	l.CreatedAt = parseTime(created)
	l.Chain, err = d.loadChain(l.ID)
	return l, err
}

func (d *DB) GetLink(id string) (models.ProxyLink, error) {
	var l models.ProxyLink
	var attrJSON, formatsJSON string
	var enabled int
	var created string
	err := d.QueryRow(`SELECT id,name,path,protocol,supported_formats_json,attributes_json,enabled,created_at FROM proxy_links WHERE id=?`, id).
		Scan(&l.ID, &l.Name, &l.Path, &l.Protocol, &formatsJSON, &attrJSON, &enabled, &created)
	if err == sql.ErrNoRows {
		return l, ErrNotFound
	}
	if err != nil {
		return l, err
	}
	l.Attributes = decMap(attrJSON)
	_ = json.Unmarshal([]byte(formatsJSON), &l.SupportedFormats)
	l.Enabled = enabled == 1
	l.CreatedAt = parseTime(created)
	l.Chain, err = d.loadChain(l.ID)
	return l, err
}

func (d *DB) loadChain(linkID string) ([]models.ChainEntry, error) {
	rows, err := d.Query(`SELECT provider_id,protocol,retry_count,fallback_model,api_key,rules_json,validation_ok,validation_error,validated_at,supported_formats_json FROM proxy_link_providers WHERE link_id=? ORDER BY position`, linkID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	chain := make([]models.ChainEntry, 0)
	for rows.Next() {
		var e models.ChainEntry
		var rulesJSON, formatsJSON string
		var validationOK sql.NullInt64
		var validatedAt sql.NullString
		if err := rows.Scan(&e.ProviderID, &e.Protocol, &e.RetryCount, &e.FallbackModel, &e.ApiKey, &rulesJSON, &validationOK, &e.ValidationError, &validatedAt, &formatsJSON); err != nil {
			return nil, err
		}
		if validationOK.Valid {
			ok := validationOK.Int64 == 1
			e.ValidationOK = &ok
		}
		if validatedAt.Valid {
			e.ValidatedAt = parseTime(validatedAt.String)
		}
		_ = json.Unmarshal([]byte(rulesJSON), &e.Rules)
		_ = json.Unmarshal([]byte(formatsJSON), &e.SupportedFormats)
		chain = append(chain, e)
	}
	return chain, rows.Err()
}

func (d *DB) DeleteLink(id string) error {
	_, err := d.Exec(`DELETE FROM proxy_links WHERE id=?`, id)
	return err
}

func (d *DB) InsertLog(l models.RequestLog) error {
	_, err := d.Exec(`INSERT INTO request_logs(id,link_id,path,provider_id,provider_name,model,status_code,latency_ms,success,error_message,request_url,request_headers_json,request_body,upstream_url,upstream_headers_json,upstream_body,response_headers_json,response_body,attributes_json,created_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		l.ID, l.LinkID, l.Path, l.ProviderID, l.ProviderName, l.Model, l.StatusCode, l.LatencyMS, btoi(l.Success), l.ErrorMessage,
		l.RequestURL, enc(l.RequestHeaders), l.RequestBody, l.UpstreamURL, enc(l.UpstreamHeaders), l.UpstreamBody, enc(l.ResponseHeaders), l.ResponseBody, enc(l.Attributes), l.CreatedAt.UTC().Format(time.RFC3339Nano))
	return err
}

func (d *DB) ListRecentLogs(limit int) ([]models.RequestLog, error) {
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	rows, err := d.Query(`SELECT id,link_id,path,provider_id,provider_name,model,status_code,latency_ms,success,error_message,request_url,request_headers_json,request_body,upstream_url,upstream_headers_json,upstream_body,response_headers_json,response_body,attributes_json,created_at
		FROM request_logs
		WHERE COALESCE(json_extract(attributes_json, '$._request_type'), '') != 'link_validation'
		ORDER BY created_at DESC, rowid DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	return scanRequestLogs(rows, limit)
}

// ListLatestValidationLogs returns the latest Link Test for every chain node,
// independent of the recent-request limit used by the statistics page.
func (d *DB) ListLatestValidationLogs() ([]models.RequestLog, error) {
	rows, err := d.Query(`SELECT id,link_id,path,provider_id,provider_name,model,status_code,latency_ms,success,error_message,request_url,request_headers_json,request_body,upstream_url,upstream_headers_json,upstream_body,response_headers_json,response_body,attributes_json,created_at
		FROM request_logs
		WHERE rowid IN (
			SELECT MAX(rowid) FROM request_logs
			WHERE json_extract(attributes_json, '$._request_type') = 'link_validation'
			GROUP BY link_id, provider_id, json_extract(attributes_json, '$._chain_position'), json_extract(attributes_json, '$._format')
		)
		ORDER BY created_at DESC, rowid DESC`)
	if err != nil {
		return nil, err
	}
	return scanRequestLogs(rows, 0)
}

func scanRequestLogs(rows *sql.Rows, capacity int) ([]models.RequestLog, error) {
	defer rows.Close()
	out := make([]models.RequestLog, 0, capacity)
	for rows.Next() {
		var l models.RequestLog
		var success int
		var requestHeadersJSON, upstreamHeadersJSON, responseHeadersJSON, attributesJSON, createdAt string
		if err := rows.Scan(&l.ID, &l.LinkID, &l.Path, &l.ProviderID, &l.ProviderName, &l.Model, &l.StatusCode, &l.LatencyMS, &success, &l.ErrorMessage,
			&l.RequestURL, &requestHeadersJSON, &l.RequestBody, &l.UpstreamURL, &upstreamHeadersJSON, &l.UpstreamBody, &responseHeadersJSON, &l.ResponseBody, &attributesJSON, &createdAt); err != nil {
			return nil, err
		}
		l.Success = success == 1
		l.RequestHeaders = decMap(requestHeadersJSON)
		l.UpstreamHeaders = decMap(upstreamHeadersJSON)
		l.ResponseHeaders = decMap(responseHeadersJSON)
		l.Attributes = decMap(attributesJSON)
		l.CreatedAt = parseTime(createdAt)
		out = append(out, l)
	}
	return out, rows.Err()
}

func (d *DB) UpsertStats(s models.Stats, attrKey, attrValue string) error {
	_, err := d.Exec(`INSERT INTO stats_hourly(link_id,provider_id,attr_key,attr_value,period,total,success,failure,total_latency_ms)
		VALUES(?,?,?,?,?,?,?,?,?)
		ON CONFLICT(link_id,provider_id,attr_key,attr_value,period) DO UPDATE SET
			total=total+excluded.total, success=success+excluded.success,
			failure=failure+excluded.failure, total_latency_ms=total_latency_ms+excluded.total_latency_ms`,
		s.LinkID, s.ProviderID, attrKey, attrValue, s.Period, s.Total, s.Success, s.Failure, s.TotalLatMS)
	return err
}

func (d *DB) DeleteAggregatedBefore(period string) error {
	_, err := d.Exec(`DELETE FROM stats_hourly WHERE period < ?`, period)
	return err
}

func btoi(b bool) int {
	if b {
		return 1
	}
	return 0
}

func (d *DB) GetMeta(key string) (string, error) {
	var v string
	err := d.QueryRow(`SELECT value FROM stats_meta WHERE key=?`, key).Scan(&v)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return v, err
}

func (d *DB) SetMeta(key, value string) error {
	_, err := d.Exec(`INSERT INTO stats_meta(key,value) VALUES(?,?)
		ON CONFLICT(key) DO UPDATE SET value=excluded.value`, key, value)
	return err
}

func parseTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}
	}
	return t
}
