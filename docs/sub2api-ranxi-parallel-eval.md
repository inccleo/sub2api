# Sub2API：ranxi 并行评估盘点

日期：2026-09-21

原则不变：保留 `inccleo/sub2api` 和生产部署；ranxi 只作为旁边的对照基线。不要把当前工作区切成 `ranxi2001/sub2api` 的 `production`。

## 已完成

1. 本地未提交改动已 stash：
   - `stash@{0}: wip: local Makefile/dev-compose/memory before ranxi parallel eval`
   - 内容：`backend/Makefile` 的 `run` target、`docker-compose.dev.yml`、`memory/2026-08-14-openai-image-octet-stream-data-url.md`
2. `sub2api/` 的 `custom-main` 已 fast-forward 到 `origin/custom-main`。
3. ranxi 已单独 clone 到 `/Users/leo/Desktop/topapi/sub2api`，固定 tag `v2.7.7`（detached HEAD `cee3c547`）。

## 对照基线

| 项 | 当前 fork | ranxi |
|---|---|---|
| 仓库 | `inccleo/sub2api` | `ranxi2001/sub2api` |
| 分支 / tag | `custom-main` @ `bd60cadc2` | `v2.7.7` @ `cee3c547` |
| 应用版本 | `0.2.5.2` | `2.7.7` |
| 发布源 | `UPDATE_REPOSITORY=inccleo/sub2api` | 写死 `githubRepo = "ranxi2001/sub2api"` |
| 生产 | Lightsail `api.topopenai.com`，systemd 二进制 | 未部署 |

注意：生产 runbook 记录的是 **2026-09-15 的 `v0.2.5.1`**。本地源码现已是 `v0.2.5.2`（favicon 热修 #20）。差量盘点以本地 `custom-main` 为准；切流量前要再核对生产实际运行版本。

## Schema / migration

同名 SQL **内容完全一致**（286 个共享文件 checksum 相同）。

当前 fork 多出 4 个 migration，ranxi 没有额外 SQL：

| 文件 | 作用 | 迁到 ranxi 的风险 |
|---|---|---|
| `192_user_platform_quotas_add_kimi.sql` | 给 `user_platform_quotas.platform` 加 `kimi` | 编号与上游 `192_group_profit_control.sql` 冲突；生产若已应用，ranxi 启动器可能看不到这条 |
| `193_daily_checkins.sql` | 新建 `daily_checkins` 表 | 编号与上游 `193_group_profit_control_auth_cache_invalidation.sql` 冲突 |
| `221_channel_monitor_v2_add_kimi.sql` | channel monitor 加 kimi | 编号与上游 `221_group_model_pricing.sql` 冲突 |
| `235a_group_model_allowlist_rollback_compat.sql` | 保留旧列 `models_list_config`，方便二进制回滚 | ranxi 没有对应兼容层；生产库已有双列 |

结论：

- 用生产库副本跑 ranxi，**大概率能起来**，因为 ranxi 没有更新的 schema 要跑。
- 真正的风险是 **当前 fork 多出来的表/列/约束**（签到表、235a 双列、kimi 约束补丁）在 ranxi 代码路径里不被认识，以及编号冲突导致日后无法干净合并 migration 历史。
- Ent schema 文件名两边都是 40 个，没有 ranxi 独有的新表。Codex ticket / Mihomo 目前走配置、Redis 和进程，不靠新 SQL。

## 当前有、ranxi 没有（必须逐项决定是否移植）

后端独有 Go 文件 29 个，前端独有 23 个。按产品能力分组：

1. **Desktop OAuth**
   - `/auth/desktop/authorize`、`desktop_auth` 配置（默认关闭）、Tauri 示例
   - 无独立 SQL，状态在 Redis
2. **Conversation image workbench**
   - `/images`、`/image-workbench/*`
   - 按分组解析能连到 `chatgpt2api` 的上游
   - 无独立 SQL
3. **每日签到**
   - `daily_checkins` 表 + 管理页 + 用户按钮
   - 有 SQL，ranxi 不认识这张表
4. **Kimi OAuth 设备码**
   - `KimiDeviceFlow`、kimi token refresh / usage
   - ranxi 有 kimi 平台约束，但没有这套 OAuth 设备码实现
5. **TopOpenAI 品牌 / 自定义前端**
   - 登录 logo、favicon 热修、自有 Custom release 工作流
6. **发布链路**
   - `.github/workflows/custom-release.yml`
   - `FORK_MAINTENANCE.md`
   - 生产脚本只吃 `inccleo/sub2api` 的四段 tag

## ranxi 有、当前没有（迁过去的收益）

后端独有 Go 文件 67 个，前端独有 21 个。主要是：

1. **Codex ticket**
   - 后台采集 / 注入 / HarvestFlow 页
   - `gateway.openai_codex_ticket`（默认关闭）
2. **Mihomo 出口管理**
   - 管理后台设置页、进程管理、打票代理
3. **DeepSeek / Codex 兼容**
   - Responses → Chat Completions、compact、工具历史
4. **独立发布和更新**
   - `deploy/install.sh`、Docker 镜像 `ghcr.io/ranxi2001/sub2api`
   - 应用内更新源写死 ranxi，不能直接接到现有 `inccleo` 发布
5. **Seedance 等周边网关能力**

这些都不改现有 SQL 编号，适合先在测试实例打开，不碰生产。

## 配置差异（测试实例要注意）

- ranxi 更新器写死 `ranxi2001/sub2api`。测试实例不要打开“在线更新”，也不要指向生产 `UPDATE_REPOSITORY`。
- Codex ticket、Mihomo 默认关闭；验证时按需打开，业务代理和打票出口必须分开。
- 当前 `desktop_auth.enabled` 默认 `false`；若生产已打开，迁 ranxi 后桌面授权会直接消失。
- 不要用 ranxi 的 `deploy/install.sh` 覆盖 `/opt/sub2api`。

## 明确不做

- 不改生产二进制、配置、数据库。
- 不把 `sub2api/` remote 改成 ranxi。
- 不把 stash 的本地开发文件混进任何基线。
- 不在当前目录做 merge / rebase。

## 旁路验证结果（2026-09-21，不切流量）

本地隔离目录：`.eval-ranxi/`（已加入 gitignore）。生产只做了只读 `pg_dump`，现网未改。

| 项 | 实际 |
|---|---|
| 应用 | ranxi `v2.7.7` darwin amd64 二进制 `cee3c547`，监听 `127.0.0.1:18080` |
| 镜像 | `ghcr.io/ranxi2001/sub2api:v2.7.7` **不存在**；compose 里的 `latest` 也未使用 |
| 数据 | 生产逻辑备份 170MB / 库约 2.5GB；本地 Postgres 16 + 空 Redis |
| 安全开关 | `token_refresh.enabled=false`；副本里关掉 channel monitor / backup cron / Codex ticket |
| 品牌 | 首页 title 是 `TOPAPI - AI API Gateway`，不是 TopOpenAI |

启动结论：**生产库副本可以直接被 ranxi 读起来**。`schema_migrations` 仍是 290 条，那 4 条当前 fork 独有 migration 还在，ranxi 没有追加新 SQL。

| 检查项 | 结果 |
|---|---|
| `GET /health` | 200 `{"status":"ok"}` |
| 管理员登录 | 200，JWT 可用 |
| 用户 / 余额 | profile 200；admin users `total=236`（全库 710，含软删） |
| API Key | `GET /api/v1/keys` 200，管理员本人 8 把；全库 831 / 未删 299 |
| 账号池 | 17 个未删账号：OpenAI 10（9 oauth + 1 apikey）、Anthropic 3、Grok 3、DeepSeek 1 |
| 分组 | 7 个；含 `Codex（Pro）`、`gpt-image-2`（`allow_image_generation=true`） |
| 订阅 | 接口 200，但 `user_subscriptions` 为 0（当前业务不靠这张表） |
| Codex harvest 页 | `GET /api/v1/admin/accounts/codex-harvest-flow` 200；sidecar 未配置 |
| 签到 | 已移植：`/api/v1/checkin/status` 200；admin stats `total_checkins=706` |
| image workbench | 已移植：`/api/v1/image-workbench/*` 200/202；`/keys` 隐藏 `__image_workbench__`；`poll_url` 走 JWT 路径 |
| desktop OAuth | 已移植：默认关 404 `desktop_auth_disabled`；打开后发码/换 JWT |
| 支付回调 | 本次未打真实入账，也未走 webhook |

注意：harvester 进程仍会启动，但 `openai_codex_ticket_enabled=false` 时探测循环会直接 return。验证结束后二进制已停；Postgres/Redis 容器仍留着方便复测。

## 移植优先级（仍以 ranxi 为新底座）

必须先移植，否则切流量会丢现网能力：

1. **Conversation image workbench + chatgpt2api 分组解析**（`gpt-image-2` 分组还在，但 workbench API/UI 没有）
2. **Desktop OAuth**（无 SQL，Redis 状态；代码整包搬）
3. **TopOpenAI / 现有品牌和 custom-release 工作流**（ranxi 更新源写死自己的仓库，且 GHCR 版本 tag 不可用）

可后置或放弃：

4. **每日签到**（表和 706 条记录还在，但 ranxi 不读；要留就得连 SQL 编号冲突一起处理）
5. **Kimi 设备码 OAuth**（账号池里这次没有 Kimi 账号）

ranxi 侧可在测试实例继续打开、不必立刻移植回当前 fork：

- Codex ticket / HarvestFlow
- Mihomo
- DeepSeek/Codex 兼容

补丁清单已拆出，见 [sub2api-ranxi-port-patches.md](./sub2api-ranxi-port-patches.md)。

P0–P3（Desktop OAuth、Image Workbench、TopOpenAI 品牌/custom-release、DailyCheckin）均已打进 `sub2api-ranxi` 并在 `.eval-ranxi` 验证通过。Kimi 设备码按约定不移植。仍不切流量。详见 [sub2api-ranxi-port-patches.md](./sub2api-ranxi-port-patches.md)。
