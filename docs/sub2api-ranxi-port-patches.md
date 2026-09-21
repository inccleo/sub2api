# ranxi 底座补丁清单：Desktop OAuth + Image Workbench

日期：2026-09-21  
底座：`ranxi2001/sub2api` `v2.7.7`（`cee3c547`）  
来源：`inccleo/sub2api` `custom-main`（`bd60cadc2`）  
原则：以 ranxi 为新主线，把现网定制打上去；不要把 DailyCheckin / Kimi 设备码一起塞进这两包。

对照评估：[sub2api-ranxi-parallel-eval.md](./sub2api-ranxi-parallel-eval.md)

旁路已证实：生产库副本能被 ranxi 读起来。P0 Desktop OAuth、P1 Image Workbench 均已打进 `sub2api-ranxi` 工作树并通过 `.eval-ranxi` 副本验证；`/api/v1/checkin/status` 仍为 404。

---

## 总顺序

1. **P0 Desktop OAuth**（独立、无 SQL、默认关闭，风险最低）
2. **P1 Image Workbench**（依赖现有 async image + 对象存储；要改 API Key 隐藏逻辑）
3. 品牌 / custom-release 另开一包，不和这两包混。
4. 签到、Kimi 设备码后置。

每包做完都在 `.eval-ranxi` 副本上验证，不切流量。`wire_gen.go` 一律 `go generate ./cmd/server`，不要手改。

---

## P0 Desktop OAuth

第一方授权码 + S256 PKCE。固定 `client_id=sub2api-desktop`、`redirect_uri=sub2api://oauth/callback`。无 SQL。状态在 Redis，TTL 60s。

### 整文件拷贝

从 `sub2api/` 拷到 ranxi 同路径：

| 文件 | 说明 |
|---|---|
| `backend/internal/handler/desktop_auth_handler.go` | Details / Authorize / Token |
| `backend/internal/service/desktop_auth_service.go` | 发码、换 JWT |
| `backend/internal/repository/desktop_auth_code_store.go` | Redis SETNX + Lua 消费 |
| `backend/internal/config/desktop_auth_test.go` | env opt-in |
| `backend/internal/server/routes/desktop_auth_test.go` | 换码/重放/过期 |
| `frontend/src/api/desktopAuth.ts` | GET/POST `/auth/desktop/authorize` |
| `frontend/src/views/auth/DesktopAuthorizeView.vue` | 必须点同意 |
| `frontend/src/i18n/locales/zh/desktopAuth.ts` | |
| `frontend/src/i18n/locales/en/desktopAuth.ts` | |
| `frontend/src/api/__tests__/desktopAuth.spec.ts` | 回调白名单 |
| `frontend/src/views/auth/__tests__/DesktopAuthorizeView.spec.ts` | |
| `docs/DESKTOP_AUTH.md` | |

不要带：`frontend/src/api/__tests__/tauriDesktopLogin.spec.ts`（依赖缺失的 `examples/tauri/desktop-auth.ts`）。

### 接入点（只加 DesktopAuth，不要带 ImageChat / DailyCheckin）

1. `backend/internal/config/config.go`
   - 在 `Config` 前加 `DesktopAuthConfig{Enabled bool}`
   - `Config` 加 `DesktopAuth DesktopAuthConfig \`mapstructure:"desktop_auth"\``
   - `setDefaults` 在 `webauthn.enabled` 前加 `viper.SetDefault("desktop_auth.enabled", false)`
   - ranxi 的 `env_reachability_test.go` 要求标量必须有 SetDefault，漏了则 `DESKTOP_AUTH_ENABLED` 静默无效
2. `backend/internal/handler/handler.go`：`Handlers` 加 `DesktopAuth *DesktopAuthHandler`（插在 `Auth` 旁）
3. `backend/internal/handler/wire.go`：`ProvideHandlers` 加参数并赋值；`ProviderSet` 加 `NewDesktopAuthHandler`
4. `backend/internal/service/wire.go`：`ProviderSet` 在 `ProvideAuthService` 前加 `NewDesktopAuthService`
5. `backend/internal/repository/wire.go`：`ProviderSet` 加 `NewDesktopAuthCodeStore`
6. `backend/internal/server/routes/auth.go`
   - 公开组，`POST /send-verify-code` 与 `POST /refresh` 之间：
     `POST /desktop/token` → `h.DesktopAuth.Token`，限流 `desktop-token` 20/min fail-close
   - 认证组，`GET /auth/me` 之前：
     `GET /auth/desktop/authorize` → `Details`
     `POST /auth/desktop/authorize` → `Authorize`，限流 `desktop-authorize` 20/min fail-close
7. `frontend/src/router/index.ts`：Setup 路由前插入

```ts
{
  path: '/oauth/authorize',
  name: 'DesktopAuthorize',
  component: () => import('@/views/auth/DesktopAuthorizeView.vue'),
  meta: { requiresAuth: true, title: 'Authorize desktop app', titleKey: 'desktopAuth.title' }
}
```

未登录 guard 已有 `redirect=to.fullPath`，不必改。
8. `frontend/src/i18n/locales/{zh,en}/index.ts`：只 import/导出 `desktopAuth`，不要带 `imageWorkbench`
9. `deploy/config.example.yaml`：补

```yaml
desktop_auth:
  enabled: false
```

### 配置

| key | 默认 | env |
|---|---|---|
| `desktop_auth.enabled` | `false`（opt-in） | `DESKTOP_AUTH_ENABLED` |

生产要启用才写 yaml/env。开关关着时路由仍注册，API 返回 **404** `desktop_auth_disabled`。

### 验证（`.eval-ranxi`，开关先 false 再 true）

- 未开：`GET /api/v1/auth/desktop/authorize` → 404 `desktop_auth_disabled`
- 打开 + 登录：`GET /oauth/authorize?...` 出同意页；点同意发码；`POST /api/v1/auth/desktop/token` 换 JWT
- Redis 停：授权/换码 503，不发无状态码
- 现网登录、API Key、账号池不受影响

### 进度（2026-09-21）

**已完成，未切流量。** 代码打在 `sub2api-ranxi` 工作树，未提交、未推远程。

测试：

| 项 | 结果 |
|---|---|
| `go test` config / routes / middleware（Desktop 相关） | 通过 |
| vitest `desktopAuth` / `DesktopAuthorizeView` / i18n 完整性 | 15/15 通过 |
| wire | `go run -mod=mod github.com/google/wire/cmd/wire` 生成 `wire_gen.go`；`go.sum` 未改 |

`.eval-ranxi` 副本（生产库副本，不切流量）：

| 检查 | 结果 |
|---|---|
| 默认关：`GET /api/v1/auth/desktop/authorize` | 404 `desktop_auth_disabled` |
| 默认关：`POST /api/v1/auth/desktop/token` | 404 `desktop_auth_disabled` |
| 打开后 GET details | 200，`client_name=Sub2API Desktop`，管理员账号 `is_admin=true`，`Cache-Control: no-store` |
| deny | `sub2api://oauth/callback?error=access_denied&state=...`，无 code |
| approve + exchange | 发 43 位 code；换出 access/refresh JWT；`/api/v1/auth/me` 200 |
| 重放 code | 400 `invalid_grant` |
| 错误 redirect | 400 `invalid_client` |
| 原会话 `GET /api/v1/keys` | 仍 200，管理员 8 把 |

未测：Redis 宕机 503（单测已覆盖 `redis_down`）；前端同意页未嵌入此次评估二进制（API 已通）。

额外接入点（相对原清单）：

- `backend_mode_guard.go`：允许 `/auth/desktop/token`
- `audit_log.go`：desktop token 记为 login，请求体不入库
- `.gitignore`：放行 `docs/DESKTOP_AUTH.md`

评估配置已改回 `desktop_auth.enabled: false`。

---

## P1 Image Workbench（对话画图）

JWT 面板 BFF，**不改** `/v1/images` 网关。浏览器只打 `/api/v1/image-workbench/*`；服务端用托管 API Key 改写到已有 async 生图。

无新 SQL。每用户一把系统 key：`name=__image_workbench__`，前缀 `sk-iwb-`。

### 整文件拷贝

| 文件 | 说明 |
|---|---|
| `backend/internal/handler/image_workbench_handler.go` | Config / Models / Submit / Get |
| `backend/internal/handler/image_workbench_upstream.go` | 按组解析 chatgpt2api `base_url` |
| `backend/internal/handler/image_workbench_handler_test.go` | |
| `backend/internal/handler/image_workbench_upstream_test.go` | |
| `backend/internal/service/api_key_service_image_workbench_test.go` | |
| `frontend/src/views/user/ImageWorkbenchView.vue` | `/images` |
| `frontend/src/views/user/__tests__/ImageWorkbenchView.spec.ts` | |
| `frontend/src/api/imageWorkbench.ts` | |
| `frontend/src/api/__tests__/imageWorkbench.spec.ts` | |
| `frontend/src/i18n/locales/zh/imageWorkbench.ts` | |
| `frontend/src/i18n/locales/en/imageWorkbench.ts` | |
| `docs/IMAGE_CHAT.md` | |

### 必须改的现有文件

1. `backend/internal/service/api_key_service.go`  
   移植：`ImageWorkbenchAPIKeyName`、`ErrImageWorkbenchUnavailable`、`IsImageWorkbenchAPIKey`、`SetImageWorkbenchGroupFilter`、`selectImageWorkbenchGroup`、`GetImageWorkbenchGroupID`、`GetOrCreateImageWorkbenchKey`。  
   选组规则：OpenAI + `AllowImageGeneration` + active；再 filter 能解析到 chatgpt2api `base_url` 的组；全空则退回候选（会打到官方账号）。  
   状态常量用现有 `StatusActive`（ranxi `domain_constants.go` 已有；`StatusAPIKeyActive` 同值 `"active"`）。
2. `backend/internal/service/api_key.go`：`APIKeyListFilters` 加 `ExcludeNames []string`
3. `backend/internal/repository/api_key_repo.go`：`apiKeyListByUserIDQuery` 对 `ExcludeNames` 做 `NameNEQ`
4. `backend/internal/handler/api_key_handler.go`：List 排除 `__image_workbench__`；Get/Update/Delete 对该 key 404；Create/Update 禁止该保留名
5. `backend/internal/server/routes/user.go`：`/keys` 组后、`/groups` 前插入

```go
imageWorkbench := authenticated.Group("/image-workbench")
imageWorkbench.Use(middleware.ClientRequestID())
{
    imageWorkbench.GET("/config", h.ImageWorkbench.Config)
    imageWorkbench.GET("/models", h.ImageWorkbench.Models)
    imageWorkbench.POST("/tasks", middleware.RequestBodyLimit(handler.ImageWorkbenchMaxRequestBody), h.ImageWorkbench.Submit)
    imageWorkbench.GET("/tasks/:task_id", h.ImageWorkbench.Get)
}
```

6. `backend/internal/handler/handler.go`：`AsyncImage` 后加 `ImageWorkbench *ImageWorkbenchHandler`
7. `backend/internal/handler/wire.go`：整段拷贝 `ProvideImageWorkbenchHandler`（来源 `sub2api/backend/internal/handler/wire.go` L15–31）；`ProvideHandlers` 加参数；`ProviderSet` 加入；补 `context` import
8. `backend/internal/config/config.go`：加 `ImageChatConfig` + `Config.ImageChat` + 四个 SetDefault
9. `backend/internal/handler/image_task_handler.go`：**必打**。ranxi Submit 现在直接用网关 path 当 `poll_url`。要加 `imageTaskPollBaseContextKey`，workbench 提交时覆盖为 `/api/v1/image-workbench/tasks/{id}`。漏了前端会拿 JWT 去轮询 `/v1/images/tasks/...` → 401。
10. `frontend/src/router/index.ts`：`/batch-image` 与 `/usage` 之间插入 `/images`（`fillHeight: true`）
11. `frontend/src/router/meta.d.ts`：加 `fillHeight?: boolean`
12. `frontend/src/components/layout/AppLayout.vue`：读 `fillHeight`，锁视口高度
13. `frontend/src/components/layout/AppSidebar.vue`：`/keys` 与 `/batch-image` 之间加 `{ path: '/images', label: t('nav.imageWorkbench'), hideInSimpleMode: true }`
14. `frontend/src/i18n/locales/{zh,en}/index.ts`：import `imageWorkbench`
15. `frontend/src/i18n/locales/{zh,en}/common.ts`：`nav.imageWorkbench`（zh=`对话画图`）

**不要改** `gateway.go` 的 `/v1/images/*`、batch image、openai images 原生实现。

### 配置

```yaml
image_chat:
  enabled: true                 # IMAGE_CHAT_ENABLED，默认 true
  base_url: http://127.0.0.1:3002
  api_key: ""
  timeout_seconds: 300
```

只用于**模型目录兜底**，不用于生图。ranxi 测试/生产应改成真实 chatgpt2api，或先 `enabled: false`，避免默认打本机 `:3002`。

对象存储未配时 Config 503（与现有 async image 同一闸门）。

### chatgpt2api 依赖

- 模型目录：组内可调度 OpenAI 账号的 `credentials.base_url` → `/api/model-catalog` 或 `/v1/models`
- 生图：托管 key 走正常 OpenAI gateway → 该组账号。官方 OAuth（无 `base_url`）解析不到上游
- 生产分组 `gpt-image-2`（id=18）和 `Codex（Pro）` 都开了 `allow_image_generation`。移植后 filter 会优先有 chatgpt2api endpoint 的组；都没有则退回 sort_order，页面能开但上游可能 4xx

### 验证

- `GET /api/v1/image-workbench/config`：对象存储开着 → 200；没开 → 503
- `GET /api/v1/image-workbench/models`：有 chatgpt2api 组 → catalog；否则 fallback `gpt-image-2` / `codex-gpt-image-2`
- 用户 `/keys` 列表看不到 `__image_workbench__`
- Submit 返回的 `poll_url` 以 `/api/v1/image-workbench/tasks/` 开头
- 原生 `POST /v1/images/generations`、batch-image 页仍可用
- 不打真实上游时只测到 Submit 受理即可，不要用生产账号刷图

### 进度（2026-09-21）

**已完成，未切流量。** 代码打在 `sub2api-ranxi` 工作树，未提交、未推远程。

测试：

| 项 | 结果 |
|---|---|
| `go test` handler `ImageWorkbench*` | 通过 |
| `go test -tags unit` service `ImageWorkbench*` | 8/8 通过 |
| vitest `imageWorkbench` / `ImageWorkbenchView` | 8/8 通过 |
| wire | `go run -mod=mod github.com/google/wire/cmd/wire` 生成 `wire_gen.go` |

`.eval-ranxi` 副本（生产库副本 + DB 内 `image_storage_config`，`image_chat.enabled: false`，不切流量）：

| 检查 | 结果 |
|---|---|
| `GET /api/v1/image-workbench/config` | 200，`ready=true` |
| `GET /api/v1/image-workbench/models` | 200，`source=fallback`，含 `gpt-image-2` / `codex-gpt-image-2` |
| `GET /api/v1/keys` | 200，7 把，**不含** `__image_workbench__` |
| `POST /api/v1/image-workbench/tasks` | 202，`poll_url` 以 `/api/v1/image-workbench/tasks/` 开头 |
| 未登录 `GET /api/v1/image-workbench/config` | 401（路由已注册，非 404） |

额外接入点（相对原清单）：

- `AppLayout.vue`：`fillHeight` 视口锁定布局
- `AppSidebar.vue`：nav 项带 `NEW` badge
- `.gitignore`：放行 `docs/IMAGE_CHAT.md`
- `deploy/config.example.yaml`：补 `image_chat` 段

---

## P2 TopOpenAI 品牌 + Custom Release

钴蓝主题 + 宽 logo 布局 + 固定方形 favicon；四段 tag 专用发布 workflow。

### 整文件拷贝

| 文件 | 说明 |
|---|---|
| `frontend/public/logo.svg` | TopOpenAI 方形 mark |
| `frontend/src/utils/branding.ts` + test | 固定 `/logo.svg` favicon |
| `frontend/src/components/layout/__tests__/AuthLayout.spec.ts` | 宽 logo 布局测试 |
| `.github/workflows/custom-release.yml` | 四段 tag `v*.*.*.*` 发布 |
| `FORK_MAINTENANCE.md` | fork 维护约定 |

### 必须改的现有文件

- `frontend/tailwind.config.js`：primary 色板 → Cobalt
- `frontend/src/main.ts`：`updateFavicon()` 无参
- `frontend/src/App.vue`：移除 siteLogo→favicon watch
- `frontend/src/components/layout/AuthLayout.vue`：`wideLogo` / `showBrandTitle` props
- `frontend/src/views/auth/LoginView.vue`：宽 logo、隐藏标题
- `frontend/src/components/layout/AppSidebar.vue`：宽 logo + VersionBadge 下移
- 8 处硬编码色（HomeView、KeyUsageView、Dashboard、3 charts、onboarding.css、SettingsView 进度条）
- `backend/internal/service/update_service.go` + test：`UPDATE_REPOSITORY`、四段 `compareVersions`
- `.github/workflows/release.yml`：排除四段 tag + 校验

### 进度（2026-09-21）

**已完成，未切流量。**

| 项 | 结果 |
|---|---|
| vitest `branding` / `AuthLayout` | 通过 |
| `go test -tags unit` `UpdateRepository` / 四段版本比较 | 通过 |

运行时：后台 `site_name` / `site_logo` 上传宽版横 logo；生产 systemd 设 `UPDATE_REPOSITORY=inccleo/sub2api`。

---

## P3 DailyCheckin（每日签到）

### Migration

| 文件 | 说明 |
|---|---|
| `239_daily_checkins.sql` | 内容同 fork 的 `193_daily_checkins.sql`；ranxi 的 193 已被 profit control 占用 |

生产库副本已有 `193_daily_checkins.sql` 记录；239 在副本上 no-op（表已存在）。

### 整文件拷贝

| 文件 |
|---|
| `backend/internal/repository/daily_checkin_repo.go` |
| `backend/internal/service/daily_checkin_service.go` + test |
| `backend/internal/handler/daily_checkin_handler.go` |
| `frontend/src/api/dailyCheckin.ts` |
| `frontend/src/api/admin/checkins.ts` |
| `frontend/src/components/common/DailyCheckinButton.vue` + test |
| `frontend/src/views/admin/CheckinsView.vue` |

### 必须改的现有文件

settings 链路（6 文件）、wire/routes（handler/service/repository/user/admin）、前端 router/AppHeader/AppSidebar/SettingsView/i18n/api。

### 进度（2026-09-21）

**已完成，未切流量。**

| 项 | 结果 |
|---|---|
| `go test -tags unit` service `DailyCheckin*` | 通过 |
| vitest `DailyCheckinButton` / i18n 完整性 | 通过 |
| wire | 已重新生成 |

`.eval-ranxi`：

| 检查 | 结果 |
|---|---|
| `GET /api/v1/checkin/status` | 200（非 404） |
| `GET /api/v1/admin/checkins/stats` | 200，`total_checkins=706` |
| `GET /api/v1/admin/checkins` | 200，`total=706` |

---

## 明确不要混进这些包

- Kimi 设备码 OAuth
- fork 其余 migration 冲突项（192 kimi quota、221 channel monitor kimi）——未随 P3 一并移植
- ranxi 的 Codex ticket / Mihomo（已经在底座里，测试实例按需开）

## 建议下一步

P0–P3 已在 `sub2api-ranxi` 打完并通过 `.eval-ranxi` 验证。下一步可选：

1. 提交/推远程 + embed 前端 build 冒烟（登录宽 logo、`/images`、`/admin/checkins`）
2. 打四段 custom tag 试跑 `custom-release.yml`
3. 其余 fork migration（192/221 kimi 等）按需单独处理
