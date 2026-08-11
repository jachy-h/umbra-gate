# UmbraGate

[English](./README.md)

> 一个可靠端点，连接所有 LLM。

**UmbraGate** 是一个自包含的 LLM 网关：为所有 AI 客户端提供一个兼容 OpenAI 的端点，而后端模型由你自由选择。路由请求、自动故障切换、查看用量，全都在本地 Web 控制台完成——无需 Docker，无需配置数据库。

```bash
brew tap jachy-h/umbragate
brew trust --tap jachy-h/umbragate
brew install umbragate
umbragate start
# 打开 http://localhost:8787
```

## 用户快速上手

1. 打开 **<http://localhost:8787>**。
2. 为预置 Provider 填入 API Key，或创建自己的 Provider。
3. 创建一个 Link：将性价比更高的 Provider 放在前面，备用 Provider 放在后面。当前者额度耗尽、被限流、超时或流量不稳定时，UmbraGate 会自动尝试备用 Provider。
4. 将 Link URL 填入 OpenCode、Cursor、ChatGPT 客户端或任何 OpenAI 兼容客户端。

客户端始终只需一个端点；UmbraGate 在背后完成路由和故障切换。

## 一个控制台，完成路由与验证

创建代理 Link，确认能力检查结果，在客户端开始使用前看清整条链路。保存 Link 时，控制台会流式显示每个 Provider 的真实校验进度，并高亮当前正在校验的 Provider。

![代理链接展示链路顺序与共享 API 能力](./imgs/links.png)

按 Link 和时间范围筛选，查看请求量、成功率、失败数、延迟及最近的 Provider 尝试。

![统计页面展示 Link 维度的请求量、可靠性、延迟与最近请求](./imgs/statistics.png)

## 面向开发者

可从源码构建并运行：`make && ./umbragate run`（需要 Go 和 Node.js）。

- UmbraGate 为应用提供一个兼容 OpenAI 的端点，并通过 Link 中配置的 Provider 路由请求。
- Provider 可声明 OpenAI 与 Anthropic 协议端点。第一个节点确定 Link 协议；每个回退节点都必须支持该协议。
- 每个链路节点都有按顺序排列的模型优先级（传入的请求模型或固定覆盖模型）以及重试次数。UmbraGate 会依序尝试这些模型优先级，再切换到下一个回退 Provider。
- 它会探测每个 OpenAI 节点对 Chat Completions 与 Responses 的支持，只暴露 Link 全链共同支持的格式；Anthropic Messages 保持原生协议。
- 只有当 Link 的能力检查结果中出现相应格式时，才可调用 `/v1/chat/completions` 或 `/v1/responses`。Anthropic 原生节点提供 `/v1/messages`。
- 可按项目或使用场景给 Link 加标签；小时统计汇总请求量、成功率、延迟和 Provider 尝试记录。

## 运行与维护

所有数据——配置和数据库——均位于 `~/.umbragate/`；移动这个目录即可迁移或重置本地安装。启动时会打印实际使用的配置文件路径；首次启动生成的配置文件说明了全部选项。

默认情况下，请求日志保留 7 天；数据库上限为 1 GiB，超限时删除最旧的 1,000 条请求日志；小时聚合统计保留 365 天。后台日志按天或达到 50 MiB 时轮转，并保留 7 个压缩备份。

```bash
umbragate start
umbragate status
umbragate restart
umbragate stop
umbragate run
umbragate --help
umbragate version # 或：umbragate -v
```

`start` 在后台运行，`run` 在前台运行。后台启动后，`start` 与 `status` 会打印 Web UI URL。UmbraGate 已在运行时再次执行 `start`，会显示状态而非报错。两种模式默认使用 `~/.umbragate/config.yaml`。自定义配置可使用 `umbragate start -config /path/to/config.yaml`、`umbragate restart -config /path/to/config.yaml` 或 `umbragate run -config /path/to/config.yaml`。

运行时文件位于 `~/.umbragate/`：`umbragate.pid` 记录后台进程，`umbragate.url` 记录 Web UI URL，`umbragate.log` 保存输出。不带命令执行 `umbragate` 会显示帮助；请使用 `umbragate run` 在前台运行。

## 发布验证

每个版本在发布前都会经过验证：CI 构建 React 前端，确认前端已嵌入，编译 Go 二进制，并运行 Go 测试套件。发布归档包含自包含二进制和 `config.yaml`，同时提供 Apple Silicon 与 Intel Mac 版本。

---

[管理 API 参考](https://github.com/jachy-h/umbragate) &nbsp;|&nbsp; [English](./README.md)
