[English](./README.md)

# UmbraGate

> 一道网关，所有模型。零摩擦。

**UmbraGate** 是一个单一二进制的生产级 LLM 网关。一行命令，你就拥有了统一 OpenAI 兼容端点，背后对接任意模型提供商 —— 附带智能故障切换、内置统计分析与完整 Web 控制台。无需 Docker，无需配置数据库，开箱即用。

```bash
brew tap jachy-h/umbragate
brew trust --tap jachy-h/umbragate
brew install umbragate
umbragate
# 打开 http://localhost:8787 —— 搞定。
```

## 为什么选 UmbraGate

- **单二进制，零依赖。** API 网关、管理后台、SQLite 全部内嵌。下载即跑。
- **链路式故障切换。** 按优先级堆叠多个 Provider，每个可配置重试次数、状态码规则、错误匹配、超时策略和降级模型。一个挂了，下一个顶上。
- **按属性统计。** 给链接和请求打上 `键:值` 属性标签，统计按 链接 × Provider × 属性 × 小时 自动聚合 —— 成本分摊、用量追踪一步到位。
- **Provider 来者不拒。** 原生支持 OpenAI、Anthropic、Gemini、DeepSeek、Qwen。任意 OpenAI 兼容 API 即可作为自定义 Provider。热加载，无需重启。
- **自带 Web 控制台。** React SPA 随二进制一同打包。在浏览器里管理链接、配置链路、查看统计 —— 不用敲 CLI，不用写配置（当然 config.yaml 需要时也在）。

## 30 秒上手

```bash
# 1. 添加 Provider
curl -X POST localhost:8787/admin/providers -H 'Content-Type: application/json' -d '{
  "name":"openai","type":"openai","base_url":"https://api.openai.com",
  "api_key":"sk-...","models":["gpt-4o-mini"],"enabled":true}'

curl -X POST localhost:8787/admin/providers -H 'Content-Type: application/json' -d '{
  "name":"deepseek","type":"deepseek","base_url":"https://api.deepseek.com",
  "api_key":"sk-...","models":["deepseek-chat"],"enabled":true}'

# 2. 创建代理链接，配置故障切换链路
curl -X POST localhost:8787/admin/links -H 'Content-Type: application/json' -d '{
  "name":"my-gateway","attributes":{"team":"core"},
  "chain":[
    {"provider_id":"<openai-id>","retry_count":1,"rules":{"on_status_codes":[429,500,503]}},
    {"provider_id":"<deepseek-id>","fallback_model":"deepseek-chat"}
  ]}'

# 3. 调用 —— 标准 OpenAI SDK / curl 兼容
curl -X POST localhost:8787/llm-gateway-lite/<path>/v1/chat/completions \
  -H 'Content-Type: application/json' \
  -d '{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hello"}]}'
```

## 安装

```bash
brew tap jachy-h/umbragate && brew trust --tap jachy-h/umbragate && brew install umbragate   # macOS / Linux
# 或者：make && ./umbragate                           # 从源码构建（需要 Go + Node.js）
```

所有数据存放在 `~/.umbragate/` 下 —— 配置、数据库都在这里。迁移或重置只需移动该目录。

---

[管理 API 参考](https://github.com/jachy-h/umbragate) &nbsp;|&nbsp; [English](./README.md)
