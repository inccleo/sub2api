# Tauri 桌面端授权登录

本功能为第一方 Sub2API Desktop 提供浏览器授权登录。用户在系统浏览器登录并明确授权，桌面端无需接触账号密码。已有邮箱密码、2FA、Passkey 和第三方登录都复用网页登录流程；浏览器已登录时直接展示授权确认页。

这是授权码 + S256 PKCE 的第一方登录接口，响应沿用 Sub2API 的 JSON 包装，不是面向任意第三方客户端的完整 OAuth/OIDC 服务。`account` 表示与网页登录相同的账号权限，管理员授权也包含管理员权限；不要将该客户端身份用于不受信任的第三方应用。公共客户端 ID 不等于应用身份认证。

## 启用

默认关闭。运行服务前在 `config.yaml` 设置：

```yaml
desktop_auth:
  enabled: true
```

也可设置 `DESKTOP_AUTH_ENABLED=true`。需要 Redis，无数据库迁移。关闭开关会停止授权和换码，已经发出的会话按原有刷新和撤销规则处理。

本次变更仅在本地实现和验证，未启用生产配置或部署。

## 固定客户端

| 参数 | 值 |
| --- | --- |
| `client_id` | `sub2api-desktop` |
| `redirect_uri` | `sub2api://oauth/callback`，严格完全匹配 |
| `response_type` | `code` |
| `scope` | `account` |
| `code_challenge_method` | `S256` |
| `state` | 每次随机生成，32–128 位 base64url 字符 |
| `code_verifier` | RFC 7636 unreserved 字符，43–128 位，建议 32 随机字节转无填充 base64url |
| 授权码有效期 | 确认授权后 60 秒，一次性使用 |

首版只注册上述 Tauri deep link，不接受任意 HTTPS 地址、动态回调或 loopback URI，也不使用 client secret。

## 请求流程

1. 桌面端为本次登录生成 `state`、`code_verifier`，只保存在内存；计算 `base64url(SHA256(code_verifier))` 作为 challenge。
2. 用系统浏览器打开站点 `/oauth/authorize`，查询参数包含上表的七个授权参数。不使用内嵌 WebView 登录。
3. 未登录时网页会转到 `/login?redirect=...`，完成登录后返回授权页。用户查看账号、权限并选择同意或拒绝。页面不会自动同意。
4. 网页通过 JWT 认证的 `GET /api/v1/auth/desktop/authorize` 验证参数，通过 `POST` 同一路径提交参数和 `decision: "approve" | "deny"`。桌面端不用直接调用这两个接口。
5. 同意返回 `sub2api://oauth/callback?code=...&state=...`；拒绝返回 `sub2api://oauth/callback?error=access_denied&state=...`。回调不包含 access/refresh token。浏览器无法自动唤起应用时，可点击“返回桌面应用”。
6. 桌面端严格校验 URI、唯一 `state` 与当前登录事务，再用原来的 verifier 换码：

```http
POST /api/v1/auth/desktop/token
Content-Type: application/json

{
  "grant_type": "authorization_code",
  "client_id": "sub2api-desktop",
  "redirect_uri": "sub2api://oauth/callback",
  "code": "<callback code>",
  "code_verifier": "<original verifier>"
}
```

也支持 `application/x-www-form-urlencoded`。成功响应：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "access_token": "...",
    "refresh_token": "...",
    "expires_in": 900,
    "token_type": "Bearer",
    "user": { "id": 1, "email": "user@example.com", "role": "user" }
  }
}
```

`expires_in` 取站点 JWT 配置，上例仅示意。错误沿用 Sub2API 的 `code/message/reason` 包装，例如 `invalid_grant`、`unsupported_grant_type`、`desktop_auth_disabled`。过期、重复换码、错误 verifier 或绑定不匹配都会失败。请求过频返回 429；Redis 不可用时拒绝授权/换码，不回退到无状态令牌。换码在签发前原子消费授权码；网络中断或签发失败时从桌面端重新发起登录，不反复重放旧 code。

7. 请求账号 API 时发送 `Authorization: Bearer <access_token>`。过期前调用已有 `POST /api/v1/auth/refresh`，body 为 `{"refresh_token":"..."}`；将返回的新 refresh token 原子替换旧值，串行刷新以避免重复使用旧令牌。退出调用 `POST /api/v1/auth/logout`，提交当前 refresh token 并清理本地凭证。已有“撤销所有会话”可撤销桌面刷新会话。

账号 JWT 用于 `/api/v1` 账号接口，不直接代替 `/v1/chat/completions` 等推理接口要求的 API Key；桌面应用可在获得账号授权后调用现有密钥管理接口。退出与撤销沿用当前会话体系的语义，不承诺已签发 access token 立即失效。

## Tauri v2 集成

可复制 [接入辅助模块](../examples/tauri/desktop-auth.ts)。模块涵盖随机 state/PKCE、事务超时、回调校验、拒绝授权、重复 deep-link 事件和换码；它通过注入方法适配 Tauri 插件，不依赖本仓库前端。

Tauri 端安装并注册 `opener`、`deep-link`、`http` 插件；使用 `single-instance` 的 deep-link 支持，将 Windows/Linux 的第二实例参数转发给已运行实例。`tauri.conf.json` 的插件配置：

```json
{
  "plugins": {
    "deep-link": { "desktop": { "schemes": ["sub2api"] } }
  }
}
```

Rust builder 中先注册 single-instance 插件，再注册 deep-link、opener、http 插件。Windows/Linux 开发时按 deep-link 插件要求调用 `register_all()`；macOS 要使用已安装、注册协议的 `.app` 测试，单纯 `tauri dev` 不保证协议可唤起。协议命名在前后端必须一致。

在应用初始化时先订阅回调，再启用登录按钮（以下 `saveRefreshToken` 为你自己的安全存储实现）：

```ts
import { openUrl } from '@tauri-apps/plugin-opener'
import { onOpenUrl, getCurrent } from '@tauri-apps/plugin-deep-link'
import { fetch as nativeFetch } from '@tauri-apps/plugin-http'
import { createDesktopLogin } from './desktop-auth'

const login = createDesktopLogin({
  serverURL: 'https://your-sub2api.example',
  openUrl,
  postJSON: async (url, body) => {
    const response = await nativeFetch(url, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body)
    })
    if (!response.ok) throw new Error('Sign-in failed; start again')
    return response.json()
  },
  saveRefreshToken // 使用系统凭证库或正确配置的 Stronghold；不要使用 localStorage/明文 Store。
})

async function accept(urls: string[]) {
  for (const url of urls) {
    try {
      const session = await login.handleCallback(url)
      if (session) {
        // 保存 access_token 到内存，更新账号 UI。
      }
    } catch {
      // 展示拒绝/过期/失败状态，允许重新发起。不要记录完整回调或令牌。
    }
  }
}

const unlisten = await onOpenUrl(accept)
await accept((await getCurrent()) ?? [])
// “使用浏览器登录”按钮调用 await login.start()；关闭窗口时调用 unlisten()。
```

示例把待完成事务保存在内存；应用在授权期间退出，冷启动回调无法恢复原 verifier，应提示重新发起登录。不要仅凭回调 code 登录，也不要把 verifier 放到浏览器 URL。授权码过期时间和本地登录事务超时是两个不同限制。

Tauri capabilities 仅允许主窗口使用所需插件，HTTP 与 opener 的 URL 权限限制到自己的站点；`postJSON` 必须使用原生 HTTP，避免为 WebView 添加全局宽泛 CORS。刷新、账号请求保持一致的原生 HTTP User-Agent，兼容站点会话指纹绑定。refresh token 使用 OS Keychain/Credential Manager/Secret Service 或正确配置的 Stronghold 加密保存。

本仓库不包含完整桌面应用；协议唤起、系统凭证存储和打包安装需要在你的 Tauri 项目中接入并验证。
