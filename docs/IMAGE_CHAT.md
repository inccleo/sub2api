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

1. 获取或创建当前用户的工作台专用密钥，使用它**实际绑定的分组**读取模型。启用 `image_chat` 时，只有可调度 OpenAI 账号的 `base_url` 与 `image_chat.base_url` 匹配的分组才合格；新密钥按排序选择合格分组，已有密钥保留合格绑定。原绑定不合格时仅改绑工作台专用密钥，不调整用户自己的 API 密钥。
2. 请求 `GET /api/model-catalog`，直接使用返回的 `image_models`（chatgpt2api 新版本，包含 `plus-codex-gpt-image-2` 这类按账号派生的模型）。
3. 上游版本较旧没有该接口时，退回请求 `GET /v1/models` 并过滤出带 `image` 标记的模型 ID。
4. 上游不可达时使用内置的兜底列表，接口返回 `"source": "fallback"`，页面提示使用内置模型。

> `allow_image_generation` 只是计费/权益开关，不代表该分组的账号能连到 chatgpt2api。若按排序选中了只有官方账号（无 `base_url`）的分组，模型目录与异步生图端点都无法解析，页面会一直提示使用内置模型。因此分组选择与上游解析共用同一套规则：选中的分组同时决定模型列表来源和生图流量去向。
>
> 启用 `image_chat` 时，若所有可用分组都不满足条件，工作台返回不可用，不回退到其他中转分组；关闭 `image_chat` 时保留原有按排序回退行为。

结果按上游地址做 5 分钟缓存，避免频繁刷新页面反复请求 chatgpt2api。

### 配置 chatgpt2api 地址

`image_chat` 默认启用，默认地址为 `http://127.0.0.1:3002`。可以在 `config.yaml` 或环境变量中配置：

```yaml
image_chat:
  enabled: true
  base_url: http://127.0.0.1:3002
  api_key: your-chatgpt2api-key
  timeout_seconds: 300
```

对应环境变量为 `IMAGE_CHAT_ENABLED`、`IMAGE_CHAT_BASE_URL`、`IMAGE_CHAT_API_KEY` 和 `IMAGE_CHAT_TIMEOUT_SECONDS`。启用时，配置地址用于分组账号准入匹配；分组账号解析失败时也可用于读取模型目录，但这不会绕过生成任务的分组准入检查。
