# Chat Completions 与 Responses 双向转换方案

## 状态

- 状态：已实现
- 目标客户端：nanobot 及其他 OpenAI Chat Completions / Responses API 客户端
- 转换范围：仅允许 `chat_completions` 与 `responses` 互相转换

## 1. 目标

UmbraGate 的 Link 对客户端提供稳定的 OpenAI 协议入口，同时允许链路中的 Provider 使用另一种 OpenAI API 格式。每次请求的格式由调用的操作 URL 决定。

以 nanobot 为例：

1. nanobot 可通过 Chat Completions 或 Responses 调用同一个 Link。
2. 访问 Chat URL 时，UmbraGate 将请求转换为 OpenCode Go 所需的 Responses 格式。
3. UmbraGate 将 OpenCode Go 的 Responses 响应转换回此次请求 URL 对应的格式。
4. OpenCode Go 欠费、超时、返回错误或转换失败时，继续调用下一个节点。
5. 下一个 DeepSeek 节点原生返回 Chat 响应，无需转换。

推荐的 nanobot Base URL：

```text
http://localhost:8787/llm-gateway-lite/{link-path}/v1
```

nanobot 实际请求：

```text
POST /llm-gateway-lite/{link-path}/v1/chat/completions
```

不建议把 `/v1/responses` 操作 URL 直接配置为 nanobot 的 Base URL。

## 2. 范围边界

### 2.1 允许的转换

只注册以下两条转换路径：

```text
chat_completions -> responses
responses -> chat_completions
```

相同格式不属于转换，直接透传：

```text
chat_completions -> chat_completions
responses -> responses
```

### 2.2 明确不允许

以下转换不在本方案范围内：

- `messages -> chat_completions`
- `messages -> responses`
- `chat_completions -> messages`
- `responses -> messages`
- 任意其他 Provider 私有格式转换

Anthropic Messages Link 和 Provider 继续使用原生 Messages 格式。路由器不得通过 Chat/Responses 转换器把 Anthropic 节点加入 OpenAI Link。

### 2.3 兼容能力

首期必须支持：

- system、developer、user、assistant 文本消息
- 非流式文本响应
- SSE 流式文本
- function/tool calls
- tool call 参数增量
- tool result
- finish reason
- usage
- reasoning 文本或 reasoning delta
- JSON Schema 结构化输出中两种协议均可表达的部分

以下 Responses 专属能力不能静默丢弃：

- hosted `web_search`
- `file_search`
- `computer_use`
- background response
- `previous_response_id` 服务端会话状态
- 无法映射的 Responses item/event
- 加密 reasoning item

遇到不支持的能力时，转换必须失败，并按 Link 的 fallback 规则尝试下一个节点。

## 3. Link 与 Provider 契约

Link 只定义协议和 Provider 链，不固定客户端格式。路由从操作 URL 推断本次请求与返回格式：

| 操作 URL | 请求格式 | 返回格式 |
|---|---|---|
| `/v1/chat/completions` | `chat_completions` | `chat_completions` |
| `/v1/responses` | `responses` | `responses` |

因此同一个 OpenAI Link 可同时服务 nanobot 的 Chat 与 Responses 调用。

Provider endpoint 继续描述真实的上游格式：

```go
type ProviderEndpoint struct {
    Protocol       string `json:"protocol"`
    RequestFormat  string `json:"request_format"`
    ResponseFormat string `json:"response_format"`
    BaseURL        string `json:"base_url"`
}
```

OpenCode Go：

```json
{
  "protocol": "openai",
  "request_format": "responses",
  "response_format": "responses",
  "base_url": "https://opencode.ai/zen/go/v1/responses"
}
```

DeepSeek：

```json
{
  "protocol": "openai",
  "request_format": "chat_completions",
  "response_format": "chat_completions",
  "base_url": "https://api.deepseek.com/v1/chat/completions"
}
```

## 4. 路由选择

路由器不能再要求 Provider 响应格式与客户端请求格式完全相同。

节点可用条件：

```go
canConvertRequest(requestFormatFromURL, endpoint.RequestFormat) &&
canConvertResponse(endpoint.ResponseFormat, requestFormatFromURL)
```

规则：

1. 格式相同可以直接透传。
2. 格式不同只能使用本方案注册的 Chat/Responses 转换器。
3. 没有合法转换路径时，跳过当前节点并记录原因。
4. 必须保持 Link 配置的节点顺序，转换能力不能改变优先级。

每个节点的完整执行顺序：

```text
选择 endpoint
-> 转换客户端请求
-> 调用 Provider
-> 验证 Provider 原生响应
-> 转换为客户端请求 URL 对应的响应格式
-> 验证转换后的客户端响应
-> 返回客户端
```

以下任何一步失败，都将当前节点记录为失败并进入下一个节点：

- 请求转换失败
- Provider 网络错误或超时
- 欠费、额度耗尽、限流
- Provider 返回配置规则指定的错误状态
- Provider 原生响应校验失败
- 响应转换失败
- 转换后的客户端响应校验失败

## 5. 转换模块

建议新增：

```text
internal/protocol/
├── converter.go
├── canonical.go
├── chat.go
├── responses.go
└── stream.go
```

接口：

```go
type Converter interface {
    CanConvertRequest(from, to string) bool
    ConvertRequest(body []byte, from, to string) ([]byte, error)

    CanConvertResponse(from, to string) bool
    ConvertResponse(body []byte, from, to string) ([]byte, error)

    ConvertStream(
        reader io.Reader,
        from string,
        to string,
        writer io.Writer,
    ) error
}
```

转换默认采用严格模式：

- 不静默删除不支持的字段。
- 不把 Responses JSON 原样返回给 Chat 客户端。
- 不把 Chat SSE 原样返回给 Responses 客户端。
- 无法保证语义时返回明确的转换错误。

## 6. 流式转换

流式协议不能使用简单字符串替换。两种格式先解析为内部统一事件，再编码为目标格式：

```go
type Event struct {
    Type      EventType
    Text      string
    Reasoning string
    ToolCall  *ToolCall
    Usage     *Usage
}
```

内部事件至少包括：

```text
text_delta
reasoning_delta
tool_call_start
tool_call_delta
tool_call_done
usage
completed
error
```

示例流程：

```text
Responses SSE
-> canonical Event
-> Chat Completions SSE
-> nanobot
```

工具调用映射必须保持：

```text
Responses function_call.call_id
-> Chat tool_calls[].id

Responses function_call.name
-> Chat tool_calls[].function.name

Responses function_call.arguments
-> Chat tool_calls[].function.arguments
```

必须保留参数增量顺序，并在结束时生成正确的 `finish_reason=tool_calls`。

## 7. 请求记录

每个 Provider attempt 都单独记录转换信息。转换标记必须使用明确字段，不能复用统计 attributes。

### 7.1 数据模型

在 `RequestLog` 中增加：

```go
type RequestLog struct {
    // Existing fields omitted.

    ClientRequestFormat    string `json:"client_request_format,omitempty"`
    ProviderRequestFormat  string `json:"provider_request_format,omitempty"`
    ProviderResponseFormat string `json:"provider_response_format,omitempty"`
    ClientResponseFormat   string `json:"client_response_format,omitempty"`

    RequestConverted  bool   `json:"request_converted"`
    ResponseConverted bool   `json:"response_converted"`
    ConversionError   string `json:"conversion_error,omitempty"`

    // Existing response_body keeps the Provider-native response.
    ClientResponseBody string `json:"client_response_body,omitempty"`
}
```

字段语义：

| 字段 | 内容 |
|---|---|
| `request_body` | Agent 发给 UmbraGate 的原始请求 |
| `upstream_body` | UmbraGate 发给 Provider 的请求，可能已经转换 |
| `response_body` | Provider 返回的原始响应 |
| `client_response_body` | UmbraGate 最终返回 Agent 的响应，可能已经转换 |
| `request_converted` | 请求是否实际发生格式转换 |
| `response_converted` | 响应是否实际发生格式转换 |
| `conversion_error` | 请求或响应转换失败原因 |

数据库新增对应列，旧记录使用空格式、`false` 和空错误，保持兼容。

### 7.2 记录时机

请求转换完成后立即设置：

```go
log.ClientRequestFormat = requestFormatFromURL
log.ProviderRequestFormat = endpoint.RequestFormat
log.RequestConverted = requestFormatFromURL != endpoint.RequestFormat
```

收到 Provider 响应后设置：

```go
log.ProviderResponseFormat = endpoint.ResponseFormat
log.ClientResponseFormat = requestFormatFromURL
log.ResponseConverted = endpoint.ResponseFormat != requestFormatFromURL
```

转换失败时：

```go
log.Success = false
log.ConversionError = err.Error()
log.ErrorMessage = "response conversion failed: " + err.Error()
```

即使转换失败并 fallback，失败节点的记录也必须保留。

## 8. 请求列表展示

请求列表在 Provider 名称旁显示转换 Badge。

推荐文案：

| 情况 | Badge |
|---|---|
| 未转换 | 不显示 |
| 仅请求转换 | `Converted · Chat → Responses` |
| 仅响应转换 | `Converted · Responses → Chat` |
| 请求和响应均转换 | `Converted · Chat ↔ Responses` |
| 转换失败 | `Conversion Failed` |

颜色建议：

- 成功转换：violet
- 转换失败：error/red

Badge 的 tooltip 显示完整链路，例如：

```text
Client request: Chat Completions
Provider request: Chat Completions
Provider response: Responses
Client response: Chat Completions
```

请求列表至少修改：

- `web/src/types.ts`
- `web/src/pages/StatsDashboard.tsx`
- Link Test 请求列表所在页面

## 9. 请求详情展示

`RequestDetailsModal` 顶部增加 “Protocol conversion” 区域：

```text
Agent request       Chat Completions
Provider request    Chat Completions
Provider response   Responses
Agent response      Chat Completions
Result              Responses → Chat converted
```

详情中的四个正文区域：

1. Agent → Gateway：`request_body`
2. Gateway → Provider：`upstream_body`
3. Provider → Gateway：`response_body`
4. Gateway → Agent：`client_response_body`

如果没有转换，详情可以折叠重复内容；如果发生转换，四个区域必须可见。

转换失败时，在详情顶部显示：

```text
Conversion failed: {conversion_error}
```

详情至少修改：

- `web/src/types.ts`
- `web/src/components/RequestDetailsModal.tsx`

## 10. Link Test

Link Test 必须明确选择要模拟的客户端操作 URL 格式，而不是仅验证 Provider endpoint。

每个节点展示：

```text
Provider request      Passed
Provider response     Passed (Responses)
Response conversion   Passed (Responses → Chat)
Client validation     Passed (Chat Completions)
```

如果 Provider 请求成功但转换失败，节点验证状态必须是失败，且 `validation_error` 包含转换阶段。

## 11. 验收标准

### 11.1 路由与 fallback

- Chat Link 的第一个 Responses 节点可以被选择，不能因为格式不同被跳过。
- 第一个节点成功且转换成功时，不调用后续节点。
- 第一个节点欠费、超时、原生响应非法或转换失败时，调用下一个节点。
- DeepSeek Chat fallback 返回内容能被 nanobot 正常解析。

### 11.2 非流式

- Chat 请求可以转换为 Responses 请求。
- Responses 响应可以转换为 Chat 响应。
- function calls 和 tool results 可完成一轮 Agent 调用。
- 不支持的 Responses 专属能力触发明确错误和 fallback。

### 11.3 流式

- nanobot 能持续收到文本 delta。
- tool call ID、名称和参数增量顺序正确。
- 流结束标记和 finish reason 正确。
- 中途解析失败时关闭当前上游并进入允许的 fallback 流程。

### 11.4 可观测性

- 发生转换的 attempt 在请求列表中有 Badge。
- 请求详情能看到四种格式、转换方向和转换结果。
- 详情能对比原始请求、Provider 请求、Provider 响应和客户端响应。
- 转换失败的 attempt 仍有完整记录。
- 未转换的旧请求和新请求继续正常展示。

## 12. 实施顺序

1. 增加转换接口、格式能力判断和非流式文本转换。
2. 增加 function/tool call 与 tool result 转换。
3. 增加流式统一事件模型和 SSE 双向编码。
4. 修改路由选择与 fallback，使转换失败成为节点失败。
5. 扩展 RequestLog、SQLite migration 和 API。
6. 在请求列表与详情中展示转换标记和四段数据。
7. 修改 Link Test，使其验证完整转换链路。
8. 使用真实 `nanobot agent -m "hi"` 和一次工具调用完成端到端验收。
