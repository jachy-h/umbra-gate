package db

import (
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/jachy-h/llm-gateway-lite/internal/models"
)

var ErrNotFound = errors.New("not found")

func (d *DB) UpsertProvider(p models.Provider) error {
	_, err := d.Exec(`INSERT INTO providers(id,name,type,base_url,api_key,models_json,extra_json,enabled,created_at)
		VALUES(?,?,?,?,?,?,?,?,?)
		ON CONFLICT(id) DO UPDATE SET name=excluded.name,type=excluded.type,base_url=excluded.base_url,
			api_key=excluded.api_key,models_json=excluded.models_json,extra_json=excluded.extra_json,enabled=excluded.enabled`,
		p.ID, p.Name, p.Type, p.BaseURL, p.APIKey, enc(p.Models), enc(p.Extra), btoi(p.Enabled), p.CreatedAt.UTC())
	return err
}

func (d *DB) ListProviders() ([]models.Provider, error) {
	rows, err := d.Query(`SELECT id,name,type,base_url,api_key,models_json,extra_json,enabled,builtin,created_at FROM providers ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]models.Provider, 0)
	for rows.Next() {
		var p models.Provider
		var modelsJSON, extraJSON string
		var enabled, builtin int
		var created string
		if err := rows.Scan(&p.ID, &p.Name, &p.Type, &p.BaseURL, &p.APIKey, &modelsJSON, &extraJSON, &enabled, &builtin, &created); err != nil {
			return nil, err
		}
		p.Models = decModels(modelsJSON)
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
	var modelsJSON, extraJSON string
	var enabled, builtin int
	var created string
	err := d.QueryRow(`SELECT id,name,type,base_url,api_key,models_json,extra_json,enabled,builtin,created_at FROM providers WHERE id=?`, id).
		Scan(&p.ID, &p.Name, &p.Type, &p.BaseURL, &p.APIKey, &modelsJSON, &extraJSON, &enabled, &builtin, &created)
	if err == sql.ErrNoRows {
		return p, ErrNotFound
	}
	if err != nil {
		return p, err
	}
	p.Models = decModels(modelsJSON)
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
	if _, err := tx.Exec(`INSERT INTO proxy_links(id,name,path,attributes_json,enabled,created_at)
		VALUES(?,?,?,?,?,?)
		ON CONFLICT(id) DO UPDATE SET name=excluded.name,path=excluded.path,attributes_json=excluded.attributes_json,enabled=excluded.enabled`,
		l.ID, l.Name, l.Path, enc(l.Attributes), btoi(l.Enabled), l.CreatedAt.UTC()); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM proxy_link_providers WHERE link_id=?`, l.ID); err != nil {
		return err
	}
	for i, e := range l.Chain {
		rules := enc(e.Rules)
		if _, err := tx.Exec(`INSERT INTO proxy_link_providers(link_id,position,provider_id,retry_count,fallback_model,api_key,rules_json)
			VALUES(?,?,?,?,?,?,?)`, l.ID, i, e.ProviderID, e.RetryCount, e.FallbackModel, e.ApiKey, rules); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (d *DB) ListLinks() ([]models.ProxyLink, error) {
	rows, err := d.Query(`SELECT id,name,path,attributes_json,enabled,created_at FROM proxy_links ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]models.ProxyLink, 0)
	for rows.Next() {
		var l models.ProxyLink
		var attrJSON string
		var enabled int
		var created string
		if err := rows.Scan(&l.ID, &l.Name, &l.Path, &attrJSON, &enabled, &created); err != nil {
			return nil, err
		}
		l.Attributes = decMap(attrJSON)
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
	var attrJSON string
	var enabled int
	var created string
	err := d.QueryRow(`SELECT id,name,path,attributes_json,enabled,created_at FROM proxy_links WHERE path=?`, path).
		Scan(&l.ID, &l.Name, &l.Path, &attrJSON, &enabled, &created)
	if err == sql.ErrNoRows {
		return l, ErrNotFound
	}
	if err != nil {
		return l, err
	}
	l.Attributes = decMap(attrJSON)
	l.Enabled = enabled == 1
	l.CreatedAt = parseTime(created)
	l.Chain, err = d.loadChain(l.ID)
	return l, err
}

func (d *DB) GetLink(id string) (models.ProxyLink, error) {
	var l models.ProxyLink
	var attrJSON string
	var enabled int
	var created string
	err := d.QueryRow(`SELECT id,name,path,attributes_json,enabled,created_at FROM proxy_links WHERE id=?`, id).
		Scan(&l.ID, &l.Name, &l.Path, &attrJSON, &enabled, &created)
	if err == sql.ErrNoRows {
		return l, ErrNotFound
	}
	if err != nil {
		return l, err
	}
	l.Attributes = decMap(attrJSON)
	l.Enabled = enabled == 1
	l.CreatedAt = parseTime(created)
	l.Chain, err = d.loadChain(l.ID)
	return l, err
}

func (d *DB) loadChain(linkID string) ([]models.ChainEntry, error) {
	rows, err := d.Query(`SELECT provider_id,retry_count,fallback_model,api_key,rules_json FROM proxy_link_providers WHERE link_id=? ORDER BY position`, linkID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	chain := make([]models.ChainEntry, 0)
	for rows.Next() {
		var e models.ChainEntry
		var rulesJSON string
		if err := rows.Scan(&e.ProviderID, &e.RetryCount, &e.FallbackModel, &e.ApiKey, &rulesJSON); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(rulesJSON), &e.Rules)
		chain = append(chain, e)
	}
	return chain, rows.Err()
}

func (d *DB) DeleteLink(id string) error {
	_, err := d.Exec(`DELETE FROM proxy_links WHERE id=?`, id)
	return err
}

func (d *DB) InsertLog(l models.RequestLog) error {
	_, err := d.Exec(`INSERT INTO request_logs(id,link_id,path,provider_id,provider_name,model,status_code,latency_ms,success,error_message,attributes_json,created_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`,
		l.ID, l.LinkID, l.Path, l.ProviderID, l.ProviderName, l.Model, l.StatusCode, l.LatencyMS, btoi(l.Success), l.ErrorMessage, enc(l.Attributes), l.CreatedAt.UTC())
	return err
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
