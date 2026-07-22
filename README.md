# llm-gateway-lite

简化版 LLM Gateway：生成可配置属性的代理链接，按 provider 链逐级 fallback，并对每个链接及其属性做统计。OpenAI Chat Completions 兼容。

## 特性

- **代理链接**：为每条链接生成一个 path token，可配置名称、`attributes`（任意键值，用于后续统计维度）、是否启用。
- **Provider 链 + Fallback**：每条链接挂载多条 provider（顺序即 fallback 优先级），每条可配置：
  - `retry_count`：同一 provider 额外重试次数
  - `fallback_model`：降级时使用的替代模型
  - `rules.on_status_codes`：命中即 fallback（如 `[429,500,503]`）
  - `rules.on_errors`：错误信息包含即 fallback
  - `rules.on_timeout`：超时即 fallback
  - 未配置规则时，传输错误与 5xx 默认触发 fallback
- **统计**：每次请求落 `request_logs`，后台定时聚合（默认 60s）到 `stats_hourly`，按 `link_id × provider_id × 属性 key/value × 小时` 维度汇总（总请求数、成功、失败、累计延迟）。
- **Provider 注册**：内置 OpenAI / Anthropic / Gemini / DeepSeek / Qwen；`type=custom` 走 OpenAI 兼容协议；用户可经管理 API 增删任意 provider。

## 运行

```bash
go mod tidy
go run . -config config.yaml
```

## 管理 API（默认 `/admin`，可加 `X-Admin-Token`）

| Method | Path | 说明 |
|---|---|---|
| GET | `/admin/types` | 支持的 provider 类型 |
| GET/POST | `/admin/providers` | 列出 / 新建 provider |
| GET/DELETE | `/admin/providers/:id` | 查 / 删 |
| GET/POST | `/admin/links` | 列出 / 新建代理链接 |
| GET/DELETE | `/admin/links/:id` | 查 / 删 |
| GET | `/admin/stats?link_id=&from=&to=` | 统计（`from/to` 为小时 bucket，如 `2026-07-21T18`） |

## 代理入口（OpenAI 兼容）

```
POST /llm-gateway-lite/{path}/v1/chat/completions
```

请求体与 OpenAI `/v1/chat/completions` 一致；可经 `X-Gateway-Attributes`（JSON）头补充本次请求的统计属性（与链接属性并集）。

## 示例

```bash
# 1. 添加 provider
curl -X POST localhost:8787/admin/providers -H 'Content-Type: application/json' -d '{
  "name":"openai-primary","type":"openai","base_url":"https://api.openai.com","api_key":"sk-...","models":["gpt-4o-mini"],"enabled":true}'

curl -X POST localhost:8787/admin/providers -H 'Content-Type: application/json' -d '{
  "name":"deepseek-fallback","type":"deepseek","base_url":"https://api.deepseek.com","api_key":"sk-...","models":["deepseek-chat"],"enabled":true}'

# 2. 创建代理链接：配置属性 + provider 链 fallback
curl -X POST localhost:8787/admin/links -H 'Content-Type: application/json' -d '{
  "name":"demo","attributes":{"team":"core","usecase":"chat"},
  "chain":[
    {"provider_id":"<openai-primary-id>","retry_count":1,"rules":{"on_status_codes":[429,500,503],"on_timeout":true}},
    {"provider_id":"<deepseek-fallback-id>","fallback_model":"deepseek-chat"}
  ]}'

# 3. 通过链接调用
curl -X POST localhost:8787/llm-gateway-lite/<path>/v1/chat/completions \
  -H 'Content-Type: application/json' \
  -d '{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hi"}]}'

# 4. 查看统计
curl 'localhost:8787/admin/stats?link_id=<link-id>'
```

## 目录结构

```
internal/
  config/   配置加载
  db/       SQLite 连接、迁移与仓储
  models/   数据模型
  providers/ provider 适配器与注册（openai/anthropic/gemini/deepseek/qwen/custom）
  proxy/    转发与 fallback 调度
  stats/    请求日志与定时聚合
  api/      管理 API 与代理入口
  server/   Gin 装配
main.go     启动
```

> 简化版：暂只支持非流式（non-streaming）请求；流式请求会被转发给上游，响应可能不被正确转换。