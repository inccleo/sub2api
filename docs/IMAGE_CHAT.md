# 对话画图

`/images` 是会话式图片工作区：左侧是对话历史，中间按轮次展示生成结果（灯箱预览、下载、继续编辑），底部是合并在一起的输入区（提示词、参考图、模型、质量、尺寸、张数）。

页面嵌在常规 `AppLayout` 中，前端只调用 Sub2API 的登录态接口，chatgpt2api 的密钥不会出现在浏览器里。

## 请求链路

生成任务和模型列表都走分组账号，不额外维护下游密钥：

```
浏览器 → Sub2API /api/v1/image-workbench/*
       → 分组账号（OpenAI 平台 + 允许生图，如 gpt-image-2）
       → chatgpt2api
```

- `POST /api/v1/image-workbench/tasks`：提交生成/编辑任务。带参考图时走 multipart 并转发到 `chatgpt2api` 的 `edits` 异步接口，否则走 `generations` 异步接口。
- `GET /api/v1/image-workbench/tasks/:task_id`：轮询任务结果。
- `GET /api/v1/image-workbench/models`：模型下拉列表。
- `GET /api/v1/image-workbench/config`：尺寸、质量、张数上限等能力声明。

## 模型列表

模型列表不再硬编码，按以下顺序解析：

1. 取当前用户可用的生图分组，并**只保留能连到 chatgpt2api 的分组**（即该分组下存在带 `base_url` 凭据的可调度 OpenAI 账号），在其中取排序最靠前的那个，用其账号的 `base_url` + `api_key` 作为上游。
2. 请求 `GET /api/model-catalog`，直接使用返回的 `image_models`（chatgpt2api 新版本，包含 `plus-codex-gpt-image-2` 这类按账号派生的模型）。
3. 上游版本较旧没有该接口时，退回请求 `GET /v1/models` 并过滤出带 `image` 标记的模型 ID。
4. 上游不可达时使用内置的兜底列表，接口返回 `"source": "fallback"`，页面提示使用内置模型。

> `allow_image_generation` 只是计费/权益开关，不代表该分组的账号能连到 chatgpt2api。若按排序选中了只有官方账号（无 `base_url`）的分组，模型目录与异步生图端点都无法解析，页面会一直提示使用内置模型。因此分组选择与上游解析共用同一套规则：选中的分组同时决定模型列表来源和生图流量去向。
>
> 若所有分组都不满足条件，会退回按排序选择，保证工作台仍可用。

结果按上游地址做 5 分钟缓存，避免频繁刷新页面反复请求 chatgpt2api。

### 可选：直接指定下游地址

分组账号不可用时（例如只想单独暴露一个模型目录），可以在 `config.yaml` 或环境变量中配置 `image_chat`：

```yaml
image_chat:
  enabled: true
  base_url: http://127.0.0.1:3002
  api_key: your-chatgpt2api-key
  timeout_seconds: 300
```

对应环境变量为 `IMAGE_CHAT_ENABLED`、`IMAGE_CHAT_BASE_URL`、`IMAGE_CHAT_API_KEY` 和 `IMAGE_CHAT_TIMEOUT_SECONDS`。配置的地址只在分组账号解析失败时用于读取模型列表。
