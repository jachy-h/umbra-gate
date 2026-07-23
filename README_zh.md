[English](./README.md)

# UmbraGate

> 一道网关，所有模型。零摩擦。

**UmbraGate** 是一个单一二进制的生产级 LLM 网关。一行命令，你就拥有了统一 OpenAI 兼容端点，背后对接任意模型提供商 —— 附带智能故障切换、内置统计分析与完整 Web 控制台。无需 Docker，无需配置数据库，开箱即用。

```bash
brew tap jachy-h/umbragate
brew trust --tap jachy-h/umbragate
brew install umbragate
umbragate start
# 打开 http://localhost:8787 —— 搞定。
```

## 为什么选 UmbraGate

- **单二进制，零依赖。** API 网关、管理后台、SQLite 全部内嵌。下载即跑。
- **链路式故障切换。** 按优先级堆叠多个 Provider，每个可配置重试次数、状态码规则、错误匹配、超时策略和降级模型。一个挂了，下一个顶上。
- **按属性统计。** 给链接打上 `键:值` 属性标签，统计按 链接 × Provider × 属性 × 小时 自动聚合 —— 成本分摊、用量追踪一步到位。
- **两类原生协议风格。** Link 明确区分 OpenAI Style 与 Anthropic Style，二者不能混编。OpenAI Chat Completions、Responses 与 Anthropic Messages 均通过厂商官方 Go SDK 发出；兼容 Provider 可声明多个端点格式和 Base URL。
- **自带 Web 控制台。** React SPA 随二进制一同打包。在浏览器里管理链接、配置链路、查看统计 —— 不用敲 CLI，不用写配置（当然 config.yaml 需要时也在）。

## 快速上手

```bash
brew tap jachy-h/umbragate && brew trust --tap jachy-h/umbragate && brew install umbragate
umbragate start
```

或从源码构建：`make && ./umbragate`（需要 Go + Node.js）。

1. 打开 **http://localhost:8787** — 内置 Web 控制台。
2. DeepSeek、OpenCode 和 OpenCode Go 已预置；填入 API Key，或按需创建其他 Provider。
3. 创建代理链接，按优先级堆叠 Provider，配置故障切换规则。
4. 复制链接 URL，填入你喜欢的 AI 客户端 —— OpenCode、Cursor、ChatGPT 客户端，或任何 OpenAI 兼容工具。

OpenAI Style Link 提供 `/v1/chat/completions` 与 `/v1/responses`；Anthropic Style Link 提供 `/v1/messages`。

搞定。请求自动带故障切换、日志记录和统计分析。

所有数据存放在 `~/.umbragate/` 下 —— 配置、数据库都在这里。迁移或重置只需移动该目录。

启动时会打印实际配置文件路径。首次启动自动生成的配置文件包含所有字段说明：请求日志默认保留 7 天，数据库默认上限为 1 GiB（超限时清理最旧的 1,000 条请求日志），小时统计默认保留 365 天。后台日志每 50 MiB 或跨天轮转，保留 7 个 gzip 压缩副本。

## 进程生命周期

以下命令可在后台运行并管理本地进程：

```bash
umbragate start
umbragate status
umbragate restart
umbragate stop
umbragate run
umbragate --help
```

`start` 在后台运行，`run` 在前台运行。后台启动后，`start` 和 `status` 会显示 Web UI URL，可直接从终端打开。两种模式默认都使用 `~/.umbragate/config.yaml`。自定义配置可使用 `umbragate start -config /path/to/config.yaml`、`umbragate restart -config /path/to/config.yaml` 或 `umbragate run -config /path/to/config.yaml`。运行时文件位于 `~/.umbragate/`：`umbragate.pid` 记录后台进程，`umbragate.url` 记录 Web UI URL，`umbragate.log` 保存输出。不带命令执行 `umbragate` 等同于 `umbragate run`。

---

[管理 API 参考](https://github.com/jachy-h/umbragate) &nbsp;|&nbsp; [English](./README.md)
